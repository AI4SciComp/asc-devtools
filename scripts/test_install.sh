#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
stage=$(mktemp -d "${TMPDIR:-/tmp}/asc-shell-install.XXXXXXXX")
cleanup() {
  [[ "${stage}" == "${TMPDIR:-/tmp}"/asc-shell-install.* && -d "${stage}" ]] || return 0
  rm -r -- "${stage}"
}
trap cleanup EXIT
"${root}/scripts/install.sh" --destdir "${stage}" >/dev/null
"${stage}/usr/local/bin/asc" --version | grep -q '^asc 0.1.0$'
bash -n "${stage}/usr/local/share/bash-completion/completions/asc"
sed -i '/^hash_11=/d; s/^count=12$/count=11/' \
  "${stage}/usr/local/share/asc-devtools/install-manifest"
rm -f -- "${stage}/usr/local/lib/asc/update.sh"
"${root}/scripts/install.sh" --destdir "${stage}" >/dev/null
[[ -f "${stage}/usr/local/lib/asc/update.sh" ]]
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
