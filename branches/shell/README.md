# asc-devtools

This branch provides the Bash implementation of `asc`, behaviorally equivalent
to the Go and Python variants maintained on `go` and `python`. Its
implementation specification is `generator.md`.

`asc` discovers AI4SciComp repositories through the GitHub REST API, clones
missing worktrees, reports Git state, performs conservative fast-forward-only
updates, and invokes repository-owned CMake presets.

## Requirements

- Bash 4.4 or newer on Ubuntu or WSL2
- Git and curl for repository and GitHub operations
- CMake and CTest for their corresponding workflow commands
- SSH only for SSH transport and the optional doctor probe
- GitHub CLI only as an optional token fallback

There are no Python, jq, framework, or third-party runtime dependencies.

## Install

Install directly under the conventional `/usr/local` prefix:

```bash
sudo ./scripts/install.sh
asc --version
```

This installs the executable, Bash modules, completion, and a hash manifest.
Remove only those verified files with:

```bash
sudo ./scripts/uninstall.sh
```

Both scripts accept `--prefix PATH` and `--destdir PATH`. They do not download
packages, invoke `sudo`, edit shell configuration, or recursively delete a
prefix. See [installation.md](docs/installation.md).

## Configuration

The default file is `~/.config/asc/config.json`:

```json
{
  "organization": "AI4SciComp",
  "workspace": "~/projects/AI4SciComp",
  "repositoryPrefix": "asc-",
  "includeDotGitHub": true,
  "cloneProtocol": "ssh",
  "remote": "origin"
}
```

Precedence is CLI, environment, JSON file, then default. Environment overrides
are `ASC_CONFIG`, `ASC_ORGANIZATION`, `ASC_WORKSPACE`,
`ASC_REPOSITORY_PREFIX`, `ASC_INCLUDE_DOT_GITHUB`, `ASC_CLONE_PROTOCOL`, and
`ASC_REMOTE`. Unknown fields, duplicate fields, and malformed types are rejected.

API token precedence is `ASC_GITHUB_TOKEN`, `GH_TOKEN`, `GITHUB_TOKEN`, then an
optional silent `gh auth token` fallback. Tokens are used only in the REST
authorization header and are never stored or printed.

## Commands

```bash
asc doctor
asc doctor --json
asc workspace
asc repo list
asc repo list --json
asc repo clone asc-cpp --protocol ssh
asc repo status
asc repo status --json
asc repo sync --dry-run
asc repo sync
asc configure asc-cpp --preset dev
asc build asc-cpp --preset dev
asc test asc-cpp --preset dev
source <(asc completion bash)
```

Global options must precede the command: `--config`, `--organization`,
`--workspace`, and `--no-color`. See [commands.md](docs/commands.md) for schemas,
exit codes, and exact external commands.

## Safety

- External commands use arrays or quoted argument lists; input is never evaluated.
- Repository names are validated components and destinations are direct children.
- Symlinked worktrees and mismatched existing remotes are refused.
- Status is local, read-only, nonrecursive, and uses porcelain v2.
- Sync skips dirty, detached, missing-upstream, wrong-remote, and divergent work.
- Sync runs only `fetch` and `merge --ff-only`; dry-run runs neither.
- There is no reset, clean, stash, rebase, checkout, commit, push, or force path.
- REST requests have time and response-size limits; error bodies are not echoed.

## Development

```bash
bash -n bin/asc lib/asc/*.sh completions/asc.bash scripts/*.sh tests/*.sh
shellcheck bin/asc lib/asc/*.sh completions/asc.bash scripts/*.sh tests/*.sh
shfmt -d -i 2 -ci bin/asc lib/asc/*.sh completions/asc.bash scripts/*.sh tests/*.sh
./tests/run_tests.sh
```

Tests use executable fakes and temporary real Git repositories. They do not
access live GitHub, credentials, user configuration, or the developer workspace.

Licensed under Apache-2.0. See [LICENSE](LICENSE).
