#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

readonly ASC_DEFAULT_ORGANIZATION="AI4SciComp"
readonly ASC_DEFAULT_REPOSITORY_PREFIX="asc-"
readonly ASC_DEFAULT_INCLUDE_DOT_GITHUB="true"
readonly ASC_DEFAULT_CLONE_PROTOCOL="ssh"
readonly ASC_DEFAULT_REMOTE="origin"
readonly ASC_OVERRIDE_UNSET="__ASC_OVERRIDE_UNSET__"

asc_config_restore_environment_value() {
  local variable_name="$1"
  local was_set="$2"
  local original_value="$3"
  if [[ "${was_set}" == "true" ]]; then
    printf -v "${variable_name}" '%s' "${original_value}"
  fi
}

asc_config_apply_override() {
  local variable_name="$1"
  local override_value="$2"
  if [[ "${override_value}" != "${ASC_OVERRIDE_UNSET}" ]]; then
    printf -v "${variable_name}" '%s' "${override_value}"
  fi
}

asc_config_load() {
  local config_override="$1"
  local organization_override="$2"
  local workspace_override="$3"
  local prefix_override="$4"
  local include_override="$5"
  local protocol_override="$6"
  local remote_override="$7"
  local config_path
  local variable_name
  local -A environment_was_set=()
  local -A environment_value=()
  local -a variables=(
    ASC_ORGANIZATION ASC_WORKSPACE ASC_REPOSITORY_PREFIX
    ASC_INCLUDE_DOT_GITHUB ASC_CLONE_PROTOCOL ASC_REMOTE
  )

  if [[ "${config_override}" != "${ASC_OVERRIDE_UNSET}" ]]; then
    config_path="${config_override}"
  elif [[ -n "${ASC_CONFIG:-}" ]]; then
    config_path="${ASC_CONFIG}"
  else
    config_path="${HOME}/.config/asc/config"
  fi
  if [[ "${config_path}" != /* ]]; then
    config_path="${PWD}/${config_path}"
  fi
  ASC_CONFIG_PATH="${config_path}"

  for variable_name in "${variables[@]}"; do
    if [[ -v "${variable_name}" ]]; then
      environment_was_set["${variable_name}"]="true"
      environment_value["${variable_name}"]="${!variable_name}"
    else
      environment_was_set["${variable_name}"]="false"
      environment_value["${variable_name}"]=""
    fi
  done

  if [[ -e "${ASC_CONFIG_PATH}" && ! -f "${ASC_CONFIG_PATH}" ]]; then
    asc_error "configuration path is not a file: ${ASC_CONFIG_PATH}"
    return 1
  fi
  if [[ -f "${ASC_CONFIG_PATH}" ]]; then
    # The configuration is explicitly trusted, user-owned executable Bash.
    # shellcheck source=/dev/null
    source "${ASC_CONFIG_PATH}"
  fi

  for variable_name in "${variables[@]}"; do
    asc_config_restore_environment_value \
      "${variable_name}" \
      "${environment_was_set[${variable_name}]}" \
      "${environment_value[${variable_name}]}"
  done

  if [[ ! -v ASC_ORGANIZATION ]]; then ASC_ORGANIZATION="${ASC_DEFAULT_ORGANIZATION}"; fi
  if [[ ! -v ASC_WORKSPACE ]]; then ASC_WORKSPACE="${HOME}/projects/AI4SciComp"; fi
  if [[ ! -v ASC_REPOSITORY_PREFIX ]]; then ASC_REPOSITORY_PREFIX="${ASC_DEFAULT_REPOSITORY_PREFIX}"; fi
  if [[ ! -v ASC_INCLUDE_DOT_GITHUB ]]; then ASC_INCLUDE_DOT_GITHUB="${ASC_DEFAULT_INCLUDE_DOT_GITHUB}"; fi
  if [[ ! -v ASC_CLONE_PROTOCOL ]]; then ASC_CLONE_PROTOCOL="${ASC_DEFAULT_CLONE_PROTOCOL}"; fi
  if [[ ! -v ASC_REMOTE ]]; then ASC_REMOTE="${ASC_DEFAULT_REMOTE}"; fi

  asc_config_apply_override ASC_ORGANIZATION "${organization_override}"
  asc_config_apply_override ASC_WORKSPACE "${workspace_override}"
  asc_config_apply_override ASC_REPOSITORY_PREFIX "${prefix_override}"
  asc_config_apply_override ASC_INCLUDE_DOT_GITHUB "${include_override}"
  asc_config_apply_override ASC_CLONE_PROTOCOL "${protocol_override}"
  asc_config_apply_override ASC_REMOTE "${remote_override}"

  # A quoted tilde is intentionally recognized as configuration input.
  # shellcheck disable=SC2088
  case "${ASC_WORKSPACE}" in
    '~') ASC_WORKSPACE="${HOME}" ;;
    '~/'*) ASC_WORKSPACE="${HOME}/${ASC_WORKSPACE#'~/'}" ;;
  esac
  if [[ "${ASC_WORKSPACE}" != /* ]]; then
    asc_error "workspace must be an absolute path or start with '~/'"
    return 1
  fi
  ASC_WORKSPACE="${ASC_WORKSPACE%/}"
  [[ -n "${ASC_WORKSPACE}" ]] || ASC_WORKSPACE="/"

  if [[ ! "${ASC_ORGANIZATION}" =~ ^[A-Za-z0-9]([A-Za-z0-9-]{0,38})$ ]]; then
    asc_error "organization must be a valid GitHub organization name"
    return 1
  fi
  if [[ -z "${ASC_REPOSITORY_PREFIX}" || ! "${ASC_REPOSITORY_PREFIX}" =~ ^[A-Za-z0-9._-]+$ ]]; then
    asc_error "repository prefix must contain only letters, digits, '.', '_', or '-'"
    return 1
  fi
  if [[ "${ASC_INCLUDE_DOT_GITHUB}" != "true" && "${ASC_INCLUDE_DOT_GITHUB}" != "false" ]]; then
    asc_error "ASC_INCLUDE_DOT_GITHUB must be 'true' or 'false'"
    return 1
  fi
  if [[ "${ASC_CLONE_PROTOCOL}" != "ssh" && "${ASC_CLONE_PROTOCOL}" != "https" ]]; then
    asc_error "ASC_CLONE_PROTOCOL must be 'ssh' or 'https'"
    return 1
  fi
  if [[ -z "${ASC_REMOTE}" || ! "${ASC_REMOTE}" =~ ^[A-Za-z0-9._-]+$ ]]; then
    asc_error "ASC_REMOTE must be a simple Git remote name"
    return 1
  fi
  if [[ "${ASC_WORKSPACE}" == "/" || "${ASC_WORKSPACE}" == "${HOME%/}" ]]; then
    asc_error "workspace is too broad; choose a dedicated directory"
    return 1
  fi
  if [[ "${ASC_WORKSPACE}" == *$'\n'* || "${ASC_WORKSPACE}" == *$'\t'* ||
    "${ASC_WORKSPACE}" == *'/../'* || "${ASC_WORKSPACE}" == *'/./'* ||
    "${ASC_WORKSPACE}" == */.. || "${ASC_WORKSPACE}" == */. ]]; then
    asc_error "workspace contains an unsafe path component"
    return 1
  fi
}
