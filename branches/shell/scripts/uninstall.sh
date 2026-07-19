#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

prefix="/usr/local"
destdir=""
relative_targets=(
  bin/asc
  lib/asc/common.sh
  lib/asc/json.sh
  lib/asc/config.sh
  lib/asc/process.sh
  lib/asc/github.sh
  lib/asc/repository.sh
  lib/asc/doctor.sh
  lib/asc/cmake.sh
  lib/asc/vendor.sh
  share/bash-completion/completions/asc
  lib/asc/update.sh
)
fail() {
  printf 'uninstall.sh: %s\n' "$*" >&2
  exit 1
}
while (($# > 0)); do
  case "$1" in
    --prefix)
      (($# >= 2)) || fail "--prefix requires a value"
      prefix="$2"
      shift 2
      ;;
    --destdir)
      (($# >= 2)) || fail "--destdir requires a value"
      destdir="$2"
      shift 2
      ;;
    -h | --help)
      printf 'Usage: ./scripts/uninstall.sh [--prefix PATH] [--destdir PATH]\n'
      exit 0
      ;;
    *) fail "unknown option: $1" ;;
  esac
done
[[ "${prefix}" == /* && "${prefix}" != / && "${prefix}" != *$'\n'* ]] ||
  fail "prefix must be an absolute path other than /"
if [[ -n "${destdir}" ]]; then
  [[ "${destdir}" == /* && "${destdir}" != *$'\n'* ]] || fail "destdir must be absolute"
  destdir="${destdir%/}"
fi
command -v sha256sum >/dev/null 2>&1 || fail "sha256sum is required"
prefix="${prefix%/}"
install_root="${destdir}${prefix}"
manifest_target="${install_root}/share/asc-devtools/install-manifest"
if [[ ! -e "${manifest_target}" && ! -L "${manifest_target}" ]]; then
  printf 'asc-devtools is not installed under %s\n' "${install_root}"
  exit 0
fi
[[ -f "${manifest_target}" && ! -L "${manifest_target}" ]] || fail "invalid manifest path"

marker="" manifest_prefix="" count=""
hashes=()
while IFS= read -r line || [[ -n "${line}" ]]; do
  case "${line}" in
    '# asc-devtools install manifest v1') marker=v1 ;;
    prefix=*) manifest_prefix="${line#prefix=}" ;;
    count=*) count="${line#count=}" ;;
    hash_*=*)
      key="${line%%=*}"
      index="${key#hash_}"
      [[ "${index}" =~ ^[0-9]+$ ]] || fail "invalid manifest index"
      hashes[index]="${line#*=}"
      ;;
  esac
done <"${manifest_target}"
[[ "${marker}" == v1 && "${manifest_prefix}" == "${prefix}" ]] || fail "invalid manifest"
[[ "${count}" =~ ^[0-9]+$ && "${count}" -gt 0 && "${count}" -le "${#relative_targets[@]}" ]] || fail "manifest file count mismatch"
for ((index = 0; index < count; index++)); do
  [[ "${hashes[index]:-}" =~ ^[0-9a-f]{64}$ ]] || fail "invalid manifest hash"
  target="${install_root}/${relative_targets[index]}"
  [[ -e "${target}" || -L "${target}" ]] || continue
  [[ -f "${target}" && ! -L "${target}" ]] || fail "refusing non-regular managed path: ${target}"
  actual=$(sha256sum -- "${target}")
  actual="${actual%% *}"
  [[ "${actual}" == "${hashes[index]}" ]] || fail "refusing modified file: ${target}"
done
for relative in "${relative_targets[@]}"; do
  target="${install_root}/${relative}"
  rm -f -- "${target}"
  printf 'Removed %s\n' "${target}"
done
rm -f -- "${manifest_target}"
printf 'Removed %s\n' "${manifest_target}"
rmdir -- \
  "${install_root}/lib/asc" \
  "${install_root}/share/asc-devtools" \
  "${install_root}/share/bash-completion/completions" \
  "${install_root}/share/bash-completion" \
  2>/dev/null || true
