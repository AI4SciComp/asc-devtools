#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

prefix="${HOME}/.local"
binary=""
while (($# > 0)); do
	case "$1" in
	--prefix)
		(($# >= 2)) || {
			printf '%s\n' 'install.sh: --prefix requires a value' >&2
			exit 2
		}
		prefix="$2"
		shift 2
		;;
	--binary)
		(($# >= 2)) || {
			printf '%s\n' 'install.sh: --binary requires a value' >&2
			exit 2
		}
		binary="$2"
		shift 2
		;;
	*)
		printf 'install.sh: unknown option: %s\n' "$1" >&2
		exit 2
		;;
	esac
done
[[ "${prefix}" == /* && "${prefix}" != / ]] || {
	printf '%s\n' 'install.sh: prefix must be an absolute path other than /' >&2
	exit 1
}

project_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
temporary_binary=""
if [[ -z "${binary}" ]]; then
	command -v go >/dev/null 2>&1 || {
		printf '%s\n' 'install.sh: Go is required when --binary is not supplied' >&2
		exit 1
	}
	temporary_binary=$(mktemp "${TMPDIR:-/tmp}/asc-build.XXXXXXXX")
	trap 'rm -f -- "${temporary_binary}"' EXIT
	(cd -- "${project_root}" && CGO_ENABLED=0 go build -trimpath -o "${temporary_binary}" ./cmd/asc)
	binary="${temporary_binary}"
fi
[[ -f "${binary}" ]] || {
	printf 'install.sh: binary not found: %s\n' "${binary}" >&2
	exit 1
}
mkdir -p -- "${prefix}/bin"
install -m 0755 -- "${binary}" "${prefix}/bin/asc"
printf 'Installed %s\n' "${prefix}/bin/asc"
