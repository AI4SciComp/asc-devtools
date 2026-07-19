#!/usr/bin/env bash
# asc-devtools managed file
set -o errexit
set -o nounset
set -o pipefail
# shellcheck disable=SC1091
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/test_helper.sh"

write_git_clone_mock() {
  local bin_directory="$1"
  cat >"${bin_directory}/git" <<'EOF'
#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail
if [[ "${1:-}" == clone ]]; then
  printf '%s\n' "$*" >>"${MOCK_LOG}"
  mkdir -p -- "$4"
  exit 0
fi
exec /usr/bin/git "$@"
EOF
  chmod +x "${bin_directory}/git"
}

test_status_json_and_partial_failure() {
  local directory workspace
  directory=$(test_case_directory status_json)
  workspace="${directory}/workspace"
  initialize_git_repository "${workspace}/asc-good"
  printf 'new\n' >"${workspace}/asc-good/untracked.txt"
  capture_command env HOME="${directory}/home" PATH="${SYSTEM_PATH}" ASC_WORKSPACE="${workspace}" \
    "${PROJECT_ROOT}/bin/asc" repo status asc-missing asc-good --json
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" '"name":"asc-missing"'
  assert_contains "${CAPTURED_OUTPUT}" '"error":"local Git working tree not found'
  assert_contains "${CAPTURED_OUTPUT}" '"clean":false'
  assert_contains "${CAPTURED_OUTPUT}" '"changes":1'
}

test_clone_uses_api_url_and_absolute_destination() {
  local directory workspace body
  directory=$(test_case_directory clone_api)
  workspace="${directory}/workspace with spaces"
  write_curl_mock "${directory}/bin"
  write_git_clone_mock "${directory}/bin"
  : >"${directory}/log"
  body="[$(github_repository_json asc-one)]"
  capture_command env HOME="${directory}/home" PATH="${directory}/bin:${SYSTEM_PATH}" \
    ASC_WORKSPACE="${workspace}" ASC_GITHUB_TOKEN=test MOCK_LOG="${directory}/log" \
    MOCK_CURL_BODY="${body}" "${PROJECT_ROOT}/bin/asc" repo clone asc-one --protocol ssh
  assert_success "${CAPTURED_STATUS}"
  assert_contains "$(<"${directory}/log")" "git@github.com:AI4SciComp/asc-one.git ${workspace}/asc-one"
  assert_directory "${workspace}/asc-one"
}

setup_sync_case() {
  local directory="$1"
  local bare="${directory}/origin.git"
  local seed="${directory}/seed"
  local repository="${directory}/workspace/asc-one"
  git init -q --bare "${bare}"
  initialize_git_repository "${seed}"
  git -C "${seed}" remote add origin "${bare}"
  git -C "${seed}" push -qu origin HEAD
  mkdir -p "${directory}/workspace"
  git clone -q "${bare}" "${repository}"
  git -C "${repository}" config user.name "Asc Tests"
  git -C "${repository}" config user.email "asc-tests@example.invalid"
}

test_sync_dry_run_and_dirty_refusal() {
  local directory workspace before after
  directory=$(test_case_directory sync_dry_run)
  setup_sync_case "${directory}"
  workspace="${directory}/workspace"
  before=$(git -C "${workspace}/asc-one" rev-parse HEAD)
  capture_command env HOME="${directory}/home" PATH="${SYSTEM_PATH}" ASC_WORKSPACE="${workspace}" \
    "${PROJECT_ROOT}/bin/asc" repo sync asc-one --dry-run
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "asc-one: planned"
  assert_contains "${CAPTURED_OUTPUT}" "fetch -- origin"
  assert_contains "${CAPTURED_OUTPUT}" "merge --ff-only"
  after=$(git -C "${workspace}/asc-one" rev-parse HEAD)
  assert_equal "${before}" "${after}"
  printf 'dirty\n' >"${workspace}/asc-one/dirty.txt"
  capture_command env HOME="${directory}/home" PATH="${SYSTEM_PATH}" ASC_WORKSPACE="${workspace}" \
    "${PROJECT_ROOT}/bin/asc" repo sync asc-one
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "working tree is dirty"
}

