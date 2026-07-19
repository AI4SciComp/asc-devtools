#!/usr/bin/env bash
# asc-devtools managed file

readonly ASC_UPDATE_REPOSITORY="AI4SciComp/asc-devtools"
readonly ASC_UPDATE_ARCHIVE="asc-devtools-shell.tar.gz"
readonly ASC_UPDATE_ROOT="asc-devtools-shell"

asc_update_parse_assets() {
  local character key name="" url="" value_type
  asc_json_expect '[' || return 1
  asc_json_skip_whitespace
  if [[ "${ASC_JSON_TEXT:ASC_JSON_POSITION:1}" == ']' ]]; then
    ((ASC_JSON_POSITION += 1))
    return 0
  fi
  while true; do
    name=""
    url=""
    asc_json_expect '{' || return 1
    asc_json_skip_whitespace
    while [[ "${ASC_JSON_TEXT:ASC_JSON_POSITION:1}" != '}' ]]; do
      asc_json_parse_string || return 1
      key="${ASC_JSON_VALUE}"
      asc_json_expect ':' || return 1
      case "${key}" in
        name | url)
          asc_json_parse_scalar || return 1
          value_type="${ASC_JSON_TYPE}"
          [[ "${value_type}" == string ]] || return 1
          [[ "${key}" == name ]] && name="${ASC_JSON_VALUE}" || url="${ASC_JSON_VALUE}"
          ;;
        *) asc_json_skip_value || return 1 ;;
      esac
      asc_json_skip_whitespace
      character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
      [[ "${character}" == '}' ]] && break
      asc_json_expect ',' || return 1
    done
    asc_json_expect '}' || return 1
    case "${name}" in
      "${ASC_UPDATE_ARCHIVE}") ASC_UPDATE_ARCHIVE_URL="${url}" ;;
      SHA256SUMS) ASC_UPDATE_SUMS_URL="${url}" ;;
    esac
    asc_json_skip_whitespace
    character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
    if [[ "${character}" == ']' ]]; then
      ((ASC_JSON_POSITION += 1))
      return 0
    fi
    asc_json_expect ',' || return 1
  done
}

asc_update_parse_release() {
  local content="$1"
  local character key value_type
  ASC_UPDATE_TAG=""
  ASC_UPDATE_ARCHIVE_URL=""
  ASC_UPDATE_SUMS_URL=""
  asc_json_begin "${content}"
  asc_json_expect '{' || return 1
  asc_json_skip_whitespace
  while [[ "${ASC_JSON_TEXT:ASC_JSON_POSITION:1}" != '}' ]]; do
    asc_json_parse_string || return 1
    key="${ASC_JSON_VALUE}"
    asc_json_expect ':' || return 1
    case "${key}" in
      tag_name)
        asc_json_parse_scalar || return 1
        value_type="${ASC_JSON_TYPE}"
        [[ "${value_type}" == string ]] || return 1
        ASC_UPDATE_TAG="${ASC_JSON_VALUE}"
        ;;
      assets) asc_update_parse_assets || return 1 ;;
      *) asc_json_skip_value || return 1 ;;
    esac
    asc_json_skip_whitespace
    character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
    [[ "${character}" == '}' ]] && break
    asc_json_expect ',' || return 1
  done
  asc_json_expect '}' || return 1
  asc_json_finish || return 1
  [[ "${ASC_UPDATE_TAG}" =~ ^v?[0-9]+(\.[0-9]+)*$ ]] || {
    asc_error "latest release has unsupported tag: ${ASC_UPDATE_TAG:-missing}"
    return 1
  }
  ASC_UPDATE_VERSION="${ASC_UPDATE_TAG#v}"
  [[ -n "${ASC_UPDATE_ARCHIVE_URL}" ]] || {
    asc_error "release ${ASC_UPDATE_TAG} does not contain ${ASC_UPDATE_ARCHIVE}"
    return 1
  }
  [[ -n "${ASC_UPDATE_SUMS_URL}" ]] || {
    asc_error "release ${ASC_UPDATE_TAG} does not contain SHA256SUMS"
    return 1
  }
}

