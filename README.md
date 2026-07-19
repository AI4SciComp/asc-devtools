# asc-devtools

This repository maintains three behavior-compatible implementations of the
`asc` developer CLI. The complete source trees are grouped under `branches/`:

| Implementation | Source directory | Standalone branch |
| --- | --- | --- |
| Go | [`branches/go`](branches/go) | `impl/go` |
| Python | [`branches/python`](branches/python) | `impl/python` |
| Bash | [`branches/shell`](branches/shell) | `impl/shell` |

Each implementation contains its own `README.md`, `generator.md`, source,
tests, documentation, installation scripts, and verified uninstall support.
They share the same command surface, JSON configuration, GitHub REST behavior,
Git safety rules, CMake/CTest invocation, and exit-code contract.

## Choosing an implementation

Go produces one static executable and is the reference implementation:

```bash
cd branches/go
sudo ./scripts/install.sh
```

Python 3.11+ uses only the standard library at runtime:

```bash
cd branches/python
sudo ./scripts/install.sh
```

Bash 4.4+ has no Python, jq, or compiled runtime dependency:

```bash
cd branches/shell
sudo ./scripts/install.sh
```

All installers default to `/usr/local`, accept custom prefix and staging
options, and maintain manifests so uninstall removes only verified managed
files. Run the matching `sudo ./scripts/uninstall.sh` to remove an installation.

## Repository policy

- `main` is the combined source view and runs all implementation test suites.
- `impl/go`, `impl/python`, and `impl/shell` are standalone distributable trees.
- Behavioral changes must be applied to all affected implementations and tested
  from both the standalone branch and the corresponding `branches/*` directory.

Licensed under Apache-2.0. See [LICENSE](LICENSE).
