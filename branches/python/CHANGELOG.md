# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- Matched the Go command, JSON configuration, direct REST, Git safety, required
  preset, diagnostics, completion, and exit-code contracts.
- Published this implementation independently on `python`.
- Updated descriptive repository topology to `asc-xde` and `asc-lab` while
  preserving dynamic API discovery.

### Added

- `/usr/local` hash-manifest installation and safe uninstall support.
- Grouped CMake workflow/preset commands and compatible top-level aliases.
- Local-only asc-cmake vendor status, deterministic plan, and guarded apply with
  strict SHA-256 manifests, source validation, staging, and rollback.
- Top-level `asc update` with a read-only check mode, release checksum
  verification, safe archive extraction, managed-install protection, and a
  deterministic Python release-packaging helper.
- Guarded `asc repo save` for reviewed add/commit/push workflows, with dry-run,
  confirmation, remote-ahead/divergence refusal, and preserved local commits on
  push failure; sync output now states that it is download-only.

## [0.1.0] - 2026-07-19

### Added

- Initial `asc` command-line interface.
- Configuration, diagnostics, GitHub discovery, cloning, status, and safe sync.
- CMake configure, build, and test preset wrappers.
- Bash completion, offline tests, documentation, and Ubuntu CI.

[Unreleased]: https://github.com/AI4SciComp/asc-devtools/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/AI4SciComp/asc-devtools/releases/tag/v0.1.0
