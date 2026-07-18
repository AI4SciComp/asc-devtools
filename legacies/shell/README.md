# asc-devtools

`asc-devtools` is a Bash 4.4+ command-line tool for conservative local workflows
across repositories owned by the
[`AI4SciComp`](https://github.com/AI4SciComp) organization. The `asc` command
discovers organization repositories, clones missing worktrees, reports local Git
state, performs fast-forward-only updates, and invokes repository-owned CMake
presets.

The tool is developer infrastructure. It does not commit, push, change branches,
publish releases, orchestrate agents, or implement scientific models.

The previous Python implementation is preserved for reference under
[`legacies/python`](legacies/python). It is not installed, tested by active CI, or
required at runtime.

## Requirements

- Ubuntu or Ubuntu under WSL2
- Bash 4.4 or newer; this project is not POSIX `sh` compatible
- Git and GitHub CLI (`gh`)
- CMake and CTest for C++ configure/build/test commands
- SSH client when using the default SSH clone protocol

There are no Python, jq, framework, or third-party runtime dependencies.
ShellCheck and shfmt are optional development tools.

## WSL2 setup

```bash
sudo apt update
sudo apt install bash git gh cmake openssh-client
gh auth login
gh auth status
ssh -T git@github.com
```

`escapetiger` is the personal account that may authenticate the tools;
`AI4SciComp` is the organization queried for and used to construct clone URLs.
GitHub CLI API discovery uses the token stored by `gh auth login`. Git SSH clones
use a GitHub-associated SSH key. Those credentials are separate.

## Run or install

Run directly from the checkout:

```bash
./bin/asc --help
```

Install to `~/.local` without sudo:

```bash
./install.sh
export PATH="${HOME}/.local/bin:${PATH}"
```

Use another absolute prefix with `./install.sh --prefix PATH`. Installation is
copy-based and safe to repeat. It refuses to overwrite unmarked files. Remove
only files owned by this tool with `./uninstall.sh [--prefix PATH]`.

## Configuration

The optional configuration is `~/.config/asc/config`, or the path in
`ASC_CONFIG`/`--config`. It is sourced as executable Bash and must be a trusted,
user-owned file. Do not copy configuration from an untrusted source.

```bash
ASC_ORGANIZATION="AI4SciComp"
ASC_WORKSPACE="${HOME}/projects/AI4SciComp"
ASC_REPOSITORY_PREFIX="asc-"
ASC_INCLUDE_DOT_GITHUB="true"
ASC_CLONE_PROTOCOL="ssh"
ASC_REMOTE="origin"
```

Precedence is explicit global CLI option, exported environment, configuration
file, then built-in default. Supported global overrides include `--organization`,
`--workspace`, `--repository-prefix`, `--clone-protocol`, `--remote`, and the
include/exclude `.github` switches.

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
source <(asc completion bash)
```

Clone without names processes all matching organization repositories. Status and
sync without names process managed Git worktrees that are direct children of the
workspace.

## Safety guarantees

- Repository arguments are names, never paths or executable syntax.
- Local discovery does not recurse into dependencies or build directories.
- Clone creates missing destinations and never replaces existing data.
- Status uses Git porcelain output and does not mutate repositories.
- Sync refuses dirty trees, detached HEAD, missing remotes/upstreams, and
  divergent histories; it fetches and merges only with `--ff-only`.
- No reset, clean, stash, rebase, checkout, commit, push, force operation, or
  broad recursive deletion exists in `asc`.
- CMake commands use arrays and repository-owned presets.
- Multi-repository operations continue safely, summarize outcomes, and return
  nonzero for incomplete work.

See [commands.md](docs/commands.md) for exact exit semantics and
[architecture.md](docs/architecture.md) for module boundaries.

## Development and testing

```bash
bash -n bin/asc lib/asc/*.sh completions/asc.bash tests/*.sh install.sh uninstall.sh
shellcheck bin/asc lib/asc/*.sh completions/asc.bash tests/*.sh install.sh uninstall.sh
shfmt -d -i 2 -ci bin/asc lib/asc/*.sh completions/asc.bash tests/*.sh install.sh uninstall.sh
./tests/run_tests.sh
```

The test suite is pure Bash and offline. It uses isolated homes, mock tools, and
temporary real Git repositories with repository-local identities.

## Organization tooling

`AI4SciComp/.github` owns organization GitHub metadata and is optionally managed
like other repositories. Each scientific repository owns its
`CMakePresets.json`. A future `asc-cmake` may provide shared CMake modules, while
`asc-devtools` remains a thin local workflow layer.

Licensed under Apache-2.0. See [LICENSE](LICENSE).
