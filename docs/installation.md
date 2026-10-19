# Installation

## Windows 11 and WSL2 Ubuntu

Install WSL2 from an elevated PowerShell prompt if needed:

```powershell
wsl --install -d Ubuntu
wsl --update
wsl --set-default-version 2
```

Keep the checkout under the WSL filesystem, such as
`~/AI4SciComp/asc-devtools`, rather than `/mnt/c`.

Inside Ubuntu, install Git and build prerequisites:

```bash
sudo apt update
sudo apt install -y build-essential ca-certificates git
```

CMake and CTest are needed only for their wrapper commands:

```bash
sudo apt install -y cmake
```

Install a supported Go 1.25 or 1.26 toolchain from the official Go distribution
when building from source. A released `asc` binary does not need Go.

## Build from source

From the repository root:

```bash
CGO_ENABLED=0 go build -buildvcs=false -trimpath \
  -ldflags "-s -w -X main.version=1.0.0" \
  -o ./asc ./cmd/asc
./asc --version
```

For a development build on the current Go path:

```bash
go install ./cmd/asc
```

That command installs to `GOBIN`, or normally `$(go env GOPATH)/bin`. A build
without linker flags reports version `dev`.

## Managed system installation

```bash
sudo ./scripts/install.sh --binary ./asc
```

The default `/usr/local` installation owns exactly:

```text
/usr/local/bin/asc
/usr/local/share/bash-completion/completions/asc
/usr/local/share/asc-devtools/install-manifest
```

The manifest records the prefix and SHA-256 hashes. A later install refuses to
overwrite modified files or an installation without a valid manifest.

The installer can also build from the checkout:

```bash
sudo ./scripts/install.sh
sudo ./scripts/install.sh --go "$(command -v go)"
```

The explicit `--go` form is useful when `sudo` has a restricted `PATH`.

## User-local installation and PATH

```bash
./scripts/install.sh --binary ./asc --prefix "${HOME}/.local"
```

If needed, add this once to `~/.bashrc`:

```bash
export PATH="${HOME}/.local/bin:${PATH}"
```

Then start a new shell or run:

```bash
source ~/.bashrc
```

Completion is installed to
`~/.local/share/bash-completion/completions/asc`. If the distribution does not
load user completion automatically, use:

```bash
source <(asc completion bash)
```

## Custom prefix and package staging

The prefix must be an absolute path other than `/`:

```bash
sudo ./scripts/install.sh --binary ./asc --prefix /opt/asc
sudo ./scripts/uninstall.sh --prefix /opt/asc
```

Packagers can stage the same paths:

```bash
./scripts/install.sh --binary ./asc --destdir "${DESTDIR}"
./scripts/uninstall.sh --destdir "${DESTDIR}"
```

`DESTDIR` changes only the staging root; the recorded prefix remains
`/usr/local` unless `--prefix` is also supplied.

## Upgrade

Build or obtain the replacement binary, verify its provenance, and rerun the
same installer with the same prefix:

```bash
sudo ./scripts/install.sh --binary ./asc
```

The old binary and completion must still match their manifest hashes. There is
no network self-update command in v1.0.

## Uninstall

For the default prefix:

```bash
sudo ./scripts/uninstall.sh
```

For a user-local prefix:

```bash
./scripts/uninstall.sh --prefix "${HOME}/.local"
```

The uninstaller verifies hashes and removes only the binary, completion, and
manifest named above. Missing managed files are tolerated. Modified files,
symlinks, malformed manifests, and unrelated installations are refused. It
never recursively deletes a prefix.

## Verify

```bash
command -v asc
asc --version
asc --help
asc doctor
```

Public REST discovery works without `gh` or a token. Private discovery needs a
documented environment token; SSH or HTTPS cloning separately needs matching
Git transport credentials.

## Install without a source checkout

The release workflow builds Linux amd64 and arm64 artifacts when a stable tag
such as `v1.0.0` is pushed. It publishes a GitHub Release only after package tests and APT installation
checks pass.

Download the matching `.deb` and `SHA256SUMS` from
[GitHub Releases](https://github.com/AI4SciComp/asc-devtools/releases).
For Ubuntu/WSL2, `dpkg --print-architecture` reports `amd64` or `arm64`.
For example, after downloading the amd64 package for version 1.0.0:

```bash
# Verify the downloaded package against the release checksum file.
grep '  asc-devtools_1.0.0_amd64.deb$' SHA256SUMS | sha256sum -c -
sudo apt install ./asc-devtools_1.0.0_amd64.deb
asc --version
```

Checksums detect corruption; obtain both files from the trusted project release.
The package installs `/usr/bin/asc`, Bash completion under `/usr/share`, and
license information under `/usr/share/doc/asc-devtools`. APT installs the Git
and CA-certificate dependencies. CMake remains optional.

Before switching from a script installation, remove it using its original
uninstaller (`sudo ./scripts/uninstall.sh`, or add your original `--prefix`).
An older `/usr/local/bin/asc` or `~/.local/bin/asc` may otherwise shadow
`/usr/bin/asc`. Use `type -a asc` to inspect all versions on PATH. Never run the
script uninstaller against a package-managed `/usr` installation.

Upgrade by downloading and installing a newer `.deb` in the same way. Remove
with `sudo apt remove asc-devtools`. No project APT repository is provided yet,
so `apt upgrade` alone cannot discover new asc-devtools releases.

For an installation without sudo, download the matching
`asc-devtools_1.0.0_linux_amd64.tar.gz` (or arm64) and verify its checksum:

```bash
grep '  asc-devtools_1.0.0_linux_amd64.tar.gz$' SHA256SUMS | sha256sum -c -
tar -xzf asc-devtools_1.0.0_linux_amd64.tar.gz
cd asc-devtools_1.0.0_linux_amd64
bash scripts/install.sh --binary ./asc --prefix "$HOME/.local"
```

Keep the included uninstaller for later removal. The archive installation does
not require Go. Ensure `~/.local/bin` is on PATH as described above.

## Install with Go

With Go 1.25 or newer and repository access, no manual clone is needed:

```bash
go install github.com/AI4SciComp/asc-devtools/cmd/asc@latest
```

Once a stable release is tagged, pin it with `@v1.0.0` instead of `@latest`.
The executable goes into `GOBIN`, or normally `~/go/bin`; add that directory to
PATH. Go installation does not install Bash completion or an uninstall manifest.
Enable completion with `source <(asc completion bash)` and remove the executable
from the Go bin directory to uninstall. Without release linker flags the current
CLI reports version `dev`.

## Build and validate release artifacts

On Linux with Go, Git, Bash, GNU tar/coreutils, `dpkg-deb` and `fakeroot` installed:

```bash
bash scripts/package.sh 1.0.0
bash scripts/test_package.sh dist 1.0.0
```

This builds both architectures, includes the version and source commit in each
binary, and writes `.tar.gz`, `.deb`, and `SHA256SUMS` files under `dist/`.
The test uses an isolated dpkg database and filesystem root; it does not change
the host installation. Dependency resolution is exercised separately by CI
using APT. Set `SOURCE_DATE_EPOCH` to override the source commit timestamp used
for archive and package metadata. Reproducibility requires the same source,
Go toolchain and packaging tools.

Maintainers should run `make check` before tagging. Push a stable `vX.Y.Z` tag
to trigger `.github/workflows/release.yml`, then verify the published
release and its assets. This workflow does not configure an APT repository or publish to npm.
