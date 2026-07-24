# Changelog

This project follows Keep a Changelog and Semantic Versioning.

## [Unreleased]

### Fixed

- Reused an existing authenticated GitHub CLI session when token environment
  variables are unset, restoring private repository listing and clone
  discovery without requiring duplicate credential configuration.

### Added

- A repository Makefile with development, installation, conservative Git,
  GitHub CLI, and `asc` workflow shortcuts.
- Dependency-free Go `asc` binary with strict JSON configuration and direct
  GitHub REST discovery.
- Safe repository clone, porcelain-v2 status, dry-run and fast-forward-only
  sync, explicit CMake/CTest preset wrappers, structured doctor checks, Bash
  completion, and hash-guarded installation.
- Standard-library tests, static builds, and Go 1.25/1.26 CI.

### Changed

- Promoted the dependency-free Go implementation to the canonical repository
  root.
- Consolidated the public CLI on the documented v0.1 command surface.
- Changed the built-in workspace from `~/projects/AI4SciComp` to
  `~/AI4SciComp`.
- Replaced legacy Python and Bash implementation snapshots with a verified
  migration ledger and preserved history.
- Added a comprehensive implementation-derived user guide.

### Security

- Preserved strict JSON, bounded REST access, token redaction, direct-child path
  containment, clone non-overwrite rules, and fast-forward-only sync.
- Confirmed that the CLI has no commit, push, history-rewrite, branch-deletion,
  release-publication, or pull-request behavior.
