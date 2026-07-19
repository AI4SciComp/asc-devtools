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
readonly ASC_DEFAULT_CMAKE_VENDOR_DIRECTORY="cmake/asc"
readonly ASC_DEFAULT_CMAKE_SOURCE_REPOSITORY="asc-cmake"
readonly ASC_OVERRIDE_UNSET="__ASC_OVERRIDE_UNSET__"

asc_config_parse_cmake() {
  local character key value value_type
  local -A seen=()
  asc_json_expect '{' || return 1
  asc_json_skip_whitespace
  if [[ "${ASC_JSON_TEXT:ASC_JSON_POSITION:1}" == '}' ]]; then
    ((ASC_JSON_POSITION += 1))
    return 0
  fi
  while true; do
    asc_json_parse_string || return 1
    key="${ASC_JSON_VALUE}"
    [[ ! -v "seen[${key}]" ]] || {
      asc_error "duplicate cmake configuration field: ${key}"
      return 1
    }
    seen["${key}"]=1
    asc_json_expect ':' || return 1
    asc_json_parse_scalar || return 1
    value="${ASC_JSON_VALUE}"
    value_type="${ASC_JSON_TYPE}"
    case "${key}" in
      vendorDirectory | sourceRepository) ;;
      *)
        asc_error "unknown cmake configuration field: ${key}"
        return 1
        ;;
    esac
    [[ "${value_type}" == string ]] || {
      asc_error "cmake configuration field ${key} must be a string"
      return 1
    }
    case "${key}" in
      vendorDirectory) ASC_CMAKE_VENDOR_DIRECTORY="${value}" ;;
      sourceRepository) ASC_CMAKE_SOURCE_REPOSITORY="${value}" ;;
    esac
    asc_json_skip_whitespace
    character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
    if [[ "${character}" == '}' ]]; then
      ((ASC_JSON_POSITION += 1))
      break
    fi
    asc_json_expect ',' || return 1
  done
}

asc_config_parse_file() {
  local content="$1"
  local character
  local key
  local value
  local value_type
  local -A seen=()
  asc_json_begin "${content}"
  asc_json_expect '{' || return 1
  asc_json_skip_whitespace
  if [[ "${ASC_JSON_TEXT:ASC_JSON_POSITION:1}" == '}' ]]; then
    ((ASC_JSON_POSITION += 1))
    asc_json_finish
    return
  fi
  while true; do
    asc_json_parse_string || return 1
    key="${ASC_JSON_VALUE}"
    [[ ! -v "seen[${key}]" ]] || {
      asc_error "duplicate configuration field: ${key}"
      return 1
    }
    seen["${key}"]=1
    asc_json_expect ':' || return 1
    if [[ "${key}" == cmake ]]; then
      asc_config_parse_cmake || return 1
      value=""
      value_type=object
    else
      asc_json_parse_scalar || return 1
      value="${ASC_JSON_VALUE}"
      value_type="${ASC_JSON_TYPE}"
    fi
    case "${key}" in
      organization | workspace | repositoryPrefix | cloneProtocol | remote)
        [[ "${value_type}" == "string" ]] || {
          asc_error "configuration field ${key} must be a string"
          return 1
        }
        ;;
      includeDotGitHub)
        [[ "${value_type}" == "boolean" ]] || {
          asc_error "configuration field includeDotGitHub must be true or false"
          return 1
        }
        ;;
      cmake) ;;
      *)
        asc_error "unknown configuration field: ${key}"
        return 1
        ;;
    esac
    case "${key}" in
      organization) ASC_ORGANIZATION="${value}" ;;
      workspace) ASC_WORKSPACE="${value}" ;;
      repositoryPrefix) ASC_REPOSITORY_PREFIX="${value}" ;;
      includeDotGitHub) ASC_INCLUDE_DOT_GITHUB="${value}" ;;
      cloneProtocol) ASC_CLONE_PROTOCOL="${value}" ;;
      remote) ASC_REMOTE="${value}" ;;
      cmake) ;;
    esac
    asc_json_skip_whitespace
    character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
    if [[ "${character}" == '}' ]]; then
      ((ASC_JSON_POSITION += 1))
      break
    fi
    asc_json_expect ',' || return 1
  done
  asc_json_finish
}

asc_config_environment_boolean() {
  [[ "${ASC_INCLUDE_DOT_GITHUB}" == "true" || "${ASC_INCLUDE_DOT_GITHUB}" == "false" ]] || {
    asc_error "ASC_INCLUDE_DOT_GITHUB must be 'true' or 'false'"
    return 1
  }
}

