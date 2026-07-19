"""Exact wrappers around repository-owned CMake and CTest presets."""

from __future__ import annotations

import re
import shlex
from collections.abc import Sequence
from pathlib import Path
from typing import TextIO

from asc_devtools.config import Config
from asc_devtools.errors import RepositoryError
from asc_devtools.process import CommandRunner
from asc_devtools.repositories import select_local_repositories

_QUOTED_PRESET = re.compile(r'"([^"\n]+)"')


def _repository(
    config: Config,
    runner: CommandRunner,
    repository_name: str,
    *,
    require_project: bool,
) -> Path:
    selected = select_local_repositories(config, [repository_name], runner)[0]
    _, repository, selection_error = selected
    if selection_error:
        raise RepositoryError(selection_error)
    if require_project and not (repository / "CMakeLists.txt").is_file():
        raise RepositoryError(f"CMakeLists.txt not found in {repository_name}")
    if not any(
        (repository / name).is_file()
        for name in ("CMakePresets.json", "CMakeUserPresets.json")
    ):
        raise RepositoryError(f"CMakePresets.json not found in {repository_name}")
    return repository


def _value(kind: str, value: str) -> str:
    if not value:
        raise RepositoryError(f"a {kind} is required")
    if value.startswith("-") or any(ord(character) < 32 for character in value):
        raise RepositoryError(f"invalid {kind} {value!r}")
    return value


def _commands(
    operation: str,
    preset: str,
    *,
    targets: Sequence[str] = (),
    label: str = "",
    output_on_failure: bool = False,
) -> tuple[list[str], list[str]]:
    preset = _value("preset", preset)
    if operation == "configure":
        return ["cmake", "--list-presets"], ["cmake", "--preset", preset]
    if operation == "build":
        command = ["cmake", "--build", "--preset", preset]
        if targets:
            command.append("--target")
            command.extend(_value("target", target) for target in targets)
        return ["cmake", "--list-presets=build"], command
    if operation == "test":
        command = ["ctest", "--preset", preset]
        if output_on_failure:
            command.append("--output-on-failure")
        if label:
            command.extend(("-L", _value("label", label)))
        return ["ctest", "--list-presets"], command
    raise RepositoryError(f"unsupported CMake operation: {operation}")


def _preset_names(output: str) -> list[str]:
    return sorted(set(_QUOTED_PRESET.findall(output)))


def run_cmake_command(
    config: Config,
    runner: CommandRunner,
    operation: str,
    repository_name: str,
    preset: str,
    *,
    targets: Sequence[str] = (),
    label: str = "",
    output_on_failure: bool = False,
) -> None:
    """Validate one explicit preset and stream the canonical command."""
    list_command, command = _commands(
        operation,
        preset,
        targets=targets,
        label=label,
        output_on_failure=output_on_failure,
    )
    repository = _repository(
        config,
        runner,
        repository_name,
        require_project=operation == "configure",
    )
    listed = runner.run(list_command, cwd=repository)
    if preset not in _preset_names(listed.stdout):
        raise RepositoryError(
            f"{operation} preset {preset!r} not found in {repository_name}"
        )
    runner.run(command, cwd=repository, capture_output=False)


def run_workflow(
    config: Config,
    runner: CommandRunner,
    repository_name: str,
    configure_preset: str,
    build_preset: str,
    test_preset: str,
    reporter: TextIO,
) -> None:
    """Run configure, build, and test, stopping on the first error."""
    _repository(config, runner, repository_name, require_project=True)
    for operation, preset in (
        ("configure", configure_preset),
        ("build", build_preset),
        ("test", test_preset),
    ):
        _, command = _commands(operation, preset)
        print(f"cmake workflow: {operation}: {shlex.join(command)}", file=reporter)
        run_cmake_command(config, runner, operation, repository_name, preset)


def list_presets(
    config: Config,
    runner: CommandRunner,
    repository_name: str,
) -> dict[str, str | list[str]]:
    """Delegate all preset interpretation to CMake and CTest."""
    repository = _repository(config, runner, repository_name, require_project=False)
    groups: dict[str, str | list[str]] = {"repository": repository_name}
    for name, command in (
        ("configure", ["cmake", "--list-presets"]),
        ("build", ["cmake", "--list-presets=build"]),
        ("test", ["ctest", "--list-presets"]),
    ):
        groups[name] = _preset_names(runner.run(command, cwd=repository).stdout)
    return groups
