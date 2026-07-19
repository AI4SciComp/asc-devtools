# Changelog

All notable changes are documented here following Keep a Changelog and Semantic
Versioning.

## [Unreleased]

### Changed

- Matched the Go command, JSON configuration, direct REST, Git safety, required
  preset, diagnostics, completion, and exit-code contracts.
- Published this implementation independently on `shell`.
- Updated repository examples to `asc-xde` and `asc-lab` without making API
  discovery static.

### Added

- `/usr/local` hash-manifest installation and safe uninstall support.
- Strict Bash JSON parsing without jq or another runtime dependency.
- Grouped CMake workflow/preset commands with compatible top-level aliases.
- Local-only asc-cmake vendor status, deterministic plan, and guarded apply with
  strict SHA-256 manifests, source validation, staging, and rollback.
- Top-level `asc update` with a read-only check mode, release checksum
  verification, safe archive extraction, managed-install protection, and a
  deterministic Shell release-packaging helper.

## [0.1.0] - 2026-07-19

### Added

- Bash `asc` CLI with modular strict-mode runtime libraries.
- Configuration, diagnostics, cloning, status, safe sync, and CMake workflows.
- Offline Bash tests, completion, documentation, and Ubuntu CI.

[Unreleased]: https://github.com/AI4SciComp/asc-devtools/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/AI4SciComp/asc-devtools/releases/tag/v0.1.0
