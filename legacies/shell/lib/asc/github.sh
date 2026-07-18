#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

asc_github_discover() {
  asc_require_command gh || return 1
  local raw_output
  local name
  local archived
  local -a repositories=()
  local -a command=(
    gh repo list "${ASC_ORGANIZATION}"
    --limit 1000
    --json "name,isArchived"
    --jq '.[] | [.name, .isArchived] | @tsv'
  )

  if ! raw_output=$("${command[@]}"); then
    asc_error "GitHub CLI could not list repositories in ${ASC_ORGANIZATION}; run 'gh auth login' and verify organization access"
    return 1
  fi
  while IFS=$'\t' read -r name archived; do
    [[ -n "${name}" ]] || continue
    if ! asc_validate_simple_repository_name "${name}" >/dev/null 2>&1; then
      asc_error "GitHub CLI returned an unsafe repository name"
      return 1
    fi
    [[ "${archived}" == "false" ]] || continue
    if [[ "${name}" == "${ASC_REPOSITORY_PREFIX}"* ]] ||
      [[ "${ASC_INCLUDE_DOT_GITHUB}" == "true" && "${name}" == ".github" ]]; then
      repositories+=("${name}")
    fi
  done <<<"${raw_output}"
  if ((${#repositories[@]} > 0)); then
    printf '%s\n' "${repositories[@]}" | LC_ALL=C sort
  fi
}
