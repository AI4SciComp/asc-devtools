#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

TESTS_DIRECTORY=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
readonly TESTS_DIRECTORY
# These globals form the interface exported to each sourced test suite.
# shellcheck disable=SC2034
PROJECT_ROOT=$(cd -- "${TESTS_DIRECTORY}/.." && pwd)
# shellcheck disable=SC2034
readonly PROJECT_ROOT
TEST_TEMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/asc-devtools-test.XXXXXXXX")
readonly TEST_TEMP_ROOT
# shellcheck disable=SC2034
readonly SYSTEM_PATH="${PATH}"

TESTS_RUN=0
TESTS_FAILED=0
# shellcheck disable=SC2034
CAPTURED_OUTPUT=""
# shellcheck disable=SC2034
CAPTURED_STATUS=0

test_cleanup() {
  if [[ "${TEST_TEMP_ROOT}" == "${TMPDIR:-/tmp}"/asc-devtools-test.* &&
    -d "${TEST_TEMP_ROOT}" ]]; then
    chmod -R u+w -- "${TEST_TEMP_ROOT}" 2>/dev/null || true
    rm -r -- "${TEST_TEMP_ROOT}"
  fi
}
trap test_cleanup EXIT

test_case_directory() {
  local name="$1"
  local directory="${TEST_TEMP_ROOT}/${name}"
  mkdir -p -- "${directory}/home" "${directory}/bin"
  printf '%s\n' "${directory}"
}

capture_command() {
  # Results are consumed by the sourcing suite.
  # shellcheck disable=SC2034
  if CAPTURED_OUTPUT=$("$@" 2>&1); then
    CAPTURED_STATUS=0
  else
    CAPTURED_STATUS=$?
  fi
}

assert_equal() {
  local expected="$1"
  local actual="$2"
  local message="${3:-values differ}"
  if [[ "${expected}" != "${actual}" ]]; then
    printf '    %s: expected <%s>, got <%s>\n' "${message}" "${expected}" "${actual}" >&2
    exit 1
  fi
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local message="${3:-output did not contain expected text}"
  if [[ "${haystack}" != *"${needle}"* ]]; then
    printf '    %s: missing <%s> in <%s>\n' "${message}" "${needle}" "${haystack}" >&2
    exit 1
  fi
}

assert_success() {
  local status="$1"
  local message="${2:-command should succeed}"
  if ((status != 0)); then
    printf '    %s: status %d\n' "${message}" "${status}" >&2
    exit 1
  fi
}

assert_failure() {
  local status="$1"
  local message="${2:-command should fail}"
  if ((status == 0)); then
    printf '    %s: status was zero\n' "${message}" >&2
    exit 1
  fi
}

assert_file() {
  local path="$1"
  [[ -f "${path}" ]] || {
    printf '    expected file: %s\n' "${path}" >&2
    exit 1
  }
}

assert_directory() {
  local path="$1"
  [[ -d "${path}" ]] || {
    printf '    expected directory: %s\n' "${path}" >&2
    exit 1
  }
}

assert_not_exists() {
  local path="$1"
  [[ ! -e "${path}" ]] || {
    printf '    path should not exist: %s\n' "${path}" >&2
    exit 1
  }
}

run_test() {
  local name="$1"
  local function_name="$2"
  ((TESTS_RUN += 1))
  if ("${function_name}"); then
    printf 'PASS %s\n' "${name}"
  else
    printf 'FAIL %s\n' "${name}" >&2
    ((TESTS_FAILED += 1))
  fi
}

finish_tests() {
  printf 'Tests: %d, failed: %d\n' "${TESTS_RUN}" "${TESTS_FAILED}"
  ((TESTS_FAILED == 0))
}

initialize_git_repository() {
  local repository="$1"
  mkdir -p -- "${repository}"
  git -C "${repository}" init -q
  git -C "${repository}" config user.name "Asc Tests"
  git -C "${repository}" config user.email "asc-tests@example.invalid"
  printf 'initial\n' >"${repository}/tracked.txt"
  git -C "${repository}" add -- tracked.txt
  git -C "${repository}" commit -qm initial
}

write_gh_mock() {
  local bin_directory="$1"
  cat >"${bin_directory}/gh" <<'EOF'
#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail
printf '%s\n' "$*" >>"${MOCK_LOG}"
case "${1:-}" in
  repo)
    [[ "${MOCK_GH_FAIL:-false}" != "true" ]] || exit 1
    printf '%b' "${MOCK_GH_REPOSITORIES:-}"
    ;;
  api) printf '%s\n' "${MOCK_GH_USER:-escapetiger}" ;;
  config) printf '%s\n' "${MOCK_GH_PROTOCOL:-ssh}" ;;
  --version) printf 'gh version test\n' ;;
esac
EOF
  chmod +x "${bin_directory}/gh"
}

write_curl_mock() {
  local bin_directory="$1"
  cat >"${bin_directory}/curl" <<'EOF'
#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail
if [[ "${1:-}" == "--version" ]]; then
  printf 'curl test\n'
  exit 0
fi
headers=""
body=""
url=""
while (($# > 0)); do
  case "$1" in
    --dump-header) headers="$2"; shift 2 ;;
    --output) body="$2"; shift 2 ;;
    --header) printf 'header:%s\n' "$2" >>"${MOCK_LOG}"; shift 2 ;;
    --write-out) shift 2 ;;
    --max-time) shift 2 ;;
    --silent | --show-error) shift ;;
    http*) url="$1"; shift ;;
    *) shift ;;
  esac
done
printf 'url:%s\n' "${url}" >>"${MOCK_LOG}"
printf '%b' "${MOCK_CURL_HEADERS:-}" >"${headers}"
if [[ "${MOCK_CURL_ROUTED:-false}" == true ]]; then
  case "${url}" in
    */releases/latest) cp -- "${MOCK_CURL_RELEASE_FILE}" "${body}" ;;
    */SHA256SUMS) cp -- "${MOCK_CURL_SUMS_FILE}" "${body}" ;;
    */asc-devtools-shell.tar.gz) cp -- "${MOCK_CURL_ARCHIVE_FILE}" "${body}" ;;
    *) printf '[]' >"${body}" ;;
  esac
else
  printf '%s' "${MOCK_CURL_BODY:-[]}" >"${body}"
fi
printf '%s' "${MOCK_CURL_STATUS:-200}"
exit "${MOCK_CURL_EXIT:-0}"
EOF
  chmod +x "${bin_directory}/curl"
}

github_repository_json() {
  local name="$1"
  local archived="${2:-false}"
  local private="${3:-false}"
  printf '{"name":"%s","archived":%s,"fork":false,"clone_url":"https://github.com/AI4SciComp/%s.git","ssh_url":"git@github.com:AI4SciComp/%s.git","default_branch":"main","private":%s}' \
    "${name}" "${archived}" "${name}" "${name}" "${private}"
}
