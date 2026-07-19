#!/usr/bin/env bash
# asc-devtools managed file
set -o errexit
set -o nounset
set -o pipefail
# shellcheck disable=SC1091
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/test_helper.sh"

test_help_version_completion_and_usage() {
  capture_command "${PROJECT_ROOT}/bin/asc" --help
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "repo clone"
  assert_contains "${CAPTURED_OUTPUT}" "repo sync"
  capture_command "${PROJECT_ROOT}/bin/asc" --version
  assert_equal "asc 0.1.0" "${CAPTURED_OUTPUT}"
  capture_command "${PROJECT_ROOT}/bin/asc" completion bash
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "complete -F _asc_completion asc"
  capture_command "${PROJECT_ROOT}/bin/asc" unknown
  assert_equal 2 "${CAPTURED_STATUS}"
}

test_doctor_json_schema() {
  local directory body
  directory=$(test_case_directory doctor_json)
  write_curl_mock "${directory}/bin"
  : >"${directory}/log"
  body="[$(github_repository_json asc-one)]"
  capture_command env HOME="${directory}/home" PATH="${directory}/bin:${SYSTEM_PATH}" \
    ASC_WORKSPACE="${directory}/workspace" ASC_GITHUB_TOKEN=test MOCK_LOG="${directory}/log" \
    MOCK_CURL_BODY="${body}" "${PROJECT_ROOT}/bin/asc" doctor --json
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" '"name":"configuration"'
  assert_contains "${CAPTURED_OUTPUT}" '"name":"github-api"'
  assert_contains "${CAPTURED_OUTPUT}" '"status":"pass"'
  assert_contains "${CAPTURED_OUTPUT}" '"name":"api-authentication"'
}

test_install_lifecycle() {
  capture_command "${PROJECT_ROOT}/scripts/test_install.sh"
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "install lifecycle tests passed"
}

test_operational_and_usage_codes() {
  local directory
  directory=$(test_case_directory exit_codes)
  write_curl_mock "${directory}/bin"
  : >"${directory}/log"
  capture_command env HOME="${directory}/home" PATH="${directory}/bin:${SYSTEM_PATH}" \
    ASC_GITHUB_TOKEN=test MOCK_LOG="${directory}/log" MOCK_CURL_STATUS=500 \
    "${PROJECT_ROOT}/bin/asc" repo list
  assert_equal 1 "${CAPTURED_STATUS}"
  capture_command "${PROJECT_ROOT}/bin/asc" repo sync --unknown
  assert_equal 2 "${CAPTURED_STATUS}"
}

run_test "help version completion and usage" test_help_version_completion_and_usage
run_test "doctor JSON schema" test_doctor_json_schema
run_test "install lifecycle" test_install_lifecycle
run_test "exit codes" test_operational_and_usage_codes
finish_tests
