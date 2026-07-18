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
if [[ "${1:-}" == "-C" ]]; then
  repository="$2"
  shift 2
  if [[ "${1:-}" == "rev-parse" && "${2:-}" == "--is-inside-work-tree" && -e "${repository}/.mock_git" ]]; then
    printf 'true\n'
    exit 0
  fi
  exit 1
fi
if [[ "${1:-}" == "clone" ]]; then
  printf '%s\n' "$*" >>"${MOCK_LOG}"
  url="$3"
  destination="$4"
  [[ "${url}" != *"asc-fail.git" ]] || exit 1
  mkdir -p -- "${destination}"
  : >"${destination}/.mock_git"
  exit 0
fi
exit 1
EOF
  chmod +x "${bin_directory}/git"
}

setup_clone_case() {
  local case_directory="$1"
  write_gh_mock "${case_directory}/bin"
  write_git_clone_mock "${case_directory}/bin"
  : >"${case_directory}/log"
}

test_repository_name_validation() {
  local case_directory
  case_directory=$(test_case_directory validation)
  local workspace="${case_directory}/workspace"
  initialize_git_repository "${workspace}/asc-good"
  capture_command env HOME="${case_directory}/home" PATH="${SYSTEM_PATH}" \
    ASC_WORKSPACE="${workspace}" "${PROJECT_ROOT}/bin/asc" repo status asc-good
  assert_success "${CAPTURED_STATUS}"
  local invalid
  for invalid in '../asc-bad' '/tmp/asc-bad' 'asc/bad' 'asc\bad' other; do
    capture_command env HOME="${case_directory}/home" PATH="${SYSTEM_PATH}" \
      ASC_WORKSPACE="${workspace}" "${PROJECT_ROOT}/bin/asc" repo status "${invalid}"
    assert_failure "${CAPTURED_STATUS}" "unsafe name should fail: ${invalid}"
  done
}

test_git_worktree_file_recognition() {
  local case_directory
  case_directory=$(test_case_directory linked_worktree)
  local workspace="${case_directory}/workspace"
  local primary="${case_directory}/primary"
  initialize_git_repository "${primary}"
  mkdir -p "${workspace}"
  git -C "${primary}" worktree add -qb linked "${workspace}/asc-linked"
  [[ -f "${workspace}/asc-linked/.git" ]]
  capture_command env HOME="${case_directory}/home" PATH="${SYSTEM_PATH}" \
    ASC_WORKSPACE="${workspace}" "${PROJECT_ROOT}/bin/asc" repo status
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "asc-linked"
}

test_clone_ssh_and_explicit_selection() {
  local case_directory
  case_directory=$(test_case_directory clone_ssh)
  setup_clone_case "${case_directory}"
  local workspace="${case_directory}/workspace with spaces"
  capture_command env HOME="${case_directory}/home" PATH="${case_directory}/bin:${SYSTEM_PATH}" \
    ASC_WORKSPACE="${workspace}" MOCK_LOG="${case_directory}/log" \
    MOCK_GH_REPOSITORIES=$'asc-two\tfalse\nasc-one\tfalse\n' \
    "${PROJECT_ROOT}/bin/asc" repo clone asc-one
  assert_success "${CAPTURED_STATUS}"
  assert_contains "$(<"${case_directory}/log")" "git@github.com:AI4SciComp/asc-one.git"
  assert_not_exists "${workspace}/asc-two"
  assert_directory "${workspace}/asc-one"
}

test_clone_https_url() {
  local case_directory
  case_directory=$(test_case_directory clone_https)
  setup_clone_case "${case_directory}"
  capture_command env HOME="${case_directory}/home" PATH="${case_directory}/bin:${SYSTEM_PATH}" \
    ASC_WORKSPACE="${case_directory}/workspace" ASC_CLONE_PROTOCOL=https \
    MOCK_LOG="${case_directory}/log" MOCK_GH_REPOSITORIES=$'asc-one\tfalse\n' \
    "${PROJECT_ROOT}/bin/asc" repo clone
  assert_success "${CAPTURED_STATUS}"
  assert_contains "$(<"${case_directory}/log")" "https://github.com/AI4SciComp/asc-one.git"
}

