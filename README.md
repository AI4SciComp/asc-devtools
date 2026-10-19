# asc-devtools

`asc-devtools` provides `asc`, a small dependency-free Go CLI for conservative
repository operations across the
[`AI4SciComp`](https://github.com/AI4SciComp) organization.

It can discover and clone organization repositories, report local Git status,
download clean fast-forwards, explicitly save local work to a tracked remote
branch, and invoke repository-owned CMake/CTest presets. It is developer
infrastructure: it does not implement scientific models, run agents, publish
releases, or open pull requests.

## Requirements

- Git for repository commands.
- Network access for GitHub discovery, clone, sync, and save.
- CMake for `configure` and `build`; CTest for `test`.
- Go 1.25 or newer only when building from source.
- Bash and `sha256sum` only for the supplied install/uninstall scripts.

The binary uses only the Go standard library. GitHub CLI (`gh`) is not required
or invoked at runtime.

## Install on Ubuntu or WSL2

The current release is [v1.0.0](https://github.com/AI4SciComp/asc-devtools/releases/tag/v1.0.0).
Install the Debian package without cloning the repository or installing Go:

```bash
arch=$(dpkg --print-architecture)  # amd64 or arm64
base=https://github.com/AI4SciComp/asc-devtools/releases/download/v1.0.0
curl -fLO "$base/asc-devtools_1.0.0_${arch}.deb"
curl -fLO "$base/SHA256SUMS"
grep "  asc-devtools_1.0.0_${arch}.deb\$" SHA256SUMS | sha256sum -c -
sudo apt install "./asc-devtools_1.0.0_${arch}.deb"
asc --version
```

The package installs `/usr/bin/asc` and Bash completion. APT installs Git and
CA certificates; CMake is optional. Download the files through the
[release page](https://github.com/AI4SciComp/asc-devtools/releases/tag/v1.0.0)
if `curl` is unavailable.

For installation without sudo, use the binary archive. Go developers can use:

```bash
go install github.com/AI4SciComp/asc-devtools/cmd/asc@v1.0.0
```

See [installation](docs/installation.md) for binary archives, PATH setup,
source builds, upgrades, removal, and switching from an existing installation.

## Five-minute start

```bash
asc doctor
asc workspace
asc repo list
asc repo clone asc-cpp --protocol ssh
asc repo status
asc repo sync --dry-run
asc repo sync
asc repo save --dry-run
asc repo save
asc configure asc-cpp --preset dev
asc build asc-cpp --preset dev
asc test asc-cpp --preset dev
```

`~/AI4SciComp` is the default umbrella. Its children are independent Git
repositories:

```text
~/AI4SciComp/
├── .github/
├── asc-devtools/
├── asc-cmake/
├── asc-cpp/
├── asc-py/
├── asc-os/
├── asc-xde/
├── asc-kinetic/
└── asc-lean/
```

The umbrella itself is not initialized as a Git repository by `asc`.
Repository discovery remains dynamic; this diagram is not an allow-list.

## Configuration

The default file is `~/.config/asc/config.json`:

```json
{
  "organization": "AI4SciComp",
  "workspace": "~/AI4SciComp",
  "repositoryPrefix": "asc-",
  "includeDotGitHub": true,
  "cloneProtocol": "ssh",
  "remote": "origin"
}
```

Precedence is command-line option, environment variable, JSON file, then
built-in default. Global options must precede the command:

```text
--config PATH
--organization NAME
--workspace PATH
--no-color
--help
--version
```

Environment overrides are `ASC_CONFIG`, `ASC_ORGANIZATION`, `ASC_WORKSPACE`,
`ASC_REPOSITORY_PREFIX`, `ASC_INCLUDE_DOT_GITHUB`, `ASC_CLONE_PROTOCOL`, and
`ASC_REMOTE`. Unknown JSON fields and unsafe paths or names are rejected.

## Authentication

Public repository discovery requires no token. For private repositories or
higher API limits, `asc` selects the first nonempty value in this order:

```text
ASC_GITHUB_TOKEN
GH_TOKEN
GITHUB_TOKEN
```

Set one without placing the value in shell history:

```bash
read -rsp 'GitHub token: ' ASC_GITHUB_TOKEN
printf '\n'
export ASC_GITHUB_TOKEN
```

The token is sent only as a GitHub REST authorization header and is never
persisted or printed. REST authentication is separate from Git transport:

- SSH Git remotes use your SSH key for clone, fetch, and save.
- HTTPS Git remotes use Git's credential handling for clone, fetch, and save.
- Repository discovery uses the REST token above.

## Command surface

```text
asc --help
asc --version
asc doctor [--json]
asc workspace
asc repo list [--json]
asc repo clone [REPOSITORY...] [--protocol ssh|https]
asc repo status [REPOSITORY...] [--json]
asc repo sync [REPOSITORY...] [--dry-run]
asc repo save [REPOSITORY] [--branch BRANCH] [--message TEXT] [--dry-run] [--yes] [--force]
asc configure REPOSITORY --preset PRESET
asc build REPOSITORY --preset PRESET
asc test REPOSITORY --preset PRESET
asc completion bash
```

See the [command reference](docs/commands.md) and the comprehensive
[user guide](docs/user-guide.md).

## Safety

- External programs receive argument slices; no shell evaluates user input.
- Repository targets must be direct workspace children.
- Repository symlinks and path traversal are refused.
- Clone never overwrites an existing destination.
- Local status uses Git porcelain v2 and does not access the network.
- Sync skips dirty, detached, no-upstream, wrong-remote, and divergent
  repositories.
- Dry-run sync performs neither fetch nor merge.
- Real sync performs only `git fetch` and `git merge --ff-only`.
- Save operates on one managed repository, shows a reviewable plan, fetches
  before committing, and refuses remote-ahead or diverged history by default.
- `repo save --branch BRANCH` pushes the current local `HEAD` to that branch on
  the configured remote without switching the worktree.
- `repo save --force` is the only history-overwrite path and must be explicitly
  requested; it force-pushes the local branch over its selected remote branch.
- Outside `repo save`, `asc` never resets, cleans, stashes, checks out, rebases,
  commits, pushes, deletes branches, resolves conflicts, or edits global
  configuration.
- Help, version, workspace, completion, and local status avoid unnecessary
  network access.
- JSON is deterministic, color-free, and written only to stdout.

## Development

```bash
make check
# Linux distribution checks (requires dpkg-deb and fakeroot):
make package-test
```

See [contributing](CONTRIBUTING.md) for development requirements and
[installation](docs/installation.md#build-and-publish-release-artifacts)
for release packaging.

Licensed under Apache-2.0. See [LICENSE](LICENSE).
