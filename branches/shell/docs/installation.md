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
