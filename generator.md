# Generator Prompt: Python `AI4SciComp/asc-devtools`

## Authoritative parity addendum

Generate the Python branch with the same observable contract as the Go reference
on `impl/go`. This addendum overrides conflicting details below:

- use strict `~/.config/asc/config.json` with camelCase keys and canonical CLI,
  environment, file, default precedence;
- use the direct GitHub REST API with bounded responses and optional
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

The GitHub identities must not be confused:

- Personal GitHub username: `escapetiger`
- GitHub organization: `AI4SciComp`
- Repository: `AI4SciComp/asc-devtools`
- Default local WSL2 workspace: `~/projects/AI4SciComp`

Work directly in the existing local checkout of `asc-devtools`. Inspect the
repository before changing anything. Do not assume that its current architecture
or implementation is correct. Preserve unrelated user changes and improve,
refactor, or replace existing code when justified by tests and a coherent design.

Do not merely propose code or provide snippets. Implement the repository, test it,
and leave it in a complete working state. Do not commit, push, publish a release,
open a pull request, delete user files, or alter other repositories unless the
user explicitly asks you to do so.

## 1. Project objective

Develop a safe, maintainable command-line tool named `asc` that standardizes
common local development operations across repositories owned by the
`AI4SciComp` organization.

The organization is expected to contain repositories such as:

```text
AI4SciComp/.github
AI4SciComp/asc-devtools
AI4SciComp/asc-cpp
AI4SciComp/asc-pde
AI4SciComp/asc-kinetic
AI4SciComp/asc-lean
AI4SciComp/asc-platform
```

The tool must help a developer using Ubuntu under WSL2 discover, clone, inspect,
update, configure, build, and test these repositories from a common local
workspace.

The project is a developer tool, not an agentic scientific-computing runtime. It
must not contain LLM orchestration, RL algorithms, PDE solvers, or scientific
models.

## 2. Required engineering approach

Before implementing:

1. Inspect the complete working tree, including existing documentation, scripts,
   package metadata, tests, Git status, and configuration.
2. Identify any `AGENTS.md`, repository instructions, or organization standards
   and follow them.
3. Determine whether the current implementation is Bash, Python, or mixed.
4. Run existing tests and record baseline failures.
5. Check installed versions of Python, Git, GitHub CLI, CMake, CTest, and relevant
   quality tools.
6. Review the supplied Google Python Style Guide. Follow its current naming,
   import, documentation, typing, exception, and main-function guidance. If the
   repository contains C++ code, also follow the supplied Google C++ Style Guide.

Prefer a Python 3.11+ implementation with a small console entry point named
`asc`. Runtime code should use only the Python standard library unless a
third-party dependency provides clear, substantial value. Do not add a framework
merely to parse a few commands or a small TOML file.

Use `pyproject.toml` and a `src` package layout. Follow these naming conventions:

```text
Distribution: asc-devtools
Python package: asc_devtools
Console command: asc
Modules/functions/variables: lower_with_under
Classes/exceptions: CapWords
Constants: CAPS_WITH_UNDER
```

Public functions and classes must have useful docstrings and type annotations.
Avoid ambiguous abbreviations. Keep functions focused. Do not invoke commands
through `shell=True`. Pass argument arrays to `subprocess` so repository names and
paths cannot be interpreted as shell code.

## 3. Scope of version 0.1

Implement these commands:

```text
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
```

You may retain shorter backward-compatible aliases such as `asc list`,
`asc clone`, `asc status`, and `asc sync` if they already exist, but the grouped
`asc repo ...` interface should be the canonical documented design because it
scales better as new command families are introduced.

Do not add commands for commit, push, reset, checkout, branch deletion, force
operations, release publication, or pull-request creation in version 0.1.

## 4. Configuration

Support a TOML configuration file at:

```text
~/.config/asc/config.toml
```

Support an alternative file with:

```text
asc --config PATH ...
```

Use the following schema:

```toml
[asc]
organization = "AI4SciComp"
workspace = "~/projects/AI4SciComp"
repository_prefix = "asc-"
include_dot_github = true
clone_protocol = "ssh"
```

Support these environment overrides:

