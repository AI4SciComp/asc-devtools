# Go-only migration ledger

This ledger records the local Gate B consolidation performed on 2026-07-24. The
canonical source was `main` at
`ec32a6155f473abbb2f857329bc77ef8789225c2`. Before consolidation, each
implementation snapshot under `main:branches/` was byte-equivalent to its
standalone remote branch:

| Implementation | Preserved branch head | Baseline validation | Disposition |
| --- | --- | --- | --- |
| Go | `038deb3308d18a3cbbe4ca9dc94819f18f60721a` | format, tidy/no dependencies, vet, unit tests, race tests, static build/smoke, shell syntax, installer lifecycle | promoted to the canonical root and extended |
| Python | `a849e65d5789718ed844652afa5d17e2f96fb688` | Python 3.12.4, 37 unit tests, compileall, mypy on 14 files, 2 installer tests; Ruff unavailable | implementation snapshot removed; behavior retained in Go |
| Bash | `4a66cec0791fb9172f7e5363cfeb4f2a3c0f5182` | Bash syntax and all 7 suites/23 checks; ShellCheck and shfmt unavailable | implementation snapshot removed; behavior retained in Go |

All refs were captured before the migration in:

```text
workspace/memory/provenance/branch-bundles/asc-devtools-all-refs-20260724.bundle
```

The bundle was verified with `git bundle verify`; its SHA-256 is
`10aa81f1d2f0530f878c21fab764b14a714d58b0ff52d5a1bac33a933ad1fe68`.
The historical checkout and all local/remote branches were retained. No branch,
tag, or remote ref was deleted.

## Behavior and regression-test disposition

| User-visible behavior | Python tests | Bash tests | Go disposition |
| --- | --- | --- | --- |
| help, version, exit codes, completion | `test_cli.py` | `test_cli.sh` | `internal/app`, `internal/completion`, and installer smoke tests |
| strict configuration and precedence | `test_config.py` | `test_config.sh` | `internal/config` tests |
| GitHub REST discovery/authentication | `test_github.py` | `test_github.sh` | `internal/github` and app HTTP tests |
| safe clone/status | `test_repositories.py` | `test_repository.sh` | `internal/git` and workspace tests |
| download-only fast-forward sync | `test_repositories.py` | `test_repository.sh` | `internal/git` tests |
| reviewed repository save | Python CLI/repository tests | CLI/repository tests | `internal/git` and app tests |
| CMake commands and workflow | `test_cmake.py` | `test_cmake.sh` | `internal/cmake` tests |
| local asc-cmake vendoring | `test_vendor.py` | `test_vendor.sh` | `internal/cmakevendor` tests |
| doctor diagnostics | `test_doctor.py` | CLI tests | `internal/doctor` and app tests |
| verified update/install/uninstall | `test_selfupdate.py`, `test_install.py` | `test_update.sh` | `internal/selfupdate` tests and `scripts/test_install.sh` |
| process execution safety | `test_process.py` | helper/CLI tests | `internal/process` tests |

The removed language-specific tests were not copied mechanically because their
runtime and packaging assumptions no longer exist. The Go regression suites
exercise the retained contracts with temporary workspaces, fake process
runners, local HTTP servers, and temporary Git repositories.

## New v0.1 foundation behavior

The migration adds narrowly scoped, standard-library implementations for:

- `asc workspace init [--dry-run]`;
- `asc workspace validate [--json]`;
- `asc agent init NAME [--dry-run]`;
- `asc workflow validate [NAME...] [--json]`.

These commands create or validate local definitions only. They do not execute
agents or workflows, access GitHub, upload data, or delete data. Workflow
manifests use the checked-in `schemas/workflow-v1.schema.json`; semantic
cross-field checks are enforced by the Go validator.

## Intentionally retained shell

`scripts/install.sh`, `scripts/uninstall.sh`, `scripts/test_install.sh`,
`scripts/package_update.sh`, and the generated Bash completion definition remain
because packaging and completion shell are explicitly permitted. Business logic
and the runtime CLI remain Go-only.