test_clone_skip_refusal_and_partial_failure() {
  local case_directory
  case_directory=$(test_case_directory clone_partial)
  setup_clone_case "${case_directory}"
  local workspace="${case_directory}/workspace"
  mkdir -p "${workspace}/asc-existing" "${workspace}/asc-data"
  : >"${workspace}/asc-existing/.mock_git"
  printf 'owned data\n' >"${workspace}/asc-data/file"
  capture_command env HOME="${case_directory}/home" PATH="${case_directory}/bin:${SYSTEM_PATH}" \
    ASC_WORKSPACE="${workspace}" MOCK_LOG="${case_directory}/log" \
    MOCK_GH_REPOSITORIES=$'asc-existing\tfalse\nasc-data\tfalse\nasc-ok\tfalse\nasc-fail\tfalse\n' \
    "${PROJECT_ROOT}/bin/asc" repo clone
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "SKIP asc-existing"
  assert_contains "${CAPTURED_OUTPUT}" "destination exists and is not a Git worktree"
  assert_contains "${CAPTURED_OUTPUT}" "failed=2"
  assert_equal "owned data" "$(<"${workspace}/asc-data/file")"
  assert_directory "${workspace}/asc-ok"
}

test_status_clean_dirty_detached_and_upstream() {
  local case_directory
  case_directory=$(test_case_directory status_states)
  local workspace="${case_directory}/workspace"
  initialize_git_repository "${workspace}/asc-one"
  capture_command env HOME="${case_directory}/home" PATH="${SYSTEM_PATH}" \
    ASC_WORKSPACE="${workspace}" "${PROJECT_ROOT}/bin/asc" repo status asc-one
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "clean; upstream=none"
  git init -q --bare "${case_directory}/status-origin.git"
  git -C "${workspace}/asc-one" remote add origin "${case_directory}/status-origin.git"
  git -C "${workspace}/asc-one" push -qu origin HEAD
  capture_command env HOME="${case_directory}/home" PATH="${SYSTEM_PATH}" \
    ASC_WORKSPACE="${workspace}" "${PROJECT_ROOT}/bin/asc" repo status asc-one
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "upstream=origin/"
  printf 'changed\n' >"${workspace}/asc-one/tracked.txt"
  capture_command env HOME="${case_directory}/home" PATH="${SYSTEM_PATH}" \
    ASC_WORKSPACE="${workspace}" "${PROJECT_ROOT}/bin/asc" repo status asc-one
  assert_contains "${CAPTURED_OUTPUT}" "dirty (1 changes)"
  git -C "${workspace}/asc-one" checkout -q --detach
  capture_command env HOME="${case_directory}/home" PATH="${SYSTEM_PATH}" \
    ASC_WORKSPACE="${workspace}" "${PROJECT_ROOT}/bin/asc" repo status asc-one
  assert_contains "${CAPTURED_OUTPUT}" "detached HEAD"
}

test_status_continues_after_failure() {
  local case_directory
  case_directory=$(test_case_directory status_continue)
  local workspace="${case_directory}/workspace"
  initialize_git_repository "${workspace}/asc-good"
  capture_command env HOME="${case_directory}/home" PATH="${SYSTEM_PATH}" \
    ASC_WORKSPACE="${workspace}" "${PROJECT_ROOT}/bin/asc" repo status asc-missing asc-good
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "FAIL asc-missing"
  assert_contains "${CAPTURED_OUTPUT}" "OK asc-good"
  assert_contains "${CAPTURED_OUTPUT}" "failed=1"
}

