#!/usr/bin/env bash
# asc-devtools managed file
set -o errexit
set -o nounset
set -o pipefail
# shellcheck disable=SC1091
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/test_helper.sh"

test_defaults() {
  local directory
  directory=$(test_case_directory defaults)
  capture_command env -u ASC_CONFIG -u ASC_WORKSPACE HOME="${directory}/home" PATH="${SYSTEM_PATH}" \
    "${PROJECT_ROOT}/bin/asc" workspace
  assert_success "${CAPTURED_STATUS}"
  assert_equal "${directory}/home/projects/AI4SciComp" "${CAPTURED_OUTPUT}"
}

test_json_and_precedence() {
  local directory config
  directory=$(test_case_directory json_precedence)
  config="${directory}/config.json"
  cat >"${config}" <<EOF
{
  "organization": "FileOrg",
  "workspace": "${directory}/from file",
  "repositoryPrefix": "project-",
  "includeDotGitHub": false,
  "cloneProtocol": "https",
  "remote": "upstream"
}
EOF
  capture_command env HOME="${directory}/home" ASC_CONFIG="${config}" \
    ASC_WORKSPACE="${directory}/from-environment" PATH="${SYSTEM_PATH}" \
    "${PROJECT_ROOT}/bin/asc" --workspace "${directory}/from-cli" workspace
  assert_success "${CAPTURED_STATUS}"
  assert_equal "${directory}/from-cli" "${CAPTURED_OUTPUT}"
}

test_strict_json() {
  local directory config
  directory=$(test_case_directory strict_json)
  config="${directory}/config.json"
  printf '{"unknown":true}\n' >"${config}"
  capture_command env HOME="${directory}/home" ASC_CONFIG="${config}" PATH="${SYSTEM_PATH}" \
    "${PROJECT_ROOT}/bin/asc" workspace
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "unknown configuration field"
  printf '{"includeDotGitHub":"yes"}\n' >"${config}"
  capture_command env HOME="${directory}/home" ASC_CONFIG="${config}" PATH="${SYSTEM_PATH}" \
    "${PROJECT_ROOT}/bin/asc" workspace
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "must be true or false"
  printf '{"workspace":"line\nbreak"}\n' >"${config}"
  capture_command env HOME="${directory}/home" ASC_CONFIG="${config}" PATH="${SYSTEM_PATH}" \
    "${PROJECT_ROOT}/bin/asc" workspace
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "unescaped control character"
}

test_json_escapes() {
  local directory config
  directory=$(test_case_directory json_escapes)
  config="${directory}/config.json"
  printf '%s\n' '{"workspace":"~/back\\slash"}' >"${config}"
  capture_command env HOME="${directory}/home" ASC_CONFIG="${config}" PATH="${SYSTEM_PATH}" \
    "${PROJECT_ROOT}/bin/asc" workspace
  assert_success "${CAPTURED_STATUS}"
  assert_equal "${directory}/home/back\\slash" "${CAPTURED_OUTPUT}"
}

test_environment_validation() {
  local directory
  directory=$(test_case_directory invalid_environment)
  capture_command env HOME="${directory}/home" ASC_INCLUDE_DOT_GITHUB=maybe PATH="${SYSTEM_PATH}" \
    "${PROJECT_ROOT}/bin/asc" workspace
  assert_failure "${CAPTURED_STATUS}"
  capture_command env HOME="${directory}/home" ASC_WORKSPACE=/ PATH="${SYSTEM_PATH}" \
    "${PROJECT_ROOT}/bin/asc" workspace
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "filesystem root"
}

run_test "configuration defaults" test_defaults
run_test "JSON and precedence" test_json_and_precedence
run_test "strict JSON" test_strict_json
run_test "JSON escapes" test_json_escapes
run_test "environment validation" test_environment_validation
finish_tests
