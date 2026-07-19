#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

readonly ASC_VENDOR_MANIFEST_NAME="ASC_CMAKE_MANIFEST.json"
readonly ASC_VENDOR_SOURCE_ID="AI4SciComp/asc-cmake"

asc_vendor_validate_path() {
  local path="$1"
  if [[ -z "${path}" || "${path}" == /* || "${path}" == *\\* ||
    "${path}" == *//* || "${path}" == "." || "${path}" == ".." ||
    "${path}" == ../* || "${path}" == */../* || "${path}" == */.. ||
    "${path}" == "${ASC_VENDOR_MANIFEST_NAME}" ||
    ! "${path}" =~ ^[A-Za-z0-9._/+\ -]+$ ]]; then
    asc_error "unsafe managed path: ${path}"
    return 1
  fi
}

asc_vendor_parse_manifest_files() {
  local character key value path hash
  local -A paths_seen=()
  asc_json_expect '[' || return 1
  asc_json_skip_whitespace
  if [[ "${ASC_JSON_TEXT:ASC_JSON_POSITION:1}" == ']' ]]; then
    ((ASC_JSON_POSITION += 1))
    return 0
  fi
  while true; do
    path=""
    hash=""
    local -A fields_seen=()
    asc_json_expect '{' || return 1
    while true; do
      asc_json_parse_string || return 1
      key="${ASC_JSON_VALUE}"
      [[ ! -v "fields_seen[${key}]" ]] || {
        asc_error "duplicate vendor manifest file field: ${key}"
        return 1
      }
      fields_seen["${key}"]=1
      asc_json_expect ':' || return 1
      asc_json_parse_scalar || return 1
      value="${ASC_JSON_VALUE}"
      [[ "${ASC_JSON_TYPE}" == string ]] || {
        asc_error "vendor manifest file fields must be strings"
        return 1
      }
      case "${key}" in
        path) path="${value}" ;;
        sha256) hash="${value}" ;;
        *)
          asc_error "unknown vendor manifest file field: ${key}"
          return 1
          ;;
      esac
      asc_json_skip_whitespace
      character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
      if [[ "${character}" == '}' ]]; then
        ((ASC_JSON_POSITION += 1))
        break
      fi
      asc_json_expect ',' || return 1
    done
    asc_vendor_validate_path "${path}" || return 1
    [[ "${hash}" =~ ^[0-9a-f]{64}$ ]] || {
      asc_error "invalid vendor manifest SHA-256 for ${path}"
      return 1
    }
    [[ ! -v "paths_seen[${path}]" ]] || {
      asc_error "duplicate vendor manifest path: ${path}"
      return 1
    }
    paths_seen["${path}"]=1
    ASC_VENDOR_MANIFEST_PATHS+=("${path}")
    ASC_VENDOR_MANIFEST_HASHES+=("${hash}")
    asc_json_skip_whitespace
    character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
    if [[ "${character}" == ']' ]]; then
      ((ASC_JSON_POSITION += 1))
      break
    fi
    asc_json_expect ',' || return 1
  done
}

