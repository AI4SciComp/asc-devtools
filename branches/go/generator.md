# Generator Prompt: Dependency-Free Go `AI4SciComp/asc-devtools`

## Authoritative v2 upgrade addendum

Upgrade the maintained implementation in place. The existing public commands,
JSON schemas, exit codes, configuration keys, installer lifecycle, tests, and
unrelated changes remain compatible. The requirements in this addendum override
conflicting v0.1 examples below.

The current dynamic organization topology is `asc-devtools`, `asc-cmake`,
`asc-cpp`, `asc-xde`, `asc-kinetic`, `asc-lean`, and `asc-lab`, plus optional
`.github`. Replace stale documentation examples `asc-pde` and `asc-platform`
with `asc-xde` and `asc-lab`; never replace arbitrary user data or turn dynamic
GitHub discovery into a hard-coded list.

Retain legacy top-level `configure`, `build`, and `test` commands and add the
canonical grouped surface:

```text
asc cmake configure REPOSITORY --preset PRESET
asc cmake build REPOSITORY --preset PRESET [--target TARGET...]
asc cmake test REPOSITORY --preset PRESET [--label LABEL] [--output-on-failure]
asc cmake workflow REPOSITORY --configure-preset PRESET \
  --build-preset PRESET --test-preset PRESET
asc cmake presets REPOSITORY [--json]
asc cmake vendor status REPOSITORY [--json]
asc cmake vendor plan REPOSITORY [--source PATH] [--ref REF] [--json]
asc cmake vendor apply REPOSITORY [--source PATH] [--ref REF] [--yes]
```

CMake operations run from the validated direct-child repository root and use
argument slices. Configure/workflow require `CMakeLists.txt`. Delegate preset
semantics to CMake/CTest list commands. Workflow prints each safe command to
stderr, stops at the first failed stage, propagates cancellation and exit
status, and never synchronizes Git or vendors implicitly.

Vendoring is local-only from the checked-out sibling `asc-cmake` (or explicit
source), never a download. The default destination is `cmake/asc`. Prefer a
strict `distribution.json`; otherwise allow only regular, nonsymlink
`modules/**/*.cmake` files and `LICENSE`. Reject escapes and duplicates. Record
a deterministic, strict schema-v1 `ASC_CMAKE_MANIFEST.json` containing source,
version, commit, and sorted SHA-256 file records. Version precedence is `VERSION`,
then the CMake project version, then an exact Git tag. The manifest omits a
timestamp so identical source produces identical bytes.

Vendor status reports `not-vendored`, `current`, `source-newer`,
`locally-modified`, `manifest-invalid`, or `source-unavailable`, including
unmanaged extra files. Plan is read-only and reports add/replace/preserve/remove;
removal is allowed only for an unchanged file recorded by the old manifest.
Apply requires the explicit subcommand and confirmation (`--yes` for a
noninteractive exact plan), revalidates immediately before mutation, refuses a
dirty or mismatched source and locally modified managed files, stages writes,
preserves unmanaged files, never deletes the complete `cmake` directory, and
never runs Git add/commit. Source origin/ref, containment, regular-file status,
and dirtiness must be validated.

Optional configuration keys are nested under `cmake.vendorDirectory` and
`cmake.sourceRepository`; defaults are `cmake/asc` and `asc-cmake`. Unknown JSON
fields remain errors. Doctor adds read-only CMake/CTest version capability and
local vendoring checks without turning absent optional tooling into unrelated
fatal failures.

Extend standard-library tests for exact CMake arguments, workflow stop/cancel,
preset listing, manifest encoding/hashes, strict parsing, paths with spaces,
source validation, all status classes, safe plans, guarded apply/confirmation,
and rollback. Tests use only temporary sources/repositories and never touch live
GitHub or sibling repositories. Update README, command/architecture/installation
docs, completion, configuration example, changelog, contributing checks, and CI
to match implemented behavior and the current topology.