test_save_commits_pushes_and_refuses_remote_ahead() {
  local directory workspace repository seed bare before remote_subject local_subject
  directory=$(test_case_directory repo_save)
  setup_sync_case "${directory}"
  workspace="${directory}/workspace"
  repository="${workspace}/asc-one"
  seed="${directory}/seed"
  bare="${directory}/origin.git"
  printf 'saved\n' >"${repository}/saved.txt"
  before=$(git -C "${bare}" rev-parse HEAD)
  capture_command env HOME="${directory}/home" PATH="${SYSTEM_PATH}" ASC_WORKSPACE="${workspace}" \
    "${PROJECT_ROOT}/bin/asc" repo save asc-one --message 'Save local work' --dry-run
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "asc-one: planned"
  assert_contains "${CAPTURED_OUTPUT}" "git -C"
  assert_contains "${CAPTURED_OUTPUT}" "commit -m Save\\ local\\ work"
  assert_equal "${before}" "$(git -C "${bare}" rev-parse HEAD)"

  capture_command env HOME="${directory}/home" PATH="${SYSTEM_PATH}" ASC_WORKSPACE="${workspace}" \
    "${PROJECT_ROOT}/bin/asc" repo save asc-one --dry-run
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "commit -m Updated\\ at\\ "

  capture_command env HOME="${directory}/home" PATH="${SYSTEM_PATH}" ASC_WORKSPACE="${workspace}" \
    "${PROJECT_ROOT}/bin/asc" repo save asc-one --yes
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "committed and pushed"
  remote_subject=$(git -C "${bare}" log -1 --format=%s)
  [[ "${remote_subject}" =~ ^Updated\ at\ [0-9]{4}-[0-9]{2}-[0-9]{2}\ [0-9]{2}:[0-9]{2}:[0-9]{2}$ ]]

  git -C "${seed}" pull -q --ff-only
  printf 'remote\n' >"${seed}/remote.txt"
  git -C "${seed}" add remote.txt
  git -C "${seed}" commit -qm 'remote update'
  git -C "${seed}" push -q
  printf 'local\n' >"${repository}/not-saved.txt"
  capture_command env HOME="${directory}/home" PATH="${SYSTEM_PATH}" ASC_WORKSPACE="${workspace}" \
    "${PROJECT_ROOT}/bin/asc" repo save asc-one --message 'Must not commit' --yes
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "reconcile them manually"
  local_subject=$(git -C "${repository}" log -1 --format=%s)
  [[ "${local_subject}" != 'Must not commit' ]]
}

test_symlink_is_not_discovered() {
  local directory workspace
  directory=$(test_case_directory symlink)
  workspace="${directory}/workspace"
  initialize_git_repository "${directory}/outside"
  mkdir -p "${workspace}"
  ln -s "${directory}/outside" "${workspace}/asc-link"
  capture_command env HOME="${directory}/home" PATH="${SYSTEM_PATH}" ASC_WORKSPACE="${workspace}" \
    "${PROJECT_ROOT}/bin/asc" repo status --json
  assert_success "${CAPTURED_STATUS}"
  assert_equal $'[\n\n]' "${CAPTURED_OUTPUT}"
}

run_test "status JSON and partial failure" test_status_json_and_partial_failure
run_test "clone API URL and absolute destination" test_clone_uses_api_url_and_absolute_destination
run_test "sync dry-run and dirty refusal" test_sync_dry_run_and_dirty_refusal
run_test "save commit/push and remote-ahead refusal" test_save_commits_pushes_and_refuses_remote_ahead
run_test "symlink exclusion" test_symlink_is_not_discovered
finish_tests
