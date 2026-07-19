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

## Prerequisites

| Implementation | Required runtime | Build or install requirement |
| --- | --- | --- |
| Go | Git for repository commands | Go 1.25+ only when building from source |
| Python | Python 3.11+ and Git | No third-party Python packages |
| Bash | Bash 4.4+, Git, curl, and standard Unix tools | No compiled-language toolchain |

All implementations call the GitHub REST API directly. GitHub CLI (`gh`) is
neither required nor invoked. Public repositories work without a token; private
repository discovery requires `ASC_GITHUB_TOKEN`, `GH_TOKEN`, or
`GITHUB_TOKEN`. SSH is needed only for SSH clone/push transport, while CMake and
CTest are needed only for their corresponding workflow commands.

## Functionality

| Functionality | Commands | Go | Python | Bash |
| --- | --- | :---: | :---: | :---: |
| Workspace discovery and diagnostics | `workspace`, `doctor` | Yes | Yes | Yes |
| GitHub repository discovery | `repo list` | Yes | Yes | Yes |
| Safe repository cloning | `repo clone` | Yes | Yes | Yes |
| Local Git status reporting | `repo status` | Yes | Yes | Yes |
| Fast-forward-only synchronization | `repo sync` | Yes | Yes | Yes |
| Reviewed commit and push workflow with timestamp default | `repo save` | Yes | Yes | Yes |
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

## GitHub branch workflow

This repository has one combined `main` branch and three standalone branches,
so use the repository Makefile instead of a generic single-branch save command.
The workflow requires GNU Make, Bash, Git, rsync, and tar; it never invokes
GitHub CLI.

```bash
make github-status
make github-import
git diff
make github-publish
```

`github-import` requires a clean, up-to-date `main` checkout and copies the
remote `go`, `python`, and `shell` trees into their matching `branches/*`
directories for review. It does not commit or push.

`github-publish` stages all intended `main` changes, commits with
`Updated at YYYY-MM-DD HH:MM:SS` by default, derives standalone commits from the
committed `branches/*` trees, verifies exact tree equality, and pushes every
changed branch with one atomic Git push. A rejected branch therefore leaves all
remote refs unchanged; the local commit remains available to inspect or retry.
Override the message with:

```bash
make github-publish MSG="Describe the coordinated update"
```

Use `make github-check` to require a clean published `main` and exact parity with
all three standalone branches. `make help` lists the complete workflow.

## Repository policy

- `main` is the combined source view and runs all implementation test suites.
- `go`, `python`, and `shell` are standalone distributable trees.
- Behavioral changes must be applied to all affected implementations and tested
  from both the standalone branch and the corresponding `branches/*` directory.

Licensed under Apache-2.0. See [LICENSE](LICENSE).
