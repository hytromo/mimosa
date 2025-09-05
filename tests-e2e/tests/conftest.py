import pytest
import yaml

from utils.docker_registry import DockerRegistryManager
from utils.test_runner import MimosaTestRunner


@pytest.fixture(scope="session")
def docker_registry_manager():
  docker_registry_manager = DockerRegistryManager()
  docker_registry_manager.start_registry()
  yield docker_registry_manager
  docker_registry_manager.stop_registry()


@pytest.fixture(scope="session")
def test_runner(docker_registry_manager):
  yield MimosaTestRunner(docker_registry_manager)


@pytest.fixture(scope="session")
def test_defaults():
  return load_test_cases("utils/test-cases/docker-buildx-build.yaml")["defaults"]


def pytest_generate_tests(metafunc):
  if "buildx_build_test_case" in metafunc.fixturenames:
    # dynamically generate the test cases from the test-cases/docker-buildx-build.yaml file
    test_cases = load_test_cases("utils/test-cases/docker-buildx-build.yaml")["tests"]
    all_ids = []
    test_case_values = []
    for test_name, test_case in test_cases.items():
      all_ids.append(test_name)
      test_case_values.append(test_case)
    metafunc.parametrize("buildx_build_test_case", test_case_values, ids=all_ids)


def load_test_cases(file_path):
  with open(file_path, "r") as file:
    return yaml.safe_load(file)
