# Command reference

All expected errors are written to standard error without a traceback. Exit `0`
means the requested operation succeeded; argument parsing exits `2`; an
interrupted operation exits `130`. Other command-specific failures exit `1`.

Global options must precede the command:

```text
--config PATH
--organization ORGANIZATION
--workspace WORKSPACE
--repository-prefix PREFIX
--clone-protocol {ssh,https}
--include-dot-github | --no-include-dot-github
```

## `asc --version`

Print the installed version and exit.

## `asc doctor`

Print resolved configuration and check Python, Git, GitHub CLI, CMake, CTest,
workspace writability, GitHub API authentication, GitHub CLI protocol,
organization visibility, and SSH authentication when SSH cloning is configured.
Each diagnostic is marked `OK` or `FAIL`. Any failed required check returns `1`.

The API and SSH results are intentionally distinct: `gh` API operations use its
stored token, while Git over SSH uses an SSH key.

## `asc workspace`

Print the expanded, absolute workspace path. The path need not exist.

## `asc repo list [--json]`

Query `gh repo list ORGANIZATION --limit 1000 --json ...`. Include non-archived
repositories beginning with the configured prefix and optionally `.github`, in
case-insensitive alphabetical order. The default is a table; `--json` emits an
array with `name`, `url`, and `description` fields.

## `asc repo clone [REPOSITORY ...]`

Discover organization repositories first. With names, validate and clone those
repositories; without names, process every discovered repository. SSH clones use
`git@github.com:ORGANIZATION/REPOSITORY.git`; HTTPS clones use the GitHub HTTPS
URL. Destinations are direct workspace children.

An existing Git checkout is reported as skipped. An existing non-Git path is
reported as failed and left untouched. Processing continues after a clone
failure, and any failure returns `1`.

## `asc repo status [REPOSITORY ...]`

Inspect selected repositories with porcelain-v2 Git status. Output includes the
branch or detached-HEAD state, clean/dirty state, changed paths, upstream, and
ahead/behind counts. Without names, inspect all managed direct-child Git
repositories. Failures are accumulated while other repositories are still
reported; any failure returns `1`.

## `asc repo sync [REPOSITORY ...]`

For each selected repository, refuse a dirty tree, resolve its upstream, record
HEAD, run `git fetch`, then run `git merge --ff-only UPSTREAM`. A repository is
reported as updated, unchanged, skipped, or failed. Processing continues and a
final count is printed. Skipped or failed repositories return `1` so automation
cannot mistake an incomplete sync for success.

Sync does not merge divergent work, rebase, stash, reset, check out another
branch, or discard files. A branch without an upstream fails with Git's error.

## CMake commands

```text
asc configure REPOSITORY [--preset PRESET]
asc build REPOSITORY [--preset PRESET]
asc test REPOSITORY [--preset PRESET]
```

`PRESET` defaults to `dev`. The repository must be a managed local direct child,
and the corresponding configure, build, or test preset must exist in its
`CMakePresets.json`. Output streams directly to the terminal. Commands execute
from the repository root as:

```text
cmake --preset PRESET
cmake --build --preset PRESET --parallel
ctest --preset PRESET --output-on-failure
```

Missing repositories, files, presets, executables, and underlying command
failures return `1`.

