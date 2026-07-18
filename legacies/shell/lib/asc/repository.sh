#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

asc_repository_discover_local() (
  set -o errexit
  set -o nounset
  set -o pipefail
  shopt -s nullglob
  local repository_path
  local -a repositories=()
  [[ -d "${ASC_WORKSPACE}" ]] || return 0
  for repository_path in "${ASC_WORKSPACE}"/* "${ASC_WORKSPACE}"/.[!.]* "${ASC_WORKSPACE}"/..?*; do
    asc_is_managed_repository_name "${repository_path##*/}" || continue
    asc_is_git_repository "${repository_path}" || continue
    repositories+=("${repository_path##*/}")
  done
  if ((${#repositories[@]} > 0)); then
    printf '%s\n' "${repositories[@]}" | LC_ALL=C sort -u
  fi
)

asc_repository_clone() {
  local -a requested=("$@")
  local discovery_output
  local name
  local destination
  local clone_url
  local cloned=0
  local skipped=0
  local failed=0
  local -a discovered=()
  local -a selected=()
  local -A available=()

  if ! discovery_output=$(asc_github_discover); then
    return 1
  fi
  if [[ -n "${discovery_output}" ]]; then
    mapfile -t discovered <<<"${discovery_output}"
  fi
  for name in "${discovered[@]}"; do available["${name}"]=1; done

  if ((${#requested[@]} > 0)); then
    for name in "${requested[@]}"; do
      if ! asc_validate_repository_name "${name}"; then
        ((failed += 1))
        continue
      fi
      if [[ ! -v "available[${name}]" ]]; then
        printf 'FAIL %s: not found in %s\n' "${name}" "${ASC_ORGANIZATION}" >&2
        ((failed += 1))
        continue
      fi
      selected+=("${name}")
    done
  else
    selected=("${discovered[@]}")
  fi

  if ! mkdir -p -- "${ASC_WORKSPACE}"; then
    asc_error "could not create workspace: ${ASC_WORKSPACE}"
    return 1
  fi
  for name in "${selected[@]}"; do
    if ! asc_validate_repository_name "${name}"; then
      ((failed += 1))
      continue
    fi
    destination="${ASC_WORKSPACE}/${name}"
    if asc_is_git_repository "${destination}"; then
      printf 'SKIP %s: already cloned\n' "${name}"
      ((skipped += 1))
      continue
    fi
    if [[ -e "${destination}" || -L "${destination}" ]]; then
      printf 'FAIL %s: destination exists and is not a Git worktree\n' "${name}" >&2
      ((failed += 1))
      continue
    fi
    if [[ "${ASC_CLONE_PROTOCOL}" == "ssh" ]]; then
      clone_url="git@github.com:${ASC_ORGANIZATION}/${name}.git"
    else
      clone_url="https://github.com/${ASC_ORGANIZATION}/${name}.git"
    fi
    local -a command=(git clone -- "${clone_url}" "${destination}")
    if "${command[@]}"; then
      printf 'OK %s: cloned\n' "${name}"
      ((cloned += 1))
    else
      printf 'FAIL %s: clone failed\n' "${name}" >&2
      ((failed += 1))
    fi
  done
  printf 'Summary: cloned=%d skipped=%d failed=%d\n' "${cloned}" "${skipped}" "${failed}"
  ((failed == 0))
}

asc_repository_status_one() {
  local name="$1"
  local repository_path="${ASC_WORKSPACE}/${name}"
  local status_output
  local line
  local branch="unknown"
  local upstream="none"
  local ahead=0
  local behind=0
  local changed=0
  local -a changes=()
  local -a command=(git -C "${repository_path}" status --porcelain=v2 --branch)

  if ! status_output=$("${command[@]}"); then
    printf 'FAIL %s: git status failed\n' "${name}" >&2
    return 1
  fi
  while IFS= read -r line; do
    case "${line}" in
      '# branch.head (detached)') branch="detached HEAD" ;;
      '# branch.head '*) branch="${line#\# branch.head }" ;;
      '# branch.upstream '*) upstream="${line#\# branch.upstream }" ;;
      '# branch.ab '*)
        local counts="${line#\# branch.ab }"
        ahead="${counts%% *}"
        ahead="${ahead#+}"
        behind="${counts##* }"
        behind="${behind#-}"
        ;;
      '# '*) ;;
      '') ;;
      *)
        ((changed += 1))
        changes+=("${line}")
        ;;
    esac
  done <<<"${status_output}"
  local state="clean"
  ((changed == 0)) || state="dirty (${changed} changes)"
  printf 'OK %s: %s; %s; upstream=%s (+%s/-%s)\n' \
    "${name}" "${branch}" "${state}" "${upstream}" "${ahead}" "${behind}"
  for line in "${changes[@]}"; do printf '  %s\n' "${line}"; done
}

