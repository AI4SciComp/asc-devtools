#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

# These constants are consumed by other sourced modules and the front end.
# shellcheck disable=SC2034
readonly ASC_VERSION="0.1.0"
# shellcheck disable=SC2034
readonly ASC_MINIMUM_BASH_MAJOR=4
# shellcheck disable=SC2034
readonly ASC_MINIMUM_BASH_MINOR=4

asc_error() {
  printf 'asc: error: %s\n' "$*" >&2
}

asc_warn() {
  printf 'WARN %s\n' "$*" >&2
}

asc_require_command() {
  local executable="$1"
  if ! command -v "${executable}" >/dev/null 2>&1; then
    asc_error "required executable not found: ${executable}"
    return 1
  fi
}

asc_validate_simple_repository_name() {
  local name="$1"
  if [[ -z "${name}" || "${name}" == "." || "${name}" == ".." ]]; then
    asc_error "invalid repository name: '${name}'"
    return 1
  fi
  if [[ "${name}" == /* || "${name}" == */* || "${name}" == *\\* ]]; then
    asc_error "repository names must not contain paths: '${name}'"
    return 1
  fi
  if [[ ! "${name}" =~ ^[A-Za-z0-9._-]+$ ]]; then
    asc_error "invalid repository name: '${name}'"
    return 1
  fi
}

asc_is_managed_repository_name() {
  local name="$1"
  [[ "${name}" == "${ASC_REPOSITORY_PREFIX}"* ]] ||
    [[ "${ASC_INCLUDE_DOT_GITHUB}" == "true" && "${name}" == ".github" ]]
}

asc_validate_repository_name() {
  local name="$1"
  asc_validate_simple_repository_name "${name}" || return 1
  if ! asc_is_managed_repository_name "${name}"; then
    asc_error "repository is not managed by asc: ${name}"
    return 1
  fi
}

asc_is_git_repository() {
  local repository_path="$1"
  [[ -d "${repository_path}" && ! -L "${repository_path}" ]] &&
    git -C "${repository_path}" rev-parse --is-inside-work-tree >/dev/null 2>&1
}

asc_first_line() {
  local value="$1"
  printf '%s\n' "${value%%$'\n'*}"
}
