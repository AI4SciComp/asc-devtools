# asc-devtools

`asc-devtools` provides the `asc` command for consistent, conservative local
development workflows across repositories owned by the
[`AI4SciComp`](https://github.com/AI4SciComp) organization. It discovers
organization repositories, clones missing checkouts, inspects and fast-forwards
local Git repositories, and invokes repository-owned CMake presets.

The GitHub identities have distinct roles: `AI4SciComp` is the organization that
owns and is queried for repositories; `escapetiger` is a developer account that
may authenticate GitHub CLI. Authentication never changes repository ownership.

Version 0.1 does not commit, push, change branches, delete files, publish
releases, or orchestrate scientific workloads.

## Requirements

- Ubuntu or Ubuntu under WSL2
- Python 3.11 or newer
- Git
- [GitHub CLI](https://cli.github.com/) authenticated for API discovery
- SSH access to GitHub when `clone_protocol = "ssh"`
- CMake and CTest for configure, build, and test commands

## Installation on WSL2

Install system prerequisites, then authenticate GitHub CLI:

```bash
sudo apt update
sudo apt install git gh python3 python3-venv pipx cmake
gh auth login
gh auth status
```

When prompted by `gh auth login`, select GitHub.com and the Git transport you
intend to configure in `asc`. API discovery uses the token stored by `gh`; SSH
cloning uses a GitHub-associated SSH key. These are separate credential paths.

From this repository, an isolated installation is preferred:

```bash
pipx install .
```

Alternatively:

```bash
python3 -m pip install --user .
```

For active development:

```bash
python3 -m venv .venv
source .venv/bin/activate
python -m pip install --editable '.[dev]'
```

See [installation.md](docs/installation.md) for PATH and completion setup.

## Configuration

The default configuration is equivalent to:

```toml
[asc]
organization = "AI4SciComp"
workspace = "~/projects/AI4SciComp"
repository_prefix = "asc-"
include_dot_github = true
clone_protocol = "ssh"
```

Place overrides in `~/.config/asc/config.toml`, use `asc --config PATH ...`, or
set `ASC_CONFIG`, `ASC_ORGANIZATION`, `ASC_WORKSPACE`,
`ASC_REPOSITORY_PREFIX`, or `ASC_CLONE_PROTOCOL`. Precedence is command-line
override, environment, configuration file, then built-in default. Paths expand
`~` and resolve without requiring the workspace to exist.

## Quick start

```bash
asc doctor
asc workspace
asc repo list
asc repo clone asc-cpp asc-pde
asc repo status
asc repo sync
asc configure asc-cpp --preset dev
asc build asc-cpp --preset dev
asc test asc-cpp --preset dev
```

`asc repo clone` without names clones every matching missing repository. Status
and sync without names operate on managed Git repositories that are direct
children of the workspace. `asc repo list --json` provides scriptable discovery
output.

## Safety guarantees

- External commands receive argument arrays and never execute through a shell.
- Repository names cannot be paths and operations stay inside the workspace.
- Clone skips Git checkouts and refuses an existing non-Git destination.
- Status is read-only and continues after a broken repository.
- Sync refuses dirty trees, fetches, and merges only with `--ff-only`.
- Sync never stashes, rebases, resets, checks out, or discards work.
- CMake commands only invoke presets committed by the selected repository.
- Expected failures are concise and do not show Python tracebacks or secrets.

See [commands.md](docs/commands.md) for exact behavior and exit statuses and
[architecture.md](docs/architecture.md) for module boundaries and the safety
model.

## Development

```bash
python -m unittest discover -s tests -v
python -m compileall -q src tests
ruff check src tests
ruff format --check src tests
mypy src
python -m build
```

Tests are offline: they use mocks and temporary Git repositories rather than the
real organization or developer workspace. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Organization tooling

`asc-devtools` manages developer workflows across organization repositories.
The `AI4SciComp/.github` repository owns organization-wide GitHub metadata and
workflow defaults; it is treated as a managed repository when configured.
Individual repositories retain their own build options and `CMakePresets.json`.
A future `asc-cmake` repository may centralize reusable CMake modules, but this
tool will continue to call repository-owned presets rather than duplicate their
configuration.

Licensed under Apache-2.0. See [LICENSE](LICENSE).
