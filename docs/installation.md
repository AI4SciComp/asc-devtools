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
  -ldflags "-s -w -X main.version=0.1.0" \
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
no network self-update command in v0.1.

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
