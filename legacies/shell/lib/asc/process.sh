#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

asc_capture_command() {
  local output_variable="$1"
  shift
  local command_output
  if command_output=$("$@" 2>&1); then
    printf -v "${output_variable}" '%s' "${command_output}"
  else
    local command_status=$?
    printf -v "${output_variable}" '%s' "${command_output}"
    return "${command_status}"
  fi
}
