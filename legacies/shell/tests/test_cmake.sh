#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/test_helper.sh"

write_cmake_mocks() {
  local bin_directory="$1"
  cat >"${bin_directory}/cmake" <<'EOF'
#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail
if [[ "${1:-}" == --list-presets=* ]]; then
  printf '  "dev"\n  "release"\n'
  exit 0
fi
printf '%s|cmake %s\n' "${PWD}" "$*" >>"${MOCK_LOG}"
exit "${MOCK_CMAKE_STATUS:-0}"
EOF
  cat >"${bin_directory}/ctest" <<'EOF'
#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail
printf '%s|ctest %s\n' "${PWD}" "$*" >>"${MOCK_LOG}"
exit "${MOCK_CTEST_STATUS:-0}"
EOF
  chmod +x "${bin_directory}/cmake" "${bin_directory}/ctest"
}

setup_cmake_case() {
  local case_directory="$1"
  initialize_git_repository "${case_directory}/workspace/asc-cpp"
  printf '{"version": 6}\n' >"${case_directory}/workspace/asc-cpp/CMakePresets.json"
  write_cmake_mocks "${case_directory}/bin"
  : >"${case_directory}/log"
}

test_commands_and_working_directory() {
  local case_directory
  case_directory=$(test_case_directory cmake_commands)
  setup_cmake_case "${case_directory}"
  local base_path="${case_directory}/bin:${SYSTEM_PATH}"
  capture_command env HOME="${case_directory}/home" PATH="${base_path}" \
    ASC_WORKSPACE="${case_directory}/workspace" MOCK_LOG="${case_directory}/log" \
    "${PROJECT_ROOT}/bin/asc" configure asc-cpp
  assert_success "${CAPTURED_STATUS}"
  capture_command env HOME="${case_directory}/home" PATH="${base_path}" \
    ASC_WORKSPACE="${case_directory}/workspace" MOCK_LOG="${case_directory}/log" \
    "${PROJECT_ROOT}/bin/asc" build asc-cpp --preset release
  assert_success "${CAPTURED_STATUS}"
  capture_command env HOME="${case_directory}/home" PATH="${base_path}" \
    ASC_WORKSPACE="${case_directory}/workspace" MOCK_LOG="${case_directory}/log" \
    "${PROJECT_ROOT}/bin/asc" test asc-cpp
  assert_success "${CAPTURED_STATUS}"
  local log
  log=$(<"${case_directory}/log")
  assert_contains "${log}" "workspace/asc-cpp|cmake --preset dev"
  assert_contains "${log}" "cmake --build --preset release --parallel"
  assert_contains "${log}" "ctest --preset dev --output-on-failure"
}

test_missing_file_and_preset() {
  local case_directory
  case_directory=$(test_case_directory cmake_missing)
  setup_cmake_case "${case_directory}"
  rm -- "${case_directory}/workspace/asc-cpp/CMakePresets.json"
  capture_command env HOME="${case_directory}/home" PATH="${case_directory}/bin:${SYSTEM_PATH}" \
    ASC_WORKSPACE="${case_directory}/workspace" MOCK_LOG="${case_directory}/log" \
    "${PROJECT_ROOT}/bin/asc" build asc-cpp
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "CMakePresets.json not found"
  printf '{"version": 6}\n' >"${case_directory}/workspace/asc-cpp/CMakePresets.json"
  capture_command env HOME="${case_directory}/home" PATH="${case_directory}/bin:${SYSTEM_PATH}" \
    ASC_WORKSPACE="${case_directory}/workspace" MOCK_LOG="${case_directory}/log" \
    "${PROJECT_ROOT}/bin/asc" build asc-cpp --preset missing
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "preset 'missing' not found"
}

test_missing_repository() {
  local case_directory
  case_directory=$(test_case_directory cmake_missing_repository)
  write_cmake_mocks "${case_directory}/bin"
  : >"${case_directory}/log"
  capture_command env HOME="${case_directory}/home" PATH="${case_directory}/bin:${SYSTEM_PATH}" \
    ASC_WORKSPACE="${case_directory}/workspace" MOCK_LOG="${case_directory}/log" \
    "${PROJECT_ROOT}/bin/asc" configure asc-missing
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "local Git worktree not found"
}

test_failure_propagation() {
  local case_directory
  case_directory=$(test_case_directory cmake_failure)
  setup_cmake_case "${case_directory}"
  capture_command env HOME="${case_directory}/home" PATH="${case_directory}/bin:${SYSTEM_PATH}" \
    ASC_WORKSPACE="${case_directory}/workspace" MOCK_LOG="${case_directory}/log" \
    MOCK_CMAKE_STATUS=7 "${PROJECT_ROOT}/bin/asc" build asc-cpp
  assert_equal 7 "${CAPTURED_STATUS}"
}

run_test "CMake commands and working directory" test_commands_and_working_directory
run_test "missing presets" test_missing_file_and_preset
run_test "missing CMake repository" test_missing_repository
run_test "CMake failure propagation" test_failure_propagation
finish_tests
