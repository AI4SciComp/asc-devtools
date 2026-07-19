# Generator Prompt: Bash `AI4SciComp/asc-devtools`

## Authoritative parity addendum

Generate the Bash branch with the same observable contract as the Go reference
on `go`. This addendum overrides conflicting details below:

- use strict `~/.config/asc/config.json` with camelCase keys and canonical CLI,
  environment, file, default precedence;
- use the direct GitHub REST API through bounded curl requests with optional
  `gh auth token` fallback, not `gh repo list`;
- support `doctor --json`, `repo status --json`, clone `--protocol`, sync
  `--dry-run`, and `completion bash` with the documented stable schemas;
- require `--preset` and invoke the exact CMake/CTest argument arrays;
- default installation to `/usr/local` and provide hash-verified uninstall;
- preserve exit codes 0, 1, and 2 and propagate CMake/CTest codes 1 through 125.

The maintained implementation, tests, README, and `docs/commands.md` are the
executable specification when older examples below disagree.

You are the principal software engineer responsible for designing and implementing
the `asc-devtools` repository for the GitHub organization **AI4SciComp**.

The identities and ownership model are:

- Personal GitHub username: `escapetiger`
- GitHub organization: `AI4SciComp`
- Repository: `AI4SciComp/asc-devtools`
- Primary development environment: Ubuntu under WSL2
- Default local workspace: `~/projects/AI4SciComp`

Work directly in the existing local checkout of `asc-devtools`. Inspect the
repository before changing anything. Preserve unrelated user changes. Improve,
refactor, or replace weak existing code when justified, but do not silently erase
user work or change the repository license.

Do not merely propose code or return isolated snippets. Implement the complete
repository, test it, and leave it in a working state. Do not commit, push, publish
a release, open a pull request, or modify other repositories unless the user
explicitly asks you to do so.

## 1. Nonnegotiable language requirement

Implement the tool entirely in **Bash**.

Hard requirements:

- Do not use Python, Perl, Ruby, Node.js, Go, Rust, or a compiled helper.
- Do not create `pyproject.toml`, Python packages, virtual environments, or pip
  installation instructions.
- Do not require a third-party runtime library.
- Do not use Bats as a required test dependency; provide a self-contained Bash
  test harness.
- Use `#!/usr/bin/env bash` for executable scripts.
- Target Bash 4.4 or later, which is available in supported Ubuntu/WSL2 systems.
- Do not claim POSIX `sh` compatibility when Bash features are used.

Shell-only compatibility does not mean writing one unmaintainable file. Use a
small executable front end and cohesive sourced Bash modules.

## 2. Project objective

Provide one command named `asc` that standardizes common local development
operations across repositories owned by the `AI4SciComp` organization.

Expected repositories include:

```text
AI4SciComp/.github
AI4SciComp/asc-devtools
AI4SciComp/asc-cpp
AI4SciComp/asc-pde
AI4SciComp/asc-kinetic
AI4SciComp/asc-lean
AI4SciComp/asc-platform
```

The tool should help an Ubuntu/WSL2 developer discover, clone, inspect, update,
configure, build, and test these repositories from a common workspace.

This repository is for developer tooling. It must not contain LLM orchestration,
reinforcement learning, PDE solvers, kinetic models, or formal mathematics.

## 3. Required preliminary inspection

Before editing:

1. Run `pwd`, inspect `git status`, and list the complete working tree.
2. Read `README.md`, `LICENSE`, `CONTRIBUTING.md`, existing scripts, workflows,
   and repository-specific instructions.
3. Search for `AGENTS.md` or equivalent instructions and follow them.
4. Identify existing user changes and preserve them.
5. Run existing tests and syntax checks to establish a baseline.
6. Check available versions of Bash, Git, GitHub CLI, CMake, CTest, ShellCheck,
   and `shfmt`.
7. Inspect the current remote with `git remote -v`, but do not change it unless
   necessary and explicitly authorized.

Do not assume code from a previous generated archive is correct. Judge the actual
checkout.

## 4. Scope of version 0.1

Implement these canonical commands:

```text
asc --help
asc --version
asc doctor
asc workspace
asc repo list
asc repo clone [REPOSITORY...]
asc repo status [REPOSITORY...]
asc repo sync [REPOSITORY...]
asc configure REPOSITORY [--preset PRESET]
asc build REPOSITORY [--preset PRESET]
asc test REPOSITORY [--preset PRESET]
asc completion bash
```

Short backward-compatible aliases may be included if they already exist:

```text
asc list
asc clone
asc status
asc sync
```

The grouped `asc repo ...` commands are the canonical documented interface.

