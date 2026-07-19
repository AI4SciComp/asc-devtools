#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

project_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
binary="${1:-}"
temporary_binary=""
test_root=$(mktemp -d "${TMPDIR:-/tmp}/asc-install-test.XXXXXXXX")

cleanup() {
	rm -rf -- "${test_root}"
	[[ -z "${temporary_binary}" ]] || rm -f -- "${temporary_binary}"
}
trap cleanup EXIT

if [[ -z "${binary}" ]]; then
	temporary_binary=$(mktemp "${TMPDIR:-/tmp}/asc-install-binary.XXXXXXXX")
	(cd -- "${project_root}" && CGO_ENABLED=0 go build -trimpath -o "${temporary_binary}" ./cmd/asc)
	binary="${temporary_binary}"
fi

prefix="/usr/local"
install_root="${test_root}${prefix}"
install_script="${project_root}/scripts/install.sh"
uninstall_script="${project_root}/scripts/uninstall.sh"

"${install_script}" --binary "${binary}" --prefix "${prefix}" --destdir "${test_root}"
"${install_root}/bin/asc" --help >/dev/null
bash -n "${install_root}/share/bash-completion/completions/asc"
[[ -f "${install_root}/share/asc-devtools/install-manifest" ]]

# A managed install can be upgraded in place.
"${install_script}" --binary "${binary}" --prefix "${prefix}" --destdir "${test_root}" >/dev/null
"${uninstall_script}" --prefix "${prefix}" --destdir "${test_root}"
[[ ! -e "${install_root}/bin/asc" ]]
[[ ! -e "${install_root}/share/bash-completion/completions/asc" ]]
[[ ! -e "${install_root}/share/asc-devtools/install-manifest" ]]

# A managed file modified after installation must survive uninstall.
"${install_script}" --binary "${binary}" --prefix "${prefix}" --destdir "${test_root}" >/dev/null
printf '\nmodified\n' >>"${install_root}/bin/asc"
if "${uninstall_script}" --prefix "${prefix}" --destdir "${test_root}" >/dev/null 2>&1; then
	printf '%s\n' 'test_install.sh: uninstaller removed a modified binary' >&2
	exit 1
fi
[[ -e "${install_root}/bin/asc" ]]
rm -rf -- "${install_root}"

# An unrelated binary without a manifest must survive install and uninstall.
mkdir -p -- "${install_root}/bin"
printf '#!/usr/bin/env bash\nexit 0\n' >"${install_root}/bin/asc"
chmod 0755 -- "${install_root}/bin/asc"
if "${install_script}" --binary "${binary}" --prefix "${prefix}" --destdir "${test_root}" >/dev/null 2>&1; then
	printf '%s\n' 'test_install.sh: installer overwrote an unrelated binary' >&2
	exit 1
fi
"${uninstall_script}" --prefix "${prefix}" --destdir "${test_root}" >/dev/null
[[ -e "${install_root}/bin/asc" ]]
rm -f -- "${install_root}/bin/asc"

# A dangling symlink must not be followed or replaced.
ln -s -- "${test_root}/missing" "${install_root}/bin/asc"
if "${install_script}" --binary "${binary}" --prefix "${prefix}" --destdir "${test_root}" >/dev/null 2>&1; then
	printf '%s\n' 'test_install.sh: installer replaced a dangling symlink' >&2
	exit 1
fi
[[ -L "${install_root}/bin/asc" ]]

printf '%s\n' 'install lifecycle tests passed'
