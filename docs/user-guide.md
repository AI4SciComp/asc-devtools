# asc-devtools user guide

This guide describes the complete v0.1 `asc` command surface as implemented.
Run `asc --help` for the concise built-in reference.

## What asc-devtools is

`asc-devtools` is a dependency-free Go command-line tool for routine work
across repositories owned by `AI4SciComp`. It:

- discovers organization repositories through the GitHub REST API;
- clones repositories into a predictable local umbrella;
- reports local Git branch and worktree state;
- downloads fast-forward updates into clean worktrees;
- reviews, commits, and pushes one repository through an explicit save workflow;
- invokes CMake and CTest presets owned by each repository;
- reports environment and authentication readiness;
- emits stable JSON for discovery, status, and diagnostics.

It is not an agent runner, workflow engine, package manager, CMake policy
repository, scientific solver, release publisher, or general Git frontend.
Commit and push behavior is isolated to `repo save`; `asc` does not reset,
clean, stash, check out, rebase, delete branches, open pull requests, or publish
releases.

## Workspace model

The built-in workspace is `~/AI4SciComp`. It is an umbrella directory, not a
Git repository. Each child remains an independent checkout:

```text
~/AI4SciComp/
├── workspace/       # optional coordination data, not managed by asc v0.1
├── asc-devtools/
├── asc-cmake/
├── asc-cpp/
├── asc-xde/
├── asc-kinetic/
├── asc-lean/
└── asc-lab/
```

Repository discovery is dynamic. The names above describe the current
organization layout; `repo list` uses the API rather than a hard-coded list.
The retired provisional names `asc-pde` and `asc-platform` are not part of the
current layout.

Repository operations are limited to direct children such as
`~/AI4SciComp/asc-cpp`. `asc` does not recursively scan nested directories and
does not follow a repository destination symlink.

## Requirements

Runtime:

- Linux or another Go-supported platform;
- Git for `repo clone`, `repo status`, `repo sync`, and `repo save`;
- network access for REST discovery, clone, real sync, and real save;
- CMake for `configure` and `build`;
- CTest for `test`;
- SSH only when using SSH clone transport or the doctor's SSH probe.

Build and installer:

- supported Go 1.25 or 1.26 when building from source;
- Bash and `sha256sum` for the supplied install/uninstall scripts.

The `asc` binary contains no third-party Go module. Runtime users do not need
Python, Node.js, Ruby, `jq`, a package manager, or GitHub CLI.

## Installation on Windows 11 with WSL2 Ubuntu

In an elevated PowerShell prompt:

```powershell
wsl --install -d Ubuntu
wsl --update
wsl --set-default-version 2
wsl --list --verbose
```

Restart Windows if requested. In Ubuntu:

```bash
sudo apt update
sudo apt install -y build-essential ca-certificates git cmake
```

Store the workspace in the WSL filesystem:

```bash
mkdir -p "${HOME}/AI4SciComp"
cd "${HOME}/AI4SciComp"
git clone git@github.com:AI4SciComp/asc-devtools.git
cd asc-devtools
```

Build and install:

```bash
CGO_ENABLED=0 go build -buildvcs=false -trimpath \
  -ldflags "-s -w -X main.version=0.1.0" \
  -o ./asc ./cmd/asc
sudo ./scripts/install.sh --binary ./asc
asc --version
```

The default installation is:

```text
/usr/local/bin/asc
/usr/local/share/bash-completion/completions/asc
/usr/local/share/asc-devtools/install-manifest
```

For an unprivileged installation:

```bash
./scripts/install.sh --binary ./asc --prefix "${HOME}/.local"
```

Ensure the binary directory is on `PATH`:

```bash
export PATH="${HOME}/.local/bin:${PATH}"
```

Add that exact line to `~/.bashrc` if it should apply to future Bash sessions.
The installer does not edit shell startup files.

## Bash completion

A managed install places completion below the selected prefix. To enable it only
for the current shell:

```bash
source <(asc completion bash)
```

Completion suggests canonical commands, `ssh`/`https`, common preset names, and
managed direct-child worktrees. Repository completion calls only local
`asc workspace`; it does not use the REST API.

## Five-minute quick start

