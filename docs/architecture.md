# Architecture

`asc` is one `CGO_ENABLED=0` Go binary built entirely from the standard library.
`cmd/asc` owns signals, linker-provided version values, and the sole `os.Exit`.
`internal/app` parses explicit commands, coordinates services, separates output
streams, and defines exit codes.

## Package boundaries

- `config` loads strict JSON, applies CLI/environment/file/default precedence,
  expands home paths, validates workspace/name safety, and selects API tokens.
- `process` is the only `os/exec` boundary. It captures or streams output,
  propagates cancellation, distinguishes missing executables, and wraps exit
  status.
- `github` calls the REST API with bounded timeouts and response sizes,
  pagination, required headers, optional bearer authentication, and actionable
  status errors.
- `workspace` constructs and verifies direct children, excludes symlinks, and
  discovers only immediate managed Git worktrees.
- `git` plans clone operations from API URLs, verifies existing remotes, parses
  porcelain-v2 status, and implements dry-run-aware fast-forward sync.
- `cmake` validates one local repository and explicit repository-owned preset,
  then streams CMake or CTest.
- `doctor` returns structured, read-only checks for configuration, tools, REST
  access/authentication, SSH, workspace, optional `gh`, and PATH.
- `completion` embeds the static Bash definition printed by the application.

Domain packages do not import CLI presentation. Interfaces are defined only at
the process and doctor consumers where substitution is needed by tests.

## Trust boundaries

Configuration, environment, API responses, paths, Git output, names, and process
errors are untrusted. JSON rejects unknown fields. A repository name must be a
single safe path segment and satisfy the managed prefix policy. `filepath.Rel`
must prove every destination is exactly one direct child. `os.Lstat` prevents a
repository symlink from crossing the workspace boundary.

Tokens remain in memory, are sent only as an authorization header, and are
excluded from errors and JSON. API error reads are capped at 8 KiB; successful
responses are capped at 4 MiB. The HTTP client has a 15-second timeout and all
operations accept `context.Context` cancellation.

External commands are executable names plus argument slices. Human dry-run
descriptions are quoted only for display and are never parsed back into commands.

## Mutation model

Clone creates a missing direct child only. An existing destination must be a
non-symlink Git worktree whose configured remote matches the API repository;
otherwise it is retained and failed.

Sync first validates cleanliness, remote, branch, and matching upstream. Dry-run
stops after these checks. A real sync fetches the configured remote and attempts
only `merge --ff-only`. Independent repository failures are accumulated and
reported deterministically.

The historical Python and Bash implementations under `legacies` are inert and
excluded from the active build and CI package paths.
