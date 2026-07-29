# Architecture

`asc` is one `CGO_ENABLED=0` Go binary built with the standard library only.
Git is its sole universal external executable; CMake and CTest are required only
by their wrappers.

## Boundaries

- `cmd/asc` owns signal cancellation, linker-provided version values, and the
  only call to `os.Exit`.
- `internal/app` parses the fixed command tree, loads configuration, coordinates
  services, writes requested data to stdout, writes diagnostics to stderr, and
  defines exit codes.
- `internal/config` loads strict JSON and applies
  CLI → environment → file → default precedence. It expands home paths, cleans
  and validates workspace paths, validates names, and selects an in-memory API
  token.
- `internal/process` is the only `os/exec` boundary. Commands are executable
  names plus argument slices. It supports context cancellation, captured or
  streamed output, missing-executable errors, external exit codes, and
  display-only quoting.
- `internal/github` calls the GitHub REST API with required headers, optional
  bearer authentication, a 15-second HTTP client timeout, caller cancellation,
  response-size limits, bounded pagination, stable filtering, and actionable
  status errors.
- `internal/workspace` proves direct-child containment, rejects repository
  symlinks, and discovers only immediate managed Git worktrees.
- `internal/git` plans and applies safe clone, parses porcelain-v2 status,
  performs download-only synchronization through fetch plus `merge --ff-only`,
  and implements the reviewed commit-and-push save workflow.
- `internal/cmake` validates a direct-child repository and explicit preset, asks
  CMake/CTest to list the relevant preset class, and streams the requested tool
  from the repository root.
- `internal/doctor` returns structured read-only checks for configuration,
  workspace, tools, REST access/authentication, SSH, and `PATH`.
- `internal/completion` embeds the static Bash definition printed by the CLI.

Domain packages do not import CLI presentation.

## Execution flow

1. `main` creates a signal-aware context and passes arguments to `app.Run`.
2. The app parses global flags before the command.
3. Help and version return before configuration loading or network work.
4. Other commands load a fully resolved, validated configuration.
5. The app selects a domain service and renders its deterministic result.
6. Errors return an exit code; only `main` terminates the process.

`workspace` and `completion` require no external command. `repo status` uses
only local Git. `repo list` and clone discovery use REST. Doctor intentionally
uses REST and a bounded SSH probe. Real sync and real save use Git network
transport.

## Configuration and identity

Configuration is data, not executable input. `encoding/json` rejects unknown
fields and trailing values. Missing configuration is normal. A leading `~` or
`~/` is expanded with `os.UserHomeDir`; other tilde forms are rejected.

Repository names must be safe GitHub path segments and satisfy the configured
prefix policy, except for optional `.github`. Organization, repository, remote,
path, environment, API response, and Git output values are all treated as
untrusted.

The API token remains in memory, is sent only in an authorization header, and
is omitted from JSON and errors. Git transport credentials remain Git's
responsibility.

## Path trust model

The workspace is a cleaned absolute path and may not be a filesystem root.
`filepath.Rel` proves that every repository destination is exactly one direct
child. `os.Lstat` rejects a destination or discovered repository symlink.

Clone creates a missing workspace and destination only through `git clone`.
An existing target must be a nonsymlink Git worktree whose configured remote
matches the API repository. Other data is never overwritten or deleted.
Before execution, clone URLs must exactly match the expected GitHub SSH or
HTTPS URL for the configured organization and repository, preventing an
untrusted response from selecting another transport or local source.

## Git mutation model

Status uses:

```text
git -C PATH status --porcelain=v2 --branch
```

Sync first validates cleanliness, the configured remote, attached branch, and
an upstream on that remote. Dry-run stops after validation and only renders:

```text
git -C PATH fetch -- REMOTE
git -C PATH merge --ff-only UPSTREAM
```

Real sync executes those exact operations. A dirty, detached, no-upstream,
wrong-remote, or non-fast-forward repository is left for manual resolution.

Save resolves one managed direct-child repository, using the current worktree
when no name is supplied. It validates the attached branch, configured-remote
upstream, and conflict-free porcelain snapshot. After plan review, it verifies
that snapshot again, fetches, compares local and upstream histories, stages all
changes, commits when needed, and pushes the explicit tracked branch ref.
Remote-ahead and diverged histories are refused by default.

`--force` is carried in the immutable save plan and adds `--force` to the push.
It deliberately permits the local branch to replace remote-ahead or diverged
history. The flag does not bypass worktree containment, attached-branch,
upstream, conflict, fetch, staging, or commit validation.

There is no code path for reset, clean, stash, checkout, rebase, branch
deletion, conflict resolution, release publication, or pull request creation.
Commit, push, and force behavior is isolated to the explicit `repo save`
workflow.

## REST safety

REST discovery:

- uses `net/http` and `context.Context`;
- sends GitHub media type, API version, and user-agent headers;
- sends `Authorization` only for a nonempty token;
- caps successful bodies at 4 MiB and error reads at 8 KiB;
- caps pagination at 1,000 pages;
- filters archived and unsafe repository data;
- returns stable name-sorted results;
- redacts response bodies and tokens from errors.

The HTTP client and base URL are injectable for deterministic `httptest`
coverage.

## CMake boundary

Scientific repositories own their CMake policy and presets. `asc` passes an
explicit preset to CMake/CTest and does not parse preset files, guess defaults,
vendor modules, sync Git, or alter build configuration.

Configure requires `CMakeLists.txt` plus a preset file. Build and test require a
preset file. Preset membership is delegated to:

```text
cmake --list-presets
cmake --list-presets=build
ctest --list-presets
```

The selected process streams output and an external exit status from 1 through
125 is preserved.

## Test strategy

Tests use only the Go standard library:

- table-driven configuration and validation tests;
- `httptest.Server` for REST headers, pagination, authentication, errors, size
  limits, timeout, and cancellation;
- fake runners for exact process argument and CLI-output contracts;
- temporary local and bare Git repositories for fast-forward and divergence;
- `t.TempDir` for workspace and installer isolation.

Tests do not depend on live GitHub, user credentials, SSH keys, the user's home,
global Git configuration, locale, or mutable sibling checkouts.

## Distribution

The runtime is the Go binary. The Bash files are narrow distribution boundaries:
the embedded completion definition and hash-guarded install/uninstall lifecycle.
They do not implement application business logic.

The module language floor is Go 1.25. CI tests the supported Go 1.25 and 1.26
lines and verifies that `go mod tidy` creates no `require` directive or
`go.sum`.
