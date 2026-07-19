# Architecture

- `config.py` strictly loads the flat JSON schema and resolves canonical
  precedence.
- `process.py` is the sole subprocess boundary and always uses argument arrays.
- `github.py` owns bounded direct REST requests, pagination, filtering, and the
  optional `gh auth token` fallback.
- `repositories.py` enforces direct-child containment, remote identity,
  porcelain-v2 inspection, download-only fast-forward synchronization, and a
  reviewed single-repository commit/push save transaction.
- `commands/cmake.py` plans grouped configure/build/test/workflow commands,
  delegates preset interpretation, and streams exact argument arrays.
- `vendor.py` owns strict manifests, source validation, SHA-256 state, read-only
  plans, staged apply, and rollback independently of CLI presentation.
- `commands/doctor.py` returns stable pass/warning/failure checks.
- `selfupdate.py` reads bounded release metadata and assets, compares versions,
  verifies SHA-256, extracts only a strict archive root, and delegates replacement
  to the managed installer.
- `cli.py` owns routing, JSON schemas, partial-failure aggregation, and exit
  codes.
- `scripts/manage_install.py` installs and removes only hash-verified files.

Runtime modules use only the Python standard library. Tests inject process and
HTTP boundaries and use temporary real Git repositories. No command evaluates
user input through a shell or performs history-rewriting Git operations.

Self-update trusts GitHub release metadata under the fixed
`AI4SciComp/asc-devtools` repository, but not archive contents. It authenticates
with the normal token when available, caps metadata/checksum/archive/extracted
sizes, requires `SHA256SUMS`, rejects link and traversal entries, and accepts only
the implementation-specific archive root. The existing installation manifest
and installer hash checks remain the final overwrite boundary.

Vendoring never downloads. It accepts a checked-out `asc-cmake` source, validates
its Git commit/origin/ref/dirtiness, and uses exact `distribution.json` files or
the conservative `LICENSE` plus `modules/**/*.cmake` fallback. The schema-v1
manifest omits timestamps, sorts slash paths, and hashes exact bytes. Extra files
are preserved. Replacement/removal requires the old managed bytes to match their
manifest, and apply writes the new manifest last with rollback of its own work.

Save snapshots porcelain-v2 status during review. Apply rejects a changed
snapshot, fetches and requires the remote not to be ahead, stages all paths,
commits only a nonempty index, and pushes the exact upstream branch. It never
pulls, rebases, forces, changes branches, or resolves conflicts; a failed push
leaves the local commit intact.
