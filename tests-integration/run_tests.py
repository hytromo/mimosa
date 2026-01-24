#!/usr/bin/env python

import asyncio
import multiprocessing
import os
import subprocess
import sys
import tempfile
import time
import traceback
import uuid
from collections import deque
from pathlib import Path
from typing import Dict, List, Tuple

from command_creator import Command, generate_docker_command
from docker import (
    decompose_tag,
    get_manifest_ids_for_tag,
    get_manifests_for_tag,
)
from test_creator import (
    create_bakefiles,
    create_dockerfiles,
    create_dockerignores,
    create_other_files,
)
from test_setup import SetupConfig


async def assert_tags_have_platforms(
    tagsByTarget: Dict[str, List[str]],
    expected_platforms: List[str],
):
    at_least_one_checked = False
    for target, tags in tagsByTarget.items():
        for tag in tags:
            hostname, image_name, tag_name = decompose_tag(tag)

            platform_manifests = await get_manifests_for_tag(
                hostname,
                image_name,
                tag_name,
            )

            assert len(platform_manifests) >= 2, (
                f"Expected at least 2 platform manifests for tag {tag} and target {target}"
            )

            available_platforms = []
            for manifest in platform_manifests:
                platform_str = (
                    f"{manifest.platform.os}/{manifest.platform.architecture}"
                )
                if manifest.platform.variant:
                    platform_str += f"/{manifest.platform.variant}"
                available_platforms.append(platform_str)

            assert all(
                platform in available_platforms for platform in expected_platforms
            ), (
                f"Expected platforms {expected_platforms} for tag {tag} and target {target} not all present"
            )

            print(f"👍 {target} {tag} has expected platforms {expected_platforms}")
            at_least_one_checked = True

    assert at_least_one_checked, "Expected at least one tag to be checked"


