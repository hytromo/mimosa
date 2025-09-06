import argparse
import os
import shutil
import sys
import tempfile
from typing import Any, Dict, List

import yaml


class TestScaffolder:
  """Creates temporary directory structures for test cases."""

  def __init__(self, test_case: Dict[str, Any], defaults: Dict[str, Any]):
    self.test_case = test_case
    self.defaults = defaults
    self.temp_dir = None
    self.cwd = None
    self.context = None

  def create_test_environment(self) -> tuple[str, str, str]:
    """
    Create the test environment and return (temp_dir, cwd, context).
    """
    # Create temporary directory
    self.temp_dir = tempfile.mkdtemp(prefix="mimosa_test_")

    # Resolve variables
    self.cwd = self._resolve_path(self.test_case.get("cwd", self.defaults.get("cwd", "$tmp")))
    self.context = self._resolve_path(self.test_case.get("context", self.defaults.get("context", "$cwd")))

    os.makedirs(self.cwd, exist_ok=True)
    os.makedirs(self.context, exist_ok=True)

    # Create directory structure
    self._create_files()

    return self.temp_dir, self.cwd, self.context

  def _resolve_path(self, path: str) -> str:
    """Resolve variables in path strings."""
    if path == "$tmp":
      return self.temp_dir
    elif path == "$cwd":
      return self.cwd if self.cwd else self.temp_dir
    elif path.startswith("$tmp/"):
      return path.replace("$tmp", self.temp_dir)
    elif path.startswith("$cwd/"):
      return path.replace("$cwd", self.cwd if self.cwd else self.temp_dir)
    else:
      # If it's a relative path, make it relative to cwd
      if not os.path.isabs(path):
        return os.path.join(self.cwd, path)
      return path

  def _create_files(self):
    """Create all files and directories specified in the test case."""
    extra_files = self.test_case.get("extra_files", {})

    # Always include directory_tree.py if specified in defaults
    if self.defaults.get("include_directory_tree_script", False):
      self._create_directory_tree_script()

    # Create all extra files
    for file_path, file_config in extra_files.items():
      self._create_file_or_directory(file_path, file_config)

  def _create_directory_tree_script(self):
    """Copy the existing directory_tree.py script to the test environment."""
    # Get the path to the existing directory_tree.py script
    current_dir = os.path.dirname(os.path.abspath(__file__))
    source_script = os.path.join(current_dir, "directory_tree.py")

    # Copy it to the test environment
    script_path = os.path.join(self.context, "directory_tree.py")
    os.makedirs(os.path.dirname(script_path), exist_ok=True)
    shutil.copy2(source_script, script_path)
    os.chmod(script_path, 0o755)

  def _create_file_or_directory(self, file_path: str, config: Any):
    """Create a file or directory based on the configuration."""
    resolved_path = self._resolve_path(file_path)

    if isinstance(config, dict) and "files" in config:
      # This is a directory with files
      os.makedirs(resolved_path, exist_ok=True)
      for sub_file, sub_config in config["files"].items():
        sub_path = os.path.join(resolved_path, sub_file)
        self._create_file_or_directory(sub_path, sub_config)
    elif isinstance(config, dict) and "content" in config:
      # This is a file with content
      parent_dir = os.path.dirname(resolved_path)
      if parent_dir:  # Only create parent dir if it's not empty
        os.makedirs(parent_dir, exist_ok=True)
      with open(resolved_path, "w") as f:
        f.write(config["content"])
    else:
      # This is a simple file with string content
      parent_dir = os.path.dirname(resolved_path)
      if parent_dir:  # Only create parent dir if it's not empty
        os.makedirs(parent_dir, exist_ok=True)
      with open(resolved_path, "w") as f:
        f.write(str(config))

  def get_rewrite_files(self) -> List[Dict[str, Any]]:
    """Get all files that need to be rewritten for cache testing."""
    rewrite_files = []
    self._collect_rewrite_files(self.test_case.get("extra_files", {}), rewrite_files)
    return rewrite_files

  def _collect_rewrite_files(
    self,
    files_config: Dict[str, Any],
    rewrite_files: List[Dict[str, Any]],
    base_path: str = "",
  ):
    """Recursively collect files that have rewrite_content configuration."""
    for file_path, file_config in files_config.items():
      full_path = os.path.join(base_path, file_path) if base_path else file_path

      if isinstance(file_config, dict):
        if "rewrite_content" in file_config:
          rewrite_config = file_config["rewrite_content"]
          resolved_path = self._resolve_path(full_path)
          rewrite_files.append(
            {
              "path": resolved_path,
              "expected_cache_hit": rewrite_config.get("cache_hit_expected", False),
            }
          )
        elif "files" in file_config:
          self._collect_rewrite_files(file_config["files"], rewrite_files, full_path)

  def cleanup(self):
    if self.temp_dir and os.path.exists(self.temp_dir):
      shutil.rmtree(self.temp_dir)


def load_test_cases(yaml_file: str) -> Dict[str, Any]:
  """Load test cases from YAML file."""
  with open(yaml_file, "r") as f:
    data = yaml.safe_load(f)

  # Resolve YAML anchors
  if "default_dockerfile_content" in data:
    default_content = data["default_dockerfile_content"]
    # Replace anchor references in test cases
    for test_name, test_case in data.get("tests", {}).items():
      if "extra_files" in test_case:
        for file_name, file_config in test_case["extra_files"].items():
          if isinstance(file_config, dict) and file_config.get("content") == "*default_dockerfile_content":
            file_config["content"] = default_content

  return data


def create_scaffolder(test_name: str, test_cases_data: Dict[str, Any]) -> TestScaffolder:
  """Create a scaffolder for a specific test case."""
  defaults = test_cases_data.get("defaults", {})
  test_case = test_cases_data["tests"][test_name]
  return TestScaffolder(test_case, defaults)


def main():
  """Main function for command-line usage."""
  parser = argparse.ArgumentParser(description="Create test directory structures from YAML test cases")
  parser.add_argument("yaml_file", help="Path to the YAML file containing test cases")
  parser.add_argument("test_case_key", help="Key of the test case to create")
  parser.add_argument(
    "--cleanup",
    action="store_true",
    help="Clean up the temporary directory after creation (for testing)",
  )

  args = parser.parse_args()

  try:
    # Load test cases from YAML file
    test_cases_data = load_test_cases(args.yaml_file)

    # Check if the test case exists
    if args.test_case_key not in test_cases_data.get("tests", {}):
      available_tests = list(test_cases_data.get("tests", {}).keys())
      print(f"Error: Test case '{args.test_case_key}' not found in {args.yaml_file}")
      print(f"Available test cases: {', '.join(available_tests)}")
      sys.exit(1)

    # Create scaffolder and generate test environment
    scaffolder = create_scaffolder(args.test_case_key, test_cases_data)
    temp_dir, cwd, context = scaffolder.create_test_environment()

    # Output the temporary directory path
    print(temp_dir)

    # If cleanup is requested, clean up after a short delay
    if args.cleanup:
      import time

      time.sleep(1)  # Give a moment to see the output
      scaffolder.cleanup()

  except FileNotFoundError:
    print(f"Error: YAML file '{args.yaml_file}' not found")
    sys.exit(1)
  except yaml.YAMLError as e:
    print(f"Error parsing YAML file: {e}")
    sys.exit(1)
  except Exception as e:
    print(f"Error: {e}")
    sys.exit(1)


if __name__ == "__main__":
  main()
