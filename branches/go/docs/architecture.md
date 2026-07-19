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
  porcelain-v2 status, implements download-only fast-forward sync, and provides
  a reviewed single-repository commit/push save transaction.
- `cmake` plans grouped configure/build/test/workflow commands, delegates preset
  interpretation to CMake/CTest, and streams from the repository root.
- `cmakevendor` validates local sources, strict manifests, hashes and update
  plans independently of CLI rendering, then applies an approved plan with
  staged replacements and rollback.
- `doctor` returns structured, read-only checks for configuration, tools, REST
  access/authentication, SSH, workspace, optional `gh`, and PATH.
- `completion` embeds the static Bash definition printed by the application.
- `selfupdate` reads bounded release metadata and assets, compares versions,
  verifies SHA-256, extracts only a strict archive root, and delegates replacement
  to the managed installer.

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

Self-update trusts GitHub release metadata under the fixed
`AI4SciComp/asc-devtools` repository, but not archive contents. It authenticates
with the normal token when available, caps metadata/checksum/archive/extracted
sizes, requires `SHA256SUMS`, rejects link and traversal entries, and accepts only
the implementation-specific archive root. The existing installation manifest
and installer hash checks remain the final overwrite boundary.

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

Save separates review from mutation. Its plan snapshots porcelain-v2 status and
renders exact argument arrays. Apply rejects a changed snapshot, fetches and
requires the remote not to be ahead, then stages all paths, commits only a
nonempty index, and pushes the exact upstream branch. It never pulls, rebases,
forces, changes branches, or resolves conflicts. A failed push leaves the local
commit intact for diagnosis or retry.

Equivalent Python and Bash implementations are maintained on separate branches;
this branch's build and CI paths contain Go only.

## Vendoring trust model

Vendoring never downloads. Its source is the checked-out sibling `asc-cmake` or
an explicit local path. Git supplies commit, origin, ref, and dirty-state checks;
`VERSION`, then CMake project version, then an exact tag supplies the version.
`distribution.json` can enumerate exact files; otherwise only `LICENSE` and
regular nonsymlink `modules/**/*.cmake` files are managed.

`ASC_CMAKE_MANIFEST.json` is strict schema-v1 JSON with sorted slash-separated
paths and SHA-256 hashes. It omits a timestamp for byte-for-byte reproducibility.
Extra files are reported and preserved. A recorded file can be replaced or
removed only while its current bytes still match the old manifest. Apply stages
new bytes below the consumer, writes the manifest last, and rolls back changes
from the current operation if a replacement fails.
