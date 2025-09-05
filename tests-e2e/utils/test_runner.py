import json
import os
import subprocess
from typing import Any, Dict, List, Optional

from pydantic import BaseModel

from utils.docker_registry import DockerRegistryManager
from utils.scaffolder import create_scaffolder


class CommandResult(BaseModel):
  cache_hit: bool
  cache_miss: bool
  output: str


class TestCaseResult(BaseModel):
  test_name: str
  error: Optional[str] = None
  extra_debug_info: Optional[str] = None


class MimosaTestRunner:
  """Runs mimosa tests with cache hit/miss verification."""

  def __init__(self, registry_manager: DockerRegistryManager, mimosa_path: str = None):
    self.registry_manager: DockerRegistryManager = registry_manager
    self.test_images = []

  def run_mimosa_remember(self, docker_command: List[str], cwd: str) -> CommandResult:
    """Run mimosa remember command and parse output."""

    cmd = ["mimosa", "remember", "--"] + docker_command
    actual_cwd = cwd

    print(f"{cwd}: {' '.join(cmd)}")
    result = subprocess.run(
      cmd, cwd=actual_cwd, capture_output=True, text=True, check=True
    )

    output = result.stdout + result.stderr

    return CommandResult(
      cache_hit="mimosa-cache-hit: true" in output,
      cache_miss="mimosa-cache-hit: false" in output,
      output=output,
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
    platform_str = platforms[0] if platforms else "linux/amd64"

    # Build the command
    cmd = [
      "docker",
      "buildx",
      "build",
      "--platform",
      platform_str,
      "--push",
      "-t",
      image_tag,
    ]

    # Check if Dockerfile is in a custom location
    dockerfile_path = self._find_dockerfile_path(test_case, cwd, context)
    if dockerfile_path and dockerfile_path != "Dockerfile":
      # Make sure the path is relative to the context
      if os.path.isabs(dockerfile_path):
        dockerfile_path = os.path.relpath(dockerfile_path, context)
      cmd.extend(["-f", dockerfile_path])

    cmd.append(context)
    return cmd

  def _find_dockerfile_path(
    self, test_case: Dict[str, Any], cwd: str, context: str
  ) -> str:
    """Find the Dockerfile path in the test case structure."""
    # Check if there's a custom dockerfile specified
    extra_files = test_case.get("extra_files", {})

    # Look for Dockerfile in the file structure
    for file_path, file_config in extra_files.items():
      if file_path == "Dockerfile":
        return "Dockerfile"
      elif isinstance(file_config, dict) and "files" in file_config:
        # Check subdirectories
        for sub_file, sub_config in file_config["files"].items():
          if sub_file == "Dockerfile":
            # Resolve the full path using the same logic as scaffolder
            full_path = os.path.join(file_path, sub_file)
            # Resolve variables in the path
            resolved_path = self._resolve_path_variables(full_path, cwd, context)
            # Make it relative to context
            if os.path.isabs(resolved_path):
              return os.path.relpath(resolved_path, context)
            else:
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
    self, test_name: str, test_case: Dict[str, Any], defaults: Dict[str, Any]
  ) -> TestCaseResult:
    """Run a single test case and return results."""
    # Create test environment
    scaffolder = create_scaffolder(
      test_name, {"defaults": defaults, "tests": {test_name: test_case}}
    )
    temp_dir, cwd, context = scaffolder.create_test_environment()
    print("temp_dir", temp_dir)

    try:
      # Generate unique image tag
      image_tag = self.registry_manager.generate_unique_tag(f"test-{test_name}")
      print("image_tag", image_tag)
      self.test_images.append(image_tag)

      # Build docker command
      docker_cmd = self.build_docker_command(
        test_case, defaults, image_tag, cwd, context
      )

      # Run mimosa remember for the first time (should be cache miss)
      result1 = self.run_mimosa_remember(docker_cmd, cwd)

      if not result1.cache_miss:
        return TestCaseResult(
          test_name=test_name,
          extra_debug_info=f"First run: {result1}",
          error="Expected cache miss on first run",
        )

      # Get rewrite files and modify them
      rewrite_files = scaffolder.get_rewrite_files()
      cache_results = {}
      print("rewrite_files", rewrite_files)

      for rewrite_file in rewrite_files:
        file_path = rewrite_file["path"]
        expected_cache_hit = rewrite_file["expected_cache_hit"]

        # Modify the file content
        with open(file_path, "a") as f:
          f.write("\n# Modified for cache test\n")

        # Run mimosa remember again
        result2 = self.run_mimosa_remember(docker_cmd, cwd)

        # Check cache behavior
        actual_cache_hit = result2.cache_hit
        cache_results[file_path] = {
          "success": actual_cache_hit == expected_cache_hit,
          "expected_cache_hit": expected_cache_hit,
          "actual_cache_hit": actual_cache_hit,
          "output": result2.output,
        }

      # Verify image contents match expectations
      self._assert_image_contents(image_tag, test_case)

      # Check if all cache results are successful
      cache_success = (
        all(r.get("success", False) for r in cache_results.values())
        if cache_results
        else True
      )

      # Overall success requires both cache and image verification to pass
      overall_success = cache_success

      return TestCaseResult(
        test_name=test_name,
        error=None if overall_success else "Overall test failed",
        extra_debug_info=f"Cache results: {cache_results}\nFirst run: {result1}\nCache success: {cache_success}",
      )

    finally:
      scaffolder.cleanup()

  def _assert_image_contents(self, image_tag: str, test_case: Dict[str, Any]):
    """Verify that the image contains the expected files using directory_tree.py compare."""
    print("Verifying image contents for", image_tag)
    # Get expected files from test case
    expected_files = test_case.get("expectations", {}).get("output", [])
    expected_files = [f.strip() for f in expected_files if f.strip()]

    # Create expected JSON
    expected_json = json.dumps(expected_files)

    # Run the image with directory_tree.py compare
    cmd = ["docker", "run", "--rm", "-i", image_tag]

    result = subprocess.run(
      cmd, input=expected_json, text=True, capture_output=True, timeout=30
    )

    # directory_tree.py compare returns exit code 0 if files match, 1 if they don't
    success = result.returncode == 0

    if success:
      return

    # Get actual files for comparison
    cmd = [
      "docker",
      "run",
      "--rm",
      "-i",
      image_tag,
      "python",
      "/app/directory_tree.py",
      "get",
      "/app/",
    ]
    result = subprocess.run(cmd, text=True, capture_output=True, timeout=30)
    actual_files = json.loads(result.stdout)
    raise Exception(
      f"Expected {set(expected_files)} but got {set(actual_files)} with error {result.stderr}"
    )

  def cleanup(self):
    self.registry_manager.cleanup_images(self.test_images)
