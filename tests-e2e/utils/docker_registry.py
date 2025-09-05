import argparse
import json
import subprocess
import sys
import time
import uuid
from typing import Any, Dict, List

import docker
from docker.errors import DockerException


class DockerRegistryManager:
  """Manages a local Docker registry for testing."""

  def __init__(self, registry_image: str = "registry:3", port: int = 5000):
    self.registry_image = registry_image
    self.port = port
    self.registry_name = f"mimosa-test-registry-{uuid.uuid4().hex[:8]}"
    self.registry_url = f"localhost:{self.port}"
    self.client = None
    self.container: docker = None

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

    # Start new registry
    try:
      self.container = self.client.containers.run(
        self.registry_image,
        name=self.registry_name,
        ports={5000: self.port},
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

  def stop_registry(self):
    """Stop and remove the Docker registry."""
    if self.container:
      try:
        self.container.kill()
        self.container.remove()
      except Exception as e:
        print(f"Warning: Failed to stop registry: {e}")
      finally:
        self.container = None

  def generate_unique_tag(self, base_name: str = "test-image") -> str:
    """Generate a unique tag for testing."""
    unique_id = uuid.uuid4().hex[:8]
    return f"{self.registry_url}/{base_name}:{unique_id}"

  def push_image(self, image_tag: str) -> bool:
    """Push an image to the registry."""
    try:
      # Tag the image for the registry
      registry_tag = f"{self.registry_url}/{image_tag.split('/')[-1]}"
      subprocess.run(
        ["docker", "tag", image_tag, registry_tag],
        check=True,
        capture_output=True,
      )

      # Push to registry
      subprocess.run(
        ["docker", "push", registry_tag],
        check=True,
        capture_output=True,
        text=True,
      )

      return True

    except subprocess.CalledProcessError as e:
      print(f"Failed to push image {image_tag}: {e.stderr}")
      return False

  def list_manifests(self, repository: str) -> List[str]:
    """List all manifests in a repository."""
    try:
      result = subprocess.run(
        ["curl", "-s", f"http://{self.registry_url}/v2/{repository}/tags/list"],
        capture_output=True,
        text=True,
        check=True,
      )

      data = json.loads(result.stdout)
      return data.get("tags", [])

    except (subprocess.CalledProcessError, json.JSONDecodeError):
      return []

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

  def cleanup_images(self, image_tags: List[str]):
    """Clean up test images."""
    for tag in image_tags:
      try:
        subprocess.run(["docker", "rmi", "-f", tag], capture_output=True)
      except Exception:
        pass  # Ignore cleanup errors


class RegistryContext:
  """Context manager for Docker registry."""

  def __init__(self, registry_image: str = "registry:3", port: int = 5000):
    self.manager = DockerRegistryManager(registry_image, port)
    self.registry_url = None

  def __enter__(self) -> DockerRegistryManager:
    self.registry_url = self.manager.start_registry()
    return self.manager

  def __exit__(self, exc_type, exc_val, exc_tb):
    self.manager.stop_registry()


def main():
  """Main function for command-line usage."""
  parser = argparse.ArgumentParser(
    description="Docker registry management for e2e tests"
  )
  parser.add_argument(
    "--registry-image",
    default="registry:3",
    help="Docker registry image to use (default: registry:3)",
  )
  parser.add_argument(
    "--port", type=int, default=5000, help="Port for the registry (default: 5000)"
  )

  subparsers = parser.add_subparsers(dest="command", help="Available commands")

  # start-registry subcommand
  start_parser = subparsers.add_parser("start-registry", help="Start a Docker registry")
  start_parser.add_argument(
    "--timeout",
    type=int,
    default=30,
    help="Timeout in seconds to wait for registry to be ready (default: 30)",
  )

  # stop-registry subcommand
  stop_parser = subparsers.add_parser("stop-registry", help="Stop a Docker registry")
  stop_parser.add_argument(
    "--registry-name", help="Name of the registry container to stop (optional)"
  )

  # generate-tag subcommand
  tag_parser = subparsers.add_parser(
    "generate-tag", help="Generate a unique tag for testing"
  )
  tag_parser.add_argument(
    "--base-name",
    default="test-image",
    help="Base name for the tag (default: test-image)",
  )

  # push-image subcommand
  push_parser = subparsers.add_parser(
    "push-image", help="Push an image to the registry"
  )
  push_parser.add_argument("image_tag", help="Image tag to push")

  # list-manifests subcommand
  list_parser = subparsers.add_parser(
    "list-manifests", help="List all manifests in a repository"
  )
  list_parser.add_argument("repository", help="Repository name to list manifests for")

  # inspect-image subcommand
  inspect_parser = subparsers.add_parser(
    "inspect-image", help="Inspect an image and return its details"
  )
  inspect_parser.add_argument("image_tag", help="Image tag to inspect")

  # run-image subcommand
  run_parser = subparsers.add_parser(
    "run-image", help="Run an image and capture its output"
  )
  run_parser.add_argument("image_tag", help="Image tag to run")

  # cleanup-images subcommand
  cleanup_parser = subparsers.add_parser("cleanup-images", help="Clean up test images")
  cleanup_parser.add_argument("image_tags", nargs="+", help="Image tags to clean up")

  args = parser.parse_args()

  if not args.command:
    parser.print_help()
    sys.exit(1)

  try:
    if args.command == "start-registry":
      manager = DockerRegistryManager(args.registry_image, args.port)
      registry_url = manager.start_registry()
      print(f"Registry started at: {registry_url}")
      print(f"Registry name: {manager.registry_name}")

    elif args.command == "stop-registry":
      if args.registry_name:
        # Stop specific registry by name
        try:
          client = docker.from_env()
          container = client.containers.get(args.registry_name)
          container.stop()
          container.remove()
          print(f"Registry '{args.registry_name}' stopped and removed")
        except docker.errors.NotFound:
          print(f"Registry '{args.registry_name}' not found")
        except Exception as e:
          print(f"Error stopping registry: {e}")
      else:
        # Stop all mimosa test registries
        try:
          client = docker.from_env()
          containers = client.containers.list(
            all=True, filters={"name": "mimosa-test-registry-"}
          )
          for container in containers:
            container.stop()
            container.remove()
            print(f"Registry '{container.name}' stopped and removed")
          if not containers:
            print("No mimosa test registries found")
        except Exception as e:
          print(f"Error stopping registries: {e}")

    elif args.command == "generate-tag":
      manager = DockerRegistryManager(args.registry_image, args.port)
      tag = manager.generate_unique_tag(args.base_name)
      print(tag)

    elif args.command == "push-image":
      manager = DockerRegistryManager(args.registry_image, args.port)
      success = manager.push_image(args.image_tag)
      if success:
        print(f"Successfully pushed {args.image_tag}")
      else:
        print(f"Failed to push {args.image_tag}")
        sys.exit(1)

    elif args.command == "list-manifests":
      manager = DockerRegistryManager(args.registry_image, args.port)
      manifests = manager.list_manifests(args.repository)
      if manifests:
        for manifest in manifests:
          print(manifest)
      else:
        print("No manifests found")

    elif args.command == "inspect-image":
      manager = DockerRegistryManager(args.registry_image, args.port)
      details = manager.inspect_image(args.image_tag)
      if details:
        print(json.dumps(details, indent=2))
      else:
        print("Failed to inspect image")
        sys.exit(1)

    elif args.command == "cleanup-images":
      manager = DockerRegistryManager(args.registry_image, args.port)
      manager.cleanup_images(args.image_tags)
      print(f"Cleaned up {len(args.image_tags)} images")

  except Exception as e:
    print(f"Error: {e}")
    sys.exit(1)


if __name__ == "__main__":
  main()
