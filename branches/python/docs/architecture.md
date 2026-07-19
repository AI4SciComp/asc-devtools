# Architecture

- `config.py` strictly loads the flat JSON schema and resolves canonical
  precedence.
- `process.py` is the sole subprocess boundary and always uses argument arrays.
- `github.py` owns bounded direct REST requests, pagination, filtering, and the
  optional `gh auth token` fallback.
- `repositories.py` enforces direct-child containment, remote identity,
  porcelain-v2 inspection, and fast-forward-only synchronization.
- `commands/cmake.py` validates explicit CMake/CTest presets and streams the
  exact canonical commands.
- `commands/doctor.py` returns stable pass/warning/failure checks.
- `cli.py` owns routing, JSON schemas, partial-failure aggregation, and exit
  codes.
- `scripts/manage_install.py` installs and removes only hash-verified files.

Runtime modules use only the Python standard library. Tests inject process and
HTTP boundaries and use temporary real Git repositories. No command evaluates
user input through a shell or performs history-rewriting Git operations.
