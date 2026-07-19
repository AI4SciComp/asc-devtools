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

Verify with `asc --version`, `asc --help`, and `asc doctor`.
