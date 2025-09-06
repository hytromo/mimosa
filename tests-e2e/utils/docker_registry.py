import json
import logging
import subprocess
import time
import uuid
from typing import Any, Dict, List
from urllib.request import urlopen

import docker
from docker.errors import DockerException

logger = logging.getLogger(__name__)


class DockerRegistryManager:
  """Manages a local Docker registry for testing."""

  def __init__(self):
    self.registry_image = "registry:3"
    random_suffix = uuid.uuid4().hex[:8]
    self.registry_name = f"mimosa-test-registry-{random_suffix}"
    self.registry_url = "localhost:5000"
    self.builder_name = f"mimosa-test-builder-{random_suffix}"
    self.client = None
    self.container: docker = None
    self.test_images: List[str] = []

  def start_registry(self) -> str:
    """Start the Docker registry and return the registry URL."""
    try:
      self.client = docker.from_env()
    except DockerException as e:
      raise RuntimeError(f"Failed to connect to Docker: {e}")

    # Stop any existing registry with the same name
    try:
      existing = self.client.containers.get(self.registry_name)
      existing.remove(force=True)
    except docker.errors.NotFound:
      pass

    # start builder with multi-platform support
    create_cmd = subprocess.run(
      [
        "docker",
        "buildx",
        "create",
        "--name",
        self.builder_name,
        "--driver",
        "docker-container",
        "--driver-opt",
        "network=host",
        "--use",
      ],
      capture_output=True,
      text=True,
    )
    if create_cmd.returncode != 0:
      raise RuntimeError(f"Failed to create builder: {create_cmd.stderr}")

    # Start new registry
    try:
      self.container = self.client.containers.run(
        self.registry_image,
        name=self.registry_name,
        network_mode="host",
        detach=True,
        remove=False,
      )

      # Wait for registry to be ready
      self._wait_for_registry()

      return self.registry_url

    except Exception as e:
      raise RuntimeError(f"Failed to start registry: {e}")

  def _wait_for_registry(self, timeout: int = 30):
    """Wait for the registry to be ready."""
    start_time = time.time()
    while time.time() - start_time < timeout:
      try:
        # Try to connect to the registry
        result = subprocess.run(
          ["curl", "-f", f"http://{self.registry_url}/v2/"],
          capture_output=True,
          timeout=5,
        )
        if result.returncode == 0:
          return
      except (subprocess.TimeoutExpired, FileNotFoundError):
        pass

      time.sleep(1)

    raise RuntimeError(f"Registry not ready after {timeout} seconds")

  def cleanup(self):
    if self.builder_name:
      try:
        subprocess.run(["docker", "buildx", "rm", self.builder_name], capture_output=True, text=True)
      except Exception as e:
        logger.warning(f"Failed to remove builder: {e}")
      finally:
        self.builder_name = None
    if self.container:
      try:
        self.container.kill()
        self.container.remove()
      except Exception as e:
        logger.warning(f"Failed to stop registry: {e}")
      finally:
        self.container = None
    for tag in self.test_images:
      try:
        subprocess.run(["docker", "rmi", "-f", tag], capture_output=True)
      except Exception:
        pass

  def generate_unique_tag(self, base_name: str = "test-image") -> str:
    """Generate a unique tag for testing."""
    unique_id = uuid.uuid4().hex[:8]
    tag = f"{self.registry_url}/{base_name}:{unique_id}"
    self.test_images.append(tag)
    return tag

  def get_manifest_digests(self, fullImageName: str) -> List[str]:
    imageName = fullImageName.split(":")[0]
    refName = fullImageName.split(":")[1]

    with urlopen(f"http://{self.registry_url}/v2/{imageName}/manifests/{refName}") as response:
      manifests = json.loads(response.read().decode("utf-8")).get("manifests", [])

    return [manifest.get("digest", "") for manifest in manifests if manifest.get("digest")]

  def inspect_image(self, image_tag: str) -> Dict[str, Any]:
    """Inspect an image and return its details."""
    try:
      result = subprocess.run(
        ["docker", "inspect", image_tag],
        capture_output=True,
        text=True,
        check=True,
      )

      data = json.loads(result.stdout)
      return data[0] if data else {}

    except (subprocess.CalledProcessError, json.JSONDecodeError):
      return {}
