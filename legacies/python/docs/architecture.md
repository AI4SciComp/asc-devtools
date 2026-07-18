# Architecture

## Package boundaries

- `config.py` loads TOML, applies precedence, expands paths, and returns an
  immutable validated `Config`.
- `process.py` is the only runtime boundary to `subprocess`. `CommandRunner`
  supports captured or terminal-streamed output and raises `ProcessError` with
  sanitized command context.
- `github.py` invokes GitHub CLI, parses JSON, filters archived and unmanaged
  repositories, and returns immutable `RemoteRepository` values.
- `repositories.py` validates names, discovers direct children, parses Git
  status, clones without replacement, and performs conservative sync.
- `commands/doctor.py` reports independent environment and credential checks.
- `commands/cmake.py` validates repository-owned preset declarations and streams
  thin CMake/CTest wrappers.
- `cli.py` owns `argparse` routing, user-facing formatting, partial-failure
  aggregation, and exit semantics.

Runtime code uses only the Python standard library. Dependency injection of
`CommandRunner` keeps command behavior testable without network or real workspace
access.

## Safety model

The trust boundary begins with configuration and repository arguments.
Configuration values are type-checked and names are restricted to GitHub-safe
components. A repository argument cannot be absolute, contain a separator, use
traversal, or fall outside the configured prefix policy. Local discovery examines
only direct workspace children, so nested dependencies and build checkouts are
out of scope.

External processes always receive sequences and `shell=True` is never used.
Error rendering conceals common token/password argument forms and URL userinfo;
the runner never prints a complete environment.

Mutation is deliberately narrow. Clone creates only missing destinations. Sync
requires a clean worktree and permits only the current branch's fast-forward to
its configured upstream. There is no reset, cleanup, automatic stash, rebase,
checkout, commit, push, or deletion path in the package.

## Failure handling

Configuration and selection errors fail before mutation. Multi-repository
operations capture per-repository failures, continue where safe, print every
outcome, and return nonzero for incomplete work. Build/test output is streamed,
while inspection output is captured for structured parsing. Expected exceptions
derive from `AscError` and become concise CLI messages.

## Testing

Configuration, runner, GitHub parsing, clone/sync orchestration, CMake routing,
and CLI presentation use isolated unit tests. Temporary real Git repositories
exercise discovery and porcelain status, including dirty and detached states.
Tests neither access GitHub nor alter global Git configuration.

