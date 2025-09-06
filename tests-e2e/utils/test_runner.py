import json
import logging
import os
import random
import shutil
import subprocess
import tempfile
from typing import Any, Dict, List, Optional

from pydantic import BaseModel

from utils.docker_registry import DockerRegistryManager
from utils.scaffolder import create_scaffolder

logger = logging.getLogger(__name__)


class CommandResult(BaseModel):
  cache_hit: bool
  cache_miss: bool
  output: str


class TestCaseResult(BaseModel):
  test_name: str
  error: Optional[str] = None
  extra_debug_info: Optional[str] = None


class MimosaTestRunner:
  def __init__(self, registry_manager: DockerRegistryManager, mimosa_path: str = None):
    self.registry_manager: DockerRegistryManager = registry_manager
    self.temp_folders: List[str] = []

  def run_mimosa_remember(self, docker_command: List[str], cwd: str) -> CommandResult:
    cmd = ["mimosa", "remember", "--"] + docker_command
    actual_cwd = cwd

    logger.debug(f"Running mimosa remember: {cwd}: {' '.join(cmd)}")
    result = subprocess.run(
      cmd,
      cwd=actual_cwd,
      text=True,
      stdout=subprocess.PIPE,
      stderr=subprocess.STDOUT,  # the whole output is piped to stdout
    )

    if result.returncode != 0:
      raise Exception(f"Failed to run mimosa remember: {result.stdout}")

    if logger.isEnabledFor(logging.DEBUG):
      temp_folder = tempfile.mkdtemp(prefix="mimosa_test_")
      temp_file = os.path.join(temp_folder, "mimosa_output.txt")
      os.makedirs(temp_folder, exist_ok=True)
      self.temp_folders.append(temp_folder)
      with open(temp_file, "w") as f:
        f.write(result.stdout)
        logger.debug(f"Wrote mimosa output to {temp_file}")

    return CommandResult(
      cache_hit="mimosa-cache-hit: true" in result.stdout,
      cache_miss="mimosa-cache-hit: false" in result.stdout,
      output=result.stdout,
    )

  def build_docker_command(
    self,
    test_case: Dict[str, Any],
    defaults: Dict[str, Any],
    image_tag: str,
    cwd: str,
    context: str,
  ) -> List[str]:
    """Build the docker buildx build command for the test case."""
    # Get platforms from test case or defaults
    platforms = test_case.get("platforms", defaults.get("platforms", ["linux/amd64"]))
    # Use single platform for testing to avoid multi-platform build issues
    platform_str = ",".join(platforms)

    # Build the command
    cmd = ["docker", "buildx", "build", "--platform", platform_str, "--builder", self.registry_manager.builder_name, "--push", "-t", image_tag]

    # Check if Dockerfile is in a custom location
    dockerfile_path = self._find_dockerfile_path(test_case, cwd, context)
    if dockerfile_path and dockerfile_path != "Dockerfile":
      # Make sure the path is relative to the context
      if os.path.isabs(dockerfile_path):
        dockerfile_path = os.path.relpath(dockerfile_path, context)
      cmd.extend(["-f", dockerfile_path])

    cmd.append(context)
    return cmd

  def _find_dockerfile_path(self, test_case: Dict[str, Any], cwd: str, context: str) -> str:
    extra_files = test_case.get("extra_files", {})

    for file_path, file_config in extra_files.items():
      if file_path.startswith("Dockerfile") and not file_path.endswith(".dockerignore"):
        return file_path
      elif isinstance(file_config, dict) and "files" in file_config:
        # Check subdirectories
        for sub_file, sub_config in file_config["files"].items():
          if sub_file.startswith("Dockerfile") and not sub_file.endswith(".dockerignore"):
            # Resolve the full path using the same logic as scaffolder
            full_path = os.path.join(file_path, sub_file)
            # Resolve variables in the path
            resolved_path = self._resolve_path_variables(full_path, cwd, context)
            # Make it relative to context
            if os.path.isabs(resolved_path):
              print(f"Returning relative path {resolved_path} ctx:{context} -> {os.path.relpath(resolved_path, cwd)}")
              return os.path.relpath(resolved_path, cwd)
            else:
              print(f"Returning resolved path {resolved_path}")
              return resolved_path

    return "Dockerfile"  # Default

  def _resolve_path_variables(self, path: str, cwd: str, context: str) -> str:
    """Resolve variables in path strings (similar to scaffolder logic)."""
    # Get temp directory from cwd (assuming cwd is under temp)
    temp_dir = os.path.dirname(cwd) if cwd != context else cwd
    replacements = {
      "$tmp": temp_dir,
      "$cwd": cwd,
    }
    for replacement_key in replacements.keys():
      if replacement_key in path:
        path = path.replace(replacement_key, replacements[replacement_key])
    return path

  def run_test_case(
    self,
    test_name: str,
    test_case: Dict[str, Any],
    defaults: Dict[str, Any],
  ) -> TestCaseResult:
    """Run a single test case and return results."""
    # Create test environment
    scaffolder = create_scaffolder(test_name, {"defaults": defaults, "tests": {test_name: test_case}})
    temp_dir, cwd, context = scaffolder.create_test_environment()

    logger.debug(f"Temp directory for test {test_name}: {temp_dir}")

    try:
      first_tag = self.registry_manager.generate_unique_tag(f"test-{test_name}")
      docker_cmd = self.build_docker_command(test_case, defaults, first_tag, cwd, context)

      first_run = self.run_mimosa_remember(docker_cmd, cwd)

      if not first_run.cache_miss:
        return TestCaseResult(
          test_name=test_name,
          extra_debug_info=f"First run: {first_run}",
          error="Expected cache miss on first run",
        )

      self._assert_image_contents(first_tag, test_case)

      # Get rewrite files and modify them
      rewrite_files = scaffolder.get_rewrite_files()
      last_tag_without_cache_hit = first_tag

      for rewrite_file in rewrite_files:
        with open(rewrite_file["path"], "a") as f:
          f.write(f"\n# Modified for cache test {random.randint(1, 1000000)}\n")

        nth_tag = self.registry_manager.generate_unique_tag(f"test-{test_name}")
        docker_cmd = self.build_docker_command(test_case, defaults, nth_tag, cwd, context)
        nth_run = self.run_mimosa_remember(docker_cmd, cwd)

        actual_cache_hit = nth_run.cache_hit
        expected_cache_hit = rewrite_file["expected_cache_hit"]
        assert actual_cache_hit == expected_cache_hit, f"Cache hit error: expected {expected_cache_hit} but got {actual_cache_hit}"

        if not actual_cache_hit:
          last_tag_without_cache_hit = nth_tag
        else:
          logger.debug(f"Asserting that cache hit by rewrote file {rewrite_file['path']} did not produce new manifests...")
          # this image needs to have the same manifests as the last tag without a cache hit - because it will be rettaged from
          # that as source tag
          self._assert_image_manifests(last_tag_without_cache_hit, nth_tag)

      return TestCaseResult(
        test_name=test_name,
        error=None,
      )

    finally:
      scaffolder.cleanup()

  def _assert_image_manifests(self, first_tag: str, nth_tag: str):
    logger.debug(f"Asserting image manifests for {first_tag} and {nth_tag}...")
    first_manifests = self.registry_manager.get_manifest_digests(first_tag.split("/")[-1])
    nth_manifests = self.registry_manager.get_manifest_digests(nth_tag.split("/")[-1])

    assert len(first_manifests) >= 2, f"Expected at least 2 manifests for {first_tag}"
    assert len(nth_manifests) >= 2, f"Expected at least 2 manifests for {nth_tag}"

    assert set(first_manifests) == set(nth_manifests), f"Expected {first_manifests} but got {nth_manifests}"

    logger.debug(f"Same manifests found: {first_manifests} == {nth_manifests}")

  def _assert_image_contents(self, image_tag: str, test_case: Dict[str, Any]):
    logger.debug(f"Asserting image contents for {image_tag}...")
    expected_files = test_case.get("expectations", {}).get("output", [])
    expected_files = [f.strip() for f in expected_files if f.strip()]

    expected_json = json.dumps(expected_files)

    result = subprocess.run(["docker", "run", "--rm", "-i", image_tag], input=expected_json, text=True, capture_output=True, timeout=30)

    if result.returncode == 0:
      return

    result = subprocess.run(
      ["docker", "run", "--rm", "-i", image_tag, "python", "/app/directory_tree.py", "get", "/app/"], text=True, capture_output=True, timeout=30
    )
    try:
      actual_files = json.loads(result.stdout)
    except json.JSONDecodeError:
      raise Exception(f"Could not get actual files from image {image_tag}: {result.stderr}")

    raise Exception(f"Expected {set(expected_files)} but got {set(actual_files)} with error {result.stderr}")

  def cleanup(self):
    for temp_folder in self.temp_folders:
      if os.path.exists(temp_folder):
        shutil.rmtree(temp_folder)
