# Contributing

## Development setup

Use Python 3.11 or newer:

```bash
python3 -m venv .venv
source .venv/bin/activate
python -m pip install --editable '.[dev]'
```

Run the same checks used by CI:

```bash
python -m unittest discover -s tests -v
python -m compileall -q src tests scripts
scripts/package_update.sh --output /tmp/asc-python-release-test
ruff check src tests scripts/manage_install.py
ruff format --check src tests scripts/manage_install.py
mypy src
asc --version
asc --help
```

Tests must be offline and must not inspect or modify a developer's real GitHub
organization or workspace. Use temporary repositories and mocked command results.
Vendor tests use synthetic temporary `asc-cmake` sources and must never inspect
or modify a real sibling checkout.
Self-update tests must use fixture transports and temporary managed prefixes.
Keep external process calls behind `CommandRunner` and pass argument arrays.
Commit and push are allowed only inside the reviewed `repo save` transaction; do
not add Git operations that discard or rewrite local work.

Submit focused changes with tests and corresponding documentation. By
contributing, you agree that your contribution is licensed under Apache-2.0.