```text
ASC_CONFIG
ASC_ORGANIZATION
ASC_WORKSPACE
ASC_REPOSITORY_PREFIX
ASC_CLONE_PROTOCOL
```

The precedence must be:

1. explicit command-line arguments;
2. environment variables;
3. configuration file;
4. built-in defaults.

Expand `~` and resolve workspace paths. Do not require the workspace to exist for
read-only configuration inspection. Validate configuration and report concise,
actionable errors without Python tracebacks during normal CLI use.

The default values must be:

```text
organization = AI4SciComp
workspace = ~/projects/AI4SciComp
repository_prefix = asc-
include_dot_github = true
clone_protocol = ssh
```

Do not confuse the organization with the authenticated user `escapetiger`.
Repository discovery must query `AI4SciComp`; authentication is handled by the
user's GitHub CLI session.

## 5. External command boundary

Create one small, well-tested abstraction for running external commands. It must:

- accept a sequence of arguments, never a shell command string;
- optionally accept a working directory;
- capture output when needed;
- support streaming output for builds and tests;
- return a typed result;
- raise a project-specific exception on failure;
- include the command and useful stderr in error messages;
- handle a missing executable cleanly;
- avoid leaking tokens or environment secrets.

Do not scatter raw `subprocess.run` calls throughout the codebase.

## 6. GitHub repository discovery

Use GitHub CLI for authenticated organization discovery:

```bash
gh repo list AI4SciComp --limit 1000 --json ...
```

Parse JSON in Python. Do not parse human-formatted terminal tables.

By default, include:

- non-archived repositories whose names start with `asc-`;
- `.github` when `include_dot_github` is true.

Exclude archived repositories by default. Ensure deterministic alphabetical
ordering. Model remote repositories with an immutable typed object.

`asc repo list` should provide a readable table. If straightforward, also support
`--json` for scripting, but do not compromise the core implementation to add it.

## 7. Cloning behavior

`asc repo clone` with no repository arguments must clone every matching missing
organization repository. With arguments, it must clone only the selected names.

Requirements:

- Create the workspace directory if it does not exist.
- Skip an already cloned Git repository and report it clearly.
- Refuse to overwrite an existing non-Git path.
- Validate requested repository names against organization discovery.
- Clone organization repositories, not repositories under `escapetiger`.
- Respect the configured clone protocol.
- For SSH, the effective remote should be equivalent to:
  `git@github.com:AI4SciComp/REPOSITORY.git`.
- It is acceptable to call `gh repo clone AI4SciComp/REPOSITORY`, provided GitHub
  CLI is configured to use SSH. Direct `git clone` with an explicit SSH URL is
  also acceptable. Choose one approach and document it precisely.
- Never delete or replace an existing directory.
- Return a nonzero exit status if any requested clone fails.

Do not use Git submodules to represent organization membership.

## 8. Local repository discovery and selection

A local managed repository is a direct child of the configured workspace that:

- contains a Git worktree or `.git` metadata;
- starts with the configured prefix, or is `.github` when enabled.

Do not recursively treat nested dependency checkouts or build directories as
organization repositories.

Repository arguments should accept names such as `asc-cpp`, not arbitrary paths.
Reject path traversal, absolute paths, separators, and names outside the managed
workspace.

## 9. Status behavior

`asc repo status` must report, for every selected local repository:

- repository name;
- current branch or detached-HEAD state;
- clean or dirty working tree;
- upstream tracking state when available;
- ahead/behind counts when practical;
- concise changed-file information.

One broken repository must not prevent status reporting for all others. Accumulate
failures and return a nonzero exit code after reporting the remaining repositories.

Do not mutate repositories in the status command.

## 10. Synchronization behavior

`asc repo sync` must be conservative. For every selected repository:

1. Verify that it is a valid Git repository.
2. Refuse to update a dirty working tree.
3. Fetch the configured remote.
4. Update only by fast-forward.
5. Never create an automatic merge commit.
6. Never rebase, stash, reset, checkout, or discard changes.
7. Continue processing other repositories after a failure.
8. Print a final summary of updated, unchanged, skipped, and failed repositories.
9. Return nonzero if any repository fails.

Using `git pull --ff-only` is acceptable for the first implementation. If separate
`fetch` and `merge --ff-only` commands provide clearer reporting, prefer them.

