#!/usr/bin/env bash
# asc-devtools managed file
set -o errexit
set -o nounset
set -o pipefail
# shellcheck disable=SC1091
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/test_helper.sh"

setup_vendor_case() {
  local directory="$1"
  local workspace="${directory}/workspace with spaces"
  initialize_git_repository "${workspace}/asc-cmake"
  initialize_git_repository "${workspace}/asc-cpp"
  mkdir -p -- "${workspace}/asc-cmake/modules"
  printf '1.2.3\n' >"${workspace}/asc-cmake/VERSION"
  printf 'Apache-2.0\n' >"${workspace}/asc-cmake/LICENSE"
  printf 'message(STATUS warnings)\n' >"${workspace}/asc-cmake/modules/ASCWarnings.cmake"
  git -C "${workspace}/asc-cmake" add -- VERSION LICENSE modules/ASCWarnings.cmake
  git -C "${workspace}/asc-cmake" commit -qm distribution
}

vendor_command() {
  local directory="$1"
  shift
  env -u ASC_CONFIG HOME="${directory}/home" PATH="${SYSTEM_PATH}" \
    ASC_WORKSPACE="${directory}/workspace with spaces" "${PROJECT_ROOT}/bin/asc" "$@"
}

test_vendor_lifecycle_and_safety() {
  local directory workspace target output
  directory=$(test_case_directory vendor_lifecycle)
  setup_vendor_case "${directory}"
  workspace="${directory}/workspace with spaces"
  target="${workspace}/asc-cpp/cmake/asc"

  output=$(vendor_command "${directory}" cmake vendor status asc-cpp --json)
  assert_contains "${output}" '"state":"not-vendored"'
  output=$(vendor_command "${directory}" cmake vendor plan asc-cpp --json)
  assert_contains "${output}" '"action":"add"'
  vendor_command "${directory}" cmake vendor apply asc-cpp --yes >/dev/null
  assert_file "${target}/ASC_CMAKE_MANIFEST.json"
  output=$(vendor_command "${directory}" cmake vendor status asc-cpp --json)
  assert_contains "${output}" '"state":"current"'

  printf 'unmanaged\n' >"${target}/notes.txt"
  output=$(vendor_command "${directory}" cmake vendor status asc-cpp --json)
  assert_contains "${output}" '"extraFiles":["notes.txt"]'
  printf 'local edit\n' >"${target}/modules/ASCWarnings.cmake"
  output=$(vendor_command "${directory}" cmake vendor status asc-cpp --json)
  assert_contains "${output}" '"state":"locally-modified"'
  if vendor_command "${directory}" cmake vendor plan asc-cpp >/dev/null 2>&1; then
    printf 'vendor plan replaced a locally modified file\n' >&2
    return 1
  fi
  printf 'message(STATUS warnings)\n' >"${target}/modules/ASCWarnings.cmake"

  printf 'message(STATUS newer)\n' >"${workspace}/asc-cmake/modules/ASCWarnings.cmake"
  git -C "${workspace}/asc-cmake" add -- modules/ASCWarnings.cmake
  git -C "${workspace}/asc-cmake" commit -qm update
  output=$(vendor_command "${directory}" cmake vendor status asc-cpp --json)
  assert_contains "${output}" '"state":"source-newer"'
  output=$(vendor_command "${directory}" cmake vendor plan asc-cpp --json)
  assert_contains "${output}" '"action":"replace"'
  vendor_command "${directory}" cmake vendor apply asc-cpp --yes >/dev/null
  assert_file "${target}/notes.txt"

  git -C "${workspace}/asc-cmake" rm -q -- modules/ASCWarnings.cmake
  git -C "${workspace}/asc-cmake" commit -qm remove
  output=$(vendor_command "${directory}" cmake vendor plan asc-cpp --json)
  assert_contains "${output}" '"action":"remove"'
  vendor_command "${directory}" cmake vendor apply asc-cpp --yes >/dev/null
  assert_not_exists "${target}/modules/ASCWarnings.cmake"
  assert_file "${target}/notes.txt"

  printf 'dirty\n' >"${workspace}/asc-cmake/VERSION"
  if vendor_command "${directory}" cmake vendor apply asc-cpp --yes >/dev/null 2>&1; then
    printf 'vendor apply accepted a dirty source\n' >&2
    return 1
  fi
  git -C "${workspace}/asc-cmake" restore -- VERSION
  mkdir -p -- "${workspace}/asc-cmake/modules"
  ln -s -- "${workspace}/asc-cmake/LICENSE" "${workspace}/asc-cmake/modules/linked.cmake"
  if vendor_command "${directory}" cmake vendor plan asc-cpp >/dev/null 2>&1; then
    printf 'vendor plan accepted a source symlink\n' >&2
    return 1
  fi
}

run_test "vendor lifecycle and safety" test_vendor_lifecycle_and_safety
finish_tests