Do not implement commit, push, reset, stash, checkout, branch deletion, force
operations, tag creation, release publication, or pull-request creation in v0.1.

## 5. Proposed repository structure

Use a structure similar to:

```text
asc-devtools/
├── bin/
│   └── asc
├── lib/
│   └── asc/
│       ├── common.sh
│       ├── config.sh
│       ├── process.sh
│       ├── github.sh
│       ├── repository.sh
│       ├── doctor.sh
│       └── cmake.sh
├── completions/
│   └── asc.bash
├── tests/
│   ├── test_helper.sh
│   ├── test_config.sh
│   ├── test_github.sh
│   ├── test_repository.sh
│   ├── test_cli.sh
│   └── run_tests.sh
├── docs/
│   ├── architecture.md
│   ├── commands.md
│   └── installation.md
├── examples/
│   └── config
├── .github/
│   └── workflows/
│       └── ci.yml
├── install.sh
├── uninstall.sh
├── README.md
├── LICENSE
├── CHANGELOG.md
├── CONTRIBUTING.md
└── CITATION.cff
```

This is guidance rather than a demand for excessive fragmentation. Each sourced
module must have a clear responsibility. Avoid circular sourcing and hidden
global state.

## 6. Bash coding standard

Every executable and sourced Bash file must use disciplined shell programming.

Requirements:

```bash
#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
```

Apply strict mode carefully. Expected nonzero commands in conditions must not
accidentally terminate the program. In functions that intentionally collect
partial failures, handle exit statuses explicitly.

Additional rules:

- Quote every parameter expansion unless deliberate word splitting is required.
- Use `[[ ... ]]` for Bash conditionals.
- Use `local` variables inside functions.
- Use `readonly` for constants.
- Prefer `printf` to `echo` for controlled output.
- Use descriptive `lower_snake_case` function and variable names.
- Prefix internal functions consistently, for example `asc_` or `_asc_`.
- Avoid global mutable variables where practical.
- Do not use `eval`.
- Do not use `source` on paths controlled by repository names or untrusted input.
- Do not build commands as one string. Use Bash arrays:

  ```bash
  local -a command=(git -C "${repository_path}" status --porcelain)
  "${command[@]}"
  ```

- Use `--` before user-influenced positional arguments where supported.
- Send errors to stderr.
- Return meaningful nonzero statuses.
- Functions that print data for command substitution must not also print progress
  messages to stdout.
- Use traps only when necessary and never let cleanup delete user data.
- Do not depend on GNU-only behavior without documenting that Ubuntu/WSL2 is the
  supported environment.

Run ShellCheck over every shell file. Address warnings rather than globally
disabling them. If a suppression is truly required, keep it narrow and explain
why. Format with `shfmt` if available and document the chosen indentation.

## 7. Configuration

Do not use TOML because the implementation must not require a TOML parser.

Use an optional trusted, user-owned Bash configuration file:

```text
~/.config/asc/config
```

Example:

```bash
ASC_ORGANIZATION="AI4SciComp"
ASC_WORKSPACE="${HOME}/projects/AI4SciComp"
ASC_REPOSITORY_PREFIX="asc-"
ASC_INCLUDE_DOT_GITHUB="true"
ASC_CLONE_PROTOCOL="ssh"
ASC_REMOTE="origin"
```

The program may source this file because it is explicitly a user-owned shell
configuration file. Document clearly that it is executable shell syntax and must
not be copied from an untrusted source.

Configuration precedence must be:

1. explicit command-line arguments, when provided;
2. environment variables already exported before `asc` starts;
3. the configuration file;
4. built-in defaults.

Preserve environment values when loading the file. A simple implementation may
record which relevant variables were already set before sourcing, source the
configuration, and restore the original environment values afterward.

Alternatively, support only defaults plus exported environment variables in v0.1
and treat the config file as a shell fragment loaded before invocation. If you
choose this simpler design, document it honestly. Do not implement an unsafe
ad-hoc parser that uses `eval`.

Defaults:

```text
ASC_ORGANIZATION=AI4SciComp
ASC_WORKSPACE=~/projects/AI4SciComp
ASC_REPOSITORY_PREFIX=asc-
ASC_INCLUDE_DOT_GITHUB=true
ASC_CLONE_PROTOCOL=ssh
ASC_REMOTE=origin
```

Support `ASC_CONFIG` to select another configuration path. Expand the workspace
using shell parameter expansion; do not use `eval` to expand arbitrary text.

Validate Boolean values, clone protocols, organization names, repository prefixes,
and workspace paths. Report actionable errors.

## 8. External dependencies

Runtime dependencies:

