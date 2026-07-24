#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

prefix="/usr/local"
destdir=""
binary=""
go_binary=""
temporary_binary=""
temporary_completion=""
temporary_manifest=""

usage() {
	cat <<'EOF'
Usage: ./scripts/install.sh [--binary PATH] [--go PATH] [--prefix PATH] [--destdir PATH]

Install asc and its Bash completion. The default prefix is /usr/local.
When building from source, --go selects the Go executable explicitly.
EOF
}

fail() {
	printf 'install.sh: %s\n' "$*" >&2
	exit 1
}

cleanup() {
	[[ -z "${temporary_binary}" ]] || rm -f -- "${temporary_binary}"
	[[ -z "${temporary_completion}" ]] || rm -f -- "${temporary_completion}"
	[[ -z "${temporary_manifest}" ]] || rm -f -- "${temporary_manifest}"
}
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
	--binary)
		(($# >= 2)) || fail "--binary requires a value"
		binary="$2"
		shift 2
		;;
	--go)
		(($# >= 2)) || fail "--go requires a value"
		go_binary="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		printf 'install.sh: unknown option: %s\n' "$1" >&2
		usage >&2
		exit 2
		;;
	esac
done

[[ "${prefix}" == /* && "${prefix}" != / && "${prefix}" != *$'\n'* ]] ||
	fail "prefix must be an absolute path other than /"
if [[ -n "${destdir}" ]]; then
	[[ "${destdir}" == /* && "${destdir}" != *$'\n'* ]] ||
		fail "destdir must be an absolute path"
	destdir="${destdir%/}"
fi
command -v sha256sum >/dev/null 2>&1 || fail "sha256sum is required"

prefix="${prefix%/}"
install_root="${destdir}${prefix}"
binary_target="${install_root}/bin/asc"
completion_target="${install_root}/share/bash-completion/completions/asc"
manifest_target="${install_root}/share/asc-devtools/install-manifest"

manifest_marker=""
manifest_prefix=""
manifest_binary_hash=""
manifest_completion_hash=""
load_manifest() {
	local line
	while IFS= read -r line || [[ -n "${line}" ]]; do
		case "${line}" in
		'# asc-devtools install manifest v1') manifest_marker="v1" ;;
		prefix=*) manifest_prefix="${line#prefix=}" ;;
		binary_sha256=*) manifest_binary_hash="${line#binary_sha256=}" ;;
		completion_sha256=*) manifest_completion_hash="${line#completion_sha256=}" ;;
		esac
	done <"${manifest_target}"
	[[ "${manifest_marker}" == "v1" && "${manifest_prefix}" == "${prefix}" ]] ||
		fail "invalid install manifest: ${manifest_target}"
	[[ "${manifest_binary_hash}" =~ ^[0-9a-f]{64}$ ]] ||
		fail "invalid binary hash in ${manifest_target}"
	[[ "${manifest_completion_hash}" =~ ^[0-9a-f]{64}$ ]] ||
		fail "invalid completion hash in ${manifest_target}"
}

verify_managed_file() {
	local target="$1"
	local expected_hash="$2"
	local actual_hash
	[[ -e "${target}" || -L "${target}" ]] || return 0
	[[ -f "${target}" && ! -L "${target}" ]] || fail "refusing non-regular managed path: ${target}"
	actual_hash=$(sha256sum -- "${target}")
	actual_hash="${actual_hash%% *}"
	[[ "${actual_hash}" == "${expected_hash}" ]] || fail "refusing to overwrite modified file: ${target}"
}

if [[ -e "${binary_target}" || -L "${binary_target}" ||
	-e "${completion_target}" || -L "${completion_target}" ||
	-e "${manifest_target}" || -L "${manifest_target}" ]]; then
	[[ -f "${manifest_target}" && ! -L "${manifest_target}" ]] ||
		fail "refusing to overwrite files without a valid manifest at ${manifest_target}"
	load_manifest
	verify_managed_file "${binary_target}" "${manifest_binary_hash}"
	verify_managed_file "${completion_target}" "${manifest_completion_hash}"
fi

project_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
if [[ -z "${binary}" ]]; then
	if [[ -n "${go_binary}" ]]; then
		[[ "${go_binary}" == /* ]] || fail "--go must be an absolute path"
		[[ -f "${go_binary}" && -x "${go_binary}" ]] ||
			fail "Go executable is not an executable regular file: ${go_binary}"
	elif go_binary=$(command -v go 2>/dev/null); then
		:
	else
		for candidate in /usr/local/go/bin/go /usr/lib/go/bin/go /snap/bin/go; do
			if [[ -f "${candidate}" && -x "${candidate}" ]]; then
				go_binary="${candidate}"
				break
			fi
		done
	fi
	[[ -n "${go_binary}" ]] ||
		fail "Go is required when --binary is not supplied; pass --go PATH if sudo hides it"
	temporary_binary=$(mktemp "${TMPDIR:-/tmp}/asc-build.XXXXXXXX")
	(cd -- "${project_root}" && CGO_ENABLED=0 "${go_binary}" build -buildvcs=false -trimpath \
		-ldflags "-s -w -X main.version=0.1.0" -o "${temporary_binary}" ./cmd/asc)
	binary="${temporary_binary}"
fi
[[ -f "${binary}" && ! -L "${binary}" && -x "${binary}" ]] ||
	fail "binary must be an executable regular file: ${binary}"

temporary_completion=$(mktemp "${TMPDIR:-/tmp}/asc-completion.XXXXXXXX")
"${binary}" completion bash >"${temporary_completion}"
bash -n "${temporary_completion}"

mkdir -p -- \
	"${install_root}/bin" \
	"${install_root}/share/bash-completion/completions" \
	"${install_root}/share/asc-devtools"
install -m 0755 -- "${binary}" "${binary_target}"
install -m 0644 -- "${temporary_completion}" "${completion_target}"

binary_hash=$(sha256sum -- "${binary_target}")
binary_hash="${binary_hash%% *}"
completion_hash=$(sha256sum -- "${completion_target}")
completion_hash="${completion_hash%% *}"
temporary_manifest=$(mktemp "${TMPDIR:-/tmp}/asc-manifest.XXXXXXXX")
{
	printf '# asc-devtools install manifest v1\n'
	printf 'prefix=%s\n' "${prefix}"
	printf 'binary_sha256=%s\n' "${binary_hash}"
	printf 'completion_sha256=%s\n' "${completion_hash}"
} >"${temporary_manifest}"
install -m 0644 -- "${temporary_manifest}" "${manifest_target}"

printf 'Installed %s\n' "${binary_target}"
printf 'Installed %s\n' "${completion_target}"
printf 'Installed %s\n' "${manifest_target}"