async def run_command_with_expectations(
    command: Command,
    cwd: Path,
    cache_hit: bool,
    expect_same_manifests_with_on_cache_hit: List[str] = None,
    extra_output_expectation=None,
):
    try:
        # Print directory structure
        tree_result = subprocess.run(
            ["tree", "-a", cwd],
            capture_output=True,
            text=True,
        )
        print(f"> tree -a '{cwd}':")
        print(tree_result.stdout)
        sys.stdout.flush()

        if "TARGET1_TAGS" in command.env and "TARGET2_TAGS" in command.env:
            print(f"export TARGET1_TAGS={command.env['TARGET1_TAGS']}")
            print(f"export TARGET2_TAGS={command.env['TARGET2_TAGS']}")

        print(f"cd {cwd} && {command.command}")

        sys.stdout.flush()

        result = subprocess.run(
            command.command,
            cwd=cwd,
            env=command.env,
            shell=True,
            check=True,
            capture_output=True,
            text=True,
        )

        print("STDOUT:")
        print(result.stdout)

        if result.stderr:
            print("STDERR:")
            print(result.stderr)

        expected_output = (
            "mimosa-cache-hit: true" if cache_hit else "mimosa-cache-hit: false"
        )

        if expected_output not in result.stdout:
            raise ValueError(f"Expected {expected_output} in stdout")
        else:
            print(f"👍 Cache hit expectation met: {expected_output}")

        if extra_output_expectation:
            if (extra_output_expectation not in result.stdout) and (
                extra_output_expectation not in result.stderr
            ):
                raise ValueError(f"Expected {extra_output_expectation} in stdout or stderr")
            else:
                print(f"👍 Extra output expectation met: {extra_output_expectation}")

        if not cache_hit:
            # the original manifests need to include information about the platforms
            await assert_tags_have_platforms(
                command.tagsByTarget,
                ["linux/amd64", "linux/arm64"],
            )

        if (
            cache_hit
            and expect_same_manifests_with_on_cache_hit
            and len(expect_same_manifests_with_on_cache_hit) > 0
        ):
            # Compare corresponding tags by index, not all combinations
            # Both lists should have the same length and correspond by position
            current_tags = command.tagsByTarget["target1"]
            assert len(current_tags) == len(expect_same_manifests_with_on_cache_hit), (
                f"Expected same number of tags: {len(current_tags)} vs {len(expect_same_manifests_with_on_cache_hit)}"
            )
            
            for tag, expect_tag in zip(current_tags, expect_same_manifests_with_on_cache_hit):
                hostname, image_name, tag_name = decompose_tag(tag)
                expect_hostname, expect_image_name, expect_tag_name = decompose_tag(
                    expect_tag
                )

                now_manifests = await get_manifest_ids_for_tag(
                    hostname,
                    image_name,
                    tag_name,
                )

                expected_manifests = await get_manifest_ids_for_tag(
                    expect_hostname,
                    expect_image_name,
                    expect_tag_name,
                )

                assert set(now_manifests) == set(expected_manifests), (
                    f"Expected manifests for tag {tag} to be the same as for tag {expect_tag}. "
                    f"Got: {now_manifests}, expected: {expected_manifests}"
                )

                assert len(now_manifests) > 0, (
                    f"Expected at least one manifest for tag {tag}"
                )

        # make sure that every target has the assumed files included
        for target, tags in command.tagsByTarget.items():
            for tag in tags:
                subprocess.run(
                    f"docker rmi --force {tag}",
                    cwd=cwd,
                    shell=True,
                    check=False,
                    capture_output=True,
                )

            for tag in tags:
                lsproc = subprocess.run(
                    f"docker run --rm {tag} ls /added_files_{target}",
                    cwd=cwd,
                    shell=True,
                    check=True,
                    capture_output=True,
                    text=True,
                )

                # make sure that "included_file.txt" is present and "excluded_file.txt" is absent
                if "included_file.txt" not in lsproc.stdout:
                    raise ValueError(
                        f"Expected included_file.txt to be present in image {tag} for target {target}"
                    )
                else:
                    print(
                        f"👍 included_file.txt is present in image {tag} for target {target}"
                    )
                if "excluded_file.txt" in lsproc.stdout:
                    raise ValueError(
                        f"Expected excluded_file.txt to be absent in image {tag} for target {target}"
                    )
                else:
                    print(
                        f"👍 excluded_file.txt is absent in image {tag} for target {target}"
                    )

    except subprocess.CalledProcessError as e:
        print(f"Command failed with return code {e.returncode}")
        print(f"stdout: {e.stdout}")
        print(f"stderr: {e.stderr}")

        raise


def generate_target_tags(
    suffix: str, existing_uuid: str = None
) -> Tuple[str, Dict[str, List[str]]]:
    existing_uuid = existing_uuid or str(uuid.uuid4())

    return existing_uuid, {
        "first_target_tags": [
            f"localhost:5000/{existing_uuid}/target1_1st:{suffix}",
            f"localhost:5000/{existing_uuid}/target1_2nd:{suffix}",
        ],
        "second_target_tags": [
            f"localhost:5000/{existing_uuid}/target2_1st:{suffix}",
            f"localhost:5000/{existing_uuid}/target2_2nd:{suffix}",
        ],
    }


def env_variable_to_list(var_name: str, default_value: List[str]) -> List[str]:
    value = os.environ.get(var_name, "")
    if not value:
        return default_value
    return [item.strip() for item in value.split(",") if item.strip()]


