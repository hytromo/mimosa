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


def decompose_tag(tag: str) -> Tuple[str, str, str]:
    # Format: registry:port/path/to/image:tag
    # Find the last colon which separates the tag
    last_colon_index = tag.rindex(":")
    tag_name = tag[last_colon_index + 1 :]
    
    # Everything before the last colon is registry:port/path/to/image
    registry_and_image = tag[:last_colon_index]
    
    # Find the first slash which separates registry from image path
    first_slash_index = registry_and_image.index("/")
    hostname = f"http://{registry_and_image[:first_slash_index]}"
    image_name = registry_and_image[first_slash_index + 1 :]

    return hostname, image_name, tag_name
