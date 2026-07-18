#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

asc_cmake_require_preset() {
  local repository_path="$1"
  local preset_type="$2"
  local preset="$3"
  local preset_output
  local line
  local found=false
  local -a command=(cmake --list-presets="${preset_type}")

  if ! preset_output=$(cd -- "${repository_path}" && "${command[@]}" 2>/dev/null); then
    asc_error "could not list ${preset_type} presets in ${repository_path##*/}"
    return 1
  fi
  while IFS= read -r line; do
    if [[ "${line}" == *\""${preset}"\"* ]]; then
      found=true
      break
    fi
  done <<<"${preset_output}"
  if [[ "${found}" != "true" ]]; then
    asc_error "${preset_type} preset '${preset}' not found in ${repository_path##*/}/CMakePresets.json"
    return 1
  fi
}

asc_cmake_run() {
  local operation="$1"
  local repository_name="$2"
  local preset="$3"
  local repository_path="${ASC_WORKSPACE}/${repository_name}"
  local preset_type
  local executable
  local -a command=()

  asc_validate_repository_name "${repository_name}" || return 1
  if ! asc_is_git_repository "${repository_path}"; then
    asc_error "local Git worktree not found: ${repository_name}"
    return 1
  fi
  if [[ ! "${preset}" =~ ^[A-Za-z0-9._-]+$ ]]; then
    asc_error "preset must be a simple preset name"
    return 1
  fi
  if [[ ! -f "${repository_path}/CMakePresets.json" ]]; then
    asc_error "CMakePresets.json not found in ${repository_name}"
    return 1
  fi

  case "${operation}" in
    configure)
      executable=cmake
      preset_type=configure
      command=(cmake --preset "${preset}")
      ;;
    build)
      executable=cmake
      preset_type=build
      command=(cmake --build --preset "${preset}" --parallel)
      ;;
    test)
      executable=ctest
      preset_type="test"
      command=(ctest --preset "${preset}" --output-on-failure)
      ;;
    *)
      asc_error "unsupported CMake operation: ${operation}"
      return 1
      ;;
  esac
  asc_require_command cmake || return 1
  asc_require_command "${executable}" || return 1
  asc_cmake_require_preset "${repository_path}" "${preset_type}" "${preset}" || return 1
  (cd -- "${repository_path}" && "${command[@]}")
}