async def main():
    max_workers = 5
    total_tests_run = 0

    # Build list of tests to run
    test_configs = []
    # the simple case:
    # export bakefile_type="single"; export bakefile_location="root"; export dockerfile_type="single"; export dockerfile_location="root"; export targets="single"; export dockerignore="single"; export context="cwd"
    for bakefile_type in env_variable_to_list(
        "bakefile_type", ["single", "multiple", "none"]
    ):
        for bakefile_location in env_variable_to_list(
            "bakefile_location", ["root", "subdir"]
        ):
            for dockerfile_type in env_variable_to_list(
                "dockerfile_type", ["single", "multiple"]
            ):
                for dockerfile_location in env_variable_to_list(
                    "dockerfile_location", ["root", "subdir"]
                ):
                    for targets in env_variable_to_list(
                        "targets", ["single", "multiple"]
                    ):
                        for dockerignore in env_variable_to_list(
                            "dockerignore", ["single", "multiple", "none"]
                        ):
                            for context in env_variable_to_list(
                                "context", ["cwd", "subdir"]
                            ):
                                if bakefile_type == "none":
                                    if (
                                        dockerfile_type == "multiple"
                                        or dockerignore == "multiple"
                                        or targets == "multiple"
                                    ):
                                        continue
                                elif (
                                    bakefile_type == "single"
                                    and context == "subdir"
                                ):
                                    continue
                                elif (
                                    bakefile_type == "multiple"
                                    and context == "subdir"
                                ):
                                    continue

                                setup_config = {
                                    "bakefile_type": bakefile_type,
                                    "bakefile_location": bakefile_location,
                                    "dockerfile_type": dockerfile_type,
                                    "dockerfile_location": dockerfile_location,
                                    "targets": targets,
                                    "dockerignore": dockerignore,
                                    "context": context,
                                }
                                output_dir = Path(
                                    tempfile.mkdtemp(
                                        prefix="mimosa_", suffix="_workdir"
                                    )
                                )
                                test_configs.append((setup_config, str(output_dir)))

    # Run tests in processes, keep per-test logs at output_dir/test.log
    pending_test_configs = deque(test_configs)
    running_processes: list[multiprocessing.Process] = []
    all_processes: list[multiprocessing.Process] = []
    process_to_config: dict[multiprocessing.Process, tuple[dict, str, int]] = {}
    failed_num = 0

    try:
        while pending_test_configs or running_processes:
            # start as many as slots available
            while pending_test_configs and len(running_processes) < max_workers:
                cfg, outdir = pending_test_configs.popleft()
                print(
                    "Running test number",
                    len(all_processes) + 1,
                    "/",
                    len(test_configs),
                    "at",
                    outdir,
                    "with config:",
                    cfg,
                )
                total_tests_run += 1
                new_process = multiprocessing.Process(
                    target=run_test_worker, args=(cfg, outdir)
                )
                new_process.start()
                running_processes.append(new_process)
                all_processes.append(new_process)
                process_to_config[new_process] = (cfg, outdir, len(all_processes))

            # poll running processes
            for running_process in list(running_processes):
                if running_process.exitcode is None:
                    continue
                running_processes.remove(running_process)
                if running_process.exitcode != 0:
                    # test failed -> terminate all and raise
                    cfg, outdir, num = process_to_config[running_process]
                    print(
                        f"Test #{num} failed for config {cfg}. Log: {outdir}-test.log"
                    )
                    # let's print the full log here:
                    with open(f"{outdir}-test.log", "r") as logf:
                        print(logf.read())
                    failed_num += 1
                    sys.stdout.flush()
                    # terminate remaining running and pending processes
                    raise SystemExit(1)
            time.sleep(0.1)
    finally:
        # ensure all processes are cleaned up
        for running_process in all_processes:
            if running_process.is_alive():
                try:
                    running_process.terminate()
                except Exception:
                    pass
            running_process.join(timeout=1)

        output_summary = {
            "total_tests_run": total_tests_run,
        }
        with open("integration_tests_summary.json", "w") as outf:
            import json

            json.dump(output_summary, outf, indent=2)

    if failed_num > 0:
        raise SystemExit(f"{failed_num} tests failed.")
    else:
        print("All tests completed successfully.")


