# Architecture

The active implementation targets Bash 4.4+ on Ubuntu/WSL2 and is not POSIX
`sh`. `bin/asc` resolves its real location, sources a fixed library set, parses
global options, loads configuration, and dispatches commands.

## Modules

- `common.sh` contains diagnostics, executable checks, name validation, and Git
  worktree recognition.
- `config.sh` implements trusted shell configuration, precedence, defaults, and
  path/value validation.
- `process.sh` provides captured-command support without string evaluation.
- `github.sh` obtains tab-separated name/archive records from `gh --jq` and
  filters them deterministically.
- `repository.sh` owns direct-child discovery, cloning, porcelain status, and
  conservative synchronization.
- `doctor.sh` reports independent API-token and SSH-key boundaries.
- `cmake.sh` validates and invokes repository-owned presets.

Modules do not source one another and are loaded in dependency order by the fixed
front end. External commands are Bash arrays or directly quoted argument lists;
there is no `eval`, generated command string, or untrusted source path.

## Configuration boundary

The selected configuration file is deliberately executable Bash. Before sourcing
it, `config.sh` records relevant exported variables and restores them afterward,
then applies explicit CLI overrides. This supplies CLI, environment, file, and
default precedence without an ad-hoc parser. The file is trusted user code, not a
data format suitable for untrusted input.

Workspace paths must be absolute (or begin with `~/`), dedicated rather than `/`
or `$HOME`, and free of traversal components. Repository names cannot contain
separators and must satisfy the configured managed-name policy.

## Mutation boundary

Clone only creates a missing direct child. Sync only mutates a clean current
branch through a fetch of the configured remote and `merge --ff-only` of its
matching upstream. There are no commands for cleanup, history rewriting, branch
switching, commits, pushes, or deletion.

The installer uses a managed-file marker and preflights every target before
copying. The uninstaller lists every owned file explicitly and refuses unmarked
files. It never recursively deletes a prefix.

## Tests

`tests/test_helper.sh` supplies assertions, command capture, exact temporary
cleanup, Git helpers, and executable mocks. Test suites run offline with isolated
`HOME`, `PATH`, workspaces, and configuration. Real temporary repositories cover
worktree files, status, fast-forward updates, dirty refusal, and divergence.

The former Python implementation is an inert historical snapshot under
`legacies/python` and is outside the active runtime and CI paths.