asc_update_compare_versions() {
  local current="${1#v}" latest="${2#v}"
  local index width left right
  local -a current_parts latest_parts
  if [[ ! "${current}" =~ ^[0-9]+(\.[0-9]+)*$ ]]; then
    ASC_UPDATE_COMPARISON=-1
    return 0
  fi
  IFS=. read -r -a current_parts <<<"${current}"
  IFS=. read -r -a latest_parts <<<"${latest}"
  width=${#current_parts[@]}
  ((${#latest_parts[@]} <= width)) || width=${#latest_parts[@]}
  ASC_UPDATE_COMPARISON=0
  for ((index = 0; index < width; index++)); do
    left=$((10#${current_parts[index]:-0}))
    right=$((10#${latest_parts[index]:-0}))
    if ((left < right)); then ASC_UPDATE_COMPARISON=-1; return 0; fi
    if ((left > right)); then ASC_UPDATE_COMPARISON=1; return 0; fi
  done
}

asc_update_download() {
  local url="$1" accept="$2" output="$3" limit="$4" headers="$5"
  local size
  local -a arguments=(
    --silent --show-error --max-time 30
    --dump-header "${headers}" --output "${output}" --write-out '%{http_code}'
    --header "Accept: ${accept}"
    --header 'X-GitHub-Api-Version: 2022-11-28'
    --header "User-Agent: asc-devtools/${ASC_VERSION}"
  )
  [[ -z "${ASC_GITHUB_TOKEN_VALUE}" ]] || arguments+=(--header "Authorization: Bearer ${ASC_GITHUB_TOKEN_VALUE}")
  arguments+=("${url}")
  if ! ASC_UPDATE_HTTP_STATUS=$(curl "${arguments[@]}"); then
    asc_error "GitHub release request failed"
    return 1
  fi
  size=$(wc -c <"${output}")
  if ((size > limit)); then
    asc_error "release response exceeded ${limit} bytes"
    return 1
  fi
}

asc_update_expected_checksum() {
  local sums="$1"
  local digest filename extra
  ASC_UPDATE_EXPECTED_HASH=""
  while read -r digest filename extra; do
    filename="${filename#\*}"
    if [[ "${filename}" == "${ASC_UPDATE_ARCHIVE}" && -z "${extra:-}" && "${digest}" =~ ^[0-9a-fA-F]{64}$ ]]; then
      ASC_UPDATE_EXPECTED_HASH="${digest,,}"
      break
    fi
  done <"${sums}"
  [[ -n "${ASC_UPDATE_EXPECTED_HASH}" ]] || {
    asc_error "SHA256SUMS has no valid entry for ${ASC_UPDATE_ARCHIVE}"
    return 1
  }
}

asc_update_extract() {
  local archive="$1" destination="$2"
  local member line entry_type listing verbose_listing extracted_size
  if ! listing=$(tar --quoting-style=escape -tzf "${archive}"); then
    asc_error "cannot list release archive"
    return 1
  fi
  while IFS= read -r member; do
    [[ -n "${member}" && "${member}" != /* && "${member}" != *\\* && "${member}" =~ ^[A-Za-z0-9._/-]+$ ]] || {
      asc_error "unsafe archive path: ${member}"
      return 1
    }
    [[ "${member}" == "${ASC_UPDATE_ROOT}" || "${member}" == "${ASC_UPDATE_ROOT}/" || "${member}" == "${ASC_UPDATE_ROOT}/"* ]] || {
      asc_error "unexpected archive path: ${member}"
      return 1
    }
    [[ "/${member}/" != */../* ]] || {
      asc_error "unsafe archive traversal: ${member}"
      return 1
    }
  done <<<"${listing}"
  if ! verbose_listing=$(tar --quoting-style=escape -tvzf "${archive}"); then
    asc_error "cannot inspect release archive"
    return 1
  fi
  while IFS= read -r line; do
    entry_type="${line:0:1}"
    [[ "${entry_type}" == - || "${entry_type}" == d ]] || {
      asc_error "release archive contains a link or special file"
      return 1
    }
  done <<<"${verbose_listing}"
  mkdir -p -- "${destination}"
  tar --extract --gzip --file "${archive}" --directory "${destination}" \
    --no-same-owner --no-same-permissions
  if find "${destination}" -type l -print -quit | grep -q .; then
    asc_error "release archive extracted a symbolic link"
    return 1
  fi
  extracted_size=$(find "${destination}" -type f -printf '%s\n' | awk '{total += $1} END {print total + 0}')
  if ((extracted_size > 268435456)); then
    asc_error "archive exceeds extracted size limit"
    return 1
  fi
}

asc_update_run() {
  local check_only="$1" assume_yes="$2" prefix="$3"
  local base_url="${ASC_GITHUB_API_URL:-https://api.github.com}"
  local temporary metadata headers sums archive body actual root answer bundle_version_output
  asc_require_command curl || return 1
  asc_require_command sha256sum || return 1
  asc_require_command tar || return 1
  temporary=$(mktemp -d "${TMPDIR:-/tmp}/asc-update.XXXXXXXX") || return 1
  metadata="${temporary}/release.json"
  headers="${temporary}/headers"
  if ! asc_update_download \
    "${base_url%/}/repos/${ASC_UPDATE_REPOSITORY}/releases/latest" \
    'application/vnd.github+json' "${metadata}" 4194304 "${headers}"; then
    rm -r -- "${temporary}"
    return 1
  fi
  if [[ "${ASC_UPDATE_HTTP_STATUS}" == 404 ]]; then
    rm -r -- "${temporary}"
    asc_error "no published asc release is available"
    return 1
  fi
  if [[ ! "${ASC_UPDATE_HTTP_STATUS}" =~ ^2[0-9][0-9]$ ]]; then
    asc_github_api_error "${ASC_UPDATE_HTTP_STATUS}" "${headers}"
    rm -r -- "${temporary}"
    return 1
  fi
  body=$(<"${metadata}")
  if ! asc_update_parse_release "${body}"; then
    rm -r -- "${temporary}"
    return 1
  fi
  asc_update_compare_versions "${ASC_VERSION}" "${ASC_UPDATE_VERSION}"
  if ((ASC_UPDATE_COMPARISON == 0)); then
    printf 'asc %s is up to date.\n' "${ASC_VERSION}"
    rm -r -- "${temporary}"
    return 0
  fi
  if ((ASC_UPDATE_COMPARISON > 0)); then
    printf 'asc %s is newer than the latest release (%s); no update performed.\n' "${ASC_VERSION}" "${ASC_UPDATE_VERSION}"
    rm -r -- "${temporary}"
    return 0
  fi
  printf 'asc %s is available (current: %s).\n' "${ASC_UPDATE_VERSION}" "${ASC_VERSION}"
  if [[ "${check_only}" == true ]]; then
    rm -r -- "${temporary}"
    return 0
  fi
  [[ -n "${prefix}" ]] || prefix="${ASC_INSTALL_ROOT}"
  [[ "${prefix}" == /* && "${prefix}" != / && "${prefix}" != *$'\n'* ]] || {
    rm -r -- "${temporary}"
    asc_error "update prefix must be an absolute path other than /"
    return 1
  }
  prefix="${prefix%/}"
  [[ -f "${prefix}/share/asc-devtools/install-manifest" && ! -L "${prefix}/share/asc-devtools/install-manifest" ]] || {
    rm -r -- "${temporary}"
    asc_error "refusing to update an unmanaged installation under ${prefix}"
    return 1
  }
  if [[ "${assume_yes}" != true ]]; then
    printf 'Update the managed installation in %s? [y/N] ' "${prefix}"
    IFS= read -r answer || true
    answer="${answer,,}"
    if [[ "${answer}" != y && "${answer}" != yes ]]; then
      printf 'Update cancelled.\n'
      rm -r -- "${temporary}"
      return 0
    fi
  fi
  sums="${temporary}/SHA256SUMS"
  archive="${temporary}/${ASC_UPDATE_ARCHIVE}"
  asc_update_download "${ASC_UPDATE_SUMS_URL}" application/octet-stream "${sums}" 1048576 "${headers}" || { rm -r -- "${temporary}"; return 1; }
  [[ "${ASC_UPDATE_HTTP_STATUS}" =~ ^2[0-9][0-9]$ ]] || { asc_error "download SHA256SUMS returned ${ASC_UPDATE_HTTP_STATUS}"; rm -r -- "${temporary}"; return 1; }
  asc_update_expected_checksum "${sums}" || { rm -r -- "${temporary}"; return 1; }
  asc_update_download "${ASC_UPDATE_ARCHIVE_URL}" application/octet-stream "${archive}" 134217728 "${headers}" || { rm -r -- "${temporary}"; return 1; }
  [[ "${ASC_UPDATE_HTTP_STATUS}" =~ ^2[0-9][0-9]$ ]] || { asc_error "download ${ASC_UPDATE_ARCHIVE} returned ${ASC_UPDATE_HTTP_STATUS}"; rm -r -- "${temporary}"; return 1; }
  actual=$(sha256sum -- "${archive}")
  actual="${actual%% *}"
  [[ "${actual}" == "${ASC_UPDATE_EXPECTED_HASH}" ]] || { asc_error "checksum mismatch for ${ASC_UPDATE_ARCHIVE}"; rm -r -- "${temporary}"; return 1; }
  asc_update_extract "${archive}" "${temporary}/bundle" || { rm -r -- "${temporary}"; return 1; }
  root="${temporary}/bundle/${ASC_UPDATE_ROOT}"
  [[ -f "${root}/scripts/install.sh" && ! -L "${root}/scripts/install.sh" ]] || { asc_error "release bundle is missing a regular installer"; rm -r -- "${temporary}"; return 1; }
  if ! bundle_version_output=$("${root}/bin/asc" --version) || [[ "${bundle_version_output}" != "asc ${ASC_UPDATE_VERSION}" ]]; then
    asc_error "release bundle version does not match release ${ASC_UPDATE_VERSION}"
    rm -r -- "${temporary}"
    return 1
  fi
  if ! "${root}/scripts/install.sh" --prefix "${prefix}"; then
    rm -r -- "${temporary}"
    asc_error "install asc ${ASC_UPDATE_VERSION} failed"
    return 1
  fi
  rm -r -- "${temporary}"
  printf 'Updated asc to %s.\n' "${ASC_UPDATE_VERSION}"
}
