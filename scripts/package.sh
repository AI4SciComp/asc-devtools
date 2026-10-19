#!/usr/bin/env bash
# Build release artifacts on Linux; no root access or third-party Go modules.
set -euo pipefail
version="${1:?Usage: scripts/package.sh VERSION [OUTPUT_DIRECTORY]}"
version="${version#v}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Expected a stable version such as 0.1.0' >&2; exit 1; }
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"
output="${2:-$root/dist}"
mkdir -p "$output"
output=$(cd "$output" && pwd)
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
export SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(git log -1 --format=%ct)}"
commit=$(git rev-parse HEAD)
# Generate completion using a host executable, never a cross-compiled one.
CGO_ENABLED=0 go build -buildvcs=false -trimpath -o "$work/asc-host" ./cmd/asc
"$work/asc-host" completion bash > "$work/completion"
bash -n "$work/completion"
artifacts=()
for arch in amd64 arm64; do
    name="asc-devtools_${version}_linux_${arch}"
    archive="$work/$name"
    mkdir -p "$archive/scripts"
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -buildvcs=false -trimpath \
        -ldflags "-s -w -X main.version=$version -X main.commit=$commit" \
        -o "$archive/asc" ./cmd/asc
    install -m 0644 LICENSE README.md "$archive/"
    install -m 0755 scripts/install.sh scripts/uninstall.sh "$archive/scripts/"
    tar --sort=name --mtime="@$SOURCE_DATE_EPOCH" --owner=0 --group=0 --numeric-owner \
        -C "$work" -czf "$output/$name.tar.gz" "$name"
    stage="$work/deb-$arch"
    mkdir -p "$stage/DEBIAN" "$stage/usr/bin" \
        "$stage/usr/share/bash-completion/completions" "$stage/usr/share/doc/asc-devtools"
    install -m 0755 "$archive/asc" "$stage/usr/bin/asc"
    install -m 0644 "$work/completion" "$stage/usr/share/bash-completion/completions/asc"
    install -m 0644 LICENSE "$stage/usr/share/doc/asc-devtools/copyright"
    cat > "$stage/DEBIAN/control" <<CONTROL
Package: asc-devtools
Version: $version
Section: devel
Priority: optional
Architecture: $arch
Maintainer: AI4SciComp <AI4SciComp@users.noreply.github.com>
Depends: git, ca-certificates
Suggests: cmake, bash-completion
Homepage: https://github.com/AI4SciComp/asc-devtools
Description: Conservative repository and build tools for AI4SciComp
 Dependency-free Go CLI for repository discovery, synchronization and builds.
CONTROL
    find "$stage" -print0 | xargs -0 touch -h -d "@$SOURCE_DATE_EPOCH"
    deb="asc-devtools_${version}_${arch}.deb"
    dpkg-deb --root-owner-group --build "$stage" "$output/$deb"
    artifacts+=("$name.tar.gz" "$deb")
done
(cd "$output" && sha256sum "${artifacts[@]}" > SHA256SUMS)
printf 'Release artifacts: %s\n' "$output"
