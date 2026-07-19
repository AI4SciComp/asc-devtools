"""Exact wrappers around repository-owned CMake and CTest presets."""

from __future__ import annotations

import re

from asc_devtools.config import Config
from asc_devtools.errors import RepositoryError
from asc_devtools.process import CommandRunner
from asc_devtools.repositories import select_local_repositories

_PRESET_PATTERN = re.compile(r"^[A-Za-z0-9._-]+$")


def run_cmake_command(
    config: Config,
    runner: CommandRunner,
    operation: str,
    repository_name: str,
    preset: str,
) -> None:
    """Validate one explicit preset and stream the canonical command."""
    if not preset:
        raise RepositoryError("a preset is required; use --preset NAME")
    if preset.startswith("-") or not _PRESET_PATTERN.fullmatch(preset):
        raise RepositoryError(f"invalid preset name {preset!r}")
    selected = select_local_repositories(config, [repository_name], runner)[0]
    _, repository, selection_error = selected
    if selection_error:
        raise RepositoryError(selection_error)
    if not any(
        (repository / name).is_file()
        for name in ("CMakePresets.json", "CMakeUserPresets.json")
    ):
        raise RepositoryError(
            f"CMakePresets.json or CMakeUserPresets.json not found in {repository_name}"
        )
    listed = runner.run(["cmake", f"--list-presets={operation}"], cwd=repository)
    if f'"{preset}"' not in listed.stdout:
        raise RepositoryError(
            f"{operation} preset {preset!r} not found in {repository_name}"
        )
    if operation == "configure":
        arguments = ["cmake", "--preset", preset]
    elif operation == "build":
        arguments = ["cmake", "--build", "--preset", preset]
    elif operation == "test":
        arguments = ["ctest", "--preset", preset]
    else:
        raise RepositoryError(f"unsupported CMake operation: {operation}")
    runner.run(arguments, cwd=repository, capture_output=False)
