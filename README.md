# asc-devtools

`asc-devtools` provides one dependency-free Go binary, `asc`, for conservative
local development workflows across repositories owned by
[`AI4SciComp`](https://github.com/AI4SciComp). It discovers organization
repositories through the GitHub REST API, clones missing worktrees, reports Git
state, performs fast-forward-only updates, and invokes repository-owned CMake
presets.

This is developer infrastructure. It does not commit, push, change branches,
publish releases, orchestrate agents, or implement scientific models.

Earlier implementations are preserved for reference under
[`legacies/python`](legacies/python) and [`legacies/shell`](legacies/shell). They
are not active runtime or CI dependencies.

## Requirements

- A release `asc` binary needs only Git for repository commands.
- CMake and CTest are needed only by their corresponding workflow commands.
- SSH is needed only for SSH transport and the optional doctor probe.
- GitHub CLI is optional and used only as a token fallback.
- Building from source requires Go 1.25 or newer.

The binary uses only the Go standard library. There are no linked third-party
modules, and runtime users do not need Go, Python, Node.js, Ruby, jq, or a package
manager.

## Build and install on WSL2

```bash
CGO_ENABLED=0 go build -trimpath -o ./dist/asc ./cmd/asc
sudo ./scripts/install.sh --binary ./dist/asc
asc --version
```

The default prefix is `/usr/local`, so the executable is installed at
`/usr/local/bin/asc` and is normally available on `PATH`. The installer also
adds Bash completion and a hash manifest used to protect upgrades and removal.

To build and install in one step when root's environment contains Go:

```bash
sudo ./scripts/install.sh
```

Use another system prefix or a packaging staging root explicitly:

```bash
sudo ./scripts/install.sh --binary ./dist/asc --prefix /opt/asc
./scripts/install.sh --binary ./dist/asc --destdir "${DESTDIR}"
```

Uninstall the exact managed files with:

```bash
sudo ./scripts/uninstall.sh
```

Neither script downloads tools, invokes `sudo`, changes shell configuration, or
recursively deletes a prefix. See [installation.md](docs/installation.md).

## Authentication

`AI4SciComp` is the repository owner. `escapetiger` is a personal account that
may supply credentials; authentication never changes repository ownership.

Public REST discovery works without authentication. Private repositories and
higher API limits require a token, selected in this order:

```text
ASC_GITHUB_TOKEN
GH_TOKEN
GITHUB_TOKEN
gh auth token (optional fallback)
```

To avoid putting a token in shell history:

```bash
read -rsp 'GitHub token: ' ASC_GITHUB_TOKEN
printf '\n'
export ASC_GITHUB_TOKEN
```

`asc` sends the token only in the GitHub API authorization header, never stores
or prints it. REST API authentication is distinct from Git transport:

- SSH clone URLs use a GitHub-associated SSH key.
- HTTPS clone URLs use Git's configured HTTPS credentials.
- The REST API uses the token above.

## Configuration

The default file is `~/.config/asc/config.json`; select another with
`ASC_CONFIG` or global `--config PATH`.

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

Precedence is CLI, environment, JSON file, then default. Supported environment
overrides are `ASC_ORGANIZATION`, `ASC_WORKSPACE`, `ASC_REPOSITORY_PREFIX`,
`ASC_INCLUDE_DOT_GITHUB`, `ASC_CLONE_PROTOCOL`, and `ASC_REMOTE`. Unknown JSON
fields and malformed values are rejected.

Global options must precede the command:

```text
--config PATH --organization NAME --workspace PATH --no-color
```

## Quick start

```bash
asc doctor
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

Clone without names processes every eligible API repository. Status and sync
without names process managed Git worktrees that are direct workspace children.

## Safety guarantees

- External commands use `os/exec` argument slices; no shell evaluates input.
- Repository names are validated GitHub path segments, not filesystem paths.
- Destinations are verified direct children, and repository symlinks are refused.
- Clone never deletes, replaces, or merges existing destination data.
- Existing worktrees must point at the expected API repository.
- Status is local, read-only, nonrecursive, and uses porcelain-v2 output.
- Sync skips dirty, detached, no-upstream, wrong-remote, and divergent states.
- Sync performs only fetch plus `merge --ff-only`; dry-run performs neither.
- There is no reset, clean, stash, rebase, checkout, commit, push, force
  operation, telemetry, or credential storage.
- GitHub responses and error bodies have size limits and HTTP requests time out.

See [commands.md](docs/commands.md) and
[architecture.md](docs/architecture.md) for exact behavior.

## Development

```bash
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
go test -race ./...
CGO_ENABLED=0 go build -trimpath -o /tmp/asc ./cmd/asc
```

Tests use only the standard `testing` package, `httptest`, fake process runners,
and temporary local Git repositories. They never use live GitHub or user state.

`AI4SciComp/.github` owns organization metadata and is optionally managed like
other repositories. Scientific repositories own their `CMakePresets.json`; a
future `asc-cmake` may supply shared CMake modules without moving build policy
into this CLI.

Licensed under Apache-2.0. See [LICENSE](LICENSE).
