# Architecture

- `bin/asc` resolves its installation root, loads a fixed module set, parses the
  canonical command surface, and owns stable JSON and exit-code behavior.
- `config.sh` and `json.sh` strictly parse the JSON schema and resolve CLI,
  environment, file, and default precedence without `eval` or executable config.
- `github.sh` owns bounded direct REST requests through curl, pagination,
  response parsing, filtering, and stable API records.
- `repository.sh` enforces direct-child containment, remote identity,
  porcelain-v2 inspection, and fast-forward-only synchronization.
- `cmake.sh` plans grouped configure/build/test/workflow commands, delegates
  preset interpretation, and invokes exact arrays.
- `vendor.sh` owns strict manifest/distribution parsing, source validation,
  SHA-256 status, read-only plans, staged apply, and rollback.
- `update.sh` reads bounded release metadata and assets, compares versions,
  verifies SHA-256, validates a strict archive root and entry types, and delegates
  replacement to the managed installer.
- `doctor.sh` returns independent pass, warning, or failure checks.
- `scripts/install.sh` and `scripts/uninstall.sh` manage an exact file set using
  a prefix-bound SHA-256 manifest.

The runtime targets Bash 4.4+ and has no Python, jq, or third-party library
dependency. External commands never receive evaluated strings. Tests replace
network and process boundaries and use temporary Git repositories.

Self-update trusts GitHub release metadata under the fixed
`AI4SciComp/asc-devtools` repository, but not archive contents. It authenticates
with the normal token when available, caps metadata/checksum/archive/extracted
sizes, requires `SHA256SUMS`, rejects links and traversal, and accepts only the
implementation-specific archive root. The existing installation manifest and
installer hash checks remain the final overwrite boundary.

Vendoring never downloads. It validates the local asc-cmake Git
commit/origin/ref/dirtiness, reads exact `distribution.json` files or the
conservative `LICENSE` plus `modules/**/*.cmake` fallback, and emits a sorted,
timestamp-free schema-v1 manifest. Extra files are preserved. Replacement or
removal requires old managed bytes to match their hash. Apply writes the manifest
last and rolls back changes from the current operation on failure.