Preserve the lifecycle contract: default installation to `/usr/local`, with
`--prefix` and `DESTDIR`; SHA-256-manifest installation/uninstallation;
`-buildvcs=false`, `-trimpath`, explicit release version; and exact-file removal
that never recursively deletes an installation prefix.

You are the principal engineer responsible for implementing the existing GitHub
repository `AI4SciComp/asc-devtools` as a small, dependable developer CLI.

Project identity:

- GitHub user: `escapetiger`
- GitHub organization: `AI4SciComp`
- Repository: `AI4SciComp/asc-devtools`
- Primary environment: Ubuntu under WSL2
- Default workspace: `~/projects/AI4SciComp`
- Executable name: `asc`

Work directly in the current checkout. Inspect it before editing, preserve all
unrelated user changes, and keep its existing license. Implement and test the
repository; do not merely return sample snippets. Do not commit, push, publish a
release, open a pull request, or modify another repository unless explicitly
asked.

## 1. Nonnegotiable implementation constraints

Implement the CLI in **Go**, using the **Go standard library only**.

Requirements:

- `go.mod` must contain no `require` directive.
- Do not use Cobra, Viper, urfave/cli, testify, go-github, TOML/YAML packages, or
  any other third-party Go module.
- Parse commands with `flag.FlagSet` or a small, explicit standard-library
  parser. Do not recreate a large framework.
- Use `encoding/json`, `net/http`, `os/exec`, `path/filepath`, `context`, and
  other standard packages where appropriate.
- Produce one self-contained `asc` binary. It must build with `CGO_ENABLED=0`.
- Use Go 1.25 as the minimum language/module version and test with supported Go
  1.25 and 1.26 toolchains. Do not use a Go 1.26-only feature.
- Runtime users must not need Go, Python, Node.js, Ruby, or a package manager.
- Git is the only universally required external executable.
- CMake and CTest are required only for their corresponding commands.
- GitHub CLI (`gh`) must not be required or invoked.
- Shell is permitted only for a small installer or generated Bash completion;
  business logic belongs in Go.

“No dependencies” means no third-party code linked into the binary. Calling Git,
CMake, or CTest is intentional process integration, not a Go library dependency.

Follow the current official Go documentation and the Google Go Style Guide:

- https://go.dev/doc/
- https://google.github.io/styleguide/go/

Favor clarity, simplicity, maintainability, and consistency. Use `gofmt`; avoid
unnecessary interfaces, abstractions, reflection, generics, and concurrency.

## 2. Objective and repository boundaries

`asc` standardizes safe local operations across AI4SciComp repositories:

```text
AI4SciComp/.github
AI4SciComp/asc-devtools
AI4SciComp/asc-cpp
AI4SciComp/asc-xde
AI4SciComp/asc-kinetic
AI4SciComp/asc-lean
AI4SciComp/asc-lab
```

This is developer infrastructure. Do not place LLM orchestration, reinforcement
learning, numerical solvers, PDE implementations, kinetic models, or Lean proofs
inside this repository.

## 3. Inspect before changing anything

Before implementation:

1. Run `pwd`, `git status --short --branch`, and list the working tree.
2. Find and follow `AGENTS.md` or repository-local instructions.
3. Read existing documentation, source, tests, workflows, license, and remotes.
4. Identify uncommitted user changes and preserve them.
5. Run existing checks to establish a baseline.
6. Record available Go, Git, CMake, and CTest versions.
7. Compare the actual repository with this specification. Adapt filenames when
   justified, while preserving the specified behavior and constraints.

Do not erase existing work just to make the tree match a proposed layout.

## 4. Version 0.1 command surface

Implement these canonical commands:

```text
asc --help
asc --version
asc doctor [--json]
asc workspace
asc repo list [--json]
asc repo clone [REPOSITORY...] [--protocol ssh|https]
asc repo status [REPOSITORY...] [--json]
asc repo sync [REPOSITORY...] [--dry-run]
asc configure REPOSITORY [--preset PRESET]
asc build REPOSITORY [--preset PRESET]
asc test REPOSITORY [--preset PRESET]
asc completion bash
```