asc_vendor_manifest_read() {
  local target="$1"
  local manifest="${target}/${ASC_VENDOR_MANIFEST_NAME}"
  local content character key value
  local -A seen=()
  ASC_VENDOR_MANIFEST_PATHS=()
  ASC_VENDOR_MANIFEST_HASHES=()
  ASC_VENDOR_MANIFEST_VERSION=""
  ASC_VENDOR_MANIFEST_COMMIT=""
  [[ -e "${manifest}" || -L "${manifest}" ]] || return 2
  [[ -f "${manifest}" && ! -L "${manifest}" ]] || {
    asc_error "vendor manifest is not a regular file"
    return 1
  }
  content=$(<"${manifest}")
  asc_json_begin "${content}"
  asc_json_expect '{' || return 1
  while true; do
    asc_json_skip_whitespace
    if [[ "${ASC_JSON_TEXT:ASC_JSON_POSITION:1}" == '}' ]]; then
      ((ASC_JSON_POSITION += 1))
      break
    fi
    asc_json_parse_string || return 1
    key="${ASC_JSON_VALUE}"
    [[ ! -v "seen[${key}]" ]] || {
      asc_error "duplicate vendor manifest field: ${key}"
      return 1
    }
    seen["${key}"]=1
    asc_json_expect ':' || return 1
    if [[ "${key}" == files ]]; then
      asc_vendor_parse_manifest_files || return 1
    else
      asc_json_parse_scalar || return 1
      value="${ASC_JSON_VALUE}"
      case "${key}" in
        schemaVersion)
          [[ "${ASC_JSON_TYPE}" == number && "${value}" == 1 ]] || {
            asc_error "unsupported vendor manifest schema: ${value}"
            return 1
          }
          ;;
        sourceRepository)
          [[ "${ASC_JSON_TYPE}" == string && "${value}" == "${ASC_VENDOR_SOURCE_ID}" ]] || {
            asc_error "invalid vendor manifest source repository"
            return 1
          }
          ;;
        version)
          [[ "${ASC_JSON_TYPE}" == string && -n "${value}" ]] || return 1
          ASC_VENDOR_MANIFEST_VERSION="${value}"
          ;;
        commit)
          [[ "${ASC_JSON_TYPE}" == string && -n "${value}" ]] || return 1
          ASC_VENDOR_MANIFEST_COMMIT="${value}"
          ;;
        *)
          asc_error "unknown vendor manifest field: ${key}"
          return 1
          ;;
      esac
    fi
    asc_json_skip_whitespace
    character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
    [[ "${character}" == '}' ]] || asc_json_expect ',' || return 1
  done
  asc_json_finish || return 1
  for key in schemaVersion sourceRepository version commit files; do
    [[ -v "seen[${key}]" ]] || {
      asc_error "vendor manifest is missing field: ${key}"
      return 1
    }
  done
}

asc_vendor_distribution_contract() {
  local source_root="$1"
  local contract="${source_root}/distribution.json"
  local content character key value
  local -A path_seen=()
  [[ -f "${contract}" && ! -L "${contract}" ]] || {
    asc_error "distribution.json must be a regular nonsymlink file"
    return 1
  }
  content=$(<"${contract}")
  asc_json_begin "${content}"
  asc_json_expect '{' || return 1
  asc_json_parse_string || return 1
  key="${ASC_JSON_VALUE}"
  [[ "${key}" == files ]] || {
    asc_error "distribution.json must contain only a files array"
    return 1
  }
  asc_json_expect ':' || return 1
  asc_json_expect '[' || return 1
  asc_json_skip_whitespace
  if [[ "${ASC_JSON_TEXT:ASC_JSON_POSITION:1}" != ']' ]]; then
    while true; do
      asc_json_parse_string || return 1
      value="${ASC_JSON_VALUE}"
      asc_vendor_validate_path "${value}" || return 1
      [[ ! -v "path_seen[${value}]" ]] || {
        asc_error "distribution contains duplicate path: ${value}"
        return 1
      }
      path_seen["${value}"]=1
      ASC_VENDOR_SOURCE_PATHS+=("${value}")
      asc_json_skip_whitespace
      character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
      if [[ "${character}" == ']' ]]; then break; fi
      asc_json_expect ',' || return 1
    done
  fi
  asc_json_expect ']' || return 1
  asc_json_expect '}' || return 1
  asc_json_finish
}

