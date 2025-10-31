from pathlib import Path
from typing import List

from jinja2 import Environment, FileSystemLoader
from pydantic import BaseModel

from test_setup import SetupConfig

jinja_env = Environment(
    loader=FileSystemLoader("./templates"), trim_blocks=True, lstrip_blocks=True
)


class TemplatedFile(BaseModel):
    location: Path
    content: str


def create_bakefiles(
    setup_config: SetupConfig, output_dir: Path
) -> List[TemplatedFile]:
    if setup_config.bakefile_type == "none":
        return []

    bakefiles_parent_dir = Path(output_dir)
    if setup_config.bakefile_location == "subdir":
        bakefiles_parent_dir = bakefiles_parent_dir / "subdir"

    bakefiles_parent_dir.mkdir(parents=True, exist_ok=True)

    bake_files = [
        TemplatedFile(
            location=bakefiles_parent_dir / "docker-bake.hcl",
            content=jinja_env.get_template("docker-bake.hcl").render(setup_config),
        )
    ]

    if setup_config.bakefile_type == "multiple":
        bake_files.append(
            TemplatedFile(
                location=bakefiles_parent_dir / "docker-bake.override.hcl",
                content=jinja_env.get_template("docker-bake.override.hcl").render(
                    setup_config
                ),
            )
        )

    for bakefile in bake_files:
        bakefile.location.write_text(bakefile.content)

    return bake_files


def create_dockerfiles(
    setup_config: SetupConfig, output_dir: Path
) -> List[TemplatedFile]:
    dockerfiles_parent_dir = Path(output_dir)

    if setup_config.dockerfile_location == "subdir":
        dockerfiles_parent_dir = dockerfiles_parent_dir / "subdir"

    dockerfiles_parent_dir.mkdir(parents=True, exist_ok=True)

    dockerfiles = [
        TemplatedFile(
            location=dockerfiles_parent_dir
            / (
                "Dockerfile"
                if setup_config.dockerfile_type == "single"
                else "Dockerfile.target1"
            ),
            content=jinja_env.get_template("Dockerfile").render(setup_config),
        )
    ]

    if setup_config.dockerfile_type == "multiple":
        dockerfiles.append(
            TemplatedFile(
                location=dockerfiles_parent_dir / "Dockerfile.target2",
                content=jinja_env.get_template("Dockerfile2").render(setup_config),
            )
        )

    for dockerfile in dockerfiles:
        dockerfile.location.write_text(dockerfile.content)

    return dockerfiles


def create_dockerignores(
    setup_config: SetupConfig, dockerfiles: List[TemplatedFile], output_dir: Path
) -> List[TemplatedFile]:
    dockerignores = []

    if setup_config.dockerfile_type == "multiple":
        # each dockerfile gets its own dockerignore file
        for dockerfile_index, dockerfile in enumerate(dockerfiles):
            dockerignores.append(
                TemplatedFile(
                    location=f"{dockerfile.location.absolute()}.dockerignore",
                    content=jinja_env.get_template(
                        "dockerignore" + str(dockerfile_index + 1)
                    ).render(setup_config),
                )
            )

        # make a "trick" dockerignore file that should not be used by docker and mimosa
        # ignore everything - this is not a the dockerignore file that will be picked up by docker buildx
        dockerignores.append(
            TemplatedFile(
                location=output_dir / ".dockerignore",
                content="**/*",
            )
        )
    else:
        dockerignore_path = Path(output_dir)
        if setup_config.context == "subdir":
            dockerignore_path = dockerignore_path / "subdir"
            dockerignore_path.mkdir(parents=True, exist_ok=True)

        dockerignores.append(
            TemplatedFile(
                location=dockerignore_path / ".dockerignore",
                content=jinja_env.get_template("dockerignore1").render(setup_config),
            )
        )

    for dockerignore in dockerignores:
        dockerignore.location.write_text(dockerignore.content)

    return dockerignores


def create_other_files(setup_config: SetupConfig, output_dir: Path):
    creation_dir = Path(output_dir)
    if setup_config.context == "subdir":
        # make sure created files are inside the build context
        creation_dir = creation_dir / "subdir"

    other_files = [
        TemplatedFile(
            location=creation_dir / "included_file.txt",
            content="This is another file in the subdir.",
        ),
        TemplatedFile(
            location=creation_dir / "excluded_file.txt",
            content="This file should be excluded by .dockerignore.",
        ),
    ]

    creation_dir.mkdir(parents=True, exist_ok=True)

    for other_file in other_files:
        other_file.location.write_text(other_file.content)

    return other_files
