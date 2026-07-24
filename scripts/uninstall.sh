#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

prefix="/usr/local"
destdir=""

usage() {
	cat <<'EOF'
Usage: ./scripts/uninstall.sh [--prefix PATH] [--destdir PATH]

Remove files recorded by the asc-devtools installer. The default prefix is
/usr/local. Modified and unrelated files are never removed.
EOF
}

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
		usage
		exit 0
		;;
	*)
		printf 'uninstall.sh: unknown option: %s\n' "$1" >&2
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

if [[ ! -e "${manifest_target}" && ! -L "${manifest_target}" ]]; then
	printf 'asc-devtools is not installed under %s\n' "${install_root}"
	exit 0
fi
[[ -f "${manifest_target}" && ! -L "${manifest_target}" ]] ||
	fail "refusing invalid manifest path: ${manifest_target}"

manifest_marker=""
manifest_prefix=""
manifest_binary_hash=""
manifest_completion_hash=""
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

verify_removal() {
	local target="$1"
	local expected_hash="$2"
	local actual_hash
	[[ -e "${target}" || -L "${target}" ]] || return 0
	[[ -f "${target}" && ! -L "${target}" ]] || fail "refusing non-regular managed path: ${target}"
	actual_hash=$(sha256sum -- "${target}")
	actual_hash="${actual_hash%% *}"
	[[ "${actual_hash}" == "${expected_hash}" ]] || fail "refusing to remove modified file: ${target}"
}

verify_removal "${binary_target}" "${manifest_binary_hash}"
verify_removal "${completion_target}" "${manifest_completion_hash}"
rm -f -- "${binary_target}" "${completion_target}" "${manifest_target}"
rmdir -- \
	"${install_root}/share/asc-devtools" \
	"${install_root}/share/bash-completion/completions" \
	"${install_root}/share/bash-completion" \
	2>/dev/null || true

printf 'Removed %s\n' "${binary_target}"
printf 'Removed %s\n' "${completion_target}"
printf 'Removed %s\n' "${manifest_target}"
