#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

asc_github_parse_response() {
  local content="$1"
  local character
  local key
  local value
  local value_type
  local name archived fork clone_url ssh_url default_branch private
  local has_name has_archived has_fork has_clone_url has_ssh_url has_default_branch has_private
  asc_json_begin "${content}"
  asc_json_expect '[' || return 1
  asc_json_skip_whitespace
  if [[ "${ASC_JSON_TEXT:ASC_JSON_POSITION:1}" == ']' ]]; then
    ((ASC_JSON_POSITION += 1))
    asc_json_finish
    return
  fi
  while true; do
    name="" archived="" fork="" clone_url="" ssh_url="" default_branch="" private=""
    has_name=false has_archived=false has_fork=false has_clone_url=false
    has_ssh_url=false has_default_branch=false has_private=false
    asc_json_expect '{' || return 1
    asc_json_skip_whitespace
    while [[ "${ASC_JSON_TEXT:ASC_JSON_POSITION:1}" != '}' ]]; do
      asc_json_parse_string || return 1
      key="${ASC_JSON_VALUE}"
      asc_json_expect ':' || return 1
      case "${key}" in
        name | clone_url | ssh_url | default_branch | archived | fork | private)
          asc_json_parse_scalar || return 1
          value="${ASC_JSON_VALUE}"
          value_type="${ASC_JSON_TYPE}"
          ;;
        *)
          asc_json_skip_value || return 1
          value=""
          value_type=""
          ;;
      esac
      case "${key}" in
        name)
          [[ "${value_type}" == string ]] || return 1
          name="${value}"
          has_name=true
          ;;
        archived)
          [[ "${value_type}" == boolean ]] || return 1
          archived="${value}"
          has_archived=true
          ;;
        fork)
          [[ "${value_type}" == boolean ]] || return 1
          fork="${value}"
          has_fork=true
          ;;
        clone_url)
          [[ "${value_type}" == string ]] || return 1
          clone_url="${value}"
          has_clone_url=true
          ;;
        ssh_url)
          [[ "${value_type}" == string ]] || return 1
          ssh_url="${value}"
          has_ssh_url=true
          ;;
        default_branch)
          [[ "${value_type}" == string ]] || return 1
          default_branch="${value}"
          has_default_branch=true
          ;;
        private)
          [[ "${value_type}" == boolean ]] || return 1
          private="${value}"
          has_private=true
          ;;
      esac
      asc_json_skip_whitespace
      character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
      [[ "${character}" == '}' ]] && break
      asc_json_expect ',' || return 1
    done
    asc_json_expect '}' || return 1
    if [[ "${has_name}" != true || "${has_archived}" != true || "${has_fork}" != true ||
      "${has_clone_url}" != true || "${has_ssh_url}" != true ||
      "${has_default_branch}" != true || "${has_private}" != true ]]; then
      asc_error "GitHub API returned an incomplete repository record"
      return 1
    fi
    if [[ "${name}${clone_url}${ssh_url}${default_branch}" == *$'\t'* ||
      "${name}${clone_url}${ssh_url}${default_branch}" == *$'\n'* ]]; then
      asc_error "GitHub API returned unsafe repository text"
      return 1
    fi
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
      "${name}" "${archived}" "${fork}" "${clone_url}" "${ssh_url}" "${default_branch}" "${private}"
    asc_json_skip_whitespace
    character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
    if [[ "${character}" == ']' ]]; then
      ((ASC_JSON_POSITION += 1))
      break
    fi
    asc_json_expect ',' || return 1
  done
  asc_json_finish
}

asc_github_api_error() {
  local status="$1"
  local headers="$2"
  case "${status}" in
    401) asc_error "GitHub API authentication failed (401): configure ASC_GITHUB_TOKEN" ;;
    403)
      if grep -qi '^X-RateLimit-Remaining: 0' "${headers}"; then
        asc_error "GitHub API rate limit exceeded (403): authenticate or wait for reset"
      else
        asc_error "GitHub API access forbidden (403): verify organization permissions"
      fi
      ;;
    404) asc_error "GitHub organization or endpoint not found (404): verify organization and token access" ;;
    *) asc_error "GitHub API returned ${status}" ;;
  esac
}

asc_github_discover() {
  asc_require_command curl || return 1
  local base_url="${ASC_GITHUB_API_URL:-https://api.github.com}"
  local temporary_directory
  local headers_file
  local body_file
  local status
  local body_size
  local body
  local records
  local page=1
  local count
  local -a curl_arguments
  local -a all_records=()
  local -a page_records=()
  temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/asc-github.XXXXXXXX") || return 1
  headers_file="${temporary_directory}/headers"
  body_file="${temporary_directory}/body"
  while true; do
    curl_arguments=(
      --silent --show-error --max-time 15
      --dump-header "${headers_file}"
      --output "${body_file}"
      --write-out '%{http_code}'
      --header 'Accept: application/vnd.github+json'
      --header 'X-GitHub-Api-Version: 2022-11-28'
      --header "User-Agent: asc-devtools/${ASC_VERSION}"
    )
    if [[ -n "${ASC_GITHUB_TOKEN_VALUE}" ]]; then
      curl_arguments+=(--header "Authorization: Bearer ${ASC_GITHUB_TOKEN_VALUE}")
    fi
    curl_arguments+=(
      "${base_url%/}/orgs/${ASC_ORGANIZATION}/repos?type=all&per_page=100&page=${page}"
    )
    if ! status=$(curl "${curl_arguments[@]}"); then
      rm -r -- "${temporary_directory}"
      asc_error "GitHub API request failed"
      return 1
    fi
    if [[ ! "${status}" =~ ^2[0-9][0-9]$ ]]; then
      asc_github_api_error "${status}" "${headers_file}"
      rm -r -- "${temporary_directory}"
      return 1
    fi
    body_size=$(wc -c <"${body_file}")
    if ((body_size > 4194304)); then
      rm -r -- "${temporary_directory}"
      asc_error "GitHub API response exceeded size limit"
      return 1
    fi
    body=$(<"${body_file}")
    if ! records=$(asc_github_parse_response "${body}"); then
      rm -r -- "${temporary_directory}"
      return 1
    fi
    if [[ -n "${records}" ]]; then
      page_records=()
      mapfile -t page_records <<<"${records}"
      all_records+=("${page_records[@]}")
      count=${#page_records[@]}
    else
      count=0
    fi
    if ! grep -q 'rel="next"' "${headers_file}" && ((count < 100)); then
      break
    fi
    ((count > 0)) || break
    ((page += 1))
  done
  rm -r -- "${temporary_directory}"

  local record
  local name archived fork clone_url ssh_url default_branch private
  local -a managed=()
  for record in "${all_records[@]}"; do
    IFS=$'\t' read -r name archived fork clone_url ssh_url default_branch private <<<"${record}"
    asc_validate_simple_repository_name "${name}" >/dev/null 2>&1 || {
      asc_error "GitHub API returned an unsafe repository name"
      return 1
    }
    [[ "${archived}" == false ]] || continue
    if [[ "${name}" == "${ASC_REPOSITORY_PREFIX}"* ]] ||
      [[ "${ASC_INCLUDE_DOT_GITHUB}" == true && "${name}" == .github ]]; then
      managed+=("${record}")
    fi
  done
  if ((${#managed[@]} > 0)); then
    printf '%s\n' "${managed[@]}" | LC_ALL=C sort -t $'\t' -k1,1
  fi
}
