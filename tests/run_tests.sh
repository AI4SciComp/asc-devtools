#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

TESTS_DIRECTORY=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
readonly TESTS_DIRECTORY
readonly SYSTEM_PATH="${PATH}"
local_failures=0
local_suites=0

for test_script in \
  "${TESTS_DIRECTORY}/test_config.sh" \
  "${TESTS_DIRECTORY}/test_github.sh" \
  "${TESTS_DIRECTORY}/test_repository.sh" \
  "${TESTS_DIRECTORY}/test_cmake.sh" \
  "${TESTS_DIRECTORY}/test_vendor.sh" \
  "${TESTS_DIRECTORY}/test_update.sh" \
  "${TESTS_DIRECTORY}/test_cli.sh"; do
  ((local_suites += 1))
  printf '\n== %s ==\n' "${test_script##*/}"
  if ! env -u ASC_CONFIG -u ASC_ORGANIZATION -u ASC_WORKSPACE \
    -u ASC_REPOSITORY_PREFIX -u ASC_INCLUDE_DOT_GITHUB \
    -u ASC_CLONE_PROTOCOL -u ASC_REMOTE PATH="${SYSTEM_PATH}" bash "${test_script}"; then
    ((local_failures += 1))
  fi
done

printf '\nSuites: %d, failed: %d\n' "${local_suites}" "${local_failures}"
((local_failures == 0))
