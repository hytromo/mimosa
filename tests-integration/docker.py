#!/usr/bin/env python3
import subprocess
import time
from typing import List, Optional, Tuple
from urllib.parse import urljoin

import httpx
import requests
from pydantic import BaseModel, Field


class Platform(BaseModel):
    os: str = Field(..., description="Operating system (e.g., 'linux', 'windows')")
    architecture: str = Field(..., description="Architecture (e.g., 'amd64', 'arm64')")
    variant: Optional[str] = Field(
        None, description="Optional variant (e.g., 'v7' for ARM v7)"
    )


class PlatformManifest(BaseModel):
    platform: Platform = Field(..., description="Platform specification")
    digest: str = Field(..., description="Digest of the platform-specific manifest")
    size: int = Field(..., description="Size of the manifest in bytes")
    mediaType: str = Field(..., description="Media type of the manifest")


class ManifestResponse(BaseModel):
    digest: str = Field(..., description="Digest from Docker-Content-Digest header")
    schemaVersion: int = Field(..., description="Schema version of the manifest")
    mediaType: str = Field(..., description="Media type of the manifest")
    manifests: List[PlatformManifest] = Field(
        default_factory=list, description="List of platform manifests"
    )


class RegistryError(Exception):
    pass


class ManifestNotFoundError(RegistryError):
    pass


class RegistryConnectionError(RegistryError):
    pass


async def get_tag_manifest(registry_url: str, image: str, tag: str) -> ManifestResponse:
    url = urljoin(registry_url.rstrip("/") + "/", f"v2/{image}/manifests/{tag}")

    # Request both the index and the image manifests
    headers = {
        "Accept": "application/vnd.oci.image.index.v1+json"  # Request OCI index format specifically
    }

    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(url, headers=headers, timeout=30.0)

            if response.status_code == 404:
                raise ManifestNotFoundError(f"Manifest not found for {image}:{tag}")
            elif response.status_code != 200:
                raise RegistryError(
                    f"Registry API error: {response.status_code} - {response.text}"
                )

            # Get digest from Docker-Content-Digest header
            digest = response.headers.get("Docker-Content-Digest", "")
            if not digest:
                raise RegistryError("Missing Docker-Content-Digest header in response")

            manifest_data = response.json()
            # Parse platform manifests from the manifest list
            platform_manifests = []
            if "manifests" in manifest_data:
                for manifest in manifest_data["manifests"]:
                    platform_data = manifest.get("platform", {})
                    platform = Platform(
                        os=platform_data.get("os", ""),
                        architecture=platform_data.get("architecture", ""),
                        variant=platform_data.get("variant"),
                    )

                    platform_manifest = PlatformManifest(
                        platform=platform,
                        digest=manifest["digest"],
                        size=manifest["size"],
                        mediaType=manifest["mediaType"],
                    )
                    platform_manifests.append(platform_manifest)

            return ManifestResponse(
                digest=digest,
                schemaVersion=manifest_data.get("schemaVersion", 2),
                mediaType=manifest_data.get("mediaType", ""),
                manifests=platform_manifests,
            )

    except httpx.TimeoutException:
        raise RegistryConnectionError(
            f"Timeout connecting to registry at {registry_url}"
        )
    except httpx.ConnectError:
        raise RegistryConnectionError(
            f"Unable to connect to registry at {registry_url}"
        )
    except httpx.HTTPError as e:
        raise RegistryError(f"HTTP error: {e}")


async def get_manifests_for_tag(
    registry_url: str, image: str, tag: str
) -> List[PlatformManifest]:
    manifest_response = await get_tag_manifest(registry_url, image, tag)
    return manifest_response.manifests


async def get_manifest_ids_for_tag(
    registry_url: str, image: str, tag: str
) -> List[str]:
    url = f"{registry_url}/v2/{image}/manifests/{tag}"
    headers = {
        "Accept": "application/vnd.docker.distribution.manifest.list.v2+json,application/vnd.oci.image.index.v1+json,application/vnd.docker.distribution.manifest.v2+json"
    }
    r = requests.get(url, headers=headers, verify=False)
    manifest_data = r.json()

    if "manifests" in manifest_data:
        return [m["digest"] for m in manifest_data["manifests"]]
    return [r.headers.get("Docker-Content-Digest") or manifest_data.get("digest")]


def create_and_use_docker_builder():
    # make sure we use a builder with access to host network - to be able to push to the local registry
    builder_name = "host-network-builder"

    existing_builders = subprocess.run(
        ["docker", "buildx", "ls"],
        capture_output=True,
        text=True,
    ).stdout
    if builder_name in existing_builders:
        subprocess.run(
            ["docker", "buildx", "use", builder_name],
            check=True,
        )
        print(f"Using existing Docker builder '{builder_name}'.")
    else:
        subprocess.run(
            [
                "docker",
                "buildx",
                "create",
                "--use",
                "--name",
                builder_name,
                "--driver",
                "docker-container",
                "--driver-opt",
                "network=host",
            ],
            check=True,
        )
        print(f"Created and using new Docker builder '{builder_name}'.")


def start_docker_registry():
    container_list = subprocess.run(
        [
            "docker",
            "container",
            "list",
            "--format",
            "json",
            "--filter",
            "name=registry",
        ],
        capture_output=True,
    )
    if container_list.stdout and b'"registry"' in container_list.stdout:
        print("Docker registry is already running.")
        return

    subprocess.Popen(
        [
            "docker",
            "run",
            "-d",
            "--rm",
            "-p",
            "5000:5000",
            "--name",
            "registry",
            "registry:3",
        ]
    )

    is_available = False
    for i in range(10):
        try:
            response = httpx.get("http://localhost:5000/v2/")
            if response.status_code == 200:
                is_available = True
                print("Docker registry is up and running.")
                return
        except httpx.HTTPError:
            pass
        time.sleep(1)

    if not is_available:
        raise RuntimeError("Failed to start Docker registry.")


def decompose_tag(tag: str) -> Tuple[str, str, str]:
    hostname_index = tag.rindex("/")
    hostname = f"http://{tag[:hostname_index]}"
    image_name_full = tag[hostname_index + 1 :]
    image_name, tag_name = image_name_full.split(":")

    return hostname, image_name, tag_name
