import json
import os
from pathlib import Path
from typing import Dict, List

from pydantic import BaseModel
from test_creator import (
    TemplatedFile,
)
from test_setup import SetupConfig

# Allow overriding the mimosa binary path via environment variable
MIMOSA_BINARY = os.environ.get("MIMOSA_BINARY", "/usr/local/bin/mimosa")


class Command(BaseModel):
    command: str
    tagsByTarget: Dict[str, List[str]] = {}
    env: Dict[str, str]
    cwd: Path


def generate_docker_command(
    setup_config: SetupConfig,
    dockerfiles: List[TemplatedFile],
    bakefiles: List[TemplatedFile],
    output_dir: Path,
    first_target_tags: List[str],
    second_target_tags: List[str],
) -> Command:
    cwd = Path(output_dir)

    if setup_config.bakefile_type == "none":
        if len(dockerfiles) > 1:
            raise ValueError("Cannot build multiple dockerfiles with no bakefile")

        dockerfileRelativePath = dockerfiles[0].location.relative_to(cwd)

        if (
            str(dockerfileRelativePath) == "Dockerfile"
            and setup_config.context == "cwd"
        ):
            dockerFileArg = ""
        else:
            dockerFileArg = f"-f '{dockerfileRelativePath}'"

        dockerTagsArg = " ".join([f"-t {tag}" for tag in first_target_tags])

        context = "." if setup_config.context == "cwd" else "subdir"

        return Command(
            command=f"{MIMOSA_BINARY} remember -- docker buildx build --push --platform linux/amd64,linux/arm64 {dockerFileArg} {dockerTagsArg} {context}",
            cwd=cwd,
            tagsByTarget={
                "target1": first_target_tags,
            },
            env={
                **os.environ.copy(),
                "LOG_LEVEL": "DEBUG",
            },
        )

    bakeFileArg = ""

    if setup_config.bakefile_location == "subdir":
        # all bakefiles are in the subdir, we need to explicitly specify them.
        bakeFileArg = " ".join(
            [f"-f {bakefile.location.relative_to(cwd)}" for bakefile in bakefiles]
        )

    tagsByTarget = {
        "target1": first_target_tags,
    }
    if setup_config.targets == "multiple":
        tagsByTarget["target2"] = second_target_tags

    # Docker buildx bake expects list variables as CSV
    # Values with special characters (like colons) may need quoting, but CSV format should work
    # Format: "value1,value2" or just "value1,value2" depending on docker buildx bake version
    return Command(
        command=f"{MIMOSA_BINARY} remember -- docker buildx bake --push {bakeFileArg}",
        cwd=cwd,
        tagsByTarget=tagsByTarget,
        env={
            **os.environ.copy(),
            "LOG_LEVEL": "DEBUG",
            "TARGET1_TAGS": ",".join(first_target_tags),
            "TARGET2_TAGS": ",".join(second_target_tags),
        },
    )