Check the environment and resolved location:

```bash
asc doctor
asc workspace
```

List repositories visible through the API:

```bash
asc repo list
```

Clone one repository with the configured SSH default:

```bash
asc repo clone asc-cpp
```

Inspect every managed local worktree:

```bash
asc repo status
```

Review a download-only synchronization:

```bash
asc repo sync --dry-run
```

Apply only clean fast-forwards:

```bash
asc repo sync
```

From within a managed repository, review and save local work:

```bash
asc repo save --dry-run
asc repo save
```

For a repository with a `dev` preset:

```bash
asc configure asc-cpp --preset dev
asc build asc-cpp --preset dev
asc test asc-cpp --preset dev
```

## Configuration

### File and defaults

The default configuration path is:

```text
~/.config/asc/config.json
```

A complete file is:

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

All fields are optional. Unknown fields, trailing JSON values, malformed JSON,
and files larger than 1 MiB are rejected.

| Field | Type | Built-in default | Meaning |
| --- | --- | --- | --- |
| `organization` | string | `AI4SciComp` | GitHub organization used by REST discovery |
| `workspace` | string | `~/AI4SciComp` | Local umbrella, resolved to a cleaned absolute path |
| `repositoryPrefix` | string | `asc-` | Managed repository-name prefix |
| `includeDotGitHub` | boolean | `true` | Include `.github` in discovery and local operations |
| `cloneProtocol` | string | `ssh` | Default `ssh` or `https` Git transport |
| `remote` | string | `origin` | Git remote used for matching and sync |

`~` and `~/...` expand through the current user's home directory.
`~anotheruser`, a filesystem root, invalid GitHub names, option-like names, and
unsafe repository/path values are rejected.

### Precedence

Values resolve in this order, highest first:

1. global command-line option;
2. environment variable;
3. JSON configuration file;
4. built-in default.

| Purpose | Global option | Environment |
| --- | --- | --- |
| Config file | `--config PATH` | `ASC_CONFIG` |
| Organization | `--organization NAME` | `ASC_ORGANIZATION` |
| Workspace | `--workspace PATH` | `ASC_WORKSPACE` |
| Repository prefix | — | `ASC_REPOSITORY_PREFIX` |
| Include `.github` | — | `ASC_INCLUDE_DOT_GITHUB` (`true` or `false`) |
| Clone transport | clone command flag | `ASC_CLONE_PROTOCOL` |
| Git remote | — | `ASC_REMOTE` |
| Color disabled | `--no-color` | `NO_COLOR` |

Global options must precede the command:

```bash
asc --workspace "${HOME}/AI4SciComp" repo status
asc --config /tmp/asc-test-config.json repo list --json
```

Command-specific options may follow their command:

```bash
asc repo clone asc-cpp --protocol https
asc repo status asc-cpp --json
```

## Tokens and transport

### REST authentication

Public discovery works anonymously. Private repositories and higher rate limits
need a token. The first nonempty variable wins:

```text
ASC_GITHUB_TOKEN
GH_TOKEN
GITHUB_TOKEN
```

Enter a token without putting its value in shell history:

```bash
read -rsp 'GitHub token: ' ASC_GITHUB_TOKEN
printf '\n'
export ASC_GITHUB_TOKEN
```

Use a token with only the access needed to read the target organization
repositories. Do not put a real token in `config.json`, documentation, command
arguments, or committed shell files. `asc` keeps the value in memory, sends it
only in the `Authorization` header, and omits it from output and errors.

### SSH versus HTTPS

REST authentication and Git transport solve different problems:

- REST token: lets `repo list` and `repo clone` discover private metadata.
- SSH: the API-provided `git@github.com:...` URL and an SSH key authenticate
  `git clone`, `fetch`, and `push`.
- HTTPS: the API-provided `https://github.com/...` URL and Git's credential
  helper authenticate Git transport.

Choosing `--protocol https` does not make the REST token a Git credential.
Choosing SSH does not make an SSH key a REST API token.

## Commands

### Help and version

```bash
asc --help
asc --version
asc repo clone --help
asc repo save --help
```

Help and version use no network. Version output is `asc dev` for an unversioned
development build or includes linker-provided metadata for a release build.

