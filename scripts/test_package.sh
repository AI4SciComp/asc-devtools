#!/usr/bin/env bash
set -euo pipefail
output="${1:?Usage: scripts/test_package.sh OUTPUT_DIRECTORY VERSION}"
output=$(cd "$output" && pwd)
version="${2:?Version required}"
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
(cd "$output" && sha256sum -c SHA256SUMS)
for arch in amd64 arm64; do
    deb="$output/asc-devtools_${version}_${arch}.deb"
    test "$(dpkg-deb -f "$deb" Package)" = asc-devtools
    test "$(dpkg-deb -f "$deb" Version)" = "$version"
    test "$(dpkg-deb -f "$deb" Architecture)" = "$arch"
    dpkg-deb -x "$deb" "$work/extract-$arch"
    test -x "$work/extract-$arch/usr/bin/asc"
    test -f "$work/extract-$arch/usr/share/doc/asc-devtools/copyright"
    test ! -e "$work/extract-$arch/usr/local"
    test ! -e "$work/extract-$arch/usr/share/asc-devtools/install-manifest"
    bash -n "$work/extract-$arch/usr/share/bash-completion/completions/asc"
    tar -xzf "$output/asc-devtools_${version}_linux_${arch}.tar.gz" -C "$work"
    cmp "$work/extract-$arch/usr/bin/asc" "$work/asc-devtools_${version}_linux_${arch}/asc"
done
arch=$(dpkg --print-architecture)
case "$arch" in amd64|arm64) ;; *) echo "Unsupported test host: $arch" >&2; exit 1;; esac
deb="$output/asc-devtools_${version}_${arch}.deb"
# Isolated dpkg database: exercise unpack/configure/reinstall/remove without sudo.
# Dependencies are checked separately; this root contains only our package.
mkdir -p "$work/root/var/lib/dpkg"
touch "$work/root/var/lib/dpkg/status"
dpkg_test() {
    fakeroot dpkg --root="$work/root" --log="$work/dpkg.log" --force-depends "$@"
}
dpkg_test --install "$deb"
"$work/root/usr/bin/asc" --version | grep -F "$version"
"$work/root/usr/bin/asc" --help >/dev/null
"$work/root/usr/bin/asc" completion bash > "$work/completion"
cmp "$work/completion" "$work/root/usr/share/bash-completion/completions/asc"
dpkg_test --install "$deb"
# Upgrade from a synthetic older package to the release under test.
mkdir -p "$work/older"
dpkg-deb -R "$deb" "$work/older"
sed -i 's/^Version: .*/Version: 0.0.0/' "$work/older/DEBIAN/control"
dpkg-deb --root-owner-group --build "$work/older" "$work/older.deb"
dpkg_test --remove asc-devtools
dpkg_test --install "$work/older.deb"
dpkg_test --install "$deb"
test "$(dpkg-query --admindir="$work/root/var/lib/dpkg" -W -f='${Version}' asc-devtools)" = "$version"
dpkg_test --remove asc-devtools
test ! -e "$work/root/usr/bin/asc"
test ! -e "$work/root/usr/share/bash-completion/completions/asc"
# The archive's existing installer must work without a checkout or Go.
archive="$work/asc-devtools_${version}_linux_${arch}"
bash "$archive/scripts/install.sh" --binary "$archive/asc" --prefix "$work/user"
"$work/user/bin/asc" --version | grep -F "$version"
bash "$archive/scripts/uninstall.sh" --prefix "$work/user"
test ! -e "$work/user/bin/asc"
echo 'Package tests passed'
