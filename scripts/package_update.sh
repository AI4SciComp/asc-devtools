#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

binary=""
output=""
target_os=""
target_arch=""

usage() {
	cat <<'EOF'
Usage: ./scripts/package_update.sh --binary PATH --output DIRECTORY --os OS --arch ARCH

Create the deterministic Go release asset consumed by `asc update` and refresh
SHA256SUMS for all Go update archives in the output directory.
EOF
}

fail() {
	printf 'package_update.sh: %s\n' "$*" >&2
	exit 1
}

while (($# > 0)); do
	case "$1" in
	--binary)
		(($# >= 2)) || fail "--binary requires a value"
		binary="$2"
		shift 2
		;;
	--output)
		(($# >= 2)) || fail "--output requires a value"
		output="$2"
		shift 2
		;;
	--os)
		(($# >= 2)) || fail "--os requires a value"
		target_os="$2"
		shift 2
		;;
	--arch)
		(($# >= 2)) || fail "--arch requires a value"
		target_arch="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*) fail "unknown option: $1" ;;
	esac
done

[[ -n "${binary}" && -f "${binary}" && -x "${binary}" && ! -L "${binary}" ]] || fail "--binary must name an executable regular file"
[[ -n "${output}" ]] || fail "--output is required"
[[ "${target_os}" =~ ^[a-z0-9]+$ ]] || fail "--os must contain lowercase letters and digits"
[[ "${target_arch}" =~ ^[a-z0-9]+$ ]] || fail "--arch must contain lowercase letters and digits"

project_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
mkdir -p -- "${output}"
output=$(cd -- "${output}" && pwd)
package_root=$(mktemp -d "${TMPDIR:-/tmp}/asc-package-go.XXXXXXXX")
trap 'rm -rf -- "${package_root}"' EXIT
bundle="${package_root}/asc-devtools-go"
mkdir -p -- "${bundle}/scripts"
install -m 0755 -- "${binary}" "${bundle}/asc"
install -m 0755 -- "${project_root}/scripts/install.sh" "${bundle}/scripts/install.sh"

archive="asc-devtools-go-${target_os}-${target_arch}.tar.gz"
tar --sort=name --owner=0 --group=0 --numeric-owner --mtime='UTC 1970-01-01' \
	-C "${package_root}" -czf "${output}/${archive}" asc-devtools-go
(
	cd -- "${output}"
	shopt -s nullglob
	archives=(asc-devtools-*.tar.gz)
	((${#archives[@]} > 0)) || fail "no update archives were created"
	sha256sum -- "${archives[@]}" >SHA256SUMS
)
printf 'Created %s\n' "${output}/${archive}"
printf 'Updated %s\n' "${output}/SHA256SUMS"
