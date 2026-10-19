# Changelog

This project follows Keep a Changelog and Semantic Versioning.

## [1.0.0] - 2026-10-19

### Added

- Linux amd64/arm64 binary archives, Debian packages and SHA-256 checksums,
  with tag-triggered GitHub Releases and isolated package lifecycle tests.
- Installation instructions for release packages and remote `go install`.
- Dependency-free Go `asc` binary with strict JSON configuration and direct
  GitHub REST discovery.
- Safe repository clone, porcelain-v2 status, dry-run and fast-forward-only
  sync, explicit CMake/CTest preset wrappers, structured doctor checks, Bash
  completion, and hash-guarded installation.
- Reviewed `repo save` workflow for the current or explicitly selected
  repository, with guarded commit/push behavior and an explicit `--force`
  remote-history override.
- Explicit `repo save --branch BRANCH` destination selection on the configured
  remote, including new remote branches and local branches without an upstream.
- Standard-library tests, static builds, and Go 1.25/1.26 CI.

### Security

- Preserved strict JSON, bounded REST access, token redaction, direct-child path
  containment, clone non-overwrite rules, and fast-forward-only sync.
- Isolated commit, push, and history replacement to the reviewed `repo save`
  workflow; history replacement requires an explicit `--force`.
- Confirmed that the CLI has no reset, clean, stash, checkout, rebase,
  branch-deletion, release-publication, or pull-request behavior.
