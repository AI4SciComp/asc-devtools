# Installation

Run directly from a checkout with `./bin/asc`, or install under `/usr/local`:

```bash
sudo ./scripts/install.sh
asc --version
```

The default layout is:

```text
/usr/local/bin/asc
/usr/local/lib/asc/*.sh
/usr/local/share/bash-completion/completions/asc
/usr/local/share/asc-devtools/install-manifest
```

Custom and staged layouts use:

```bash
sudo ./scripts/install.sh --prefix /opt/asc
./scripts/install.sh --destdir "${DESTDIR}"
```

The installer refuses untracked existing targets. An upgrade requires a valid
manifest and refuses symlinks or locally modified managed files.

## Self-update

```bash
asc update --check
asc update
asc update --yes
sudo asc update --yes  # for a root-owned /usr/local installation
```

The command infers the prefix from the installed launcher; pass
`--prefix /opt/asc` when needed. It downloads `asc-devtools-shell.tar.gz` and
`SHA256SUMS` from the latest GitHub release, verifies the hash, safely extracts
the bundle, and calls the normal installer. It refuses unmanaged or locally
modified installations and never invokes `sudo`.

Release maintainers create the update bundle with:

```bash
./scripts/package_update.sh --output ./dist/release
```

Upload the resulting archive and `SHA256SUMS` as assets on the same release.
The embedded `ASC_VERSION` must match the release tag.

Uninstall with matching arguments:

```bash
sudo ./scripts/uninstall.sh
```

The uninstaller verifies every managed SHA-256 hash before removal. It removes
only the explicit file list and empty asc-specific directories; it never
recursively deletes a prefix. Missing installations are successful no-ops.

The standard completion directory is loaded automatically where
`bash-completion` is configured. It can also be loaded with
`source <(asc completion bash)`.

Installation does not vendor CMake modules. Clone `asc-cmake`, run
`asc cmake vendor plan REPOSITORY`, and review the plan before explicit apply.