Global options:

```text
--config PATH
--organization NAME
--workspace PATH
--no-color
--help
--version
```

The parser must accept global options only in a clearly documented position,
preferably before the subcommand. Unknown commands/options, missing arguments,
and invalid values must return concise errors plus the relevant usage text.

Do not implement commit, push, reset, stash, checkout, branch deletion, force
operations, release publishing, or pull-request creation in v0.1.

## 5. Suggested package layout

Use a cohesive structure similar to:

```text
asc-devtools/
├── cmd/asc/main.go
├── internal/
│   ├── app/app.go
│   ├── cli/cli.go
│   ├── config/config.go
│   ├── process/runner.go
│   ├── github/client.go
│   ├── git/repository.go
│   ├── workspace/workspace.go
│   ├── cmake/cmake.go
│   ├── doctor/doctor.go
│   └── output/output.go
├── completions/asc.bash
├── docs/
│   ├── architecture.md
│   ├── commands.md
│   └── installation.md
├── examples/config.json
├── .github/workflows/ci.yml
├── scripts/install.sh
├── go.mod
├── README.md
├── CHANGELOG.md
├── CONTRIBUTING.md
├── CITATION.cff
└── LICENSE
```

This is a responsibility map, not a demand for tiny packages. Avoid import
cycles. Keep `cmd/asc/main.go` thin. Packages under `internal` must not depend on
CLI presentation when they represent domain behavior.

Use this module path:

```text
module github.com/AI4SciComp/asc-devtools

go 1.25
```

After `go mod tidy`, `go.mod` must still have no `require` block and no `go.sum`
should be necessary.

## 6. Configuration model

Use JSON so parsing remains in the standard library. Default configuration path:

```text
~/.config/asc/config.json
```

Example:

```json
{
  "organization": "AI4SciComp",
  "workspace": "~/projects/AI4SciComp",
  "repositoryPrefix": "asc-",
  "includeDotGitHub": true,
  "cloneProtocol": "ssh",
  "remote": "origin"
}
```

Configuration precedence, highest first:

1. command-line option;
2. environment variable;
3. JSON configuration file;
4. built-in default.

Environment variables:

```text
ASC_CONFIG
ASC_ORGANIZATION
ASC_WORKSPACE
ASC_REPOSITORY_PREFIX
ASC_INCLUDE_DOT_GITHUB
ASC_CLONE_PROTOCOL
ASC_REMOTE
ASC_GITHUB_TOKEN
GH_TOKEN
GITHUB_TOKEN
NO_COLOR
```

Token precedence is `ASC_GITHUB_TOKEN`, `GH_TOKEN`, then `GITHUB_TOKEN`. Do not
invoke GitHub CLI for a fallback token. Public repository discovery must work
without a token; private organization repositories require authenticated API
access through one of the environment variables.

Use `os.UserHomeDir`, not raw string replacement, to expand a leading `~` or
`~/`. Reject `~otheruser`. Clean and make the workspace absolute. Reject a
workspace that resolves to a filesystem root. Missing config is normal; malformed
config is an error naming the file and JSON problem. Reject unknown JSON fields
with `json.Decoder.DisallowUnknownFields` to catch mistakes.

Repository names must be a single safe GitHub path segment. Permit `.github` and
names containing ASCII letters, digits, `.`, `_`, and `-`; reject empty names,
`..`, separators, whitespace, URL syntax, control characters, and option-like
names beginning with `-`.

## 7. GitHub repository discovery

Use the GitHub REST API directly with `net/http`:

```text
GET https://api.github.com/orgs/{organization}/repos?type=all&per_page=100&page=N
```

Requirements:

