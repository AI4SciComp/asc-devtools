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
if [[ "${1:-}" == --list-presets=* ]]; then printf '  "dev"\n'; exit 0; fi
printf '%s|cmake %s\n' "${PWD}" "$*" >>"${MOCK_LOG}"
exit "${MOCK_CMAKE_STATUS:-0}"
EOF
  cat >"${bin_directory}/ctest" <<'EOF'
#!/usr/bin/env bash
printf '%s|ctest %s\n' "${PWD}" "$*" >>"${MOCK_LOG}"
exit "${MOCK_CTEST_STATUS:-0}"
EOF
  chmod +x "${bin_directory}/cmake" "${bin_directory}/ctest"
}

setup_case() {
  local directory="$1"
  initialize_git_repository "${directory}/workspace/asc-cpp"
  printf '{}\n' >"${directory}/workspace/asc-cpp/CMakePresets.json"
  write_cmake_mocks "${directory}/bin"
  : >"${directory}/log"
}

test_exact_commands_and_required_preset() {
  local directory log
  directory=$(test_case_directory cmake_exact)
  setup_case "${directory}"
  for operation in configure build test; do
    capture_command env HOME="${directory}/home" PATH="${directory}/bin:${SYSTEM_PATH}" \
      ASC_WORKSPACE="${directory}/workspace" MOCK_LOG="${directory}/log" \
      "${PROJECT_ROOT}/bin/asc" "${operation}" asc-cpp --preset dev
    assert_success "${CAPTURED_STATUS}"
  done
  log=$(<"${directory}/log")
  assert_contains "${log}" "cmake --preset dev"
  assert_contains "${log}" "cmake --build --preset dev"
  assert_contains "${log}" "ctest --preset dev"
  [[ "${log}" != *"--parallel"* ]]
  [[ "${log}" != *"--output-on-failure"* ]]
  capture_command env HOME="${directory}/home" PATH="${directory}/bin:${SYSTEM_PATH}" \
    ASC_WORKSPACE="${directory}/workspace" "${PROJECT_ROOT}/bin/asc" build asc-cpp
  assert_equal 2 "${CAPTURED_STATUS}"
}

test_external_exit_propagation() {
  local directory
  directory=$(test_case_directory cmake_exit)
  setup_case "${directory}"
  capture_command env HOME="${directory}/home" PATH="${directory}/bin:${SYSTEM_PATH}" \
    ASC_WORKSPACE="${directory}/workspace" MOCK_LOG="${directory}/log" MOCK_CMAKE_STATUS=7 \
    "${PROJECT_ROOT}/bin/asc" build asc-cpp --preset dev
  assert_equal 7 "${CAPTURED_STATUS}"
}

run_test "exact commands and required preset" test_exact_commands_and_required_preset
run_test "external exit propagation" test_external_exit_propagation
finish_tests