## 11. Environment diagnostics

`asc doctor` must report:

- resolved configuration path;
- organization;
- workspace;
- authenticated GitHub username from `gh api user` when available;
- Git protocol configured in GitHub CLI;
- availability and version of Python, Git, GitHub CLI, CMake, and CTest;
- whether the workspace exists and is writable;
- whether GitHub CLI can access `AI4SciComp` repositories;
- clear remediation suggestions for failed checks.

Diagnostics must distinguish:

- Git SSH authentication;
- GitHub CLI API authentication;
- organization repository permissions.

Do not claim that Git and `gh` use the same credential for every operation. Git
SSH transport uses the SSH key; GitHub CLI API operations use its stored token.

## 12. CMake workflow

The tool must not duplicate project-specific CMake options. Each C++ repository
owns its committed `CMakePresets.json`.

Implement thin wrappers:

```bash
asc configure asc-cpp --preset dev
asc build asc-cpp --preset dev
asc test asc-cpp --preset dev
```

They should execute, from the repository root:

```bash
cmake --preset dev
cmake --build --preset dev --parallel
ctest --preset dev --output-on-failure
```

Validate that the selected local repository exists. Give a useful error if the
required preset or executable is absent. Stream build and test output. Propagate
the underlying failure through the CLI exit status.

Do not introduce a shared build directory outside repositories in version 0.1.

## 13. Safety requirements

These are hard requirements:

- Never call `rm -rf`, `git reset --hard`, `git clean`, force push, or automatic
  stash.
- Never modify a repository merely to inspect it.
- Never operate outside the resolved workspace.
- Never interpret repository names as shell syntax.
- Never silently ignore a failed repository operation.
- Never print access tokens, complete environments, credentials, or private keys.
- Never automatically commit or push generated changes.
- Treat an existing non-Git directory as user-owned data and leave it untouched.
- Prefer refusal with a clear explanation over guessing how to resolve dirty or
  divergent Git state.

## 14. Package and repository layout

Use a coherent structure similar to:

```text
asc-devtools/
├── pyproject.toml
├── README.md
├── LICENSE
├── CHANGELOG.md
├── CONTRIBUTING.md
├── CITATION.cff
├── config.example.toml
├── src/
│   └── asc_devtools/
│       ├── __init__.py
│       ├── __main__.py
│       ├── cli.py
│       ├── config.py
│       ├── process.py
│       ├── github.py
│       ├── repositories.py
│       └── commands/
│           ├── __init__.py
│           ├── doctor.py
│           ├── repo.py
│           └── cmake.py
├── tests/
├── docs/
│   ├── architecture.md
│   ├── commands.md
│   └── installation.md
├── completions/
│   └── asc.bash
└── .github/
    └── workflows/
        └── ci.yml
```

This layout is guidance, not an instruction to create meaningless one-function
modules. Adjust it if the current repository is simpler, but keep CLI parsing,
external process execution, configuration, GitHub discovery, and local Git logic
separated and independently testable.

## 15. Installation

Support a standard installation from the repository:

```bash
python3 -m pip install --user .
```

Also document `pipx` as the preferred isolated installation when available:

```bash
pipx install .
```

For active development:

```bash
python3 -m venv .venv
source .venv/bin/activate
python -m pip install --editable .
```

Do not require users to paste a large function into `.bashrc`. A small PATH or
completion initialization line is acceptable.

Provide Bash completion for commands, repository names when feasible, and common
preset names. Completion must fail quietly when the workspace is missing.

## 16. Documentation

Write a complete `README.md` that includes:

- project purpose and scope;
- distinction between `escapetiger` and `AI4SciComp`;
- requirements;
- WSL2 installation;
- GitHub CLI authentication;
- configuration;
- quick start;
- command examples;
- safety guarantees;
- development and testing;
- relationship with `.github`, future `asc-cmake`, and other repositories.

Write a command reference that documents exact behavior and exit semantics.
Write an architecture document explaining module boundaries and the safety model.
Do not document functionality that is not implemented.

Use Apache-2.0 only if that matches the existing repository license decision. If
the existing repository already has another license, preserve it unless the user
explicitly authorizes relicensing. Do not silently replace an existing license.