async def run_test(setup_config: SetupConfig, output_dir: Path):
    try:
        print("=" * 90)
        print("Setup config:")
        print(setup_config)
        print("=" * 90)
        print("\n" * 2)

        output_dir.mkdir(parents=True, exist_ok=True)
        bakefiles = create_bakefiles(setup_config, output_dir)
        dockerfiles = create_dockerfiles(setup_config, output_dir)
        create_dockerignores(setup_config, dockerfiles, output_dir)

        other_files = create_other_files(setup_config, output_dir)
        original_uuid, target_tags = generate_target_tags("original")
        original_command_options = {
            "setup_config": setup_config,
            "dockerfiles": dockerfiles,
            "bakefiles": bakefiles,
            "output_dir": output_dir,
            **target_tags,
        }
        initial_1st_target_tags: List[str] = original_command_options[
            "first_target_tags"
        ]
        command = generate_docker_command(**original_command_options)
        # Remove old target tags and add new ones
        command_options_no_tags = original_command_options.copy()
        command_options_no_tags.pop("first_target_tags", None)
        command_options_no_tags.pop("second_target_tags", None)

        await run_command_with_expectations(command, cwd=output_dir, cache_hit=False)

        # change the content of an excluded file and expect cache hit
        tested_changes = {}
        # order matters here, we are comparing with the original 1st target tags,
        # so if we modify an included first, we need to compare the excluded tag with the included tag - not the original
        for file_type in ["excluded", "included"]:
            for file in other_files:
                if file_type in file.location.name:
                    print(f"> Modifying {file_type} file to test cache hit...")
                    file.location.write_text(f"Modified at {time.time()}")

                    _, new_tags = generate_target_tags(
                        f"modified-{file_type}", original_uuid
                    )
                    cache_hit_expected = True if file_type == "excluded" else False
                    await run_command_with_expectations(
                        generate_docker_command(
                            **command_options_no_tags,
                            **new_tags,
                        ),
                        cwd=output_dir,
                        cache_hit=cache_hit_expected,
                        expect_same_manifests_with_on_cache_hit=initial_1st_target_tags,
                    )
                    tested_changes[file_type] = True

        assert tested_changes.get("included")
        assert tested_changes.get("excluded")

        # make sure we change the basic file(s) (Dockerfile, bake file) and we always have a cache miss
        at_least_one_major_file_change = False
        if len(bakefiles):
            for bakefile in bakefiles:
                print(
                    f"> Modifying bakefile {bakefile.location.absolute()} to test cache miss..."
                )
                with open(bakefile.location, "a") as f:
                    f.write(f"\n# Modified at {time.time()}\n")

                _, new_tags = generate_target_tags(
                    "modified-cache-miss-bakefile", original_uuid
                )
                await run_command_with_expectations(
                    generate_docker_command(
                        **command_options_no_tags,
                        **new_tags,
                    ),
                    cwd=output_dir,
                    cache_hit=False,
                )
                at_least_one_major_file_change = True

        if len(dockerfiles):
            for dockerfile in dockerfiles:
                print(
                    f"> Modifying dockerfile {dockerfile.location.absolute()} to test cache miss..."
                )
                with open(dockerfile.location, "a") as f:
                    f.write(f"\n# Modified at {time.time()}\n")

                _, new_tags = generate_target_tags(
                    "modified-cache-miss-dockerfile", original_uuid
                )
                await run_command_with_expectations(
                    generate_docker_command(
                        **command_options_no_tags,
                        **new_tags,
                    ),
                    cwd=output_dir,
                    cache_hit=False,
                )
                at_least_one_major_file_change = True

        assert at_least_one_major_file_change

        print("=" * 80)
        print()
        sys.stdout.flush()
    except Exception as e:
        print("Exception during test run:")
        traceback.print_exc()
        raise e


def run_test_worker(
    setup_config_dict: dict,
    output_dir_str: str,
):
    log_path = f"{output_dir_str}-test.log"
    try:
        setup = SetupConfig(**setup_config_dict)
        output_dir = Path(output_dir_str)
        with open(log_path, "w", buffering=1) as logf:
            # redirect both stdout and stderr to the per-test log file
            sys.stdout = logf
            sys.stderr = logf
            # run the async test
            asyncio.run(run_test(setup, output_dir))
    except Exception:
        tb = traceback.format_exc()
        try:
            with open(log_path, "a") as logf:
                logf.write("\n\n=== EXCEPTION ===\n")
                logf.write(tb)
        except Exception:
            pass
        # ensure non-zero exit code
        raise


if __name__ == "__main__":
    asyncio.run(main())
