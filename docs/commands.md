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

## `asc update [--check] [--yes] [--prefix PATH]`

Queries `GET /repos/AI4SciComp/asc-devtools/releases/latest` and compares its
numeric release tag with the running version. `--check` reports only; otherwise
the command asks for confirmation unless `--yes` is supplied. The prefix
normally comes from the resolved `PREFIX/bin/asc` location; it may instead be
supplied as an absolute non-root path.

The Shell implementation downloads `asc-devtools-shell.tar.gz` and
`SHA256SUMS`, requires an exact SHA-256 match, rejects absolute paths, traversal,
links, devices, control-character names, and unexpected archive roots, then
invokes the bundled installer. The existing installation manifest must be
present and the installer independently refuses modified managed files. The
command does not invoke `sudo`; run it with suitable permissions. A missing
latest release or required asset is an operational error.

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
asc cmake configure REPOSITORY --preset PRESET
asc cmake build REPOSITORY --preset PRESET [--target TARGET]...
asc cmake test REPOSITORY --preset PRESET [--label LABEL] [--output-on-failure]
asc cmake workflow REPOSITORY \
  --configure-preset PRESET --build-preset PRESET --test-preset PRESET
asc cmake presets REPOSITORY [--json]
```

Legacy top-level configure/build/test remain compatible. Presets are explicit,
and asc delegates inheritance and conditions to `cmake --list-presets`,
`cmake --list-presets=build`, and `ctest --list-presets`. Configure/workflow
require `CMakeLists.txt`. Operations run from a validated direct-child Git
worktree with exact Bash arrays. Workflow prints each safely rendered array to
stderr, stops on first failure, and never syncs or vendors implicitly.

## Local asc-cmake vendoring

```text
asc cmake vendor status REPOSITORY [--json]
asc cmake vendor plan REPOSITORY [--source PATH] [--ref REF] [--json]
asc cmake vendor apply REPOSITORY [--source PATH] [--ref REF] [--yes]
```

Vendoring copies only from `<workspace>/asc-cmake` or an explicit local source.
It validates the Git worktree, optional origin, commit/ref, dirtiness,
containment, and regular nonsymlink files without fetch or checkout. Apply
refuses dirty sources. Version precedence is `VERSION`, CMake project version,
then an exact Git tag.

The default target is `cmake/asc`. Strict `distribution.json` enumerates exact
files; fallback allows `LICENSE` and `modules/**/*.cmake` only.
`ASC_CMAKE_MANIFEST.json` is deterministic schema-v1 JSON with source, version,
commit, sorted paths, and SHA-256 hashes; it has no timestamp.

Status values are `not-vendored`, `current`, `source-newer`,
`locally-modified`, `manifest-invalid`, and `source-unavailable`. Extra files are
reported and preserved. Plan emits sorted add/replace/preserve/remove actions and
refuses changed managed bytes. Apply prompts unless `--yes`, recomputes the exact
plan, stages 0644 files, writes the manifest last, and rolls back its own work on
failure. It never invokes Git add/commit. Review with `git diff -- cmake/asc`.

Optional strict configuration is:

```json
{"cmake":{"vendorDirectory":"cmake/asc","sourceRepository":"asc-cmake"}}
```

Asc orchestrates but does not define CMake policy. Consumers own
`CMakePresets.json`; configure never modifies vendored modules.

## `asc completion bash`

Prints static Bash completion for commands, flags, common presets, and local
repository names. It calls only local `asc workspace`; ordinary completion never
uses the network.
