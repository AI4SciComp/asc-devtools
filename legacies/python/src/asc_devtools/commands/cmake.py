"""Thin wrappers around repository-owned CMake presets."""

from __future__ import annotations

import json
from pathlib import Path

from asc_devtools.config import Config
from asc_devtools.errors import RepositoryError
from asc_devtools.process import CommandRunner
from asc_devtools.repositories import select_local_repositories

_PRESET_SECTION = {
    "configure": "configurePresets",
    "build": "buildPresets",
    "test": "testPresets",
}


def _read_presets(repository: Path) -> dict[str, object]:
    preset_path = repository / "CMakePresets.json"
    if not preset_path.is_file():
        raise RepositoryError(f"CMakePresets.json not found in {repository.name}")
    try:
        with preset_path.open(encoding="utf-8") as preset_file:
            document = json.load(preset_file)
    except (OSError, json.JSONDecodeError) as error:
        raise RepositoryError(
            f"could not read {repository.name}/CMakePresets.json: {error}"
        ) from error
    if not isinstance(document, dict):
        raise RepositoryError(
            f"{repository.name}/CMakePresets.json must contain a JSON object"
        )
    return document


def _require_preset(repository: Path, operation: str, preset: str) -> None:
    document = _read_presets(repository)
    records = document.get(_PRESET_SECTION[operation], [])
    if not isinstance(records, list) or not any(
        isinstance(record, dict) and record.get("name") == preset for record in records
    ):
        raise RepositoryError(
            f"{operation} preset {preset!r} not found in "
            f"{repository.name}/CMakePresets.json"
        )


def run_cmake_command(
    config: Config,
    runner: CommandRunner,
    operation: str,
    repository_name: str,
    preset: str,
) -> None:
    """Validate and stream one CMake preset operation."""
    repository = select_local_repositories(config, [repository_name])[0]
    _require_preset(repository, operation, preset)
    if operation == "configure":
        arguments = ["cmake", "--preset", preset]
    elif operation == "build":
        arguments = ["cmake", "--build", "--preset", preset, "--parallel"]
    else:
        arguments = ["ctest", "--preset", preset, "--output-on-failure"]
    runner.run(arguments, cwd=repository, capture_output=False)
