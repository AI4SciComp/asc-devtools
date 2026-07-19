#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

asc_doctor_add() {
  ASC_DOCTOR_NAMES+=("$1")
  ASC_DOCTOR_STATUSES+=("$2")
  ASC_DOCTOR_DETAILS+=("$3")
  ASC_DOCTOR_REMEDIES+=("${4:-}")
}

asc_doctor_tool() {
  local name="$1"
  local executable="$2"
  local required="$3"
  shift 3
  local output
  local status=warning
  [[ "${required}" == true ]] && status=failure
  if ! command -v "${executable}" >/dev/null 2>&1; then
    asc_doctor_add "${name}" "${status}" "required executable not found: ${executable}" "install ${executable}"
    return
  fi
  if output=$("$@" 2>&1); then
    asc_doctor_add "${name}" pass "$(asc_first_line "${output}")"
  else
    asc_doctor_add "${name}" "${status}" "could not determine version" "install ${executable}"
  fi
}

asc_doctor_workspace() {
  local candidate="${ASC_WORKSPACE}"
  if [[ -e "${candidate}" ]]; then
    if [[ -d "${candidate}" ]]; then
      asc_doctor_add workspace pass "${candidate} exists"
    else
      asc_doctor_add workspace failure "${candidate} is not a directory"
    fi
    return
  fi
  while [[ ! -e "${candidate}" && "${candidate}" != / ]]; do
    candidate="${candidate%/*}"
    [[ -n "${candidate}" ]] || candidate=/
  done
  if [[ -d "${candidate}" && -w "${candidate}" ]]; then
    asc_doctor_add workspace pass "${ASC_WORKSPACE} can be created"
  else
    asc_doctor_add workspace failure "${ASC_WORKSPACE} cannot be created"
  fi
}

asc_doctor() {
  local json_output="$1"
  local api_output=""
  local ssh_output=""
  local index
  local failures=0
  local first=true
  ASC_DOCTOR_NAMES=()
  ASC_DOCTOR_STATUSES=()
  ASC_DOCTOR_DETAILS=()
  ASC_DOCTOR_REMEDIES=()
  asc_doctor_add configuration pass "${ASC_CONFIG_PATH}"
  asc_doctor_workspace
  if ((BASH_VERSINFO[0] > ASC_MINIMUM_BASH_MAJOR)) ||
    ((BASH_VERSINFO[0] == ASC_MINIMUM_BASH_MAJOR && BASH_VERSINFO[1] >= ASC_MINIMUM_BASH_MINOR)); then
    asc_doctor_add bash pass "${BASH_VERSION}"
  else
    asc_doctor_add bash failure "${BASH_VERSION}" "Bash 4.4 or newer is required"
  fi
  asc_doctor_tool git git true git --version
  asc_doctor_tool curl curl true curl --version
  asc_doctor_tool cmake cmake false cmake --version
  asc_doctor_tool ctest ctest false ctest --version
  local vendor_source="${ASC_WORKSPACE}/${ASC_CMAKE_SOURCE_REPOSITORY}"
  if [[ ! -e "${vendor_source}" ]]; then
    asc_doctor_add asc-cmake-source warning "${vendor_source} is not checked out" \
      "clone asc-cmake before using vendor commands"
  elif [[ -L "${vendor_source}" || ! -d "${vendor_source}" ]]; then
    asc_doctor_add asc-cmake-source warning "${vendor_source} is not a regular directory"
  elif [[ "$(git -C "${vendor_source}" rev-parse --is-inside-work-tree 2>/dev/null || true)" == true ]]; then
    asc_doctor_add asc-cmake-source pass "${vendor_source} is available"
  else
    asc_doctor_add asc-cmake-source warning "${vendor_source} is not a Git worktree"
  fi
  if command -v gh >/dev/null 2>&1; then
    asc_doctor_add gh pass "optional GitHub CLI is available"
  else
    asc_doctor_add gh warning "optional GitHub CLI is not installed"
  fi
  if [[ -n "${ASC_GITHUB_TOKEN_VALUE}" ]]; then
    asc_doctor_add api-authentication pass "GitHub API token is available from ${ASC_GITHUB_TOKEN_SOURCE}"
  else
    asc_doctor_add api-authentication warning "using unauthenticated public GitHub API access" \
      "set ASC_GITHUB_TOKEN for private repositories and higher rate limits"
  fi
  if api_output=$(asc_github_discover 2>/dev/null); then
    local repository_count=0
    [[ -z "${api_output}" ]] || repository_count=$(wc -l <<<"${api_output}")
    asc_doctor_add github-api pass "can list ${repository_count} managed repositories in ${ASC_ORGANIZATION}"
  else
    asc_doctor_add github-api failure "GitHub API request failed" \
      "check connectivity, organization, token permissions, and rate limits"
  fi
  if ! command -v ssh >/dev/null 2>&1; then
    asc_doctor_add ssh warning "ssh is not installed" "install OpenSSH for SSH clones"
  else
    ssh_output=$(ssh -T -o BatchMode=yes -o ConnectTimeout=5 git@github.com 2>&1 || true)
    if [[ "${ssh_output,,}" == *"successfully authenticated"* ]]; then
      asc_doctor_add ssh pass "GitHub accepted the configured SSH key"
    else
      asc_doctor_add ssh warning "GitHub SSH authentication was not confirmed" "run ssh -T git@github.com"
    fi
  fi
  if command -v asc >/dev/null 2>&1; then
    asc_doctor_add path pass "asc is available on PATH"
  else
    asc_doctor_add path warning "asc is not available on PATH" "install asc under /usr/local/bin"
  fi

  if [[ "${json_output}" == true ]]; then
    printf '[\n'
    for ((index = 0; index < ${#ASC_DOCTOR_NAMES[@]}; index++)); do
      [[ "${first}" == true ]] || printf ',\n'
      first=false
      printf '  {"name":%s,"status":%s,"detail":%s' \
        "$(asc_json_quote "${ASC_DOCTOR_NAMES[index]}")" \
        "$(asc_json_quote "${ASC_DOCTOR_STATUSES[index]}")" \
        "$(asc_json_quote "${ASC_DOCTOR_DETAILS[index]}")"
      [[ -z "${ASC_DOCTOR_REMEDIES[index]}" ]] ||
        printf ',"remedy":%s' "$(asc_json_quote "${ASC_DOCTOR_REMEDIES[index]}")"
      printf '}'
    done
    printf '\n]\n'
  else
    for ((index = 0; index < ${#ASC_DOCTOR_NAMES[@]}; index++)); do
      printf '%-7s %-20s %s' \
        "${ASC_DOCTOR_STATUSES[index]^^}" "${ASC_DOCTOR_NAMES[index]}" "${ASC_DOCTOR_DETAILS[index]}"
      [[ -z "${ASC_DOCTOR_REMEDIES[index]}" ]] || printf '; %s' "${ASC_DOCTOR_REMEDIES[index]}"
      printf '\n'
    done
  fi
  for index in "${!ASC_DOCTOR_STATUSES[@]}"; do
    [[ "${ASC_DOCTOR_STATUSES[index]}" != failure ]] || ((failures += 1))
  done
  ((failures == 0))
}
