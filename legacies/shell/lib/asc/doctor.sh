#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

asc_doctor_line() {
  local status="$1"
  local label="$2"
  local detail="$3"
  printf '%-4s %-32s %s\n' "${status}" "${label}" "${detail}"
}

asc_doctor_tool() {
  local label="$1"
  local executable="$2"
  shift 2
  local version_output
  if ! command -v "${executable}" >/dev/null 2>&1; then
    asc_doctor_line FAIL "${label}" "not found; install ${executable}"
    return 1
  fi
  if version_output=$("$@" 2>&1); then
    asc_doctor_line OK "${label}" "$(asc_first_line "${version_output}")"
  else
    asc_doctor_line FAIL "${label}" "could not determine version"
    return 1
  fi
}

asc_doctor_workspace_writable() {
  local candidate="${ASC_WORKSPACE}"
  if [[ -e "${candidate}" ]]; then
    [[ -d "${candidate}" && -w "${candidate}" ]]
    return
  fi
  while [[ ! -e "${candidate}" && "${candidate}" != "/" ]]; do
    candidate="${candidate%/*}"
    [[ -n "${candidate}" ]] || candidate="/"
  done
  [[ -d "${candidate}" && -w "${candidate}" ]]
}

asc_doctor() {
  local failures=0
  local username
  local protocol
  local discovery_output
  local ssh_output
  local bash_supported=false

  printf 'Configuration: %s\n' "${ASC_CONFIG_PATH}"
  printf 'Organization:  %s\n' "${ASC_ORGANIZATION}"
  printf 'Workspace:     %s\n' "${ASC_WORKSPACE}"
  printf 'Clone protocol: %s\n' "${ASC_CLONE_PROTOCOL}"

  if ((BASH_VERSINFO[0] > ASC_MINIMUM_BASH_MAJOR)) ||
    ((BASH_VERSINFO[0] == ASC_MINIMUM_BASH_MAJOR && BASH_VERSINFO[1] >= ASC_MINIMUM_BASH_MINOR)); then
    bash_supported=true
  fi
  if [[ "${bash_supported}" == "true" ]]; then
    asc_doctor_line OK "Bash" "${BASH_VERSION}"
  else
    asc_doctor_line FAIL "Bash" "${BASH_VERSION}; Bash 4.4+ is required"
    ((failures += 1))
  fi

  asc_doctor_tool Git git git --version || ((failures += 1))
  asc_doctor_tool "GitHub CLI" gh gh --version || ((failures += 1))
  asc_doctor_tool CMake cmake cmake --version || ((failures += 1))
  asc_doctor_tool CTest ctest ctest --version || ((failures += 1))

  if command -v shellcheck >/dev/null 2>&1; then
    asc_doctor_line OK ShellCheck "optional development tool available"
  else
    asc_doctor_line WARN ShellCheck "optional; install shellcheck for development"
  fi
  if command -v shfmt >/dev/null 2>&1; then
    asc_doctor_line OK shfmt "optional development tool available"
  else
    asc_doctor_line WARN shfmt "optional; install shfmt for development"
  fi

  if username=$(gh api user --jq .login 2>/dev/null) && [[ -n "${username}" ]]; then
    asc_doctor_line OK "GitHub CLI API authentication" "authenticated as ${username}"
  else
    asc_doctor_line FAIL "GitHub CLI API authentication" "run 'gh auth login'"
    ((failures += 1))
  fi

  if protocol=$(gh config get git_protocol --host github.com 2>/dev/null) &&
    [[ "${protocol}" == "ssh" || "${protocol}" == "https" ]]; then
    if [[ "${protocol}" == "${ASC_CLONE_PROTOCOL}" ]]; then
      asc_doctor_line OK "GitHub CLI Git protocol" "${protocol}"
    else
      asc_doctor_line WARN "GitHub CLI Git protocol" "${protocol}; asc clone protocol is ${ASC_CLONE_PROTOCOL}"
    fi
  else
    asc_doctor_line FAIL "GitHub CLI Git protocol" "run 'gh config set git_protocol ssh --host github.com'"
    ((failures += 1))
  fi

  if discovery_output=$(asc_github_discover 2>/dev/null); then
    asc_doctor_line OK "Organization repository access" "can list ${ASC_ORGANIZATION}"
  else
    asc_doctor_line FAIL "Organization repository access" "check API token access to ${ASC_ORGANIZATION}"
    ((failures += 1))
  fi
  : "${discovery_output:-}"

  if [[ "${ASC_CLONE_PROTOCOL}" == "ssh" ]]; then
    if ssh_output=$(ssh -T -o BatchMode=yes -o ConnectTimeout=5 git@github.com 2>&1); then
      :
    else
      :
    fi
    if [[ "${ssh_output}" == *"successfully authenticated"* ]]; then
      asc_doctor_line OK "Git SSH authentication" "GitHub accepted the SSH key"
    else
      asc_doctor_line FAIL "Git SSH authentication" "add an SSH key to GitHub; test with 'ssh -T git@github.com'"
      ((failures += 1))
    fi
  else
    asc_doctor_line OK "Git SSH authentication" "not used for HTTPS clones"
  fi

  if asc_doctor_workspace_writable; then
    if [[ -d "${ASC_WORKSPACE}" ]]; then
      asc_doctor_line OK Workspace "exists and is writable"
    else
      asc_doctor_line OK Workspace "does not exist; nearest parent is writable"
    fi
  else
    asc_doctor_line FAIL Workspace "check directory ownership and permissions"
    ((failures += 1))
  fi
  ((failures == 0))
}
