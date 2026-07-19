#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

prefix="/usr/local"
destdir=""
temporary_manifest=""
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
  printf 'install.sh: %s\n' "$*" >&2
  exit 1
}
cleanup() { [[ -z "${temporary_manifest}" ]] || rm -f -- "${temporary_manifest}"; }
trap cleanup EXIT

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
      printf 'Usage: ./scripts/install.sh [--prefix PATH] [--destdir PATH]\n'
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
source_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)

source_file() {
  local relative="$1"
  if [[ "${relative}" == share/bash-completion/completions/asc ]]; then
    printf '%s/completions/asc.bash\n' "${source_root}"
  else
    printf '%s/%s\n' "${source_root}" "${relative}"
  fi
}

manifest_marker=""
manifest_prefix=""
manifest_count=""
manifest_hashes=()
load_manifest() {
  local line key index value
  while IFS= read -r line || [[ -n "${line}" ]]; do
    case "${line}" in
      '# asc-devtools install manifest v1') manifest_marker=v1 ;;
      prefix=*) manifest_prefix="${line#prefix=}" ;;
      count=*) manifest_count="${line#count=}" ;;
      hash_*=*)
        key="${line%%=*}"
        index="${key#hash_}"
        value="${line#*=}"
        [[ "${index}" =~ ^[0-9]+$ ]] || fail "invalid manifest index"
        manifest_hashes[index]="${value}"
        ;;
    esac
  done <"${manifest_target}"
  [[ "${manifest_marker}" == v1 && "${manifest_prefix}" == "${prefix}" ]] || fail "invalid manifest"
  [[ "${manifest_count}" =~ ^[0-9]+$ && "${manifest_count}" -gt 0 && "${manifest_count}" -le "${#relative_targets[@]}" ]] || fail "manifest file count mismatch"
  for ((index = 0; index < manifest_count; index++)); do
    [[ "${manifest_hashes[index]:-}" =~ ^[0-9a-f]{64}$ ]] || fail "invalid manifest hash"
  done
}

verify_existing() {
  local index target actual
  for ((index = 0; index < manifest_count; index++)); do
    target="${install_root}/${relative_targets[index]}"
    [[ -e "${target}" || -L "${target}" ]] || continue
    [[ -f "${target}" && ! -L "${target}" ]] || fail "refusing non-regular managed path: ${target}"
    actual=$(sha256sum -- "${target}")
    actual="${actual%% *}"
    [[ "${actual}" == "${manifest_hashes[index]}" ]] || fail "refusing modified file: ${target}"
  done
}

existing=false
for relative in "${relative_targets[@]}"; do
  target="${install_root}/${relative}"
  [[ ! -e "${target}" && ! -L "${target}" ]] || existing=true
done
[[ ! -e "${manifest_target}" && ! -L "${manifest_target}" ]] || existing=true
if [[ "${existing}" == true ]]; then
  [[ -f "${manifest_target}" && ! -L "${manifest_target}" ]] || fail "refusing files without a valid manifest"
  load_manifest
  verify_existing
fi

for index in "${!relative_targets[@]}"; do
  relative="${relative_targets[index]}"
  source=$(source_file "${relative}")
  target="${install_root}/${relative}"
  [[ -f "${source}" && ! -L "${source}" ]] || fail "source file is missing: ${source}"
  mkdir -p -- "${target%/*}"
  [[ "${relative}" == bin/asc ]] && mode=0755 || mode=0644
  install -m "${mode}" -- "${source}" "${target}"
  printf 'Installed %s\n' "${target}"
done

mkdir -p -- "${manifest_target%/*}"
temporary_manifest=$(mktemp "${TMPDIR:-/tmp}/asc-manifest.XXXXXXXX")
{
  printf '# asc-devtools install manifest v1\n'
  printf 'prefix=%s\n' "${prefix}"
  printf 'count=%d\n' "${#relative_targets[@]}"
  for index in "${!relative_targets[@]}"; do
    target="${install_root}/${relative_targets[index]}"
    hash=$(sha256sum -- "${target}")
    hash="${hash%% *}"
    printf 'hash_%d=%s\n' "${index}" "${hash}"
  done
} >"${temporary_manifest}"
install -m 0644 -- "${temporary_manifest}" "${manifest_target}"
printf 'Installed %s\n' "${manifest_target}"
