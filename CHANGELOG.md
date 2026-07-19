# Changelog

This project follows Keep a Changelog and Semantic Versioning.

## [Unreleased]

### Added

- Hash-verified uninstall support for the executable and Bash completion.

### Changed

- The installer now defaults to the conventional `/usr/local` system prefix and
  supports `DESTDIR` staging.

## [0.1.0] - 2026-07-19

### Added

- Dependency-free Go `asc` binary with JSON configuration and direct GitHub REST
  discovery.
- Safe clone, porcelain status, dry-run and fast-forward sync, explicit CMake
  preset wrappers, structured doctor checks, and Bash completion.
- Standard-library tests, static builds, and Go 1.25/1.26 CI.

### Changed

- Published the Go implementation independently from the equivalent Python and
  Bash variants.

[Unreleased]: https://github.com/AI4SciComp/asc-devtools/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/AI4SciComp/asc-devtools/releases/tag/v0.1.0
