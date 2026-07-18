#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/test_helper.sh"

test_filters_and_sorts() {
  local case_directory
  case_directory=$(test_case_directory github_filter)
  write_gh_mock "${case_directory}/bin"
  : >"${case_directory}/log"
  capture_command env HOME="${case_directory}/home" PATH="${case_directory}/bin:${SYSTEM_PATH}" \
    MOCK_LOG="${case_directory}/log" \
    MOCK_GH_REPOSITORIES=$'asc-z\tfalse\nother\tfalse\nasc-old\ttrue\n.github\tfalse\nasc-a\tfalse\n' \
    "${PROJECT_ROOT}/bin/asc" repo list
  assert_success "${CAPTURED_STATUS}"
  assert_equal $'.github\nasc-a\nasc-z' "${CAPTURED_OUTPUT}"
  assert_contains "$(<"${case_directory}/log")" "repo list AI4SciComp"
}

test_dot_github_exclusion() {
  local case_directory
  case_directory=$(test_case_directory github_dot_exclusion)
  write_gh_mock "${case_directory}/bin"
  : >"${case_directory}/log"
  capture_command env HOME="${case_directory}/home" PATH="${case_directory}/bin:${SYSTEM_PATH}" \
    MOCK_LOG="${case_directory}/log" MOCK_GH_REPOSITORIES=$'.github\tfalse\nasc-one\tfalse\n' \
    "${PROJECT_ROOT}/bin/asc" --no-include-dot-github repo list
  assert_success "${CAPTURED_STATUS}"
  assert_equal "asc-one" "${CAPTURED_OUTPUT}"
}

test_authentication_failure() {
  local case_directory
  case_directory=$(test_case_directory github_failure)
  write_gh_mock "${case_directory}/bin"
  : >"${case_directory}/log"
  capture_command env HOME="${case_directory}/home" PATH="${case_directory}/bin:${SYSTEM_PATH}" \
    MOCK_LOG="${case_directory}/log" MOCK_GH_FAIL=true \
    "${PROJECT_ROOT}/bin/asc" repo list
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "gh auth login"
}

run_test "GitHub filtering and ordering" test_filters_and_sorts
run_test ".github exclusion" test_dot_github_exclusion
run_test "GitHub authentication failure" test_authentication_failure
finish_tests
