#!/usr/bin/env bash
# asc-devtools managed file
set -o errexit
set -o nounset
set -o pipefail
# shellcheck disable=SC1091
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/test_helper.sh"

test_check_and_verified_update() {
  local directory assets prefix release repack
  directory=$(test_case_directory verified_update)
  assets="${directory}/assets"
  prefix="${directory}/prefix"
  write_curl_mock "${directory}/bin"
  : >"${directory}/log"
  "${PROJECT_ROOT}/scripts/package_update.sh" --output "${assets}" >/dev/null
  repack="${directory}/repack"
  mkdir -p -- "${repack}"
  tar -xzf "${assets}/asc-devtools-shell.tar.gz" -C "${repack}"
  sed -i 's/readonly ASC_VERSION="0.1.0"/readonly ASC_VERSION="0.2.0"/' \
    "${repack}/asc-devtools-shell/lib/asc/common.sh"
  tar -C "${repack}" -czf "${assets}/asc-devtools-shell.tar.gz" asc-devtools-shell
  (cd -- "${assets}" && sha256sum -- asc-devtools-shell.tar.gz >SHA256SUMS)
  release="${directory}/release.json"
  printf '{"tag_name":"v0.2.0","assets":[{"name":"asc-devtools-shell.tar.gz","url":"https://test/asc-devtools-shell.tar.gz"},{"name":"SHA256SUMS","url":"https://test/SHA256SUMS"}]}' >"${release}"

  capture_command env HOME="${directory}/home" PATH="${directory}/bin:${SYSTEM_PATH}" \
    MOCK_LOG="${directory}/log" MOCK_CURL_ROUTED=true \
    MOCK_CURL_RELEASE_FILE="${release}" \
    MOCK_CURL_ARCHIVE_FILE="${assets}/asc-devtools-shell.tar.gz" \
    MOCK_CURL_SUMS_FILE="${assets}/SHA256SUMS" \
    "${PROJECT_ROOT}/bin/asc" update --check
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "0.2.0 is available"

  "${PROJECT_ROOT}/scripts/install.sh" --prefix "${prefix}" >/dev/null
  capture_command env HOME="${directory}/home" PATH="${directory}/bin:${SYSTEM_PATH}" \
    MOCK_LOG="${directory}/log" MOCK_CURL_ROUTED=true \
    MOCK_CURL_RELEASE_FILE="${release}" \
    MOCK_CURL_ARCHIVE_FILE="${assets}/asc-devtools-shell.tar.gz" \
    MOCK_CURL_SUMS_FILE="${assets}/SHA256SUMS" \
    "${PROJECT_ROOT}/bin/asc" update --yes --prefix "${prefix}"
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "Updated asc to 0.2.0"
  assert_file "${prefix}/lib/asc/update.sh"
}

test_missing_release_and_unsafe_archive() {
  local directory archive
  directory=$(test_case_directory unsafe_update)
  write_curl_mock "${directory}/bin"
  : >"${directory}/log"
  capture_command env HOME="${directory}/home" PATH="${directory}/bin:${SYSTEM_PATH}" \
    MOCK_LOG="${directory}/log" MOCK_CURL_STATUS=404 MOCK_CURL_BODY='' \
    "${PROJECT_ROOT}/bin/asc" update --check
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "no published asc release"

  mkdir -p -- "${directory}/bad/${ASC_UPDATE_ROOT:-asc-devtools-shell}"
  ln -s -- /tmp "${directory}/bad/asc-devtools-shell/link"
  archive="${directory}/unsafe.tar.gz"
  tar -C "${directory}/bad" -czf "${archive}" asc-devtools-shell
  # shellcheck disable=SC1091
  source "${PROJECT_ROOT}/lib/asc/common.sh"
  # shellcheck disable=SC1091
  source "${PROJECT_ROOT}/lib/asc/update.sh"
  capture_command asc_update_extract "${archive}" "${directory}/extract"
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "link or special file"
}

run_test "check and verified update" test_check_and_verified_update
run_test "missing release and unsafe archive" test_missing_release_and_unsafe_archive
finish_tests
