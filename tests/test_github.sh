#!/usr/bin/env bash
# asc-devtools managed file
set -o errexit
set -o nounset
set -o pipefail
# shellcheck disable=SC1091
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/test_helper.sh"

test_filter_sort_and_json_schema() {
  local directory body
  directory=$(test_case_directory github_filter)
  write_curl_mock "${directory}/bin"
  : >"${directory}/log"
  body="[$(github_repository_json asc-z),$(github_repository_json other),$(github_repository_json asc-old true),$(github_repository_json .github),$(github_repository_json asc-a)]"
  capture_command env HOME="${directory}/home" PATH="${directory}/bin:${SYSTEM_PATH}" \
    ASC_GITHUB_TOKEN=test MOCK_LOG="${directory}/log" MOCK_CURL_BODY="${body}" \
    "${PROJECT_ROOT}/bin/asc" repo list --json
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" '"name":".github"'
  assert_contains "${CAPTURED_OUTPUT}" '"clone_url":"https://github.com/AI4SciComp/asc-a.git"'
  assert_contains "${CAPTURED_OUTPUT}" '"private":false'
  assert_contains "$(<"${directory}/log")" "Authorization: Bearer test"
  assert_contains "$(<"${directory}/log")" "per_page=100&page=1"
}

test_dot_github_environment_exclusion() {
  local directory body
  directory=$(test_case_directory github_exclusion)
  write_curl_mock "${directory}/bin"
  : >"${directory}/log"
  body="[$(github_repository_json .github),$(github_repository_json asc-one)]"
  capture_command env HOME="${directory}/home" PATH="${directory}/bin:${SYSTEM_PATH}" \
    ASC_INCLUDE_DOT_GITHUB=false ASC_GITHUB_TOKEN=test MOCK_LOG="${directory}/log" \
    MOCK_CURL_BODY="${body}" "${PROJECT_ROOT}/bin/asc" repo list
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "asc-one"
  [[ "${CAPTURED_OUTPUT}" != *".github"* ]]
}

test_known_error_hides_body() {
  local directory
  directory=$(test_case_directory github_error)
  write_curl_mock "${directory}/bin"
  : >"${directory}/log"
  capture_command env HOME="${directory}/home" PATH="${directory}/bin:${SYSTEM_PATH}" \
    ASC_GITHUB_TOKEN=test MOCK_LOG="${directory}/log" MOCK_CURL_STATUS=401 \
    MOCK_CURL_BODY="secret body" "${PROJECT_ROOT}/bin/asc" repo list
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "authentication failed"
  [[ "${CAPTURED_OUTPUT}" != *"secret body"* ]]
}

run_test "REST filtering and JSON schema" test_filter_sort_and_json_schema
run_test ".github environment exclusion" test_dot_github_environment_exclusion
run_test "known API error redaction" test_known_error_hides_body
finish_tests