## 17. Testing requirements

Use standard-library `unittest` or the repository's established testing framework.
Tests must not access the real GitHub organization, mutate the developer's actual
repositories, or depend on network availability.

At minimum, test:

### Configuration

- defaults;
- TOML parsing;
- environment overrides;
- explicit configuration selection;
- `~` expansion;
- invalid values.

### Process runner

- success;
- nonzero exit;
- missing executable;
- captured output;
- streamed output behavior where testable.

### GitHub discovery

- prefix filtering;
- `.github` inclusion/exclusion;
- archived repository filtering;
- deterministic sorting;
- malformed JSON or failed `gh` command.

### Local repositories

- direct-child discovery;
- exclusion of unrelated and nested repositories;
- path traversal rejection;
- clean and dirty status;
- detached HEAD;
- missing upstream;

### Clone

- missing repository clone;
- existing Git repository skip;
- existing non-Git destination refusal;
- requested-name validation;
- partial failure exit status.

### Sync

- clean fast-forward update;
- dirty repository skip;
- divergence failure;
- continued processing after one failure;
- accurate final summary.

### CMake commands

- correct command and working directory;
- default and explicit presets;
- missing repository;
- propagated command failure.

### CLI

- help and version;
- command routing;
- useful user-facing errors without tracebacks;
- exit codes.

Use mocks for GitHub CLI and external tools. Use temporary real Git repositories
for behavior that is safer and clearer to test with Git itself. Configure a local
test identity inside temporary repositories; do not modify the user's global Git
configuration.

## 18. Continuous integration and quality

Create GitHub Actions CI for supported Python versions on Ubuntu. macOS may be
included if it adds little maintenance cost. WSL2 itself is not a GitHub-hosted
runner, so Ubuntu CI should exercise portable behavior while WSL-specific setup is
documented and manually smoke-tested.

CI must at least:

1. check out the repository;
2. set up Python 3.11 and 3.12;
3. install the package;
4. run unit tests;
5. execute `asc --version` and `asc --help` smoke tests;
6. run the chosen formatter/linter/type checker if configured.

If adding Ruff or mypy as development dependencies, define them in an optional
dependency group and pin only as tightly as necessary. Runtime dependencies must
remain empty unless clearly justified.

Ensure all text files use LF line endings. Ensure executable scripts have correct
permissions. Do not commit virtual environments, build outputs, caches, coverage
files, tokens, or user configuration.

## 19. Acceptance criteria

The work is complete only when all of the following are true:

- A fresh installation exposes `asc` on the command line.
- `asc --help` and every subcommand help page are clear.
- `asc doctor` distinguishes API and SSH authentication.
- Repository discovery queries `AI4SciComp` and filters correctly.
- Cloning never overwrites existing data.
- Status operates across multiple repositories without mutation.
- Sync refuses dirty repositories and uses fast-forward-only updates.
- CMake wrappers use repository-owned presets.
- All tests pass offline.
- The package builds successfully from a clean checkout.
- CI configuration matches commands that work locally.
- Documentation matches implemented behavior.
- Git status shows only intentional project changes.
- No existing user changes were overwritten.

## 20. Verification procedure

Run an appropriate final verification sequence. It should include equivalents of:

```bash
python -m unittest discover -s tests -v
python -m compileall -q src tests
python -m build
python -m venv /tmp/asc-devtools-smoke
/tmp/asc-devtools-smoke/bin/python -m pip install .
/tmp/asc-devtools-smoke/bin/asc --version
/tmp/asc-devtools-smoke/bin/asc --help
```

If optional quality tools are configured, also run them. Test shell scripts with
`bash -n`; use ShellCheck if it is configured and available.

Do not claim a check passed unless you ran it successfully. If a tool is missing,
state that explicitly and run the strongest available alternative.

## 21. Final response

At completion, report:

1. the implemented architecture;
2. all commands provided;
3. major safety decisions;
4. files added or materially changed;
5. exact verification commands and results;
6. any remaining limitations;
7. the recommended next development milestone.

Do not include a long diary of implementation steps. Lead with the working result.
Do not push, commit, or open a pull request unless separately requested.
