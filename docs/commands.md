# Command reference

Global options must precede the command:

```text
--config PATH
--organization NAME
--workspace PATH
--no-color
-h, --help
--version
```

`--config`, `--organization`, and `--workspace` override environment and file
configuration. `--no-color` and `NO_COLOR` are accepted for compatibility; the
current output is always free of ANSI color.

Requested output goes to stdout. Progress, usage, and diagnostics go to stderr.

## Exit codes

| Code | Meaning |
| ---: | --- |
| `0` | Success |
| `1` | Operational error or partial multi-repository failure |
| `2` | Invalid command, option, or argument |

For `configure`, `build`, and `test`, an external CMake/CTest exit code from 1
through 125 is propagated.

## `asc --help`

Prints top-level help to stdout without loading GitHub data or running an
external program.

## `asc --version`

Prints `asc VERSION` and any linker-provided commit/build date. It performs no
network operation.

## `asc doctor [--json]`

Checks:

- resolved configuration and workspace usability;
- required Git availability;
- optional CMake and CTest availability;
- REST token availability;
- GitHub organization API access;
- SSH executable and bounded GitHub authentication probe;
- whether `asc` is on `PATH`.

Statuses are `pass`, `warning`, and `failure`. A warning does not change the
exit code; any failure returns `1`.

JSON schema:

```json
[
  {
    "name": "git",
    "status": "pass",
    "detail": "git version 2.34.1",
    "remedy": "optional text"
  }
]
```

## `asc workspace`

Prints only the cleaned absolute workspace path. The path need not exist. The
command does not access the network or run Git.

## `asc repo list [--json]`

Pages through:

```text
GET /orgs/ORGANIZATION/repos?type=all&per_page=100&page=N
```

It returns sorted, non-archived repositories matching `repositoryPrefix`, plus
`.github` when `includeDotGitHub` is true.

Plain output columns are repository, visibility, and default branch. JSON is an
array in repository-name order:

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

## `asc repo clone [REPOSITORY...] [--protocol ssh|https]`

Discovers repositories through the REST API, then invokes:

```text
git clone -- API_URL ABSOLUTE_DESTINATION
```

Without names, it processes all eligible API repositories. Explicit names must
be returned by the API. `--protocol` overrides the configured default.

An existing destination is accepted only when it is a nonsymlink Git worktree
whose configured remote matches the API repository. Other existing paths are
reported as failures and left untouched. Processing continues after independent
failures.

Outcomes are `cloned`, `already-present`, and `failed`. Any failure returns `1`.

## `asc repo status [REPOSITORY...] [--json]`

With names, inspects those direct workspace children. Without names, discovers
all managed nonsymlink Git worktrees immediately below the workspace. It runs:

```text
git -C PATH status --porcelain=v2 --branch
```

No network operation occurs. JSON is sorted by requested/discovered order:

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
    "error": "optional error"
  }
]
```

Requested repositories retain the requested order; automatic discovery is
name-sorted. An inspection error is reported in the item and makes the command
return `1`.

## `asc repo sync [REPOSITORY...] [--dry-run]`

Sync is download-only. For each target, it verifies:

1. the direct child is a Git worktree rather than a symlink;
2. tracked and untracked state is clean;
3. the configured remote exists;
4. HEAD is attached to a branch;
5. the branch has an upstream on the configured remote.

Dry-run prints the safely quoted plan and executes neither fetch nor merge:

```text
git -C PATH fetch -- REMOTE
git -C PATH merge --ff-only UPSTREAM
```

Real sync runs those exact operations. It never uploads local commits. A merge
that cannot fast-forward is refused without changing history.

Outcomes are `planned`, `updated`, `unchanged`, `skipped`, and `failed`.
`skipped` or `failed` makes the overall command return `1` after all independent
repositories are processed.

## `asc repo save [REPOSITORY] [--message TEXT] [--dry-run] [--yes] [--force]`

Save stages all changes, creates a commit when needed, and pushes the current
branch to its configured upstream. When `REPOSITORY` is omitted, the Git
worktree containing the current directory must be a managed direct child of the
configured workspace. An explicit repository name selects that managed
worktree instead.

The default commit message is `Updated at YYYY-MM-DD HH:MM:SS` in local time.
`--message` accepts a nonempty, single-line message of at most 500 characters.
The branch must be attached and track a branch on the configured remote;
unresolved conflicts are refused.

Save first creates a local-only plan containing the applicable commands:

```text
git -C PATH fetch -- REMOTE
git -C PATH add --all --
git -C PATH commit -m MESSAGE
git -C PATH push -- REMOTE HEAD:refs/heads/BRANCH
```

`--dry-run` prints the plan without fetching, staging, committing, or pushing.
A real save prints the plan to stderr and asks for confirmation unless `--yes`
is present. After confirmation it rechecks the working-tree snapshot, fetches,
and refuses remote-ahead or diverged history before staging local changes.

`--force` explicitly selects the destructive override path. It changes the
final command to:

```text
git -C PATH push --force -- REMOTE HEAD:refs/heads/BRANCH
```

With this option, remote-ahead and diverged history do not block the save; the
local branch replaces the tracked remote branch. Detached branches, missing or
wrong-remote upstreams, conflicts, fetch failures, and commit failures remain
blocked. Outcomes are `saved`, `unchanged`, `skipped`, and `failed`.

## `asc configure REPOSITORY --preset PRESET`

Validates a direct-child Git worktree, a preset file, and `CMakeLists.txt`.
It confirms the configure preset with `cmake --list-presets`, then runs:

```text
cmake --preset PRESET
```

The process working directory is the repository root.

## `asc build REPOSITORY --preset PRESET`

Confirms the build preset with `cmake --list-presets=build`, then runs:

```text
cmake --build --preset PRESET
```

## `asc test REPOSITORY --preset PRESET`

Confirms the test preset with `ctest --list-presets`, then runs:

```text
ctest --preset PRESET
```

`asc` does not guess a preset, interpret inheritance, sync repositories, or
change build policy.

## `asc completion bash`

Prints the embedded Bash completion definition. Completion suggests canonical
commands, common presets, and managed direct-child repository names. It calls
only local `asc workspace` while completing repositories and never discovers
them through the network.

Load it for the current shell:

```bash
source <(asc completion bash)
```
