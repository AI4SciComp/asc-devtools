#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

ASC_JSON_TEXT=""
ASC_JSON_POSITION=0
ASC_JSON_LENGTH=0
ASC_JSON_VALUE=""
ASC_JSON_TYPE=""

asc_json_begin() {
  ASC_JSON_TEXT="$1"
  ASC_JSON_POSITION=0
  ASC_JSON_LENGTH=${#ASC_JSON_TEXT}
  ASC_JSON_VALUE=""
  ASC_JSON_TYPE=""
}

asc_json_skip_whitespace() {
  local character
  while ((ASC_JSON_POSITION < ASC_JSON_LENGTH)); do
    character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
    case "${character}" in
      ' ' | $'\t' | $'\r' | $'\n') ((ASC_JSON_POSITION += 1)) ;;
      *) return 0 ;;
    esac
  done
}

asc_json_expect() {
  local expected="$1"
  asc_json_skip_whitespace
  if [[ "${ASC_JSON_TEXT:ASC_JSON_POSITION:1}" != "${expected}" ]]; then
    asc_error "invalid JSON at byte ${ASC_JSON_POSITION}: expected ${expected}"
    return 1
  fi
  ((ASC_JSON_POSITION += 1))
}

asc_json_parse_string() {
  local character
  local escape
  local hex
  local decoded
  ASC_JSON_VALUE=""
  ASC_JSON_TYPE="string"
  asc_json_expect '"' || return 1
  while ((ASC_JSON_POSITION < ASC_JSON_LENGTH)); do
    character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
    ((ASC_JSON_POSITION += 1))
    case "${character}" in
      '"') return 0 ;;
      \\)
        ((ASC_JSON_POSITION < ASC_JSON_LENGTH)) || {
          asc_error "invalid JSON escape at end of input"
          return 1
        }
        escape="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
        ((ASC_JSON_POSITION += 1))
        case "${escape}" in
          '"' | \\ | '/') ASC_JSON_VALUE+="${escape}" ;;
          b) ASC_JSON_VALUE+=$'\b' ;;
          f) ASC_JSON_VALUE+=$'\f' ;;
          n) ASC_JSON_VALUE+=$'\n' ;;
          r) ASC_JSON_VALUE+=$'\r' ;;
          t) ASC_JSON_VALUE+=$'\t' ;;
          u)
            hex="${ASC_JSON_TEXT:ASC_JSON_POSITION:4}"
            [[ "${hex}" =~ ^[0-9A-Fa-f]{4}$ ]] || {
              asc_error "invalid JSON unicode escape"
              return 1
            }
            [[ "${hex}" != "0000" ]] || {
              asc_error "JSON strings cannot contain a null byte"
              return 1
            }
            printf -v decoded '%b' "\\u${hex}"
            ASC_JSON_VALUE+="${decoded}"
            ((ASC_JSON_POSITION += 4))
            ;;
          *)
            asc_error "invalid JSON escape: \\${escape}"
            return 1
            ;;
        esac
        ;;
      $'\001' | $'\002' | $'\003' | $'\004' | $'\005' | $'\006' | $'\a' | $'\b' | $'\t' | $'\n' | $'\v' | $'\f' | $'\r' | $'\016' | $'\017' | $'\020' | $'\021' | $'\022' | $'\023' | $'\024' | $'\025' | $'\026' | $'\027' | $'\030' | $'\031' | $'\032' | $'\e' | $'\034' | $'\035' | $'\036' | $'\037')
        asc_error "unescaped control character in JSON string"
        return 1
        ;;
      *) ASC_JSON_VALUE+="${character}" ;;
    esac
  done
  asc_error "unterminated JSON string"
  return 1
}

asc_json_parse_scalar() {
  local remainder
  local character
  local start
  asc_json_skip_whitespace
  character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
  if [[ "${character}" == '"' ]]; then
    asc_json_parse_string
    return
  fi
  remainder="${ASC_JSON_TEXT:ASC_JSON_POSITION}"
  case "${remainder}" in
    true*)
      ASC_JSON_TYPE="boolean"
      ASC_JSON_VALUE="true"
      ((ASC_JSON_POSITION += 4))
      return 0
      ;;
    false*)
      ASC_JSON_TYPE="boolean"
      ASC_JSON_VALUE="false"
      ((ASC_JSON_POSITION += 5))
      return 0
      ;;
    null*)
      ASC_JSON_TYPE="null"
      ASC_JSON_VALUE=""
      ((ASC_JSON_POSITION += 4))
      return 0
      ;;
  esac
  start=${ASC_JSON_POSITION}
  while ((ASC_JSON_POSITION < ASC_JSON_LENGTH)); do
    character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
    [[ "${character}" =~ [0-9eE+.-] ]] || break
    ((ASC_JSON_POSITION += 1))
  done
  ((ASC_JSON_POSITION > start)) || {
    asc_error "invalid JSON value at byte ${ASC_JSON_POSITION}"
    return 1
  }
  # ASC_JSON_TYPE is part of the parser interface consumed by other modules.
  # shellcheck disable=SC2034
  ASC_JSON_TYPE="number"
  ASC_JSON_VALUE="${ASC_JSON_TEXT:start:ASC_JSON_POSITION-start}"
}

asc_json_skip_value() {
  local character
  asc_json_skip_whitespace
  character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
  case "${character}" in
    '{')
      asc_json_expect '{' || return 1
      asc_json_skip_whitespace
      [[ "${ASC_JSON_TEXT:ASC_JSON_POSITION:1}" == '}' ]] && {
        ((ASC_JSON_POSITION += 1))
        return 0
      }
      while true; do
        asc_json_parse_string || return 1
        asc_json_expect ':' || return 1
        asc_json_skip_value || return 1
        asc_json_skip_whitespace
        character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
        if [[ "${character}" == '}' ]]; then
          ((ASC_JSON_POSITION += 1))
          return 0
        fi
        asc_json_expect ',' || return 1
      done
      ;;
    '[')
      asc_json_expect '[' || return 1
      asc_json_skip_whitespace
      [[ "${ASC_JSON_TEXT:ASC_JSON_POSITION:1}" == ']' ]] && {
        ((ASC_JSON_POSITION += 1))
        return 0
      }
      while true; do
        asc_json_skip_value || return 1
        asc_json_skip_whitespace
        character="${ASC_JSON_TEXT:ASC_JSON_POSITION:1}"
        if [[ "${character}" == ']' ]]; then
          ((ASC_JSON_POSITION += 1))
          return 0
        fi
        asc_json_expect ',' || return 1
      done
      ;;
    *) asc_json_parse_scalar ;;
  esac
}

asc_json_finish() {
  asc_json_skip_whitespace
  if ((ASC_JSON_POSITION != ASC_JSON_LENGTH)); then
    asc_error "trailing data after JSON value"
    return 1
  fi
}

asc_json_quote() {
  local value="$1"
  local output='"'
  local character
  local code
  local index
  for ((index = 0; index < ${#value}; index++)); do
    character="${value:index:1}"
    case "${character}" in
      '"') output+='\"' ;;
      \\) output+="\\\\" ;;
      $'\b') output+='\b' ;;
      $'\f') output+='\f' ;;
      $'\n') output+='\n' ;;
      $'\r') output+='\r' ;;
      $'\t') output+='\t' ;;
      *)
        LC_CTYPE=C printf -v code '%d' "'${character}"
        if ((code < 32)); then
          printf -v character '\\u%04x' "${code}"
        fi
        output+="${character}"
        ;;
    esac
  done
  printf '%s"' "${output}"
}
