# Command reference

Global options must precede `COMMAND`:

```text
--config PATH
--organization NAME
--workspace PATH
--no-color
--help
--version
```

Output requested by a command is written to stdout. Diagnostics and usage errors
use stderr. The current plain output contains no ANSI color; `NO_COLOR` and
`--no-color` are accepted for stable compatibility.

Exit codes:

```text
0  success
1  operational or partial multi-repository failure
2  invalid command, option, or argument
```

CMake/CTest failures propagate their external exit code when it is between 1 and
125.

## `asc doctor [--json]`

Checks configuration, workspace, Bash, Git, curl, GitHub REST reachability, API
token availability, SSH, CMake, CTest, optional `gh`, and whether `asc` is on PATH.
Checks are `pass`, `warning`, or `failure`. Any failure returns `1`.

JSON schema:

```json
[{"name":"git","status":"pass","detail":"git version ...","remedy":"optional text"}]
```

## `asc workspace`

Prints only the cleaned absolute workspace. It performs no network operation and
does not require the workspace to exist.

## `asc repo list [--json]`

Pages through `GET /orgs/ORGANIZATION/repos?type=all&per_page=100&page=N`, then
sorts and returns non-archived prefix matches plus optional `.github`.

JSON is an array with stable API-derived fields:

```json
[{"name":"asc-cpp","archived":false,"fork":false,"clone_url":"https://...","ssh_url":"git@...","default_branch":"main","private":false}]
```

## `asc repo clone [REPOSITORY...] [--protocol ssh|https]`

Discovers through the REST API. Explicit names must be returned by the
organization. SSH uses the API `ssh_url`; HTTPS uses `clone_url`. Git executes:

```text
git clone -- URL ABSOLUTE_DESTINATION
```

An expected existing worktree is `already-present`. A symlink, nonrepository, or
wrong remote is failed and untouched. Processing continues and a deterministic
summary is printed. Any failure returns `1`.

## `asc repo status [REPOSITORY...] [--json]`

Inspects named repositories or all managed direct children with
`git status --porcelain=v2 --branch`. Plain output includes repository, branch,
state, upstream, and ahead/behind. JSON schema:

```json
[{"name":"asc-cpp","branch":"main","detached":false,"upstream":"origin/main","ahead":0,"behind":0,"clean":true,"changes":0,"error":"optional"}]
```

Failures do not stop later repositories and make the command return `1`.

## `asc repo sync [REPOSITORY...] [--dry-run]`

Validates a Git worktree, clean status including untracked files, configured
remote, attached branch, and upstream on that remote. Dry-run prints safely
quoted planned commands but does not fetch or merge. Real execution uses:

```text
git -C PATH fetch -- REMOTE
git -C PATH merge --ff-only UPSTREAM
```

Outcomes are `planned`, `updated`, `unchanged`, `skipped`, or `failed`. Skips and
failures return `1` after all independent repositories are processed.

## CMake commands

```text
asc configure REPOSITORY --preset PRESET
asc build REPOSITORY --preset PRESET
asc test REPOSITORY --preset PRESET
```

A preset is intentionally required; asc never guesses. The repository must have
`CMakePresets.json` or `CMakeUserPresets.json`. `cmake --list-presets=TYPE`
validates direct and included presets, then output streams from the repository
root:

```text
cmake --preset PRESET
cmake --build --preset PRESET
ctest --preset PRESET
```

## `asc completion bash`

Prints static Bash completion for commands, flags, common presets, and local
repository names. It calls only local `asc workspace`; ordinary completion never
uses the network.