- Set `Accept: application/vnd.github+json`.
- Set `X-GitHub-Api-Version: 2022-11-28`.
- Set a useful `User-Agent`, such as `asc-devtools/<version>`.
- Send `Authorization: Bearer <token>` only when a token exists.
- Never print, persist, or include a token in an error.
- Configure an HTTP client timeout and propagate `context.Context` cancellation.
- Support pagination via the `Link` header or successive pages until complete.
- Decode only the fields required: name, archived, fork, clone URL, SSH URL,
  default branch, and visibility/private state.
- Use returned `ssh_url` or `clone_url`; do not build an unsafe shell command.
- Make the HTTP client and base URL injectable for `httptest` tests.
- Interpret 401, 403, 404, rate limiting, malformed JSON, and network failures
  with actionable messages.
- Filter to `.github` and repositories with prefix `asc-` by default. Sort names
  deterministically. Do not include archived repositories unless explicitly
  documented and deliberately supported.

`asc repo list` lists organization repositories from the API, not merely local
directories. `--json` must emit stable machine-readable JSON and no progress text
on stdout.

## 8. Process execution design

All external execution must use `os/exec` with argument slices. Never invoke
`sh -c`, concatenate a command string, or use user input as shell syntax.

Define a narrow runner abstraction only if it materially improves testing. It
should support:

- context cancellation;
- executable name plus distinct arguments;
- working directory;
- inherited stdin when interaction is intended;
- captured stdout/stderr when parsing is required;
- streamed stdout/stderr for builds and tests;
- exit status and wrapped error reporting;
- a dry-run representation that is safely quoted for humans but never re-parsed.

Never include environment secrets in logged command descriptions. Preserve exit
codes where meaningful and distinguish “executable missing” from process failure.

## 9. Workspace and cloning behavior

`asc workspace` prints the fully resolved workspace path only.

`asc repo clone` behavior:

- Create the workspace with normal user permissions when it is absent.
- With no names, discover and clone every eligible repository not already
  present.
- With names, validate each name, verify it is returned by the organization API,
  then clone it.
- SSH protocol runs `git clone -- <ssh_url> <absolute_destination>`.
- HTTPS protocol runs `git clone -- <clone_url> <absolute_destination>`.
- Clone only by passing the API-provided URL to `git clone`.
- If the destination is already a valid Git working tree for the expected
  organization repository, report “already present” and continue.
- If a non-repository file/directory occupies the destination, report a conflict
  and do not alter it.
- Never delete, replace, or merge destination content.
- Process multiple repositories deterministically, continue after independent
  failures, print a final summary, and return nonzero if any item failed.

Resolve destinations with `filepath.Join`, then verify they remain direct
children of the absolute workspace using `filepath.Rel`. Do not follow a
destination symlink outside the workspace.

## 10. Repository status

Inspect direct child repositories only. Do not recursively search arbitrary
directories.

Use Git plumbing/porcelain output intended for programs. Prefer:

```text
git -C PATH status --porcelain=v2 --branch
```

Report, where available:

- repository name;
- branch or detached HEAD;
- upstream;
- ahead/behind counts;
- clean or dirty state;
- operation failure.

Do not parse localized human-readable `git status`. Be robust to spaces in the
workspace path and filenames. Default output should be a concise aligned summary;
`--json` must have a documented, stable schema and send diagnostics to stderr.

## 11. Safe synchronization

`asc repo sync` is deliberately conservative:

1. Validate and locate the repository.
2. Confirm it is a Git working tree.
3. Check tracked and untracked changes. If dirty, skip it without modifying it.
4. Confirm the configured remote exists.
5. Fetch that remote.
6. Determine the current branch and upstream.
7. Update only with fast-forward semantics, for example
   `git -C PATH merge --ff-only @{upstream}` after a successful fetch.
8. If detached, missing an upstream, diverged, or otherwise ambiguous, skip with
   an actionable message.
9. Continue through independent repositories and return a summary.

`--dry-run` must validate and describe planned operations without fetching or
merging. Never run `reset --hard`, `clean`, stash, checkout, rebase, force push,
commit, or push. Never auto-resolve conflicts.