- Bash 4.4+
- Git
- GitHub CLI (`gh`)
- CMake and CTest only for C++ build commands

Optional development dependencies:

- ShellCheck
- shfmt

Do not require external `jq`. Use GitHub CLI's built-in `--json` and `--jq`
support for repository discovery. Keep `--jq` expressions simple and test their
output assumptions.

Do not parse human-oriented `gh` tables.

## 9. GitHub identities and authentication

The implementation and documentation must distinguish:

```text
Authenticated personal user: escapetiger
Repository owner: AI4SciComp
```

Repository discovery queries the organization:

```bash
gh repo list AI4SciComp ...
```

Git SSH operations authenticate using the SSH key associated with `escapetiger`.
GitHub CLI API calls authenticate using the token stored by `gh auth login`.

Do not claim the SSH key authenticates GitHub API calls. `asc doctor` must explain
and test these two layers separately.

## 10. Repository discovery

Use GitHub CLI to list repositories. Prefer a stable machine-oriented output such
as tab-separated fields produced with `--jq`:

```text
name<TAB>nameWithOwner<TAB>isArchived
```

Default selection:

- include non-archived repositories beginning with `ASC_REPOSITORY_PREFIX`;
- include `.github` when `ASC_INCLUDE_DOT_GITHUB=true`;
- exclude archived repositories;
- sort deterministically by repository name.

Handle repository names without using them as executable syntax. Model discovery
with Bash arrays or line-oriented records; do not encode structured data into
fragile space-separated strings.

## 11. Clone behavior

`asc repo clone` with no names clones all selected missing repositories. With
names, it clones only requested organization repositories.

Requirements:

- Create the workspace if absent.
- Validate requested repository names against organization discovery.
- Skip an existing Git repository and report it.
- Refuse an existing non-Git destination and leave it untouched.
- Never overwrite a directory.
- Continue after one failure and summarize results.
- Return nonzero when any requested clone fails.
- Never assume organization repositories belong to `escapetiger`.

For SSH, deliberately use Git rather than passing an SSH URL to `gh repo clone`:

```bash
git clone -- "git@github.com:AI4SciComp/REPOSITORY.git" DESTINATION
```

For HTTPS, use:

```bash
git clone -- "https://github.com/AI4SciComp/REPOSITORY.git" DESTINATION
```

This avoids ambiguity between the accepted argument formats of `git clone` and
`gh repo clone`. GitHub CLI remains responsible for organization discovery.

## 12. Local repository discovery and validation

A managed local repository is a direct child of `ASC_WORKSPACE` that:

- is a Git worktree;
- starts with `ASC_REPOSITORY_PREFIX`, or is `.github` when enabled.

Use Git itself where possible to recognize worktrees:

```bash
git -C "${path}" rev-parse --is-inside-work-tree
```

Do not require `.git` to be a directory, because worktrees can use a `.git` file.

Repository arguments must be simple repository names. Reject:

- empty names;
- absolute paths;
- `/` or backslashes;
- `.` and `..`;
- path traversal;
- names outside the configured prefix, except `.github` when allowed.

Resolve every target below the workspace and never operate on a broad path such as
`$HOME` or `/`.

## 13. Status behavior

`asc repo status` must report for each selected repository:

- repository name;
- current branch or detached HEAD;
- clean/dirty state;
- upstream branch when configured;
- ahead and behind counts when available;
- concise changed-file output.

Use Git porcelain formats intended for scripts. Do not parse colored human output.

One broken repository must not prevent inspection of all others. Continue,
accumulate failures, print a summary, and return nonzero if any target failed.

The command must be read-only.

## 14. Synchronization behavior

`asc repo sync` must be conservative:

1. Validate the target repository.
2. Refuse a dirty working tree.
3. Verify the configured remote exists.
4. Fetch from the remote.
5. Update only by fast-forward.
6. Never create a merge commit.
7. Never rebase, stash, reset, checkout, or discard changes.
8. Continue after individual failures.
9. Print a summary with updated, unchanged, skipped, and failed counts.
10. Return nonzero if a repository failed or was unsafe to update.

`git pull --ff-only` is acceptable for v0.1. Separate `git fetch` followed by a
fast-forward-only update is preferable if it leads to clearer diagnostics without
introducing fragile branch assumptions.

Do not fetch or pull `.github` differently from other repositories.

## 15. Environment diagnostics

`asc doctor` must report:

