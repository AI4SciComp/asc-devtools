# Architecture

- `bin/asc` resolves its installation root, loads a fixed module set, parses the
  canonical command surface, and owns stable JSON and exit-code behavior.
- `config.sh` and `json.sh` strictly parse the flat JSON schema and resolve CLI,
  environment, file, and default precedence without `eval` or executable config.
- `github.sh` owns bounded direct REST requests through curl, pagination,
  response parsing, filtering, and stable API records.
- `repository.sh` enforces direct-child containment, remote identity,
  porcelain-v2 inspection, and fast-forward-only synchronization.
- `cmake.sh` validates an explicit repository-owned preset and invokes exact
  CMake or CTest argument arrays.
- `doctor.sh` returns independent pass, warning, or failure checks.
- `scripts/install.sh` and `scripts/uninstall.sh` manage an exact file set using
  a prefix-bound SHA-256 manifest.

The runtime targets Bash 4.4+ and has no Python, jq, or third-party library
dependency. External commands never receive evaluated strings. Tests replace
network and process boundaries and use temporary Git repositories.