asc_config_load() {
  local config_override="$1"
  local organization_override="$2"
  local workspace_override="$3"
  local config_path
  local config_size
  local config_content=""
  local env_organization="${ASC_ORGANIZATION-}"
  local env_workspace="${ASC_WORKSPACE-}"
  local env_prefix="${ASC_REPOSITORY_PREFIX-}"
  local env_include="${ASC_INCLUDE_DOT_GITHUB-}"
  local env_protocol="${ASC_CLONE_PROTOCOL-}"
  local env_remote="${ASC_REMOTE-}"
  local has_env_organization=false
  local has_env_workspace=false
  local has_env_prefix=false
  local has_env_include=false
  local has_env_protocol=false
  local has_env_remote=false
  [[ -v ASC_ORGANIZATION ]] && has_env_organization=true
  [[ -v ASC_WORKSPACE ]] && has_env_workspace=true
  [[ -v ASC_REPOSITORY_PREFIX ]] && has_env_prefix=true
  [[ -v ASC_INCLUDE_DOT_GITHUB ]] && has_env_include=true
  [[ -v ASC_CLONE_PROTOCOL ]] && has_env_protocol=true
  [[ -v ASC_REMOTE ]] && has_env_remote=true

  if [[ "${config_override}" != "${ASC_OVERRIDE_UNSET}" ]]; then
    config_path="${config_override}"
  elif [[ -n "${ASC_CONFIG:-}" ]]; then
    config_path="${ASC_CONFIG}"
  else
    config_path="${HOME}/.config/asc/config.json"
  fi
  case "${config_path}" in
    '~') config_path="${HOME}" ;;
    \~/*) config_path="${HOME}/${config_path#\~/}" ;;
    '~'*)
      asc_error "only '~' and '~/' home expansion are supported"
      return 1
      ;;
  esac
  [[ "${config_path}" == /* ]] || config_path="${PWD}/${config_path}"
  ASC_CONFIG_PATH=$(realpath -m -- "${config_path}")

  ASC_ORGANIZATION="${ASC_DEFAULT_ORGANIZATION}"
  ASC_WORKSPACE="${HOME}/projects/AI4SciComp"
  ASC_REPOSITORY_PREFIX="${ASC_DEFAULT_REPOSITORY_PREFIX}"
  ASC_INCLUDE_DOT_GITHUB="${ASC_DEFAULT_INCLUDE_DOT_GITHUB}"
  ASC_CLONE_PROTOCOL="${ASC_DEFAULT_CLONE_PROTOCOL}"
  ASC_REMOTE="${ASC_DEFAULT_REMOTE}"
  ASC_CMAKE_VENDOR_DIRECTORY="${ASC_DEFAULT_CMAKE_VENDOR_DIRECTORY}"
  ASC_CMAKE_SOURCE_REPOSITORY="${ASC_DEFAULT_CMAKE_SOURCE_REPOSITORY}"

  if [[ -L "${ASC_CONFIG_PATH}" || (-e "${ASC_CONFIG_PATH}" && ! -f "${ASC_CONFIG_PATH}") ]]; then
    asc_error "configuration path is not a regular file: ${ASC_CONFIG_PATH}"
    return 1
  fi
  if [[ -f "${ASC_CONFIG_PATH}" ]]; then
    config_size=$(wc -c <"${ASC_CONFIG_PATH}")
    ((config_size <= 1048576)) || {
      asc_error "configuration exceeds 1 MiB limit: ${ASC_CONFIG_PATH}"
      return 1
    }
    config_content=$(<"${ASC_CONFIG_PATH}")
    asc_config_parse_file "${config_content}" || return 1
  fi

  [[ "${has_env_organization}" == true ]] && ASC_ORGANIZATION="${env_organization}"
  [[ "${has_env_workspace}" == true ]] && ASC_WORKSPACE="${env_workspace}"
  [[ "${has_env_prefix}" == true ]] && ASC_REPOSITORY_PREFIX="${env_prefix}"
  [[ "${has_env_include}" == true ]] && ASC_INCLUDE_DOT_GITHUB="${env_include}"
  [[ "${has_env_protocol}" == true ]] && ASC_CLONE_PROTOCOL="${env_protocol}"
  [[ "${has_env_remote}" == true ]] && ASC_REMOTE="${env_remote}"
  [[ "${organization_override}" == "${ASC_OVERRIDE_UNSET}" ]] || ASC_ORGANIZATION="${organization_override}"
  [[ "${workspace_override}" == "${ASC_OVERRIDE_UNSET}" ]] || ASC_WORKSPACE="${workspace_override}"

  case "${ASC_WORKSPACE}" in
    '~') ASC_WORKSPACE="${HOME}" ;;
    \~/*) ASC_WORKSPACE="${HOME}/${ASC_WORKSPACE#\~/}" ;;
    '~'*)
      asc_error "only '~' and '~/' home expansion are supported"
      return 1
      ;;
  esac
  [[ "${ASC_WORKSPACE}" == /* ]] || ASC_WORKSPACE="${PWD}/${ASC_WORKSPACE}"
  ASC_WORKSPACE=$(realpath -m -- "${ASC_WORKSPACE}")

  [[ "${ASC_ORGANIZATION}" =~ ^[A-Za-z0-9]([A-Za-z0-9-]{0,38})$ ]] || {
    asc_error "organization must be a valid GitHub organization name"
    return 1
  }
  [[ -n "${ASC_REPOSITORY_PREFIX}" && "${ASC_REPOSITORY_PREFIX}" =~ ^[A-Za-z0-9._-]+$ ]] || {
    asc_error "repository prefix contains invalid characters"
    return 1
  }
  asc_config_environment_boolean || return 1
  [[ "${ASC_CLONE_PROTOCOL}" == "ssh" || "${ASC_CLONE_PROTOCOL}" == "https" ]] || {
    asc_error "clone protocol must be 'ssh' or 'https'"
    return 1
  }
  [[ -n "${ASC_REMOTE}" && "${ASC_REMOTE}" != -* && "${ASC_REMOTE}" =~ ^[A-Za-z0-9._-]+$ ]] || {
    asc_error "remote must be a simple Git remote name"
    return 1
  }
  [[ "${ASC_WORKSPACE}" != "/" ]] || {
    asc_error "workspace must not be a filesystem root"
    return 1
  }
  [[ -n "${ASC_CMAKE_VENDOR_DIRECTORY}" &&
    "${ASC_CMAKE_VENDOR_DIRECTORY}" != /* &&
    "${ASC_CMAKE_VENDOR_DIRECTORY}" != *\\* &&
    "${ASC_CMAKE_VENDOR_DIRECTORY}" != *//* &&
    "${ASC_CMAKE_VENDOR_DIRECTORY}" != "." &&
    "${ASC_CMAKE_VENDOR_DIRECTORY}" != ".." &&
    "${ASC_CMAKE_VENDOR_DIRECTORY}" != ../* &&
    "${ASC_CMAKE_VENDOR_DIRECTORY}" != */../* &&
    "${ASC_CMAKE_VENDOR_DIRECTORY}" != */.. ]] || {
    asc_error "CMake vendor directory must be a safe relative path"
    return 1
  }
  [[ -n "${ASC_CMAKE_SOURCE_REPOSITORY}" &&
    "${ASC_CMAKE_SOURCE_REPOSITORY}" != -* &&
    "${ASC_CMAKE_SOURCE_REPOSITORY}" =~ ^[A-Za-z0-9._-]+$ ]] || {
    asc_error "CMake source repository must be a simple name"
    return 1
  }

  ASC_GITHUB_TOKEN_VALUE=""
  ASC_GITHUB_TOKEN_SOURCE=""
  if [[ -n "${ASC_GITHUB_TOKEN:-}" ]]; then
    ASC_GITHUB_TOKEN_VALUE="${ASC_GITHUB_TOKEN}"
    ASC_GITHUB_TOKEN_SOURCE="ASC_GITHUB_TOKEN"
  elif [[ -n "${GH_TOKEN:-}" ]]; then
    ASC_GITHUB_TOKEN_VALUE="${GH_TOKEN}"
    ASC_GITHUB_TOKEN_SOURCE="GH_TOKEN"
  elif [[ -n "${GITHUB_TOKEN:-}" ]]; then
    ASC_GITHUB_TOKEN_VALUE="${GITHUB_TOKEN}"
    ASC_GITHUB_TOKEN_SOURCE="GITHUB_TOKEN"
  elif command -v gh >/dev/null 2>&1; then
    ASC_GITHUB_TOKEN_VALUE=$(gh auth token 2>/dev/null || true)
    # Consumed by doctor.sh after this module is sourced.
    # shellcheck disable=SC2034
    [[ -z "${ASC_GITHUB_TOKEN_VALUE}" ]] || ASC_GITHUB_TOKEN_SOURCE="gh auth token"
  fi
}