## 12. CMake and CTest wrappers

Commands operate on exactly one validated local repository.

Default forms:

```text
asc configure REPOSITORY --preset PRESET
  -> cmake --preset PRESET

asc build REPOSITORY --preset PRESET
  -> cmake --build --preset PRESET

asc test REPOSITORY --preset PRESET
  -> ctest --preset PRESET
```

Run them with the repository root as the working directory and stream their
output. Detect missing `CMakePresets.json`/`CMakeUserPresets.json` and missing
executables early. If no preset was supplied, either choose a clearly documented
default only when it exists or return guidance; do not guess silently. Propagate
process failure faithfully. Do not attempt to understand or rewrite each
scientific repository's build system in v0.1.

## 13. Doctor command

`asc doctor` must check without changing user state:

- resolved configuration and workspace validity;
- Git availability and version;
- GitHub API reachability and authentication status;
- whether private-repository discovery is likely available;
- SSH executable availability and, optionally, a bounded GitHub SSH probe;
- CMake and CTest availability and versions;
- whether the binary is found on `PATH`.

Label checks as pass, warning, or failure. `--json` must provide structured
results. Do not expose tokens, full sensitive environment data, or unrelated Git
configuration. Network checks require timeouts and should distinguish offline,
authentication, authorization, and rate-limit failures.

## 14. CLI output and exit behavior

- stdout contains requested data; stderr contains progress and diagnostics.
- Detect terminal capability before ANSI color. Honor `NO_COLOR` and `--no-color`.
- Never color JSON.
- Output ordering must be deterministic.
- Success is exit 0. Invalid invocation, configuration failure, dependency
  absence, API failure, and partial multi-repository failure must be nonzero.
- Define exit codes in one place and document them; avoid excessive categories.
- Every error must state what failed and, when practical, the next corrective
  action.
- Wrap errors with `%w` when callers need `errors.Is`/`errors.As`; error strings
  begin lowercase and omit punctuation unless wrapping a complete external
  message.

## 15. Version information

Support build-time values for version, commit, and build date through `-ldflags`.
Development builds may fall back to `runtime/debug.ReadBuildInfo` and a value
such as `dev`. `asc --version` should be deterministic and must not make a network
request.

Document an example reproducible build:

```bash
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=0.1.0" \
  -o ./dist/asc ./cmd/asc
```

Place injectable version variables in a package/location that the actual linker
path can address reliably.

## 16. Completion and installation

`asc completion bash` must print a Bash completion script to stdout. It may be
maintained as a file and embedded with `//go:embed`. Completion must never make a
network call while completing ordinary command names.

Document these development/user installation paths:

```bash
go install ./cmd/asc
```

and:

```bash
CGO_ENABLED=0 go build -trimpath -o asc ./cmd/asc
install -Dm755 asc "${HOME}/.local/bin/asc"
```

If `scripts/install.sh` exists, keep it short, auditable, fail-fast, and limited
to installing an already-built binary or building with an existing Go toolchain.
It must not use `curl | sh`, sudo, package installation, telemetry, or hidden
network access. Do not provide an uninstaller that recursively deletes broad or
computed paths.

## 17. Go engineering standard

- Run `gofmt` on all Go source.
- Package names are short, lowercase, and descriptive.
- Export only symbols genuinely needed across packages; document exported APIs.
- Accept interfaces at the consumer only when multiple implementations or test
  substitution justify them. Keep interfaces small.
- Prefer concrete data types and explicit control flow.
- Pass `context.Context` as the first argument where cancellation applies; do not
  store it in a struct.
- Close response bodies and files promptly; check meaningful close/flush errors.
- Set restrictive permissions for any file that could contain sensitive data.
- Avoid package-level mutable state.
- Do not call `os.Exit` outside the thin `main`; return an exit code from the app.
- Do not use `log.Fatal` in libraries.
- Do not panic for user input or recoverable runtime conditions.
- Avoid goroutines unless concurrency provides a demonstrated benefit. If used,
  bound it, preserve deterministic output, propagate cancellation, and test it.