- Bash version and whether it meets the minimum;
- resolved configuration path;
- organization and workspace;
- availability and versions of Git, GitHub CLI, CMake, and CTest;
- optional availability of ShellCheck and shfmt;
- authenticated GitHub username using `gh api user --jq .login`;
- GitHub CLI Git protocol using `gh config get git_protocol --host github.com`;
- result of GitHub CLI API authentication;
- whether GitHub CLI can list repositories in `AI4SciComp`;
- SSH authentication result using `ssh -T git@github.com`, interpreted carefully
  because GitHub's successful SSH test may return a nonstandard status;
- whether the workspace exists and is writable;
- remediation suggestions for failed checks.

Never print tokens, private keys, or complete environments.

## 16. CMake commands

Each C++ repository owns its committed `CMakePresets.json`. `asc-devtools` must
not duplicate project-specific CMake flags.

Implement:

```bash
asc configure asc-cpp --preset dev
asc build asc-cpp --preset dev
asc test asc-cpp --preset dev
```

Execute from the selected repository root:

```bash
cmake --preset dev
cmake --build --preset dev --parallel
ctest --preset dev --output-on-failure
```

Use Bash command arrays. Stream output directly. Return the external tool's failure
through the CLI. Give a clear error for a missing repository, executable, preset,
or `CMakePresets.json`.

Do not introduce organization-wide build directories in v0.1.

## 17. Output and user experience

Use consistent, plain output suitable for terminals and logs. Color is optional
and must be disabled when stdout is not a terminal or when `NO_COLOR` is set.

Recommended status words:

```text
OK
SKIP
FAIL
WARN
```

Do not make Unicode symbols necessary to understand output.

Every multi-repository mutation command must end with a summary. Progress messages
go to stderr if stdout is reserved for machine-readable data.

Help must include examples and explain the workspace model. Unknown commands and
invalid options must return status 2. Operational failure should return status 1.
Success should return status 0.

## 18. Installation and uninstallation

Implement an idempotent `install.sh` that installs:

```text
bin/asc -> ~/.local/bin/asc
lib/asc/* -> ~/.local/lib/asc/*
completions/asc.bash -> ~/.local/share/bash-completion/completions/asc
```

Copying files is preferable to symlinking for a normal installation. Support an
optional prefix:

```bash
./install.sh --prefix "${HOME}/.local"
```

The installer must:

- create only its required directories;
- never overwrite unrelated files;
- report files it installs;
- be safe to rerun for the same tool;
- preserve executable permissions;
- avoid `sudo` by default;
- explain how to add `~/.local/bin` to PATH.

Implement a matching `uninstall.sh` that removes only files installed by
`asc-devtools`. It must use explicit validated paths, not recursive deletion of a
broad directory. If using a manifest, keep it within the installation and validate
each entry before removal.

Also support running directly from the checkout:

```bash
./bin/asc --help
```

The executable must locate its libraries both in the source tree and after
installation. Resolve symbolic links carefully if symlink execution is supported.

## 19. Bash completion

Provide completion for:

- top-level commands;
- `repo` subcommands;
- locally cloned repository names;
- common preset names such as `dev` and `release`;
- option names.

Completion must:

- remain quiet;
- avoid network calls on every Tab press;
- tolerate a missing workspace;
- avoid executing mutating commands;
- not fail the interactive shell under strict mode.

## 20. Safety requirements

These requirements are mandatory:

- Never run `rm -rf`.
- Never run `git reset --hard`, `git clean`, automatic stash, or force push.
- Never delete, replace, or move an existing repository.
- Never operate outside the resolved workspace.
- Never evaluate repository names as shell code.
- Never use unresolved globs for destructive operations.
- Never silently discard a failed operation.
- Never automatically commit or push.
- Never print credentials or sensitive environment contents.
- Treat an existing non-Git destination as user-owned data.
- Refuse ambiguous or unsafe Git states with a clear explanation.

## 21. Testing without third-party frameworks

Create a self-contained Bash test harness. It should support:

- defining a test function;
- assertions for equality, success, failure, files, directories, and output;
- temporary directories created with `mktemp -d`;
- cleanup through safe traps targeting only the exact temporary directory;
- per-test pass/fail reporting;
- a final summary and nonzero status on failure.

Tests must not:

- access the live GitHub organization;
- clone over the network;
- mutate real repositories;
- change global Git configuration;
- depend on the user's home configuration.

Mock external commands by creating temporary executables named `gh`, `git`,
`cmake`, or `ctest` at the front of a test-specific `PATH`. For Git semantics that
are clearer with real Git, initialize repositories inside a temporary directory
and set repository-local test identity:

```bash
git -C "${temporary_repository}" config user.name "Test User"
git -C "${temporary_repository}" config user.email "test@example.com"
```

At minimum, test:

### Configuration

