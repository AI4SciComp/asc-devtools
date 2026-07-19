# Changelog

This project follows Keep a Changelog and Semantic Versioning.

## [Unreleased]

### Added

- Hash-verified uninstall support for the executable and Bash completion.
- Grouped `asc cmake` configure, build, test, workflow, and preset commands while
  preserving the legacy top-level aliases.
- Local-only `asc-cmake` vendor status, deterministic plan, and guarded apply
  with strict manifests, SHA-256 verification, source validation, and rollback.
- Top-level `asc update` with a read-only check mode, release checksum
  verification, safe archive extraction, managed-install protection, and a
  deterministic Go release-packaging helper.
- Guarded `asc repo save` for reviewed add/commit/push workflows, with dry-run,
  confirmation, remote-ahead/divergence refusal, and preserved local commits on
  push failure; sync output now states that it is download-only.

### Changed

- `asc repo save` now defaults to `Updated at YYYY-MM-DD HH:MM:SS` when
  `--message` is omitted.
- Removed the GitHub CLI token fallback; API authentication now uses only the
  documented environment variables.
- The installer now defaults to the conventional `/usr/local` system prefix and
  supports `DESTDIR` staging.
- Source installation now discovers Go in conventional system locations when
  `sudo` restricts `PATH`, with `--go PATH` available for other toolchains.
- Repository examples now use `asc-xde` and `asc-lab`; discovery remains dynamic.

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
