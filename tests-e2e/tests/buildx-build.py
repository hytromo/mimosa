#!/usr/bin/env python3

from utils.test_runner import MimosaTestRunner


def test_buildx_build(
  test_runner: MimosaTestRunner, buildx_build_test_case, test_defaults, request
):
  """Main test function that runs all test cases dynamically."""

  test_id = request.node.name.split("[")[1].split("]")[0]
  test_result = test_runner.run_test_case(
    test_id, buildx_build_test_case, test_defaults
  )

  assert test_result.error is None, (
    f"Test {request.node.name} failed: {test_result.error}"
    + f"\nExtra debug info: {test_result.extra_debug_info}"
  )