### Doctor

```bash
asc doctor
asc doctor --json
```

Doctor is read-only. It checks configuration/workspace, Git, optional
CMake/CTest, REST authentication/API access, SSH, and `PATH`. Warnings such as a
missing optional CMake installation do not fail the command. Configuration,
required Git, or API failures return `1`.

Doctor does contact GitHub and performs a bounded SSH probe. Use local commands
such as `asc workspace` or `asc repo status` when an offline-only check is
required.

### Workspace

```bash
asc workspace
```

Prints one cleaned absolute path and does not require it to exist.

### Repository list

```bash
asc repo list
asc repo list --json
```

Results come from the REST API, not the local filesystem. Archived and
nonmatching repositories are filtered; names are sorted.

### Repository clone

```bash
asc repo clone asc-cmake asc-cpp
asc repo clone asc-cpp --protocol https
asc repo clone
```

No names means every eligible API repository. Clone creates the workspace when
missing. It never clones over a file, symlink, unrelated directory, wrong-remote
worktree, or existing data.

### Repository status

```bash
asc repo status
asc repo status asc-cpp asc-xde
asc repo status asc-cpp --json
```

No names means every managed direct-child Git worktree. Status reports branch,
detached state, upstream, ahead/behind, cleanliness, and change count. It is
local and nonrecursive.

### Repository sync

Always review first:

```bash
asc repo sync --dry-run
asc repo sync asc-cpp --dry-run
```

Then apply:

```bash
asc repo sync
asc repo sync asc-cpp asc-xde
```

Sync skips dirty worktrees, detached HEAD, missing/wrong upstreams, and
fast-forward refusals. It does not stash or resolve the situation. Handle those
cases with Git after reviewing the repository.

### Repository save

From anywhere inside a managed direct-child repository:

```bash
asc repo save --dry-run
asc repo save
```

You may instead select one repository explicitly and supply a commit message:

```bash
asc repo save asc-cpp --message "Describe the change" --dry-run
asc repo save asc-cpp --message "Describe the change" --yes
```

Save stages all changes, commits them when needed, and pushes existing plus new
local commits to the current branch's configured-remote upstream. Without
`--message`, the commit subject is `Updated at YYYY-MM-DD HH:MM:SS`. Without
`--yes`, a real save prints the plan and prompts before doing network or
filesystem mutations.

To select the destination branch on the configured remote explicitly:

```bash
asc repo save --branch feature/selected --dry-run
asc repo save --branch feature/selected --yes
```

This pushes the current local `HEAD` to `feature/selected`; it does not switch
the worktree. The option can create a new remote branch and can be used when
the attached local branch has no upstream. Without `--branch`, save retains the
existing behavior of targeting the configured-remote upstream.

After confirmation, save rechecks the reviewed status, fetches, and refuses
remote-ahead or diverged history before it stages or commits. To deliberately
replace that remote history with the local branch:

```bash
asc repo save --force
asc repo save --branch main --force
```

The force flag changes the final operation to `git push --force`; it is
destructive and can discard remote commits. It does not bypass containment,
branch-name, conflict, fetch, stage, or commit checks.

### Configure, build, and test

```bash
asc configure asc-cpp --preset dev
asc build asc-cpp --preset dev
asc test asc-cpp --preset dev
```

The preset is required. `asc` confirms it using the tool's preset-list command
and streams the selected tool from the repository root. Each repository owns
its `CMakePresets.json` or `CMakeUserPresets.json`; `asc` does not generate one.

### Completion

```bash
asc completion bash
source <(asc completion bash)
```

Only Bash completion is supported in v0.1.

## Normal daily workflow

1. At the start of a setup or after toolchain changes:

   ```bash
   asc doctor
   ```

2. Discover and clone missing repositories:

   ```bash
   asc repo list
   asc repo clone asc-cpp asc-xde
   ```

3. Review local state:

   ```bash
   asc repo status
   ```

4. Preview downloads:

   ```bash
   asc repo sync --dry-run
   ```

5. Apply safe updates:

   ```bash
   asc repo sync
   ```

6. From within a repository, review and upload completed work:

   ```bash
   asc repo save --dry-run
   asc repo save
   ```