asc_vendor_source_files() {
  local source_root="$1"
  local path relative digest
  local -a discovered=()
  local -A seen=()
  ASC_VENDOR_SOURCE_PATHS=()
  ASC_VENDOR_SOURCE_HASHES=()
  if [[ -e "${source_root}/distribution.json" || -L "${source_root}/distribution.json" ]]; then
    asc_vendor_distribution_contract "${source_root}" || return 1
  else
    [[ ! -f "${source_root}/LICENSE" || -L "${source_root}/LICENSE" ]] || ASC_VENDOR_SOURCE_PATHS+=(LICENSE)
    if [[ -d "${source_root}/modules" ]]; then
      if [[ -n "$(find -P "${source_root}/modules" -type l -print -quit)" ]]; then
        asc_error "distribution contains a symlink"
        return 1
      fi
      mapfile -d '' -t discovered < <(find -P "${source_root}/modules" -type f -name '*.cmake' -print0 | LC_ALL=C sort -z)
      for path in "${discovered[@]}"; do
        ASC_VENDOR_SOURCE_PATHS+=("${path#"${source_root}/"}")
      done
    fi
  fi
  ((${#ASC_VENDOR_SOURCE_PATHS[@]} > 0)) || {
    asc_error "asc-cmake distribution contains no managed files"
    return 1
  }
  mapfile -t ASC_VENDOR_SOURCE_PATHS < <(printf '%s\n' "${ASC_VENDOR_SOURCE_PATHS[@]}" | LC_ALL=C sort)
  for relative in "${ASC_VENDOR_SOURCE_PATHS[@]}"; do
    asc_vendor_validate_path "${relative}" || return 1
    [[ ! -v "seen[${relative}]" ]] || {
      asc_error "distribution contains duplicate path: ${relative}"
      return 1
    }
    seen["${relative}"]=1
    path="${source_root}/${relative}"
    [[ -f "${path}" && ! -L "${path}" ]] || {
      asc_error "distribution file is not a regular nonsymlink file: ${relative}"
      return 1
    }
    digest=$(sha256sum -- "${path}")
    ASC_VENDOR_SOURCE_HASHES+=("${digest%% *}")
  done
}

asc_vendor_source_version() {
  local source_root="$1"
  local content tag
  ASC_VENDOR_SOURCE_VERSION=""
  if [[ -f "${source_root}/VERSION" && ! -L "${source_root}/VERSION" ]]; then
    ASC_VENDOR_SOURCE_VERSION=$(<"${source_root}/VERSION")
    ASC_VENDOR_SOURCE_VERSION="${ASC_VENDOR_SOURCE_VERSION//$'\r'/}"
    ASC_VENDOR_SOURCE_VERSION="${ASC_VENDOR_SOURCE_VERSION//$'\n'/}"
  fi
  if [[ -z "${ASC_VENDOR_SOURCE_VERSION}" && -f "${source_root}/CMakeLists.txt" && ! -L "${source_root}/CMakeLists.txt" ]]; then
    content=$(<"${source_root}/CMakeLists.txt")
    if [[ "${content}" =~ VERSION[[:space:]]+([0-9]+(\.[0-9]+){1,3}([-+][A-Za-z0-9.-]+)?) ]]; then
      ASC_VENDOR_SOURCE_VERSION="${BASH_REMATCH[1]}"
    fi
  fi
  if [[ -z "${ASC_VENDOR_SOURCE_VERSION}" ]]; then
    tag=$(git -C "${source_root}" describe --tags --exact-match HEAD 2>/dev/null || true)
    ASC_VENDOR_SOURCE_VERSION="${tag#v}"
  fi
  [[ -n "${ASC_VENDOR_SOURCE_VERSION}" ]] || {
    asc_error "asc-cmake version not found in VERSION, CMake project(), or exact tag"
    return 1
  }
}

asc_vendor_source_load() {
  local source_path="$1"
  local requested_ref="$2"
  local allow_dirty="$3"
  local inside resolved origin status
  [[ -n "${source_path}" ]] || source_path="${ASC_WORKSPACE}/${ASC_CMAKE_SOURCE_REPOSITORY}"
  ASC_VENDOR_SOURCE_ROOT=$(realpath -m -- "${source_path}")
  [[ -d "${ASC_VENDOR_SOURCE_ROOT}" && ! -L "${ASC_VENDOR_SOURCE_ROOT}" ]] || {
    asc_error "asc-cmake source is not a regular directory: ${ASC_VENDOR_SOURCE_ROOT}"
    return 1
  }
  inside=$(git -C "${ASC_VENDOR_SOURCE_ROOT}" rev-parse --is-inside-work-tree 2>/dev/null || true)
  [[ "${inside}" == true ]] || {
    asc_error "asc-cmake source is not a Git working tree: ${ASC_VENDOR_SOURCE_ROOT}"
    return 1
  }
  ASC_VENDOR_SOURCE_COMMIT=$(git -C "${ASC_VENDOR_SOURCE_ROOT}" rev-parse HEAD) || return 1
  if [[ -n "${requested_ref}" ]]; then
    resolved=$(git -C "${ASC_VENDOR_SOURCE_ROOT}" rev-parse --verify "${requested_ref}^{commit}" 2>/dev/null || true)
    [[ "${resolved}" == "${ASC_VENDOR_SOURCE_COMMIT}" ]] || {
      asc_error "requested ref does not match checked-out source commit: ${requested_ref}"
      return 1
    }
  fi
  origin=$(git -C "${ASC_VENDOR_SOURCE_ROOT}" remote get-url origin 2>/dev/null || true)
  origin="${origin%.git}"
  if [[ -n "${origin}" && "${origin}" != *github.com/AI4SciComp/asc-cmake && "${origin}" != *github.com:AI4SciComp/asc-cmake ]]; then
    asc_error "asc-cmake source origin is not ${ASC_VENDOR_SOURCE_ID}: ${origin}"
    return 1
  fi
  status=$(git -C "${ASC_VENDOR_SOURCE_ROOT}" status --porcelain --untracked-files=normal) || return 1
  ASC_VENDOR_SOURCE_DIRTY=false
  [[ -z "${status}" ]] || ASC_VENDOR_SOURCE_DIRTY=true
  if [[ "${ASC_VENDOR_SOURCE_DIRTY}" == true && "${allow_dirty}" != true ]]; then
    asc_error "refusing to apply from a dirty asc-cmake source"
    return 1
  fi
  asc_vendor_source_files "${ASC_VENDOR_SOURCE_ROOT}" || return 1
  asc_vendor_source_version "${ASC_VENDOR_SOURCE_ROOT}" || return 1
}

asc_vendor_target() {
  local repository="$1"
  local repository_path="${ASC_WORKSPACE}/${repository}"
  local current part
  asc_validate_repository_name "${repository}" || return 1
  asc_is_git_repository "${repository_path}" || {
    asc_error "local Git worktree not found: ${repository}"
    return 1
  }
  current="${repository_path}"
  local -a directory_parts=()
  IFS=/ read -r -a directory_parts <<<"${ASC_CMAKE_VENDOR_DIRECTORY}"
  for part in "${directory_parts[@]}"; do
    current+="/${part}"
    [[ ! -L "${current}" ]] || {
      asc_error "refusing symlinked vendor path: ${current}"
      return 1
    }
  done
  ASC_VENDOR_TARGET="${repository_path}/${ASC_CMAKE_VENDOR_DIRECTORY}"
}

asc_vendor_inspect_manifest() {
  local target="$1"
  local index path actual
  ASC_VENDOR_LOCALLY_MODIFIED=false
  ASC_VENDOR_EXTRA_FILES=()
  local -A managed=()
  for index in "${!ASC_VENDOR_MANIFEST_PATHS[@]}"; do
    path="${target}/${ASC_VENDOR_MANIFEST_PATHS[index]}"
    managed["${ASC_VENDOR_MANIFEST_PATHS[index]}"]=1
    if [[ ! -f "${path}" || -L "${path}" ]]; then
      ASC_VENDOR_LOCALLY_MODIFIED=true
      continue
    fi
    actual=$(sha256sum -- "${path}")
    [[ "${actual%% *}" == "${ASC_VENDOR_MANIFEST_HASHES[index]}" ]] || ASC_VENDOR_LOCALLY_MODIFIED=true
  done
  [[ -d "${target}" ]] || return 0
  while IFS= read -r -d '' path; do
    path="${path#"${target}/"}"
    [[ "${path}" == "${ASC_VENDOR_MANIFEST_NAME}" || -v "managed[${path}]" ]] || ASC_VENDOR_EXTRA_FILES+=("${path}")
  done < <(find -P "${target}" \( -type f -o -type l \) -print0 | LC_ALL=C sort -z)
}

asc_vendor_status() {
  local repository="$1"
  local read_status index state detail=""
  asc_vendor_target "${repository}" || {
    ASC_VENDOR_STATUS_STATE=manifest-invalid
    ASC_VENDOR_STATUS_DETAIL="invalid consumer repository"
    return 0
  }
  if asc_vendor_manifest_read "${ASC_VENDOR_TARGET}"; then
    read_status=0
  else
    read_status=$?
  fi
  if ((read_status == 2)); then
    ASC_VENDOR_STATUS_STATE=not-vendored
    ASC_VENDOR_STATUS_DETAIL=""
    ASC_VENDOR_EXTRA_FILES=()
    return 0
  fi
  if ((read_status != 0)); then
    ASC_VENDOR_STATUS_STATE=manifest-invalid
    ASC_VENDOR_STATUS_DETAIL="invalid vendor manifest"
    ASC_VENDOR_EXTRA_FILES=()
    return 0
  fi
  asc_vendor_inspect_manifest "${ASC_VENDOR_TARGET}" || return 1
  if [[ "${ASC_VENDOR_LOCALLY_MODIFIED}" == true ]]; then
    ASC_VENDOR_STATUS_STATE=locally-modified
    ASC_VENDOR_STATUS_DETAIL="managed files differ from the manifest"
    return 0
  fi
  if ! asc_vendor_source_load "" "" true >/dev/null 2>&1; then
    ASC_VENDOR_STATUS_STATE=source-unavailable
    ASC_VENDOR_STATUS_DETAIL="default asc-cmake source is unavailable"
    return 0
  fi
  state=current
  [[ "${ASC_VENDOR_MANIFEST_VERSION}" == "${ASC_VENDOR_SOURCE_VERSION}" &&
    "${ASC_VENDOR_MANIFEST_COMMIT}" == "${ASC_VENDOR_SOURCE_COMMIT}" &&
    ${#ASC_VENDOR_MANIFEST_PATHS[@]} -eq ${#ASC_VENDOR_SOURCE_PATHS[@]} ]] || state=source-newer
  if [[ "${state}" == current ]]; then
    for index in "${!ASC_VENDOR_SOURCE_PATHS[@]}"; do
      if [[ "${ASC_VENDOR_MANIFEST_PATHS[index]}" != "${ASC_VENDOR_SOURCE_PATHS[index]}" ||
        "${ASC_VENDOR_MANIFEST_HASHES[index]}" != "${ASC_VENDOR_SOURCE_HASHES[index]}" ]]; then
        state=source-newer
        break
      fi
    done
  fi
  [[ "${ASC_VENDOR_SOURCE_DIRTY}" != true ]] || detail="source checkout is dirty"
  ASC_VENDOR_STATUS_STATE="${state}"
  ASC_VENDOR_STATUS_DETAIL="${detail}"
}

asc_vendor_plan() {
  local repository="$1"
  local source_path="$2"
  local requested_ref="$3"
  local read_status index path action
  local -A old_hashes=()
  asc_vendor_target "${repository}" || return 1
  asc_vendor_source_load "${source_path}" "${requested_ref}" true || return 1
  if asc_vendor_manifest_read "${ASC_VENDOR_TARGET}"; then
    read_status=0
  else
    read_status=$?
  fi
  ((read_status == 0 || read_status == 2)) || return 1
  if ((read_status == 0)); then
    asc_vendor_inspect_manifest "${ASC_VENDOR_TARGET}" || return 1
    [[ "${ASC_VENDOR_LOCALLY_MODIFIED}" != true ]] || {
      asc_error "locally modified managed vendored files must be resolved before planning"
      return 1
    }
    for index in "${!ASC_VENDOR_MANIFEST_PATHS[@]}"; do
      old_hashes["${ASC_VENDOR_MANIFEST_PATHS[index]}"]="${ASC_VENDOR_MANIFEST_HASHES[index]}"
    done
  fi
  ASC_VENDOR_PLAN_PATHS=()
  ASC_VENDOR_PLAN_ACTIONS=()
  for index in "${!ASC_VENDOR_SOURCE_PATHS[@]}"; do
    path="${ASC_VENDOR_SOURCE_PATHS[index]}"
    action=add
    if [[ -v "old_hashes[${path}]" ]]; then
      [[ "${old_hashes[${path}]}" == "${ASC_VENDOR_SOURCE_HASHES[index]}" ]] && action=preserve || action=replace
      unset 'old_hashes[$path]'
    fi
    ASC_VENDOR_PLAN_PATHS+=("${path}")
    ASC_VENDOR_PLAN_ACTIONS+=("${action}")
  done
  for path in "${!old_hashes[@]}"; do
    ASC_VENDOR_PLAN_PATHS+=("${path}")
    ASC_VENDOR_PLAN_ACTIONS+=(remove)
  done
  local -a sorted=()
  mapfile -t sorted < <(for index in "${!ASC_VENDOR_PLAN_PATHS[@]}"; do printf '%s\t%s\n' "${ASC_VENDOR_PLAN_PATHS[index]}" "${ASC_VENDOR_PLAN_ACTIONS[index]}"; done | LC_ALL=C sort)
  ASC_VENDOR_PLAN_PATHS=()
  ASC_VENDOR_PLAN_ACTIONS=()
  for path in "${sorted[@]}"; do
    ASC_VENDOR_PLAN_PATHS+=("${path%%$'\t'*}")
    ASC_VENDOR_PLAN_ACTIONS+=("${path#*$'\t'}")
  done
  ASC_VENDOR_PLAN_REPOSITORY="${repository}"
  ASC_VENDOR_PLAN_FINGERPRINT=$(for index in "${!ASC_VENDOR_PLAN_PATHS[@]}"; do printf '%s\t%s\n' "${ASC_VENDOR_PLAN_PATHS[index]}" "${ASC_VENDOR_PLAN_ACTIONS[index]}"; done | sha256sum)
  ASC_VENDOR_PLAN_FINGERPRINT="${ASC_VENDOR_SOURCE_COMMIT}:${ASC_VENDOR_PLAN_FINGERPRINT%% *}"
}

asc_vendor_manifest_write() {
  local destination="$1"
  local index
  {
    printf '{\n  "schemaVersion": 1,\n'
    printf '  "sourceRepository": %s,\n' "$(asc_json_quote "${ASC_VENDOR_SOURCE_ID}")"
    printf '  "version": %s,\n' "$(asc_json_quote "${ASC_VENDOR_SOURCE_VERSION}")"
    printf '  "commit": %s,\n' "$(asc_json_quote "${ASC_VENDOR_SOURCE_COMMIT}")"
    printf '  "files": ['
    for index in "${!ASC_VENDOR_SOURCE_PATHS[@]}"; do
      ((index == 0)) || printf ','
      printf '\n    {"path": %s, "sha256": %s}' \
        "$(asc_json_quote "${ASC_VENDOR_SOURCE_PATHS[index]}")" \
        "$(asc_json_quote "${ASC_VENDOR_SOURCE_HASHES[index]}")"
    done
    ((${#ASC_VENDOR_SOURCE_PATHS[@]} == 0)) || printf '\n  '
    printf ']\n}\n'
  } >"${destination}"
  chmod 0644 -- "${destination}"
}

asc_vendor_rollback() {
  local index destination backup installed
  for ((index = ${#ASC_VENDOR_CHANGED_DESTINATIONS[@]} - 1; index >= 0; index--)); do
    destination="${ASC_VENDOR_CHANGED_DESTINATIONS[index]}"
    backup="${ASC_VENDOR_CHANGED_BACKUPS[index]}"
    installed="${ASC_VENDOR_CHANGED_INSTALLED[index]}"
    [[ "${installed}" != true ]] || rm -f -- "${destination}"
    if [[ -n "${backup}" && -e "${backup}" ]]; then
      mkdir -p -- "${destination%/*}"
      mv -- "${backup}" "${destination}" || true
    fi
  done
}

asc_vendor_apply_current_plan() {
  local parent stage backup index path action destination saved
  parent="${ASC_VENDOR_TARGET%/*}"
  mkdir -p -- "${parent}"
  stage=$(mktemp -d "${parent}/.asc-vendor-stage.XXXXXXXX") || return 1
  backup=$(mktemp -d "${parent}/.asc-vendor-backup.XXXXXXXX") || {
    rm -r -- "${stage}"
    return 1
  }
  ASC_VENDOR_CHANGED_DESTINATIONS=()
  ASC_VENDOR_CHANGED_BACKUPS=()
  ASC_VENDOR_CHANGED_INSTALLED=()
  for index in "${!ASC_VENDOR_SOURCE_PATHS[@]}"; do
    path="${ASC_VENDOR_SOURCE_PATHS[index]}"
    destination="${stage}/${path}"
    mkdir -p -- "${destination%/*}"
    install -m 0644 -- "${ASC_VENDOR_SOURCE_ROOT}/${path}" "${destination}" || {
      rm -r -- "${stage}" "${backup}"
      return 1
    }
  done
  asc_vendor_manifest_write "${stage}/${ASC_VENDOR_MANIFEST_NAME}" || {
    rm -r -- "${stage}" "${backup}"
    return 1
  }
  mkdir -p -- "${ASC_VENDOR_TARGET}"
  for index in "${!ASC_VENDOR_PLAN_PATHS[@]}"; do
    path="${ASC_VENDOR_PLAN_PATHS[index]}"
    action="${ASC_VENDOR_PLAN_ACTIONS[index]}"
    [[ "${action}" != preserve ]] || continue
    destination="${ASC_VENDOR_TARGET}/${path}"
    saved=""
    if [[ -e "${destination}" || -L "${destination}" ]]; then
      saved="${backup}/${path}"
      mkdir -p -- "${saved%/*}"
      mv -- "${destination}" "${saved}" || {
        asc_vendor_rollback
        rm -r -- "${stage}" "${backup}"
        return 1
      }
    fi
    ASC_VENDOR_CHANGED_DESTINATIONS+=("${destination}")
    ASC_VENDOR_CHANGED_BACKUPS+=("${saved}")
    ASC_VENDOR_CHANGED_INSTALLED+=(false)
    if [[ "${action}" != remove ]]; then
      mkdir -p -- "${destination%/*}"
      if ! mv -- "${stage}/${path}" "${destination}"; then
        asc_vendor_rollback
        rm -r -- "${stage}" "${backup}"
        return 1
      fi
      ASC_VENDOR_CHANGED_INSTALLED[${#ASC_VENDOR_CHANGED_INSTALLED[@]}-1]=true
    fi
  done
  destination="${ASC_VENDOR_TARGET}/${ASC_VENDOR_MANIFEST_NAME}"
  saved=""
  if [[ -e "${destination}" || -L "${destination}" ]]; then
    saved="${backup}/${ASC_VENDOR_MANIFEST_NAME}"
    mv -- "${destination}" "${saved}" || {
      asc_vendor_rollback
      rm -r -- "${stage}" "${backup}"
      return 1
    }
  fi
  ASC_VENDOR_CHANGED_DESTINATIONS+=("${destination}")
  ASC_VENDOR_CHANGED_BACKUPS+=("${saved}")
  ASC_VENDOR_CHANGED_INSTALLED+=(false)
  if ! mv -- "${stage}/${ASC_VENDOR_MANIFEST_NAME}" "${destination}"; then
    asc_vendor_rollback
    rm -r -- "${stage}" "${backup}"
    return 1
  fi
  ASC_VENDOR_CHANGED_INSTALLED[${#ASC_VENDOR_CHANGED_INSTALLED[@]}-1]=true
  rm -r -- "${stage}" "${backup}"
}

asc_vendor_print_plan() {
  local index
  printf 'asc-cmake %s (%s) -> %s\n' "${ASC_VENDOR_SOURCE_VERSION}" "${ASC_VENDOR_SOURCE_COMMIT}" "${ASC_VENDOR_PLAN_REPOSITORY}"
  for index in "${!ASC_VENDOR_PLAN_PATHS[@]}"; do
    printf '  %s\t%s\n' "${ASC_VENDOR_PLAN_ACTIONS[index]}" "${ASC_VENDOR_PLAN_PATHS[index]}"
  done
}

asc_vendor_plan_json() {
  local index
  printf '{"repository":%s,"source":%s,"version":%s,"commit":%s,"actions":[' \
    "$(asc_json_quote "${ASC_VENDOR_PLAN_REPOSITORY}")" \
    "$(asc_json_quote "${ASC_VENDOR_SOURCE_ROOT}")" \
    "$(asc_json_quote "${ASC_VENDOR_SOURCE_VERSION}")" \
    "$(asc_json_quote "${ASC_VENDOR_SOURCE_COMMIT}")"
  for index in "${!ASC_VENDOR_PLAN_PATHS[@]}"; do
    ((index == 0)) || printf ','
    printf '{"path":%s,"action":%s}' \
      "$(asc_json_quote "${ASC_VENDOR_PLAN_PATHS[index]}")" \
      "$(asc_json_quote "${ASC_VENDOR_PLAN_ACTIONS[index]}")"
  done
  printf ']}\n'
}
