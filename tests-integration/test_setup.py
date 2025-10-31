from typing import Literal

from pydantic import BaseModel


class SetupConfig(BaseModel):
    bakefile_type: Literal["single", "multiple", "none"]
    bakefile_location: Literal["root", "subdir"]
    dockerfile_type: Literal["single", "multiple"]
    dockerfile_location: Literal["root", "subdir"]
    targets: Literal["single", "multiple"]
    dockerignore: Literal["single", "multiple", "none"]
    context: Literal["cwd", "subdir"]
    cache_source: Literal["disk", "memory"]