- Keep functions small enough to reason about, without mechanical fragmentation.

## 18. Required testing

Use only the standard `testing` package and standard-library test helpers. Do not
add Testify or another assertion framework.

Provide table-driven unit tests and meaningful integration tests for:

- configuration precedence and malformed/unknown JSON;
- home/workspace expansion and path containment;
- repository-name validation;
- GitHub REST pagination, filtering, authentication headers, redaction, API
  errors, rate limits, timeouts, and malformed responses using `httptest.Server`;
- process runner cancellation and missing executables;
- clone planning and destination conflicts;
- porcelain-v2 status parsing, including detached HEAD and ahead/behind;
- safe sync decisions for clean, dirty, diverged, detached, and no-upstream repos;
- CMake/CTest argument construction and exit propagation;
- CLI help, invalid invocations, stdout/stderr separation, JSON, and exit codes;
- doctor results and version output.

Use `t.TempDir`. For Git behavior, create temporary local repositories with Git
when appropriate. For deterministic command tests, place tiny fake `git`,
`cmake`, or `ctest` executables in a temporary `PATH`; never modify the
developer's real repositories or global Git configuration. Set local test-repo
user identity when commits are needed.

Tests must not depend on live GitHub, the user's token, SSH keys, home directory,
network, clock timing, or current locale. Run live smoke checks only as an
explicitly separate, opt-in mechanism.

Required local verification:

```bash
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
go test -race ./...
CGO_ENABLED=0 go build -trimpath -o /tmp/asc ./cmd/asc
/tmp/asc --help
/tmp/asc --version
```

Also verify that `go mod tidy` introduces no dependency and that source control
contains no accidental binary, token, test home directory, or generated cache.

## 19. Continuous integration

Add a minimal GitHub Actions workflow for pushes and pull requests. It should:

- test supported Go 1.25.x and 1.26.x versions on Ubuntu;
- check formatting with `gofmt`;
- run `go vet ./...`, `go test ./...`, and a race test on at least one version;
- build with `CGO_ENABLED=0`;
- confirm `go.mod` has no third-party requirements after `go mod tidy`;
- use least-privilege `contents: read` permissions;
- use action versions pinned to stable major versions or immutable commits,
  following the repository's established policy;
- not upload secrets or depend on an organization token.

A GitHub Action used by CI is automation infrastructure, not a Go runtime
dependency. Keep the workflow small and transparent.

## 20. Documentation deliverables

Update or create:

- `README.md`: purpose, scope, fast start, requirements, command examples,
  configuration, token setup, SSH versus HTTPS, and safety guarantees;
- `docs/architecture.md`: package boundaries, execution flow, API integration,
  configuration precedence, and trust boundaries;
- `docs/commands.md`: complete command/flag reference, JSON schemas, exit codes,
  and examples;
- `docs/installation.md`: build/install on WSL2 Ubuntu, `~/.local/bin`, shell
  completion, release binaries, and upgrades;
- `CONTRIBUTING.md`: Go prerequisites and exact validation commands;
- `CHANGELOG.md`: initial v0.1 entry;
- `CITATION.cff`: only if appropriate for the existing project;
- `.gitignore`: Go build/test outputs and local editor files, without hiding
  source or important configuration.

Document authentication without putting a real token in shell history. Explain
that REST authentication uses only the documented environment variables and
that Git transport authentication (SSH key or HTTPS credentials) is distinct.

## 21. Security and safety invariants

Treat organization names, repository names, paths, API responses, Git output,
configuration, and environment values as untrusted input.

Mandatory invariants:

