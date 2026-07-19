#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
stage=$(mktemp -d "${TMPDIR:-/tmp}/asc-shell-install.XXXXXXXX")
trap 'rm -rf -- "${stage}"' EXIT
"${root}/scripts/install.sh" --destdir "${stage}" >/dev/null
"${stage}/usr/local/bin/asc" --version | grep -q '^asc 0.1.0$'
bash -n "${stage}/usr/local/share/bash-completion/completions/asc"
"${root}/scripts/install.sh" --destdir "${stage}" >/dev/null
"${root}/scripts/uninstall.sh" --destdir "${stage}" >/dev/null
[[ ! -e "${stage}/usr/local/bin/asc" ]]
"${root}/scripts/install.sh" --destdir "${stage}" >/dev/null
printf '\nmodified\n' >>"${stage}/usr/local/bin/asc"
if "${root}/scripts/uninstall.sh" --destdir "${stage}" >/dev/null 2>&1; then
  printf '%s\n' 'uninstaller removed a modified file' >&2
  exit 1
fi
[[ -e "${stage}/usr/local/bin/asc" ]]
printf '%s\n' 'install lifecycle tests passed'
