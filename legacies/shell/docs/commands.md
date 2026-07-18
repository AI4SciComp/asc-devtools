# Command reference

Global options precede the command. Success returns `0`, an operational failure
returns `1`, and invalid commands/options return `2`. CMake/CTest commands
propagate the underlying tool status.

## Global options

```text
--config PATH
--organization ORGANIZATION
--workspace PATH
--repository-prefix PREFIX
--clone-protocol ssh|https
--remote NAME
--include-dot-github | --no-include-dot-github
```

## `asc doctor`

Reports Bash, resolved configuration, workspace writability, required and
optional tools, GitHub CLI API identity, organization repository visibility,
GitHub CLI protocol, and Git SSH authentication. API token authentication and SSH
key authentication are separate checks. Failed required checks return `1`.

## `asc workspace`

Prints the validated absolute workspace path. The workspace need not exist.

## `asc repo list`

Runs `gh repo list ORGANIZATION --limit 1000 --json name,isArchived --jq ...`.
Output is the deterministic list of active repositories beginning with the
configured prefix, plus `.github` when enabled. Archived repositories are
excluded.

## `asc repo clone [REPOSITORY ...]`

Discovers the organization before cloning. Explicit names must be discovered
managed repositories. Without names, all discovered repositories are processed.
The workspace is created when needed.

SSH uses `git@github.com:ORGANIZATION/REPOSITORY.git`; HTTPS uses
`https://github.com/ORGANIZATION/REPOSITORY.git`. Existing Git worktrees are
skipped. Any other existing destination is failed and untouched. Processing
continues, a cloned/skipped/failed summary is printed, and failures return `1`.

## `asc repo status [REPOSITORY ...]`

Uses `git status --porcelain=v2 --branch` to report branch or detached HEAD,
clean/dirty state, upstream, ahead/behind counts, and changed records. Without
names it inspects managed direct children. Broken targets do not prevent later
inspection. A summary is printed and any failure returns `1`.

## `asc repo sync [REPOSITORY ...]`

For each selected worktree, checks cleanliness, the configured remote, current
branch, and an upstream on that remote. It then runs:

```text
git -C REPOSITORY fetch -- REMOTE
git -C REPOSITORY merge --ff-only UPSTREAM
```

Updated, unchanged, skipped, and failed counts are printed. Dirty or detached
repositories are skipped as unsafe. Missing configuration, fetch failures, and
divergence fail. Any skip or failure returns `1`.

## CMake commands

```text
asc configure REPOSITORY [--preset PRESET]
asc build REPOSITORY [--preset PRESET]
asc test REPOSITORY [--preset PRESET]
```

`PRESET` defaults to `dev`. The repository must contain `CMakePresets.json`.
`cmake --list-presets=TYPE` validates direct or included presets before output is
streamed from the repository root:

```text
cmake --preset PRESET
cmake --build --preset PRESET --parallel
ctest --preset PRESET --output-on-failure
```

## `asc completion bash`

Prints the installed Bash completion definition. It completes commands, options,
local managed repository names, and common presets without network calls.

