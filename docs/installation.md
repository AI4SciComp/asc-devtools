# Installation

Python 3.11 or newer is required at runtime. Install from the checkout:

```bash
sudo ./scripts/install.sh
```

The default `/usr/local` layout is:

```text
/usr/local/bin/asc
/usr/local/lib/asc-devtools/asc_devtools/**
/usr/local/share/bash-completion/completions/asc
/usr/local/share/asc-devtools/install-manifest.json
```

The launcher resolves the package relative to its prefix and invokes
`python3 -m asc_devtools`. No pip environment or third-party package is needed.

Custom and staged layouts use:

```bash
sudo ./scripts/install.sh --prefix /opt/asc
./scripts/install.sh --destdir "${DESTDIR}"
```

Uninstall the exact managed files with matching arguments:

```bash
sudo ./scripts/uninstall.sh
```

The versioned manifest records every relative path and SHA-256 hash. Upgrade and
uninstall refuse missing manifests, symlinks, changed files, or unexpected file
sets. Removal never recursively deletes a prefix.

## Self-update

```bash
asc update --check
asc update
asc update --yes
sudo asc update --yes  # for a root-owned /usr/local installation
```

The command infers the prefix from the installed module layout; pass
`--prefix /opt/asc` when needed. It downloads `asc-devtools-python.tar.gz` and
`SHA256SUMS` from the latest GitHub release, verifies the hash, safely extracts
the bundle, and calls the normal installer. It refuses unmanaged or locally
modified installations and never invokes `sudo`.

Release maintainers create the update bundle with:

```bash
./scripts/package_update.sh --output ./dist/release
```

Upload the resulting archive and `SHA256SUMS` as assets on the same release.
The source version in the bundle must match the release tag.

Verify with `asc --version`, `asc --help`, and `asc doctor`.
Installation does not vendor CMake modules. Clone `asc-cmake`, run
`asc cmake vendor plan REPOSITORY`, and review the plan before explicit apply.