7. Configure, build, and test a repository:

   ```bash
   asc configure asc-cpp --preset dev
   asc build asc-cpp --preset dev
   asc test asc-cpp --preset dev
   ```

Use normal reviewed Git and GitHub workflows for branch management and pull
requests.

## JSON output and scripting

JSON output is UTF-8, indented, deterministically ordered, and never colored.
Requested JSON goes to stdout; diagnostics go to stderr.

### Repository list schema

```json
[
  {
    "name": "asc-cpp",
    "archived": false,
    "fork": false,
    "clone_url": "https://github.com/AI4SciComp/asc-cpp.git",
    "ssh_url": "git@github.com:AI4SciComp/asc-cpp.git",
    "default_branch": "main",
    "private": true
  }
]
```

### Repository status schema

```json
[
  {
    "name": "asc-cpp",
    "branch": "main",
    "detached": false,
    "upstream": "origin/main",
    "ahead": 0,
    "behind": 0,
    "clean": true,
    "changes": 0,
    "error": "optional"
  }
]
```

### Doctor schema

```json
[
  {
    "name": "workspace",
    "status": "pass",
    "detail": "/home/user/AI4SciComp exists",
    "remedy": "optional"
  }
]
```

Examples with `jq` when it is installed:

```bash
asc repo list --json | jq -r '.[].name'
asc repo status --json | jq -e 'all(.[]; .clean and .error == null)'
asc doctor --json | jq -r '.[] | select(.status != "pass")'
```

Preserve exit status in scripts:

```bash
if ! status_json=$(asc repo status --json); then
  printf '%s\n' 'one or more repositories could not be inspected' >&2
  exit 1
fi
printf '%s\n' "${status_json}"
```

## Exit behavior

| Code | Meaning |
| ---: | --- |
| `0` | Requested operation completed |
| `1` | Operational error, refused save, or partial repository failure |
| `2` | Invalid invocation |

CMake/CTest exit codes 1–125 propagate through their wrappers. Multi-repository
commands continue after independent failures and return `1` after printing all
outcomes.

## Safety guarantees

- JSON configuration rejects unknown fields.
- Repository names are safe single path segments and must match policy.
- Workspace and destination paths are cleaned and checked for direct-child
  containment.
- Existing clone destinations are never replaced or merged.
- Repository symlinks are not followed.
- Git status uses porcelain intended for programs.
- Dry-run sync does not fetch or merge.
- Real sync is limited to fetch plus `merge --ff-only`.
- Save rechecks its reviewed snapshot and fetches before staging or committing.
- Ordinary save refuses remote-ahead and diverged histories.
- Force save is the only remote-history overwrite path and requires an explicit
  `--force`.
- Detached and conflicted worktrees are not saved. No-upstream or wrong-remote
  worktrees require an explicit configured-remote destination via `--branch`.
- External commands use `os/exec` argument slices, never `sh -c`.
- REST requests use cancellation, a 15-second client timeout, response-size
  limits, pagination bounds, and required GitHub headers.
- Tokens are neither logged nor persisted.
- stdout and stderr have separate contracts.
- No telemetry is collected.

Intentionally unsupported operations include reset, clean, stash, checkout,
rebase, branch deletion, release publication, pull-request creation, and
conflict resolution. Commit, push, and force operations exist only within the
explicit `repo save` workflow.

## Troubleshooting

### REST authentication or authorization

Symptoms include `401`, `403`, or private repositories missing from `repo list`.

```bash
asc doctor
```

Set a readable token with `ASC_GITHUB_TOKEN`. A `401` means authentication
failed. A non-rate-limit `403` usually means organization/repository permission
is missing. A `404` can mean the organization is hidden from the token.

### API rate limit

An unauthenticated request has a lower limit. When `asc` reports rate limiting,
wait for reset or set `ASC_GITHUB_TOKEN`. The token itself is never printed.

### SSH clone failure

```bash
ssh -T git@github.com
asc doctor
```

Confirm an SSH public key is associated with the intended GitHub account. Or
choose HTTPS:

```bash
asc repo clone asc-cpp --protocol https
```

### HTTPS clone failure