- built-in defaults;
- trusted config loading;
- environment precedence;
- alternative `ASC_CONFIG`;
- invalid Boolean and protocol values;
- paths containing spaces;

### Repository discovery

- prefix filtering;
- `.github` inclusion/exclusion;
- archived filtering;
- deterministic ordering;
- failed GitHub CLI authentication;

### Validation

- valid repository names;
- rejection of traversal, separators, and absolute paths;
- worktree recognition when `.git` is a file;

### Clone

- missing repository clone command;
- existing Git repository skip;
- existing non-Git destination refusal;
- explicit selection;
- partial failure summary and exit status;
- SSH and HTTPS URL formation.

### Status

- clean and dirty repository;
- branch and detached HEAD;
- upstream present and absent;
- continued processing after failure.

### Sync

- clean fast-forward;
- dirty skip;
- divergent branch refusal;
- missing remote;
- continued processing and final summary.

### CMake wrappers

- working directory;
- command-array construction;
- default and explicit presets;
- missing `CMakePresets.json`;
- propagated failure.

### CLI

- help;
- version;
- grouped command routing;
- invalid command status 2;
- operational failure status 1.

Run every test in a controlled environment with `HOME`, `PATH`, workspace, and
configuration isolated where practical.

## 22. Continuous integration

Create GitHub Actions CI on Ubuntu. It must:

1. check out the repository;
2. install ShellCheck and shfmt, or use trusted pinned actions;
3. run `bash -n` over all shell files;
4. run ShellCheck over all shell files;
5. check formatting with `shfmt -d`;
6. execute the self-contained test suite;
7. run installer tests with a temporary prefix;
8. run `asc --version` and `asc --help` smoke tests from the installed prefix.

Keep action permissions minimal:

```yaml
permissions:
  contents: read
```

Pin action major versions at minimum. Do not put tokens in repository files.

## 23. Documentation

Write a complete README with:

- purpose and non-goals;
- Bash and external-tool requirements;
- WSL2 installation;
- direct-checkout use;
- distinction between `escapetiger` and `AI4SciComp`;
- Git SSH versus GitHub CLI API authentication;
- configuration;
- quick start;
- every command with examples;
- safety guarantees;
- testing and development;
- relationship with `AI4SciComp/.github`, possible future `asc-cmake`, and the
  scientific repositories.

Write exact command and architecture documents. Do not document unimplemented
features. Explain that `asc-devtools` is Bash-specific, not POSIX `sh`.

Preserve the existing license unless the user explicitly authorizes changing it.

## 24. Acceptance criteria

The implementation is complete only when:

- no Python implementation or Python runtime requirement remains;
- `./bin/asc --help` works from the checkout;
- installation into a temporary prefix succeeds;
- the installed `asc` locates installed libraries;
- `asc doctor` distinguishes SSH and API authentication;
- repository discovery queries `AI4SciComp`;
- clone uses correctly constructed Git SSH or HTTPS URLs;
- clone never overwrites existing data;
- status is read-only and handles multiple repositories;
- sync refuses dirty or divergent repositories and fast-forwards only;
- CMake commands use repository-owned presets;
- all Bash tests pass without network access;
- `bash -n` passes;
- ShellCheck passes;
- shfmt reports no formatting differences;
- documentation matches behavior;
- no unrelated user change is overwritten;
- Git status contains only intentional changes.

## 25. Final verification

Run the strongest available equivalent of:

```bash
find . -type f -name '*.sh' -o -path './bin/asc'
bash -n bin/asc lib/asc/*.sh tests/*.sh install.sh uninstall.sh
shellcheck bin/asc lib/asc/*.sh tests/*.sh install.sh uninstall.sh
shfmt -d -i 2 -ci bin/asc lib/asc/*.sh tests/*.sh install.sh uninstall.sh
./tests/run_tests.sh

temporary_prefix="$(mktemp -d)"
./install.sh --prefix "${temporary_prefix}"
"${temporary_prefix}/bin/asc" --version
"${temporary_prefix}/bin/asc" --help
./uninstall.sh --prefix "${temporary_prefix}"
```

Use a safe cleanup trap for the exact temporary prefix. Never delete a broad or
unresolved path.

Do not claim ShellCheck or shfmt passed if they were unavailable. If a tool is
missing, report that fact and run all other checks.

## 26. Final response

Lead with the working outcome. Report:

1. implemented architecture;
2. commands and behavior;
3. safety decisions;
4. important files changed;
5. exact tests and checks run with results;
6. any unavailable verification tool;
7. remaining limitations;
8. recommended next milestone.

Do not include a long chronological diary. Do not commit, push, publish, or open a
pull request unless separately requested.
