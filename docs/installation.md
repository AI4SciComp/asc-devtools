# Installation

The current release is [v1.0.0](https://github.com/AI4SciComp/asc-devtools/releases/tag/v1.0.0).
Prebuilt packages support Linux amd64 and arm64, including Ubuntu in WSL2.
Go is required only for source builds and `go install`.

## Ubuntu and WSL2: Debian package

In Windows, set up WSL2 from an elevated PowerShell prompt if needed:

```powershell
wsl --install -d Ubuntu
wsl --update
wsl --set-default-version 2
```

Run the following in Ubuntu. `dpkg --print-architecture` must report `amd64`
or `arm64`:

```bash
arch=$(dpkg --print-architecture)
base=https://github.com/AI4SciComp/asc-devtools/releases/download/v1.0.0
curl -fLO "$base/asc-devtools_1.0.0_${arch}.deb"
curl -fLO "$base/SHA256SUMS"
grep "  asc-devtools_1.0.0_${arch}.deb\$" SHA256SUMS | sha256sum -c -
sudo apt install "./asc-devtools_1.0.0_${arch}.deb"
asc --version
```

You can also download the package and `SHA256SUMS` through the release page.
Checksums detect corruption; obtain both files from the trusted project release.

APT installs Git and CA certificates. The package owns:

```text
/usr/bin/asc
/usr/share/bash-completion/completions/asc
/usr/share/doc/asc-devtools/copyright
```

CMake and CTest are needed only for `configure`, `build`, and `test`:

```bash
sudo apt install cmake
```

To upgrade, download and verify the newer `.deb`, then install it with
`sudo apt install ./PACKAGE.deb`. There is no project APT software source, so
`apt upgrade` alone cannot discover new asc-devtools releases.

Remove the package with:

```bash
sudo apt remove asc-devtools
```

## Binary archive: installation without sudo

Download the archive matching the Ubuntu/Debian architecture:

```bash
arch=$(dpkg --print-architecture)
base=https://github.com/AI4SciComp/asc-devtools/releases/download/v1.0.0
curl -fLO "$base/asc-devtools_1.0.0_linux_${arch}.tar.gz"
curl -fLO "$base/SHA256SUMS"
grep "  asc-devtools_1.0.0_linux_${arch}.tar.gz\$" SHA256SUMS | sha256sum -c -
tar -xzf "asc-devtools_1.0.0_linux_${arch}.tar.gz"
cd "asc-devtools_1.0.0_linux_${arch}"
bash scripts/install.sh --binary ./asc --prefix "$HOME/.local"
```

On other Linux distributions, choose `amd64` for x86-64 or `arm64` for AArch64
from the release page. The installer requires Bash and `sha256sum`.

Ensure the installation directory is on PATH. Add this line to `~/.bashrc`
for Bash, or `~/.zshrc` for Zsh, then start a new shell:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

The installer owns the binary, Bash completion and hash manifest under the
selected prefix. It refuses to overwrite modified files or unrelated installs.
Keep the included scripts for later upgrades and removal:

```bash
bash scripts/uninstall.sh --prefix "$HOME/.local"
```

Upgrade by downloading and verifying a newer archive, then running its installer
with the same prefix. For a system installation, use
`sudo bash scripts/install.sh --binary ./asc` (default prefix `/usr/local`)
and `sudo bash scripts/uninstall.sh` to remove it.

## Install with Go

With Go 1.25 or newer, install a specific release without cloning:

```bash
go install github.com/AI4SciComp/asc-devtools/cmd/asc@v1.0.0
```

Use `@latest` to install the latest stable version. The executable goes into
`GOBIN`, or normally `~/go/bin`; add that directory to PATH.

Go installation does not install Bash completion or an uninstall manifest.
Without release linker flags, the CLI reports version `dev`; prebuilt release
binaries report `1.0.0` and their source commit. Remove the executable from the
Go bin directory to uninstall.

## Build from source

For development, install Git and Go 1.25 or newer, then clone the repository:

```bash
git clone https://github.com/AI4SciComp/asc-devtools.git
cd asc-devtools
make build
./asc --version
```

`make install-user` installs under `~/.local`. `make install` uses sudo to
install under `/usr/local`. Corresponding removal commands are
`make uninstall-user` and `make uninstall`.

The installer can also build directly, using `--go` if sudo hides Go from PATH:

```bash
sudo ./scripts/install.sh --go "$(command -v go)"
```

## Switch installation methods

Check existing commands before switching:

```bash
type -a asc
```

Remove a script installation using its original uninstaller and prefix before
installing a `.deb`. An executable in `/usr/local/bin`, `~/.local/bin` or the
Go bin directory may otherwise shadow `/usr/bin/asc`.

Remove Debian packages through APT. Use the supplied uninstaller only for
script installations; it verifies manifest hashes before removing managed
files. User configuration and workspaces remain in place.

## Bash completion and verification

Debian packages and script installations include Bash completion. If your shell
does not load it automatically, enable it in the current Bash session:

```bash
source <(asc completion bash)
```

Verify the selected installation:

```bash
command -v asc
asc --version
asc --help
asc doctor
```

Public repository discovery needs no token. Private discovery requires
`ASC_GITHUB_TOKEN`, `GH_TOKEN`, or `GITHUB_TOKEN`. SSH or HTTPS Git transport
uses Git credentials independently; see [authentication](../README.md#authentication).

## Build and publish release artifacts

On Linux, install Go, Git, Bash, GNU tar/coreutils, `dpkg-deb` and `fakeroot`:

```bash
make check
make package-test
```

`make package` builds Linux amd64 and arm64 `.tar.gz` and `.deb` files plus
`SHA256SUMS` under `dist/`. Override the version with `VERSION=X.Y.Z`.
Package tests use an isolated dpkg database and filesystem root; host packages
are unchanged. CI separately exercises real APT installation and removal.

Archive and package timestamps use the source commit time, or
`SOURCE_DATE_EPOCH` when supplied. Reproducibility requires identical source,
Go toolchain and packaging tools.

The [release workflow](../.github/workflows/release.yml) publishes assets when
a stable `vX.Y.Z` tag is pushed, after package and APT checks pass.