Configure a Git credential helper or other approved Git credential mechanism.
The REST environment token is not automatically passed to `git clone`.

### Missing tools

`required executable not found: git` is fatal for repository operations.
Install CMake for configure/build and CTest for tests. Doctor treats CMake and
CTest as optional because other commands do not need them.

### Dirty repository

Sync reports `working tree is dirty` and skips the repository. Review:

```bash
git -C "${HOME}/AI4SciComp/asc-cpp" status
```

Commit, discard, or otherwise reconcile changes manually, or use `repo save` if
the intended action is to commit every listed change and push it.

### Detached HEAD or no upstream

Sync skips ambiguous state. Inspect:

```bash
git -C "${HOME}/AI4SciComp/asc-cpp" status --short --branch
git -C "${HOME}/AI4SciComp/asc-cpp" branch -vv
```

Choose a branch/upstream manually using normal Git workflows. For save only,
an attached local branch can instead select its configured-remote destination
explicitly with `asc repo save --branch BRANCH`; sync still requires an
upstream.

### Diverged repository

`merge --ff-only` refuses divergence. Review local and upstream history, then
select merge, rebase, or another resolution outside `asc`.

### Missing preset

List repository-owned presets:

```bash
(cd "${HOME}/AI4SciComp/asc-cpp" && cmake --list-presets)
(cd "${HOME}/AI4SciComp/asc-cpp" && cmake --list-presets=build)
(cd "${HOME}/AI4SciComp/asc-cpp" && ctest --list-presets)
```

Run `asc` with an exact listed preset. A local `CMakeUserPresets.json` is also
accepted.

### `asc` not on PATH

```bash
command -v asc
printf '%s\n' "${PATH}"
```

For a user-local install:

```bash
export PATH="${HOME}/.local/bin:${PATH}"
```

### WSL2 path and permissions

Prefer `~/AI4SciComp` inside WSL rather than `/mnt/c`. Check the active distro
and filesystem:

```bash
uname -a
pwd
df -T .
```

Run Linux `asc` from the WSL shell. Windows and WSL have separate path and
credential contexts.

## Upgrade and uninstall

Upgrade by obtaining/building a replacement and rerunning the installer with the
same prefix:

```bash
sudo ./scripts/install.sh --binary ./asc
```

The installer checks the existing manifest and hashes before replacement.
There is no v0.1 self-update command.

Uninstall the default managed files:

```bash
sudo ./scripts/uninstall.sh
```

Or a user-local install:

```bash
./scripts/uninstall.sh --prefix "${HOME}/.local"
```

The uninstaller targets only:

```text
PREFIX/bin/asc
PREFIX/share/bash-completion/completions/asc
PREFIX/share/asc-devtools/install-manifest
```

It verifies hashes and never recursively deletes `PREFIX`.

## Contributor architecture

`cmd/asc/main.go` owns signal handling, linker version fields, and the only
`os.Exit`. `internal/app` parses the explicit CLI and separates output streams.
Domain packages are:

- `config`: strict JSON, precedence, token selection, and validation;
- `github`: bounded REST discovery and filtering;
- `process`: the only external-command boundary;
- `workspace`: direct-child containment and local discovery;
- `git`: clone, porcelain-v2 status, and fast-forward-only sync;
- `cmake`: configure/build/test preset wrappers;
- `doctor`: structured read-only diagnostics;
- `completion`: embedded Bash completion.

Tests use the standard `testing` package, `httptest.Server`, fake runners,
`t.TempDir`, and temporary local/bare Git repositories. They do not need live
GitHub, user tokens, SSH keys, global Git identity, or the user's home.

Run the release-validation set:

```bash
gofmt -w ./cmd ./internal
go mod tidy
test ! -e go.sum
! grep -q '^require' go.mod
go vet ./...
go test ./...
go test -race ./...
CGO_ENABLED=0 go build -trimpath -o /tmp/asc ./cmd/asc
/tmp/asc --help
/tmp/asc --version
scripts/test_install.sh /tmp/asc
git diff --check
```

CI repeats formatting, dependency, vet, test, race, static-build, smoke, and
installer checks on supported Go 1.25 and 1.26 lines.