setup_sync_case() {
  local case_directory="$1"
  local name="$2"
  local bare="${case_directory}/${name}.git"
  local seed="${case_directory}/${name}-seed"
  local workspace="${case_directory}/workspace"
  git init -q --bare "${bare}"
  initialize_git_repository "${seed}"
  git -C "${seed}" remote add origin "${bare}"
  git -C "${seed}" push -qu origin HEAD
  mkdir -p "${workspace}"
  git clone -q "${bare}" "${workspace}/${name}"
  git -C "${workspace}/${name}" config user.name "Asc Tests"
  git -C "${workspace}/${name}" config user.email "asc-tests@example.invalid"
}

test_sync_fast_forward_and_dirty_skip() {
  local case_directory
  case_directory=$(test_case_directory sync_fast_forward)
  setup_sync_case "${case_directory}" asc-one
  printf 'remote update\n' >"${case_directory}/asc-one-seed/tracked.txt"
  git -C "${case_directory}/asc-one-seed" commit -qam update
  git -C "${case_directory}/asc-one-seed" push -q
  capture_command env HOME="${case_directory}/home" PATH="${SYSTEM_PATH}" \
    ASC_WORKSPACE="${case_directory}/workspace" "${PROJECT_ROOT}/bin/asc" repo sync asc-one
  assert_success "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "updated=1"
  assert_equal "remote update" "$(<"${case_directory}/workspace/asc-one/tracked.txt")"
  printf 'dirty\n' >"${case_directory}/workspace/asc-one/tracked.txt"
  capture_command env HOME="${case_directory}/home" PATH="${SYSTEM_PATH}" \
    ASC_WORKSPACE="${case_directory}/workspace" "${PROJECT_ROOT}/bin/asc" repo sync asc-one
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "working tree is dirty"
  assert_contains "${CAPTURED_OUTPUT}" "skipped=1"
}

test_sync_divergence_missing_remote_and_continuation() {
  local case_directory
  case_directory=$(test_case_directory sync_failures)
  setup_sync_case "${case_directory}" asc-diverged
  printf 'local\n' >"${case_directory}/workspace/asc-diverged/local.txt"
  git -C "${case_directory}/workspace/asc-diverged" add local.txt
  git -C "${case_directory}/workspace/asc-diverged" commit -qm local
  printf 'remote\n' >"${case_directory}/asc-diverged-seed/remote.txt"
  git -C "${case_directory}/asc-diverged-seed" add remote.txt
  git -C "${case_directory}/asc-diverged-seed" commit -qm remote
  git -C "${case_directory}/asc-diverged-seed" push -q
  initialize_git_repository "${case_directory}/workspace/asc-no-remote"
  capture_command env HOME="${case_directory}/home" PATH="${SYSTEM_PATH}" \
    ASC_WORKSPACE="${case_directory}/workspace" "${PROJECT_ROOT}/bin/asc" \
    repo sync asc-diverged asc-no-remote asc-missing
  assert_failure "${CAPTURED_STATUS}"
  assert_contains "${CAPTURED_OUTPUT}" "fast-forward refused"
  assert_contains "${CAPTURED_OUTPUT}" "remote origin is not configured"
  assert_contains "${CAPTURED_OUTPUT}" "local Git worktree not found"
  assert_contains "${CAPTURED_OUTPUT}" "failed=3"
}

run_test "repository name validation" test_repository_name_validation
run_test "linked worktree recognition" test_git_worktree_file_recognition
run_test "SSH clone and explicit selection" test_clone_ssh_and_explicit_selection
run_test "HTTPS clone URL" test_clone_https_url
run_test "clone skip, refusal, and partial failure" test_clone_skip_refusal_and_partial_failure
run_test "status states" test_status_clean_dirty_detached_and_upstream
run_test "status continues after failure" test_status_continues_after_failure
run_test "sync fast-forward and dirty refusal" test_sync_fast_forward_and_dirty_skip
run_test "sync failures and continuation" test_sync_divergence_missing_remote_and_continuation
finish_tests
