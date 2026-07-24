# Go-only migration ledger

This ledger records the consolidation begun on 2026-07-24 from `main` at
`ec32a6155f473abbb2f857329bc77ef8789225c2`.

Before migration, `main` contained embedded Go, Python, and Bash snapshots whose
trees matched the standalone `go`, `python`, and `shell` branch heads.

| Implementation | Legacy head | Baseline evidence | Final disposition |
| --- | --- | --- | --- |
| Go | `038deb3308d18a3cbbe4ca9dc94819f18f60721a` | format, tidy/no dependencies, vet, unit/race tests, static smoke, shell syntax, installer lifecycle | promoted to the repository root and narrowed to the canonical v0.1 interface |
| Python | `a849e65d5789718ed844652afa5d17e2f96fb688` | Python 3.12.4, 37 unit tests, compileall, mypy on 14 files, 2 installer tests; Ruff unavailable | implementation snapshot removed after behavior/test disposition |
| Bash | `4a66cec0791fb9172f7e5363cfeb4f2a3c0f5182` | Bash syntax and 7 suites/23 checks; ShellCheck and shfmt unavailable | implementation snapshot removed after behavior/test disposition |

All refs were captured before migration in:

```text
workspace/memory/provenance/branch-bundles/asc-devtools-all-refs-20260724.bundle
```

`git bundle verify` reports a complete history. Its SHA-256 is:

```text
10aa81f1d2f0530f878c21fab764b14a714d58b0ff52d5a1bac33a933ad1fe68
```

Before obsolete branches are removed, their legacy heads are also preserved by
explicit `archive/legacy-*-20260724` tags. The final bootstrap validation record
contains the resulting tag objects, deleted branch names, and remote evidence.

## Behavior and regression-test disposition

| User-visible behavior | Python/Bash evidence | Go disposition |
| --- | --- | --- |
| help, version, exit codes, completion | CLI suites | `internal/app`, embedded completion, smoke tests |
| strict configuration and precedence | configuration suites | `internal/config` table tests |
| GitHub REST discovery/authentication | GitHub suites | `internal/github` `httptest` coverage |
| safe clone and local status | repository suites | `internal/git` and workspace tests |
| download-only fast-forward sync | repository suites | fake-runner decisions and temporary bare-remote integration |
| CMake/CTest wrappers | CMake suites | exact argument and exit-propagation tests |
| doctor diagnostics | doctor/CLI suites | `internal/doctor` and application JSON tests |
| process execution safety | process/helper suites | `internal/process` cancellation, missing executable, and exit tests |
| installation lifecycle | installer suites | hash-guarded staged install/uninstall test |

Language-specific tests were not copied mechanically because their runtimes and
packaging no longer exist. The Go tests retain the canonical contracts with
temporary workspaces, local HTTP servers, fake runners, and temporary Git
repositories.

## Excluded migration-stage behavior

Earlier local migration work experimented with workspace/agent/workflow
scaffolding, grouped CMake vendoring, self-update, and repository commit/push
commands. The final product decision excludes them from v0.1. Their code,
configuration, completion, tests, and documentation were removed before
publication.

The canonical binary does not reset, clean, stash, check out, rebase, commit,
push, delete branches, force updates, publish releases, or create pull requests.

## Intentionally retained shell

`scripts/install.sh`, `scripts/uninstall.sh`, `scripts/test_install.sh`, and the
embedded Bash completion definition remain narrow distribution/test boundaries.
Application business logic and the runtime CLI are Go-only.
