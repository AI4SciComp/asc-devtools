#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

output=""

usage() {
  cat <<'EOF'
Usage: ./scripts/package_update.sh --output DIRECTORY

Create asc-devtools-shell.tar.gz for `asc update` and refresh SHA256SUMS for
all asc-devtools update archives already present in the output directory.
EOF
}

fail() {
  printf 'package_update.sh: %s\n' "$*" >&2
  exit 1
}

while (($# > 0)); do
  case "$1" in
    --output)
      (($# >= 2)) || fail "--output requires a value"
      output="$2"
      shift 2
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *) fail "unknown option: $1" ;;
  esac
done

[[ -n "${output}" ]] || fail "--output is required"
project_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
mkdir -p -- "${output}"
output=$(cd -- "${output}" && pwd)
package_root=$(mktemp -d "${TMPDIR:-/tmp}/asc-package-shell.XXXXXXXX")
trap 'rm -rf -- "${package_root}"' EXIT
bundle="${package_root}/asc-devtools-shell"
mkdir -p -- "${bundle}/bin" "${bundle}/lib/asc" "${bundle}/scripts" "${bundle}/completions"
install -m 0755 -- "${project_root}/bin/asc" "${bundle}/bin/asc"
install -m 0755 -- "${project_root}/scripts/install.sh" "${bundle}/scripts/install.sh"
install -m 0644 -- "${project_root}/completions/asc.bash" "${bundle}/completions/asc.bash"
for source in "${project_root}"/lib/asc/*.sh; do
  [[ -f "${source}" && ! -L "${source}" ]] || fail "invalid library source: ${source}"
  install -m 0644 -- "${source}" "${bundle}/lib/asc/${source##*/}"
done

archive="asc-devtools-shell.tar.gz"
tar --sort=name --owner=0 --group=0 --numeric-owner --mtime='UTC 1970-01-01' \
  -C "${package_root}" -czf "${output}/${archive}" asc-devtools-shell
(
  cd -- "${output}"
  shopt -s nullglob
  archives=(asc-devtools-*.tar.gz)
  ((${#archives[@]} > 0)) || fail "no update archives were created"
  sha256sum -- "${archives[@]}" >SHA256SUMS
)
printf 'Created %s\n' "${output}/${archive}"
printf 'Updated %s\n' "${output}/SHA256SUMS"
