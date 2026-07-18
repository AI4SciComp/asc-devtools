#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/test_helper.sh"

test_help_and_version() {
  capture_command "${PROJECT_ROOT}/bin/asc" --help
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "asc repo clone"
  capture_command "${PROJECT_ROOT}/bin/asc" --version
  assert_success "${CAPTURED_STATUS}"
  assert_equal "asc 0.1.0" "${CAPTURED_OUTPUT}"
}

test_invalid_command_status() {
  capture_command "${PROJECT_ROOT}/bin/asc" unknown
  assert_equal 2 "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "unknown command"
}

test_operational_failure_status() {
  local case_directory
  case_directory=$(test_case_directory cli_operational)
  write_gh_mock "${case_directory}/bin"
  : >"${case_directory}/log"
  capture_command env HOME="${case_directory}/home" PATH="${case_directory}/bin:${SYSTEM_PATH}" \
    MOCK_LOG="${case_directory}/log" MOCK_GH_FAIL=true \
    "${PROJECT_ROOT}/bin/asc" repo clone
  assert_equal 1 "${CAPTURED_STATUS}"
}

test_completion_command() {
  capture_command "${PROJECT_ROOT}/bin/asc" completion bash
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "complete -F _asc_completion asc"
}

test_doctor_separates_api_and_ssh_authentication() {
  local case_directory
  case_directory=$(test_case_directory doctor)
  write_gh_mock "${case_directory}/bin"
  cat >"${case_directory}/bin/ssh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "Hi escapetiger! You've successfully authenticated, but GitHub does not provide shell access." >&2
exit 1
EOF
  chmod +x "${case_directory}/bin/ssh"
  : >"${case_directory}/log"
  capture_command env HOME="${case_directory}/home" PATH="${case_directory}/bin:${SYSTEM_PATH}" \
    ASC_WORKSPACE="${case_directory}/workspace" MOCK_LOG="${case_directory}/log" \
    MOCK_GH_REPOSITORIES=$'asc-one\tfalse\n' \
    "${PROJECT_ROOT}/bin/asc" doctor
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "GitHub CLI API authentication"
  assert_contains "${CAPTURED_OUTPUT}" "authenticated as escapetiger"
  assert_contains "${CAPTURED_OUTPUT}" "Git SSH authentication"
  assert_contains "${CAPTURED_OUTPUT}" "GitHub accepted the SSH key"
}

test_installer_and_uninstaller() {
  local case_directory
  case_directory=$(test_case_directory installer)
  local prefix="${case_directory}/prefix"
  capture_command "${PROJECT_ROOT}/install.sh" --prefix "${prefix}"
  assert_success "${CAPTURED_STATUS}"
  assert_file "${prefix}/bin/asc"
  assert_file "${prefix}/lib/asc/common.sh"
  capture_command "${prefix}/bin/asc" --version
  assert_success "${CAPTURED_STATUS}"
  assert_equal "asc 0.1.0" "${CAPTURED_OUTPUT}"
  capture_command "${PROJECT_ROOT}/install.sh" --prefix "${prefix}"
  assert_success "${CAPTURED_STATUS}" "installer should be idempotent"
  capture_command "${PROJECT_ROOT}/uninstall.sh" --prefix "${prefix}"
  assert_success "${CAPTURED_STATUS}"
  assert_not_exists "${prefix}/bin/asc"
}

test_installer_refuses_unrelated_file() {
  local case_directory
  case_directory=$(test_case_directory installer_refusal)
  local prefix="${case_directory}/prefix"
  mkdir -p "${prefix}/bin"
  printf 'unrelated\n' >"${prefix}/bin/asc"
  capture_command "${PROJECT_ROOT}/install.sh" --prefix "${prefix}"
  assert_failure "${CAPTURED_STATUS}"
  assert_equal "unrelated" "$(<"${prefix}/bin/asc")"
}

run_test "help and version" test_help_and_version
run_test "invalid command status" test_invalid_command_status
run_test "operational failure status" test_operational_failure_status
run_test "completion command" test_completion_command
run_test "doctor authentication boundaries" test_doctor_separates_api_and_ssh_authentication
run_test "install and uninstall" test_installer_and_uninstaller
run_test "installer refuses unrelated files" test_installer_refuses_unrelated_file
finish_tests