asc_repository_status() {
  local -a requested=("$@")
  local discovery_output=""
  local name
  local succeeded=0
  local failed=0
  local -a selected=()
  if ((${#requested[@]} == 0)); then
    discovery_output=$(asc_repository_discover_local)
    [[ -z "${discovery_output}" ]] || mapfile -t selected <<<"${discovery_output}"
  else
    selected=("${requested[@]}")
  fi
  for name in "${selected[@]}"; do
    if ! asc_validate_repository_name "${name}" ||
      ! asc_is_git_repository "${ASC_WORKSPACE}/${name}"; then
      printf 'FAIL %s: local Git worktree not found\n' "${name}" >&2
      ((failed += 1))
      continue
    fi
    if asc_repository_status_one "${name}"; then
      ((succeeded += 1))
    else
      ((failed += 1))
    fi
  done
  printf 'Summary: inspected=%d failed=%d\n' "${succeeded}" "${failed}"
  ((failed == 0))
}

asc_repository_sync_one() {
  local name="$1"
  local repository_path="${ASC_WORKSPACE}/${name}"
  local dirty
  local branch
  local upstream
  local before
  local after

  if ! dirty=$(git -C "${repository_path}" status --porcelain); then
    printf 'FAIL %s: could not inspect working tree\n' "${name}" >&2
    return 2
  fi
  if [[ -n "${dirty}" ]]; then
    printf 'SKIP %s: working tree is dirty\n' "${name}" >&2
    return 3
  fi
  if ! git -C "${repository_path}" remote get-url -- "${ASC_REMOTE}" >/dev/null 2>&1; then
    printf 'FAIL %s: remote %s is not configured\n' "${name}" "${ASC_REMOTE}" >&2
    return 2
  fi
  if ! branch=$(git -C "${repository_path}" symbolic-ref --quiet --short HEAD); then
    printf 'SKIP %s: detached HEAD cannot be synchronized safely\n' "${name}" >&2
    return 3
  fi
  if ! upstream=$(git -C "${repository_path}" rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null); then
    printf 'FAIL %s: branch %s has no upstream\n' "${name}" "${branch}" >&2
    return 2
  fi
  if [[ "${upstream%%/*}" != "${ASC_REMOTE}" ]]; then
    printf 'FAIL %s: upstream %s does not use configured remote %s\n' \
      "${name}" "${upstream}" "${ASC_REMOTE}" >&2
    return 2
  fi
  before=$(git -C "${repository_path}" rev-parse HEAD)
  local -a fetch_command=(git -C "${repository_path}" fetch -- "${ASC_REMOTE}")
  local -a merge_command=(git -C "${repository_path}" merge --ff-only "${upstream}")
  if ! "${fetch_command[@]}"; then
    printf 'FAIL %s: fetch failed\n' "${name}" >&2
    return 2
  fi
  if ! "${merge_command[@]}"; then
    printf 'FAIL %s: fast-forward refused\n' "${name}" >&2
    return 2
  fi
  after=$(git -C "${repository_path}" rev-parse HEAD)
  if [[ "${before}" == "${after}" ]]; then
    printf 'OK %s: unchanged\n' "${name}"
    return 1
  fi
  printf 'OK %s: updated\n' "${name}"
}

asc_repository_sync() {
  local -a requested=("$@")
  local discovery_output=""
  local name
  local result
  local updated=0
  local unchanged=0
  local skipped=0
  local failed=0
  local -a selected=()
  if ((${#requested[@]} == 0)); then
    discovery_output=$(asc_repository_discover_local)
    [[ -z "${discovery_output}" ]] || mapfile -t selected <<<"${discovery_output}"
  else
    selected=("${requested[@]}")
  fi
  for name in "${selected[@]}"; do
    if ! asc_validate_repository_name "${name}" ||
      ! asc_is_git_repository "${ASC_WORKSPACE}/${name}"; then
      printf 'FAIL %s: local Git worktree not found\n' "${name}" >&2
      ((failed += 1))
      continue
    fi
    if asc_repository_sync_one "${name}"; then
      ((updated += 1))
    else
      result=$?
      case "${result}" in
        1) ((unchanged += 1)) ;;
        3) ((skipped += 1)) ;;
        *) ((failed += 1)) ;;
      esac
    fi
  done
  printf 'Summary: updated=%d unchanged=%d skipped=%d failed=%d\n' \
    "${updated}" "${unchanged}" "${skipped}" "${failed}"
  ((failed == 0 && skipped == 0))
}
