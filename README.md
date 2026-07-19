# asc-devtools

This repository maintains three behavior-compatible implementations of the
`asc` developer CLI. The complete source trees are grouped under `branches/`:

| Implementation | Source directory | Standalone branch |
| --- | --- | --- |
| Go | [`branches/go`](branches/go) | `go` |
| Python | [`branches/python`](branches/python) | `python` |
| Bash | [`branches/shell`](branches/shell) | `shell` |

Each implementation contains its own `README.md`, `generator.md`, source,
tests, documentation, installation scripts, and verified uninstall support.
They share the same command surface, JSON configuration, GitHub REST behavior,
Git safety rules, CMake/CTest invocation, and exit-code contract.

## Functionality

| Functionality | Commands | Go | Python | Bash |
| --- | --- | :---: | :---: | :---: |
| Workspace discovery and diagnostics | `workspace`, `doctor` | Yes | Yes | Yes |
| GitHub repository discovery | `repo list` | Yes | Yes | Yes |
| Safe repository cloning | `repo clone` | Yes | Yes | Yes |
| Local Git status reporting | `repo status` | Yes | Yes | Yes |
| Fast-forward-only synchronization | `repo sync` | Yes | Yes | Yes |
| Reviewed commit and push workflow | `repo save` | Yes | Yes | Yes |
| CMake configure, build, and test | `configure`, `build`, `test` | Yes | Yes | Yes |
| CMake workflows and preset discovery | `cmake workflow`, `cmake presets` | Yes | Yes | Yes |
| Guarded CMake module vendoring | `cmake vendor` | Yes | Yes | Yes |
| Verified self-update | `update` | Yes | Yes | Yes |
| Bash completion | `completion bash` | Yes | Yes | Yes |
| Manifest-protected install and uninstall | `scripts/install.sh`, `scripts/uninstall.sh` | Yes | Yes | Yes |

## Choosing an implementation

Go produces one static executable and is the reference implementation:

```bash
cd branches/go
sudo ./scripts/install.sh
```

The Go installer searches conventional system locations even when `sudo`
restricts `PATH`. For a toolchain installed elsewhere, pass it explicitly with
`sudo ./scripts/install.sh --go "$(command -v go)"`.

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
- `go`, `python`, and `shell` are standalone distributable trees.
- Behavioral changes must be applied to all affected implementations and tested
  from both the standalone branch and the corresponding `branches/*` directory.

Licensed under Apache-2.0. See [LICENSE](LICENSE).
