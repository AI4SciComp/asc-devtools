#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/test_helper.sh"

test_defaults() {
  local case_directory
  case_directory=$(test_case_directory defaults)
  capture_command env -u ASC_CONFIG -u ASC_ORGANIZATION -u ASC_WORKSPACE \
    HOME="${case_directory}/home" PATH="${SYSTEM_PATH}" "${PROJECT_ROOT}/bin/asc" workspace
  assert_success "${CAPTURED_STATUS}"
  assert_equal "${case_directory}/home/projects/AI4SciComp" "${CAPTURED_OUTPUT}"
}

test_trusted_config_and_space_path() {
  local case_directory
  case_directory=$(test_case_directory trusted_config)
  local config="${case_directory}/config"
  cat >"${config}" <<EOF
ASC_ORGANIZATION="ConfiguredOrg"
ASC_WORKSPACE="${case_directory}/workspace with spaces"
ASC_REPOSITORY_PREFIX="project-"
ASC_INCLUDE_DOT_GITHUB="false"
ASC_CLONE_PROTOCOL="https"
ASC_REMOTE="upstream"
EOF
  capture_command env -u ASC_WORKSPACE HOME="${case_directory}/home" \
    ASC_CONFIG="${config}" PATH="${SYSTEM_PATH}" "${PROJECT_ROOT}/bin/asc" workspace
  assert_success "${CAPTURED_STATUS}"
  assert_equal "${case_directory}/workspace with spaces" "${CAPTURED_OUTPUT}"
}

test_environment_precedes_config() {
  local case_directory
  case_directory=$(test_case_directory environment_precedence)
  local config="${case_directory}/config"
  printf 'ASC_WORKSPACE="%s"\n' "${case_directory}/from-config" >"${config}"
  capture_command env HOME="${case_directory}/home" ASC_CONFIG="${config}" \
    ASC_WORKSPACE="${case_directory}/from-environment" PATH="${SYSTEM_PATH}" \
    "${PROJECT_ROOT}/bin/asc" workspace
  assert_success "${CAPTURED_STATUS}"
  assert_equal "${case_directory}/from-environment" "${CAPTURED_OUTPUT}"
}

test_cli_precedes_environment() {
  local case_directory
  case_directory=$(test_case_directory cli_precedence)
  capture_command env HOME="${case_directory}/home" \
    ASC_WORKSPACE="${case_directory}/from-environment" PATH="${SYSTEM_PATH}" \
    "${PROJECT_ROOT}/bin/asc" --workspace "${case_directory}/from-cli" workspace
  assert_success "${CAPTURED_STATUS}"
  assert_equal "${case_directory}/from-cli" "${CAPTURED_OUTPUT}"
}

test_alternative_config_option() {
  local case_directory
  case_directory=$(test_case_directory config_option)
  local config="${case_directory}/alternate"
  printf 'ASC_WORKSPACE="%s"\n' "${case_directory}/alternate-workspace" >"${config}"
  capture_command env -u ASC_CONFIG -u ASC_WORKSPACE HOME="${case_directory}/home" \
    PATH="${SYSTEM_PATH}" "${PROJECT_ROOT}/bin/asc" --config "${config}" workspace
  assert_success "${CAPTURED_STATUS}"
  assert_equal "${case_directory}/alternate-workspace" "${CAPTURED_OUTPUT}"
}

test_invalid_boolean_and_protocol() {
  local case_directory
  case_directory=$(test_case_directory invalid_values)
  capture_command env HOME="${case_directory}/home" ASC_INCLUDE_DOT_GITHUB=maybe \
    PATH="${SYSTEM_PATH}" "${PROJECT_ROOT}/bin/asc" workspace
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "must be 'true' or 'false'"
  capture_command env HOME="${case_directory}/home" ASC_CLONE_PROTOCOL=ftp \
    PATH="${SYSTEM_PATH}" "${PROJECT_ROOT}/bin/asc" workspace
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "must be 'ssh' or 'https'"
}

run_test "configuration defaults" test_defaults
run_test "trusted config and paths with spaces" test_trusted_config_and_space_path
run_test "environment precedence" test_environment_precedes_config
run_test "CLI precedence" test_cli_precedes_environment
run_test "alternative config" test_alternative_config_option
run_test "invalid configuration" test_invalid_boolean_and_protocol
finish_tests
