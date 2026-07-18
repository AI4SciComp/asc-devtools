# Installation

## Isolated installation

Python 3.11 or newer is required. From a local checkout:

```bash
pipx install .
```

Ensure the pipx application directory is on `PATH`:

```bash
pipx ensurepath
```

For a user-site installation instead:

```bash
python3 -m pip install --user .
```

The active Python user binary directory must then be on `PATH`.

## Development installation

```bash
python3 -m venv .venv
source .venv/bin/activate
python -m pip install --editable '.[dev]'
```

## GitHub authentication

Authenticate the GitHub CLI token used for API discovery:

```bash
gh auth login
gh auth status
```

For the default SSH clone protocol, also verify that a public SSH key is added to
the GitHub account and that GitHub accepts it:

```bash
ssh -T git@github.com
```

GitHub's successful SSH greeting normally exits nonzero because it does not
provide shell access. `asc doctor` recognizes the greeting rather than relying
only on that exit code.

## Bash completion

Source the completion shipped by the checkout:

```bash
source /path/to/asc-devtools/completions/asc.bash
```

To load it in future shells, place that single source command in `~/.bashrc`.
Completion provides command names, local managed repository names, protocols,
and preset names read from local `CMakePresets.json`. It produces no errors when
the workspace is absent.

## Initial verification

```bash
asc --version
asc --help
asc doctor
```

Resolve any `FAIL` entries before cloning or building repositories.

