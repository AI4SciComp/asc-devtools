#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

asc_cmake_validate_value() {
  local kind="$1"
  local value="$2"
  if [[ -z "${value}" || "${value}" == -* || "${value}" == *$'\n'* || "${value}" == *$'\r'* || "${value}" == *$'\t'* ]]; then
    asc_error "invalid ${kind}: '${value}'"
    return 1
  fi
}

asc_cmake_repository() {
  local repository_name="$1"
  local require_project="$2"
  local repository_path="${ASC_WORKSPACE}/${repository_name}"
  asc_validate_repository_name "${repository_name}" || return 1
  if ! asc_is_git_repository "${repository_path}"; then
    asc_error "local Git worktree not found: ${repository_name}"
    return 1
  fi
  if [[ "${require_project}" == true && ! -f "${repository_path}/CMakeLists.txt" ]]; then
    asc_error "CMakeLists.txt not found in ${repository_name}"
    return 1
  fi
  if [[ ! -f "${repository_path}/CMakePresets.json" && ! -f "${repository_path}/CMakeUserPresets.json" ]]; then
    asc_error "CMakePresets.json not found in ${repository_name}"
    return 1
  fi
  printf '%s\n' "${repository_path}"
}

asc_cmake_list_command() {
  local operation="$1"
  case "${operation}" in
    configure) ASC_CMAKE_LIST_COMMAND=(cmake --list-presets) ;;
    build) ASC_CMAKE_LIST_COMMAND=(cmake --list-presets=build) ;;
    test) ASC_CMAKE_LIST_COMMAND=(ctest --list-presets) ;;
    *)
      asc_error "unsupported CMake operation: ${operation}"
      return 1
      ;;
  esac
}

asc_cmake_require_preset() {
  local repository_path="$1"
  local operation="$2"
  local preset="$3"
  local preset_output line
  local found=false
  ASC_CMAKE_LIST_COMMAND=()
  asc_cmake_list_command "${operation}" || return 1
  asc_require_command "${ASC_CMAKE_LIST_COMMAND[0]}" || return 1
  if ! preset_output=$(cd -- "${repository_path}" && "${ASC_CMAKE_LIST_COMMAND[@]}" 2>/dev/null); then
    asc_error "could not list ${operation} presets in ${repository_path##*/}"
    return 1
  fi
  while IFS= read -r line; do
    if [[ "${line}" =~ \"([^\"]+)\" && "${BASH_REMATCH[1]}" == "${preset}" ]]; then
      found=true
      break
    fi
  done <<<"${preset_output}"
  if [[ "${found}" != true ]]; then
    asc_error "${operation} preset '${preset}' not found in ${repository_path##*/}"
    return 1
  fi
}

asc_cmake_run() {
  local operation="$1"
  local repository_name="$2"
  local preset="$3"
  shift 3
  local repository_path
  local label=""
  local output_on_failure=false
  local -a targets=()
  local -a command=()
  while (($# > 0)); do
    case "$1" in
      --target)
        (($# >= 2)) || return 2
        targets+=("$2")
        shift 2
        ;;
      --label)
        (($# >= 2)) || return 2
        label="$2"
        shift 2
        ;;
      --output-on-failure)
        output_on_failure=true
        shift
        ;;
      *)
        asc_error "unsupported internal CMake option: $1"
        return 2
        ;;
    esac
  done
  asc_cmake_validate_value preset "${preset}" || return 1
  repository_path=$(asc_cmake_repository "${repository_name}" "$([[ "${operation}" == configure ]] && printf true || printf false)") || return 1
  case "${operation}" in
    configure) command=(cmake --preset "${preset}") ;;
    build)
      command=(cmake --build --preset "${preset}")
      if ((${#targets[@]} > 0)); then
        command+=(--target)
        local target
        for target in "${targets[@]}"; do
          asc_cmake_validate_value target "${target}" || return 1
          command+=("${target}")
        done
      fi
      ;;
    test)
      command=(ctest --preset "${preset}")
      [[ "${output_on_failure}" == false ]] || command+=(--output-on-failure)
      if [[ -n "${label}" ]]; then
        asc_cmake_validate_value label "${label}" || return 1
        command+=(-L "${label}")
      fi
      ;;
    *)
      asc_error "unsupported CMake operation: ${operation}"
      return 1
      ;;
  esac
  asc_cmake_require_preset "${repository_path}" "${operation}" "${preset}" || return 1
  asc_require_command "${command[0]}" || return 1
  (cd -- "${repository_path}" && "${command[@]}")
}

asc_cmake_describe() {
  local value
  local rendered=""
  for value in "$@"; do
    printf -v value '%q' "${value}"
    rendered+="${rendered:+ }${value}"
  done
  printf '%s\n' "${rendered}"
}

asc_cmake_workflow() {
  local repository="$1"
  local configure_preset="$2"
  local build_preset="$3"
  local test_preset="$4"
  local operation preset
  local -a command=()
  asc_cmake_repository "${repository}" true >/dev/null || return 1
  for operation in configure build test; do
    case "${operation}" in
      configure) preset="${configure_preset}"; command=(cmake --preset "${preset}") ;;
      build) preset="${build_preset}"; command=(cmake --build --preset "${preset}") ;;
      test) preset="${test_preset}"; command=(ctest --preset "${preset}") ;;
    esac
    printf 'cmake workflow: %s: %s\n' "${operation}" "$(asc_cmake_describe "${command[@]}")" >&2
    asc_cmake_run "${operation}" "${repository}" "${preset}" || return $?
  done
}

asc_cmake_extract_presets() {
  local output="$1"
  local line name
  local -A seen=()
  while IFS= read -r line; do
    [[ "${line}" =~ \"([^\"]+)\" ]] || continue
    name="${BASH_REMATCH[1]}"
    [[ -v "seen[${name}]" ]] || {
      seen["${name}"]=1
      printf '%s\n' "${name}"
    }
  done <<<"${output}"
}

asc_cmake_presets() {
  local repository="$1"
  local json_output="$2"
  local repository_path operation output name
  local first_name
  local -a names=()
  repository_path=$(asc_cmake_repository "${repository}" false) || return 1
  [[ "${json_output}" == true ]] && printf '{"repository":%s' "$(asc_json_quote "${repository}")"
  for operation in configure build test; do
    ASC_CMAKE_LIST_COMMAND=()
    asc_cmake_list_command "${operation}" || return 1
    asc_require_command "${ASC_CMAKE_LIST_COMMAND[0]}" || return 1
    output=$(cd -- "${repository_path}" && "${ASC_CMAKE_LIST_COMMAND[@]}") || return 1
    names=()
    mapfile -t names < <(asc_cmake_extract_presets "${output}" | LC_ALL=C sort)
    if [[ "${json_output}" == true ]]; then
      printf ',"%s":[' "${operation}"
      first_name=true
      for name in "${names[@]}"; do
        [[ "${first_name}" == true ]] || printf ','
        first_name=false
        asc_json_quote "${name}"
      done
      printf ']'
    else
      for name in "${names[@]}"; do
        printf '%s\t%s\n' "${operation}" "${name}"
      done
    fi
  done
  [[ "${json_output}" != true ]] || printf '}\n'
}
