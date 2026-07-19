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

Checks configuration, workspace, Git, GitHub REST reachability, API token
availability, SSH, CMake, CTest, optional `gh`, and whether `asc` is on PATH.
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
the command asks for confirmation unless `--yes` is supplied. The install prefix
is inferred from `PREFIX/bin/asc`, or may be supplied explicitly as an absolute
non-root path.

The Go implementation downloads
`asc-devtools-go-OS-ARCH.tar.gz` and `SHA256SUMS`, requires an exact SHA-256
match, rejects absolute paths, traversal, links, devices, and unexpected archive
roots, then invokes the bundled installer. The existing installation manifest
must be present and the installer independently refuses modified managed files.
The command does not invoke `sudo`; run the command with suitable permissions.
A missing latest release or required asset is an operational error.

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

Sync is download-only; it never uploads local commits or files.
Validates a Git worktree, clean status including untracked files, configured
remote, attached branch, and upstream on that remote. Dry-run prints safely
quoted planned commands but does not fetch or merge. Real execution uses:

```text
git -C PATH fetch -- REMOTE
git -C PATH merge --ff-only UPSTREAM
```

Outcomes are `planned`, `updated`, `unchanged`, `skipped`, or `failed`. Skips and
failures return `1` after all independent repositories are processed.

## `asc repo save REPOSITORY --message TEXT [--dry-run] [--yes]`

Save is the explicit upload workflow inspired by the `git-save` Make target. It
operates on exactly one managed direct-child repository and requires a one-line
commit message. `--dry-run` prints the exact fetch/add/commit/push plan without
network or filesystem mutation. Without `--yes`, the real command displays that
plan and asks for confirmation.

Before staging anything, save rechecks the reviewed status, fetches the configured
remote, and compares `HEAD` with the tracked upstream. It refuses detached HEAD,
missing or wrong-remote upstreams, unresolved conflicts, remote-ahead state, and
divergence. When safe, it executes:

```text
git -C PATH add --all --
git -C PATH commit -m MESSAGE       # only when staged changes exist
git -C PATH push -- REMOTE HEAD:refs/heads/UPSTREAM_BRANCH
```

A clean branch with existing local commits is pushed without an empty commit. If
nothing needs committing or pushing, the outcome is `unchanged`. If push fails,
the new local commit is retained and reported. Git credentials remain Git's
responsibility; asc neither reads nor stores them.

## CMake commands

```text
asc cmake configure REPOSITORY --preset PRESET
asc cmake build REPOSITORY --preset PRESET [--target TARGET]...
asc cmake test REPOSITORY --preset PRESET [--label LABEL] [--output-on-failure]
asc cmake workflow REPOSITORY \
  --configure-preset PRESET --build-preset PRESET --test-preset PRESET
asc cmake presets REPOSITORY [--json]
```

The original `asc configure`, `asc build`, and `asc test` forms remain aliases
with their v0.1 arguments. A preset is always explicit; asc never guesses or
interprets preset inheritance. The repository must be a managed direct-child Git
worktree and have `CMakePresets.json` or local `CMakeUserPresets.json`.
Configure and workflow also require `CMakeLists.txt`.

Preset membership is delegated to `cmake --list-presets`,
`cmake --list-presets=build`, and `ctest --list-presets`. Execution uses exact
argument arrays from the repository root:

```text
cmake --preset PRESET
cmake --build --preset PRESET --target TARGET...
ctest --preset PRESET --output-on-failure -L LABEL
```

Workflow prints each stage and safely rendered command to stderr, stops on the
first failure, and propagates the external exit status. It never performs Git
sync or vendoring. Preset JSON has this stable shape:

```json
{"repository":"asc-cpp","configure":["dev"],"build":["dev"],"test":["dev"]}
```

## Local asc-cmake vendoring

```text
asc cmake vendor status REPOSITORY [--json]
asc cmake vendor plan REPOSITORY [--source PATH] [--ref REF] [--json]
asc cmake vendor apply REPOSITORY [--source PATH] [--ref REF] [--yes]
```

The default source is `<workspace>/asc-cmake`; `--source` deliberately permits
another local checkout. There is no network download. The source must be a Git
worktree with a matching origin when one is configured. `--ref` must resolve to
the checked-out commit and never causes checkout. Apply refuses a dirty source.

The default target is `<consumer>/cmake/asc`. A strict `distribution.json` may
list exact relative files; otherwise the allowlist is `LICENSE` and nonsymlink
`modules/**/*.cmake`. The deterministic schema-v1 manifest records
`AI4SciComp/asc-cmake`, version, commit, sorted file paths, and SHA-256 hashes. A
timestamp is omitted. Status values are `not-vendored`, `current`,
`source-newer`, `locally-modified`, `manifest-invalid`, and
`source-unavailable`. Extra unmanaged files are reported without deletion.

Plan is read-only and emits sorted `add`, `replace`, `preserve`, and `remove`
actions. It refuses locally modified managed files. Apply recomputes the exact
plan, prompts unless `--yes` is supplied, stages replacements, writes the
manifest last, preserves unmanaged files, and rolls back its own replacements
on failure. It never runs Git add or commit. Review the result with:

```bash
git diff -- cmake/asc
```

Configuration can relocate the managed directory and default source name:

```json
{"cmake":{"vendorDirectory":"cmake/asc","sourceRepository":"asc-cmake"}}
```

Each consumer continues to own CMake policy and `CMakePresets.json`. Vendoring
supports offline reproducible builds; configure never changes vendored files.

## `asc completion bash`

Prints static Bash completion for commands, flags, common presets, and local
repository names. It calls only local `asc workspace`; ordinary completion never
uses the network.
