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
  local path
  local -a names=()
  [[ -d "${ASC_WORKSPACE}" ]] || return 0
  for path in "${ASC_WORKSPACE}"/* "${ASC_WORKSPACE}"/.[!.]* "${ASC_WORKSPACE}"/..?*; do
    asc_is_managed_repository_name "${path##*/}" || continue
    asc_is_git_repository "${path}" || continue
    names+=("${path##*/}")
  done
  ((${#names[@]} == 0)) || printf '%s\n' "${names[@]}" | LC_ALL=C sort -u
)

asc_repository_clone() {
  local protocol="$1"
  shift
  local -a requested=("$@")
  local discovery record name _archived _fork clone_url ssh_url _default_branch _private
  local destination actual selected_url
  local cloned=0 already_present=0 failed=0
  local -a names=()
  local -A available=() https_urls=() ssh_urls=()
  if ! discovery=$(asc_github_discover); then return 1; fi
  while IFS= read -r record; do
    [[ -n "${record}" ]] || continue
    IFS=$'\t' read -r name _archived _fork clone_url ssh_url _default_branch _private <<<"${record}"
    available["${name}"]=1
    https_urls["${name}"]="${clone_url}"
    ssh_urls["${name}"]="${ssh_url}"
    ((${#requested[@]} > 0)) || names+=("${name}")
  done <<<"${discovery}"
  ((${#requested[@]} == 0)) || names=("${requested[@]}")
  mkdir -p -- "${ASC_WORKSPACE}" || {
    printf 'workspace: failed: could not create %s\n' "${ASC_WORKSPACE}"
    return 1
  }
  for name in "${names[@]}"; do
    if ! asc_validate_repository_name "${name}"; then
      printf '%s: failed: invalid or unmanaged repository name\n' "${name}"
      ((failed += 1))
      continue
    fi
    if [[ ! -v "available[${name}]" ]]; then
      printf '%s: failed: repository was not returned by the organization API\n' "${name}"
      ((failed += 1))
      continue
    fi
    destination="${ASC_WORKSPACE}/${name}"
    if [[ -L "${destination}" ]]; then
      printf '%s: failed: destination is a symbolic link\n' "${name}"
      ((failed += 1))
      continue
    fi
    if [[ -e "${destination}" ]]; then
      if ! asc_is_git_repository "${destination}"; then
        printf '%s: failed: destination exists and is not a Git working tree\n' "${name}"
        ((failed += 1))
        continue
      fi
      actual=$(git -C "${destination}" remote get-url -- "${ASC_REMOTE}" 2>/dev/null || true)
      actual="${actual%/}"
      if [[ "${actual}" != "${https_urls[${name}]%/}" && "${actual}" != "${ssh_urls[${name}]%/}" ]]; then
        printf '%s: failed: existing working tree remote does not match the organization repository\n' "${name}"
        ((failed += 1))
      else
        printf '%s: already-present\n' "${name}"
        ((already_present += 1))
      fi
      continue
    fi
    [[ "${protocol}" == ssh ]] && selected_url="${ssh_urls[${name}]}" || selected_url="${https_urls[${name}]}"
    if [[ -z "${selected_url}" ]]; then
      printf '%s: failed: %s clone URL is missing\n' "${name}" "${protocol}"
      ((failed += 1))
      continue
    fi
    if git clone -- "${selected_url}" "${destination}"; then
      printf '%s: cloned\n' "${name}"
      ((cloned += 1))
    else
      printf '%s: failed: clone failed\n' "${name}"
      ((failed += 1))
    fi
  done
  printf 'Summary: already-present=%d cloned=%d failed=%d\n' "${already_present}" "${cloned}" "${failed}"
  ((failed == 0))
}

asc_repository_select() {
  local -a requested=("$@")
  local discovery=""
  if ((${#requested[@]} == 0)); then
    discovery=$(asc_repository_discover_local)
    [[ -z "${discovery}" ]] || mapfile -t ASC_SELECTED_REPOSITORIES <<<"${discovery}"
  else
    ASC_SELECTED_REPOSITORIES=("${requested[@]}")
  fi
}

asc_repository_read_status() {
  local name="$1"
  local path="${ASC_WORKSPACE}/${name}"
  local output line counts
  ASC_STATUS_NAME="${name}"
  ASC_STATUS_BRANCH=""
  ASC_STATUS_DETACHED=false
  ASC_STATUS_UPSTREAM=""
  ASC_STATUS_AHEAD=0
  ASC_STATUS_BEHIND=0
  ASC_STATUS_CLEAN=true
  ASC_STATUS_CHANGES=0
  ASC_STATUS_ERROR=""
  if ! asc_validate_repository_name "${name}" >/dev/null 2>&1 || ! asc_is_git_repository "${path}"; then
    ASC_STATUS_CLEAN=false
    ASC_STATUS_ERROR="local Git working tree not found: ${name}"
    return 1
  fi
  if ! output=$(git -C "${path}" status --porcelain=v2 --branch); then
    ASC_STATUS_CLEAN=false
    ASC_STATUS_ERROR="git status failed"
    return 1
  fi
  while IFS= read -r line; do
    case "${line}" in
      '# branch.head (detached)')
        ASC_STATUS_BRANCH=HEAD
        ASC_STATUS_DETACHED=true
        ;;
      '# branch.head '*) ASC_STATUS_BRANCH="${line#\# branch.head }" ;;
      '# branch.upstream '*) ASC_STATUS_UPSTREAM="${line#\# branch.upstream }" ;;
      '# branch.ab '*)
        counts="${line#\# branch.ab }"
        ASC_STATUS_AHEAD="${counts%% *}"
        ASC_STATUS_AHEAD="${ASC_STATUS_AHEAD#+}"
        ASC_STATUS_BEHIND="${counts##* }"
        ASC_STATUS_BEHIND="${ASC_STATUS_BEHIND#-}"
        ;;
      '# '* | '') ;;
      *)
        ASC_STATUS_CLEAN=false
        ((ASC_STATUS_CHANGES += 1))
        ;;
    esac
  done <<<"${output}"
}

asc_repository_status_json_record() {
  printf '{"name":%s,"branch":%s,"detached":%s,"upstream":%s,"ahead":%d,"behind":%d,"clean":%s,"changes":%d' \
    "$(asc_json_quote "${ASC_STATUS_NAME}")" "$(asc_json_quote "${ASC_STATUS_BRANCH}")" \
    "${ASC_STATUS_DETACHED}" "$(asc_json_quote "${ASC_STATUS_UPSTREAM}")" \
    "${ASC_STATUS_AHEAD}" "${ASC_STATUS_BEHIND}" "${ASC_STATUS_CLEAN}" "${ASC_STATUS_CHANGES}"
  [[ -z "${ASC_STATUS_ERROR}" ]] || printf ',"error":%s' "$(asc_json_quote "${ASC_STATUS_ERROR}")"
  printf '}'
}

asc_repository_status() {
  local json_output="$1"
  shift
  local name
  local failed=0
  local first=true
  local state branch upstream
  ASC_SELECTED_REPOSITORIES=()
  asc_repository_select "$@"
  [[ "${json_output}" == true ]] && printf '[\n' || printf 'REPOSITORY\tBRANCH\tSTATE\tUPSTREAM\tAHEAD/BEHIND\n'
  for name in "${ASC_SELECTED_REPOSITORIES[@]}"; do
    asc_repository_read_status "${name}" || ((failed += 1))
    if [[ "${json_output}" == true ]]; then
      [[ "${first}" == true ]] || printf ',\n'
      first=false
      printf '  '
      asc_repository_status_json_record
      continue
    fi
    if [[ -n "${ASC_STATUS_ERROR}" ]]; then
      printf '%s\tERROR\t%s\n' "${name}" "${ASC_STATUS_ERROR}"
      continue
    fi
    [[ "${ASC_STATUS_CLEAN}" == true ]] && state=clean || state="dirty(${ASC_STATUS_CHANGES})"
    [[ "${ASC_STATUS_DETACHED}" == true ]] && branch='detached HEAD' || branch="${ASC_STATUS_BRANCH}"
    upstream="${ASC_STATUS_UPSTREAM:--}"
    printf '%s\t%s\t%s\t%s\t+%d/-%d\n' \
      "${name}" "${branch}" "${state}" "${upstream}" "${ASC_STATUS_AHEAD}" "${ASC_STATUS_BEHIND}"
  done
  [[ "${json_output}" == true ]] && printf '\n]\n'
  ((failed == 0))
}

asc_repository_sync_one() {
  local name="$1"
  local dry_run="$2"
  local path="${ASC_WORKSPACE}/${name}"
  local dirty branch upstream before after
  ASC_SYNC_OUTCOME=""
  ASC_SYNC_DETAIL=""
  ASC_SYNC_PLAN_ONE=""
  ASC_SYNC_PLAN_TWO=""
  if ! asc_validate_repository_name "${name}" >/dev/null 2>&1 || ! asc_is_git_repository "${path}"; then
    ASC_SYNC_OUTCOME=failed
    ASC_SYNC_DETAIL="local Git working tree not found: ${name}"
    return
  fi
  if ! dirty=$(git -C "${path}" status --porcelain); then
    ASC_SYNC_OUTCOME=failed
    ASC_SYNC_DETAIL="could not inspect working tree"
    return
  fi
  if [[ -n "${dirty}" ]]; then
    ASC_SYNC_OUTCOME=skipped
    ASC_SYNC_DETAIL="working tree is dirty"
    return
  fi
  if ! git -C "${path}" remote get-url -- "${ASC_REMOTE}" >/dev/null 2>&1; then
    ASC_SYNC_OUTCOME=skipped
    ASC_SYNC_DETAIL="configured remote is not available: ${ASC_REMOTE}"
    return
  fi
  if ! branch=$(git -C "${path}" symbolic-ref --quiet --short HEAD); then
    ASC_SYNC_OUTCOME=skipped
    ASC_SYNC_DETAIL="detached HEAD cannot be synchronized safely"
    return
  fi
  if ! upstream=$(git -C "${path}" rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null); then
    ASC_SYNC_OUTCOME=skipped
    ASC_SYNC_DETAIL="branch ${branch} has no upstream"
    return
  fi
  if [[ "${upstream}" != "${ASC_REMOTE}/"* ]]; then
    ASC_SYNC_OUTCOME=skipped
    ASC_SYNC_DETAIL="upstream does not use configured remote: ${upstream}"
    return
  fi
  printf -v ASC_SYNC_PLAN_ONE '%q ' git -C "${path}" fetch -- "${ASC_REMOTE}"
  printf -v ASC_SYNC_PLAN_TWO '%q ' git -C "${path}" merge --ff-only "${upstream}"
  ASC_SYNC_PLAN_ONE="${ASC_SYNC_PLAN_ONE% }"
  ASC_SYNC_PLAN_TWO="${ASC_SYNC_PLAN_TWO% }"
  if [[ "${dry_run}" == true ]]; then
    ASC_SYNC_OUTCOME=planned
    return
  fi
  before=$(git -C "${path}" rev-parse HEAD) || {
    ASC_SYNC_OUTCOME=failed
    ASC_SYNC_DETAIL="could not read HEAD"
    return
  }
  if ! git -C "${path}" fetch -- "${ASC_REMOTE}"; then
    ASC_SYNC_OUTCOME=failed
    ASC_SYNC_DETAIL="fetch failed"
    return
  fi
  if ! git -C "${path}" merge --ff-only "${upstream}"; then
    ASC_SYNC_OUTCOME=skipped
    ASC_SYNC_DETAIL="fast-forward refused"
    return
  fi
  after=$(git -C "${path}" rev-parse HEAD) || {
    ASC_SYNC_OUTCOME=failed
    ASC_SYNC_DETAIL="could not read HEAD"
    return
  }
  [[ "${before}" == "${after}" ]] && ASC_SYNC_OUTCOME=unchanged || ASC_SYNC_OUTCOME=updated
}

asc_repository_sync() {
  local dry_run="$1"
  shift
  local name detail
  local updated=0 unchanged=0 planned=0 skipped=0 failed=0
  ASC_SELECTED_REPOSITORIES=()
  asc_repository_select "$@"
  for name in "${ASC_SELECTED_REPOSITORIES[@]}"; do
    asc_repository_sync_one "${name}" "${dry_run}"
    detail=""
    [[ -z "${ASC_SYNC_DETAIL}" ]] || detail=": ${ASC_SYNC_DETAIL}"
    printf '%s: %s%s\n' "${name}" "${ASC_SYNC_OUTCOME}" "${detail}"
    if [[ "${ASC_SYNC_OUTCOME}" == planned ]]; then
      printf '  %s\n  %s\n' "${ASC_SYNC_PLAN_ONE}" "${ASC_SYNC_PLAN_TWO}"
    fi
    case "${ASC_SYNC_OUTCOME}" in
      updated) ((updated += 1)) ;;
      unchanged) ((unchanged += 1)) ;;
      planned) ((planned += 1)) ;;
      skipped) ((skipped += 1)) ;;
      failed) ((failed += 1)) ;;
    esac
  done
  printf 'Summary: failed=%d planned=%d skipped=%d unchanged=%d updated=%d\n' \
    "${failed}" "${planned}" "${skipped}" "${unchanged}" "${updated}"
  ((failed == 0 && skipped == 0))
}