- no shell evaluation;
- no token logging;
- no credential storage by `asc`;
- no operation outside the resolved workspace;
- no recursive repository discovery;
- no destructive Git operation;
- no deletion or replacement of existing clone destinations;
- no following repository symlinks outside the workspace;
- no silent dirty-worktree changes;
- no automatic conflict resolution;
- no telemetry;
- no implicit write to global Git, SSH, Go, or shell configuration;
- no network operation for `--help`, `--version`, `workspace`, completion, or
  purely local status.

Use bounded HTTP timeouts and response-size protections. Avoid reading an
unbounded error body. Preserve enough context for diagnosis without leaking
secrets.

## 22. Implementation sequence

Work in reviewable phases, while completing the whole task in the current
checkout:

1. inspect repository and record baseline;
2. establish module, package boundaries, version, and exit-code design;
3. implement configuration and validation with tests;
4. implement process runner with tests;
5. implement GitHub REST client with `httptest` tests;
6. implement workspace discovery, clone, status, and sync with tests;
7. implement CMake/CTest and doctor;
8. implement CLI parsing, output, JSON, version, and completion;
9. write documentation and CI;
10. run formatting, vet, unit/integration/race tests, and static build;
11. inspect `git diff --check`, `git status`, and the final diff for accidental
    changes or secrets.

Fix failures instead of weakening or deleting tests. If a requirement conflicts
with the existing repository, explain the conflict in the final report and choose
the smallest safe adaptation.

## 23. Definition of done

The task is complete only when:

- `asc` builds as a statically usable `CGO_ENABLED=0` binary;
- `go.mod` contains no third-party dependency;
- every required command has help, validation, and safe error behavior;
- public discovery works without a token; authenticated discovery supports
  private repositories through the documented environment variables;
- clone/status/sync preserve every safety invariant;
- standard-library tests cover success and significant failure paths;
- `gofmt`, `go vet`, tests, race tests, and build pass;
- documentation accurately matches observed behavior;
- no user change, credential, binary, or unrelated file was overwritten;
- no commit, push, release, or external repository change was made.

## 24. Final response format

At completion, respond with:

1. a concise outcome summary;
2. key architecture and safety decisions;
3. files added or materially changed;
4. exact checks run and their results;
5. any remaining limitations or follow-up work;
6. confirmation that no third-party Go dependency was introduced;
7. confirmation that nothing was committed or pushed.

Do not claim success for checks that were not actually run. If blocked by the
environment, report the precise command, failure, and remaining verification.

## 25. User-requested self-update extension (2026-07-19)

This section supersedes the earlier exclusion of release-related functionality
only where needed for installing a published asc release. Implement a top-level
`asc update [--check] [--yes] [--prefix PATH]`; it is not an alias for
`asc repo sync`, and `repo update` must not exist. The command checks the latest
release in `AI4SciComp/asc-devtools`, selects the Go asset for the running OS and
architecture, verifies it against the release `SHA256SUMS`, rejects unsafe
archive contents, and runs the existing hash-guarded installer. It refuses
unmanaged or modified installations, never invokes `sudo`, supports the normal
GitHub token precedence, bounds all downloads, and has offline HTTP-fixture
tests. Include a deterministic packaging helper and document the release asset
contract.

## 26. User-requested repository save extension (2026-07-19)

This section supersedes the earlier prohibition on application commit/push
functionality only for one explicit command. Add
`asc repo save REPOSITORY [--message TEXT] [--dry-run] [--yes]`. Keep `repo sync`
download-only and document that it fetches plus fast-forward merges clean
worktrees; it never uploads. Save operates on exactly one managed worktree and
defaults to `Updated at YYYY-MM-DD HH:MM:SS` in local time when `--message` is
omitted. It prints an exact plan and prompts unless `--yes`. Before staging,
revalidate the reviewed status and fetch the configured
remote, and refuse detached HEAD, conflicts, missing/wrong upstream, remote-ahead,
or diverged histories. Then stage all changes, commit only a nonempty index, and
push the exact tracked branch without force. A clean locally-ahead branch may be
pushed without an empty commit. Preserve and report a local commit if push fails.
Add real bare-remote tests. No other command may commit or push.
