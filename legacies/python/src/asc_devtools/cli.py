"""Command-line parsing and presentation for asc."""

from __future__ import annotations

import argparse
import json
import sys
from collections.abc import Sequence
from pathlib import Path
from typing import TextIO

from asc_devtools import __version__
from asc_devtools.commands.cmake import run_cmake_command
from asc_devtools.commands.doctor import run_doctor
from asc_devtools.config import Config, ConfigOverrides, load_config
from asc_devtools.errors import AscError, ProcessError, RepositoryError
from asc_devtools.github import discover_repositories
from asc_devtools.process import CommandRunner
from asc_devtools.repositories import (
    clone_repositories,
    repository_status,
    select_local_repositories,
    sync_repositories,
    validate_repository_name,
)


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="asc",
        description="Safely manage local AI4SciComp development repositories.",
    )
    parser.add_argument("--version", action="version", version=f"asc {__version__}")
    parser.add_argument("--config", metavar="PATH", help="use an alternate TOML file")
    parser.add_argument("--organization", help="override the GitHub organization")
    parser.add_argument("--workspace", help="override the local workspace")
    parser.add_argument("--repository-prefix", help="override the managed name prefix")
    parser.add_argument(
        "--clone-protocol", choices=("ssh", "https"), help="override clone transport"
    )
    parser.add_argument(
        "--include-dot-github",
        action=argparse.BooleanOptionalAction,
        default=None,
        help="include or exclude the .github repository",
    )
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("doctor", help="diagnose tools, credentials, and access")
    commands.add_parser("workspace", help="print the resolved workspace path")

    repo_parser = commands.add_parser("repo", help="discover and manage repositories")
    repo_commands = repo_parser.add_subparsers(dest="repo_command", required=True)
    list_parser = repo_commands.add_parser(
        "list", help="list organization repositories"
    )
    list_parser.add_argument("--json", action="store_true", help="emit JSON")
    for name, help_text in (
        ("clone", "clone missing organization repositories"),
        ("status", "show local repository status"),
        ("sync", "fast-forward clean local repositories"),
    ):
        child = repo_commands.add_parser(name, help=help_text)
        child.add_argument("repositories", nargs="*", metavar="REPOSITORY")

    for name, help_text in (
        ("configure", "configure a repository with a CMake preset"),
        ("build", "build a repository with a CMake preset"),
        ("test", "test a repository with a CTest preset"),
    ):
        child = commands.add_parser(name, help=help_text)
        child.add_argument("repository", metavar="REPOSITORY")
        child.add_argument("--preset", default="dev", help="preset name (default: dev)")
    return parser


def _config(arguments: argparse.Namespace) -> Config:
    return load_config(
        arguments.config,
        overrides=ConfigOverrides(
            organization=arguments.organization,
            workspace=arguments.workspace,
            repository_prefix=arguments.repository_prefix,
            include_dot_github=arguments.include_dot_github,
            clone_protocol=arguments.clone_protocol,
        ),
    )


def _repo_list(
    config: Config, runner: CommandRunner, json_output: bool, output: TextIO
) -> int:
    repositories = discover_repositories(config, runner)
    if json_output:
        print(
            json.dumps(
                [
                    {
                        "name": repository.name,
                        "url": repository.url,
                        "description": repository.description,
                    }
                    for repository in repositories
                ],
                indent=2,
            ),
            file=output,
        )
        return 0
    if not repositories:
        print("No matching repositories found.", file=output)
        return 0
    width = max(len(repository.name) for repository in repositories)
    print(f"{'REPOSITORY':<{width}}  DESCRIPTION", file=output)
    for repository in repositories:
        print(f"{repository.name:<{width}}  {repository.description}", file=output)
    return 0


def _repo_clone(
    config: Config,
    runner: CommandRunner,
    names: Sequence[str],
    output: TextIO,
) -> int:
    remotes = discover_repositories(config, runner)
    results = clone_repositories(config, remotes, names, runner)
    for result in results:
        detail = f": {result.detail}" if result.detail else ""
        print(f"{result.name}: {result.outcome}{detail}", file=output)
    return 1 if any(result.outcome == "failed" for result in results) else 0


def _requested_paths(
    config: Config, names: Sequence[str], output: TextIO
) -> tuple[tuple[Path, ...], bool]:
    if not names:
        return select_local_repositories(config, []), False
    paths: list[Path] = []
    failed = False
    for name in names:
        try:
            validate_repository_name(name, config)
            path = config.workspace / name
            if not (path.is_dir() and (path / ".git").exists()):
                raise RepositoryError(f"local Git repository not found: {name}")
            paths.append(path)
        except RepositoryError as error:
            failed = True
            print(f"{name}: failed: {error}", file=output)
    return tuple(paths), failed


def _repo_status(
    config: Config,
    runner: CommandRunner,
    names: Sequence[str],
    output: TextIO,
) -> int:
    paths, failed = _requested_paths(config, names, output)
    if not paths and not failed:
        print("No local managed repositories found.", file=output)
    for path in paths:
        try:
            status = repository_status(path, runner)
            tracking = "no upstream"
            if status.upstream:
                tracking = f"{status.upstream} (+{status.ahead}/-{status.behind})"
            state = (
                "clean" if status.clean else f"dirty ({len(status.changes)} changes)"
            )
            print(f"{status.name}: {status.branch}; {state}; {tracking}", file=output)
            for change in status.changes:
                print(f"  {change.code:2} {change.path}", file=output)
        except ProcessError as error:
            failed = True
            print(f"{path.name}: failed: {error}", file=output)
    return 1 if failed else 0


def _repo_sync(
    config: Config,
    runner: CommandRunner,
    names: Sequence[str],
    output: TextIO,
) -> int:
    paths, selection_failed = _requested_paths(config, names, output)
    results = sync_repositories(paths, runner)
    for result in results:
        detail = f": {result.detail}" if result.detail else ""
        print(f"{result.name}: {result.outcome}{detail}", file=output)
    counts = {
        outcome: sum(result.outcome == outcome for result in results)
        for outcome in ("updated", "unchanged", "skipped", "failed")
    }
    print(
        "Summary: " + ", ".join(f"{key}={value}" for key, value in counts.items()),
        file=output,
    )
    unsuccessful = selection_failed or bool(counts["skipped"] or counts["failed"])
    return 1 if unsuccessful else 0


def main(
    argv: Sequence[str] | None = None,
    *,
    output: TextIO | None = None,
    error_output: TextIO | None = None,
    runner: CommandRunner | None = None,
) -> int:
    """Run the asc CLI and return a process exit status."""
    output = output or sys.stdout
    error_output = error_output or sys.stderr
    runner = runner or CommandRunner()
    try:
        arguments = _parser().parse_args(argv)
        config = _config(arguments)
        if arguments.command == "doctor":
            return run_doctor(config, runner, output)
        if arguments.command == "workspace":
            print(config.workspace, file=output)
            return 0
        if arguments.command == "repo":
            if arguments.repo_command == "list":
                return _repo_list(config, runner, arguments.json, output)
            if arguments.repo_command == "clone":
                return _repo_clone(config, runner, arguments.repositories, output)
            if arguments.repo_command == "status":
                return _repo_status(config, runner, arguments.repositories, output)
            return _repo_sync(config, runner, arguments.repositories, output)
        run_cmake_command(
            config,
            runner,
            arguments.command,
            arguments.repository,
            arguments.preset,
        )
        return 0
    except AscError as error:
        print(f"asc: error: {error}", file=error_output)
        return 1
    except KeyboardInterrupt:
        print("asc: interrupted", file=error_output)
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
