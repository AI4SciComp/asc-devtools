# Installation

## Release binary

A release binary does not require Go. Install it under the conventional
`/usr/local` prefix:

```bash
sudo ./scripts/install.sh --binary ./asc
```

This installs:

- `/usr/local/bin/asc`
- `/usr/local/share/bash-completion/completions/asc`
- `/usr/local/share/asc-devtools/install-manifest`

The manifest records hashes for the managed executable and completion. An
upgrade refuses to overwrite either file if it was modified or if the manifest
is missing.

## Build on Ubuntu/WSL2

Go 1.25 or newer is required to build:

```bash
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=0.1.0" \
  -o ./dist/asc ./cmd/asc
sudo ./scripts/install.sh --binary ./dist/asc
```

The installer can build from the checkout when the invoking environment has Go:

```bash
sudo ./scripts/install.sh
```

Building before `sudo` is recommended because it uses the developer's selected
Go toolchain and cache. The scripts never invoke `sudo` themselves.

Select a different absolute prefix when required:

```bash
sudo ./scripts/install.sh --binary ./dist/asc --prefix /opt/asc
sudo ./scripts/uninstall.sh --prefix /opt/asc
```

Packagers and lifecycle tests can stage the same layout without privilege:

```bash
./scripts/install.sh --binary ./dist/asc --destdir "${DESTDIR}"
./scripts/uninstall.sh --destdir "${DESTDIR}"
```

`DESTDIR` changes the filesystem staging root, not the recorded `/usr/local`
prefix. Neither script downloads files, uses telemetry, or edits shell startup
configuration.

## Uninstall

```bash
sudo ./scripts/uninstall.sh
```

The uninstaller reads the versioned manifest, verifies both managed file hashes,
and removes only the three paths listed above. Missing files are tolerated;
modified files, symlinks, malformed manifests, and unrelated installations are
refused. It never recursively removes `/usr/local` or any other prefix.

## Completion

System installation places completion in
`/usr/local/share/bash-completion/completions/asc` automatically.

For the current shell only:

```bash
source <(asc completion bash)
```

## Verification

```bash
asc --version
asc --help
asc doctor
```

API discovery works publicly without GitHub CLI. Configure a token for private
repositories and separately configure Git SSH or HTTPS credentials for cloning.
