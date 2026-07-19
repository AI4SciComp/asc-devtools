"""Canonical asc command parser and presentation."""

from __future__ import annotations

import argparse
import json
import sys
from collections import Counter
from collections.abc import Callable, Sequence
from dataclasses import replace
from typing import NoReturn, TextIO

from asc_devtools import __version__
from asc_devtools.commands.cmake import run_cmake_command
from asc_devtools.commands.doctor import run_doctor
from asc_devtools.completion import BASH_COMPLETION
from asc_devtools.config import Config, ConfigOverrides, load_config
from asc_devtools.errors import AscError, ProcessError
from asc_devtools.github import (
    GitHubClient,
    discover_repositories,
    token_with_optional_gh,
)
from asc_devtools.process import CommandRunner
from asc_devtools.repositories import (
    OperationResult,
    clone_repositories,
    inspect_repositories,
    operations_failed,
    sync_repositories,
)

GLOBAL_USAGE = """Usage: asc [GLOBAL OPTIONS] COMMAND [ARGS]

Safely manage AI4SciComp repositories in one local workspace.

Commands:
  doctor [--json]                 Diagnose tools and GitHub access
  workspace                       Print the resolved workspace
  repo list [--json]              List organization repositories
  repo clone [NAME...]            Clone missing repositories
  repo status [NAME...] [--json]  Inspect local repositories
  repo sync [NAME...] [--dry-run] Fast-forward clean repositories
  configure NAME --preset PRESET  Configure a CMake preset
  build NAME --preset PRESET      Build a CMake preset
  test NAME --preset PRESET       Run a CTest preset
  completion bash                 Print Bash completion

Global options (must precede COMMAND):
  --config PATH
  --organization NAME
  --workspace PATH
  --no-color
  -h, --help
  --version
"""

USAGE = {
    "doctor": "Usage: asc doctor [--json]\n",
    "workspace": "Usage: asc workspace\n",
    "repo": "Usage: asc repo {list|clone|status|sync} [OPTIONS] [REPOSITORY...]\n",
    "repo list": "Usage: asc repo list [--json]\n",
    "repo clone": "Usage: asc repo clone [REPOSITORY...] [--protocol ssh|https]\n",
    "repo status": "Usage: asc repo status [REPOSITORY...] [--json]\n",
    "repo sync": "Usage: asc repo sync [REPOSITORY...] [--dry-run]\n",
    "completion": "Usage: asc completion bash\n",
    "configure": "Usage: asc configure REPOSITORY --preset PRESET\n",
    "build": "Usage: asc build REPOSITORY --preset PRESET\n",
    "test": "Usage: asc test REPOSITORY --preset PRESET\n",
}


class UsageError(Exception):
    pass


class Parser(argparse.ArgumentParser):
    def error(self, message: str) -> NoReturn:
        raise UsageError(message)


def _help_flag(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("-h", "--help", action="store_true")


def _parser() -> Parser:
    parser = Parser(prog="asc", add_help=False)
    parser.add_argument("--config", metavar="PATH")
    parser.add_argument("--organization")
    parser.add_argument("--workspace")
    parser.add_argument("--no-color", action="store_true")
    parser.add_argument("--version", action="store_true")
    _help_flag(parser)
    commands = parser.add_subparsers(dest="command")

    doctor = commands.add_parser("doctor", add_help=False)
    doctor.add_argument("--json", action="store_true")
    _help_flag(doctor)
    workspace = commands.add_parser("workspace", add_help=False)
    _help_flag(workspace)

    repo = commands.add_parser("repo", add_help=False)
    _help_flag(repo)
    repo_commands = repo.add_subparsers(dest="repo_command")
    repo_list = repo_commands.add_parser("list", add_help=False)
    repo_list.add_argument("--json", action="store_true")
    _help_flag(repo_list)
    clone = repo_commands.add_parser("clone", add_help=False)
    clone.add_argument("repositories", nargs="*")
    clone.add_argument("--protocol", choices=("ssh", "https"))
    _help_flag(clone)
    status = repo_commands.add_parser("status", add_help=False)
    status.add_argument("repositories", nargs="*")
    status.add_argument("--json", action="store_true")
    _help_flag(status)
    sync = repo_commands.add_parser("sync", add_help=False)
    sync.add_argument("repositories", nargs="*")
    sync.add_argument("--dry-run", action="store_true")
    _help_flag(sync)

    for name in ("configure", "build", "test"):
        child = commands.add_parser(name, add_help=False)
        child.add_argument("repository", nargs="?")
        child.add_argument("--preset")
        _help_flag(child)
    completion = commands.add_parser("completion", add_help=False)
    completion.add_argument("shell", nargs="?")
    _help_flag(completion)
    return parser


def _requested_help(arguments: argparse.Namespace) -> str | None:
    if arguments.help:
        if arguments.command is None:
            return GLOBAL_USAGE
        if arguments.command == "repo" and arguments.repo_command:
            return USAGE[f"repo {arguments.repo_command}"]
        return USAGE[arguments.command]
    return None


def _resolved_client(
    config: Config,
    runner: CommandRunner,
    factory: Callable[[str], GitHubClient] | None,
) -> tuple[Config, GitHubClient]:
    token, source = token_with_optional_gh(config, runner)
    if token and not config.github_token:
        config = replace(config, github_token=token, github_token_source=source)
    return config, (factory(token) if factory else GitHubClient(token))


def _print_operations(results: Sequence[OperationResult], output: TextIO) -> None:
    for result in results:
        detail = f": {result.detail}" if result.detail else ""
        print(f"{result.name}: {result.outcome}{detail}", file=output)
        for command in result.plan:
            print(f"  {command}", file=output)
    counts = Counter(result.outcome for result in results)
    print(
        "Summary:" + "".join(f" {name}={counts[name]}" for name in sorted(counts)),
        file=output,
    )


def main(
    argv: Sequence[str] | None = None,
    *,
    output: TextIO | None = None,
    error_output: TextIO | None = None,
    runner: CommandRunner | None = None,
    github_client_factory: Callable[[str], GitHubClient] | None = None,
) -> int:
    """Run asc and return its documented exit code."""
    output = output or sys.stdout
    error_output = error_output or sys.stderr
    runner = runner or CommandRunner()
    raw_arguments = list(sys.argv[1:] if argv is None else argv)
    try:
        arguments = _parser().parse_args(raw_arguments)
        help_text = _requested_help(arguments)
        if help_text:
            print(help_text, end="", file=output)
            return 0
        if arguments.version:
            print(f"asc {__version__}", file=output)
            return 0
        if arguments.command is None:
            raise UsageError("a command is required")
        if arguments.command == "repo" and arguments.repo_command is None:
            raise UsageError("a repo command is required")
        config = load_config(
            arguments.config,
            overrides=ConfigOverrides(
                organization=arguments.organization,
                workspace=arguments.workspace,
            ),
        )
        if arguments.command == "workspace":
            print(config.workspace, file=output)
            return 0
        if arguments.command == "completion":
            if arguments.shell != "bash":
                raise UsageError("only Bash completion is supported")
            print(BASH_COMPLETION, end="", file=output)
            return 0
        if arguments.command == "doctor":
            config, client = _resolved_client(config, runner, github_client_factory)
            return run_doctor(
                config,
                runner,
                output,
                json_output=arguments.json,
                client=client,
            )
        if arguments.command == "repo":
            if arguments.repo_command in {"list", "clone"}:
                config, client = _resolved_client(config, runner, github_client_factory)
                repositories = discover_repositories(config, runner, client=client)
                if arguments.repo_command == "list":
                    if arguments.json:
                        json.dump(
                            [repository.as_json() for repository in repositories],
                            output,
                            indent=2,
                        )
                        print(file=output)
                    else:
                        print("REPOSITORY\tVISIBILITY\tDEFAULT BRANCH", file=output)
                        for repository in repositories:
                            visibility = "private" if repository.private else "public"
                            print(
                                f"{repository.name}\t{visibility}\t{repository.default_branch}",
                                file=output,
                            )
                    return 0
                results = clone_repositories(
                    config,
                    repositories,
                    arguments.repositories,
                    arguments.protocol or config.clone_protocol,
                    runner,
                )
                _print_operations(results, output)
                return 1 if operations_failed(results) else 0
            if arguments.repo_command == "status":
                statuses = inspect_repositories(config, arguments.repositories, runner)
                if arguments.json:
                    json.dump(
                        [status.as_json() for status in statuses], output, indent=2
                    )
                    print(file=output)
                else:
                    print(
                        "REPOSITORY\tBRANCH\tSTATE\tUPSTREAM\tAHEAD/BEHIND", file=output
                    )
                    for status in statuses:
                        if status.error:
                            print(f"{status.name}\tERROR\t{status.error}", file=output)
                            continue
                        state = "clean" if status.clean else f"dirty({status.changes})"
                        branch = "detached HEAD" if status.detached else status.branch
                        identity = (
                            f"{status.name}\t{branch}\t{state}"
                            f"\t{status.upstream or '-'}"
                        )
                        print(
                            f"{identity}\t+{status.ahead}/-{status.behind}",
                            file=output,
                        )
                return 1 if any(status.error for status in statuses) else 0
            results = sync_repositories(
                config,
                arguments.repositories,
                runner,
                dry_run=arguments.dry_run,
            )
            _print_operations(results, output)
            return 1 if operations_failed(results) else 0
        if not arguments.repository or not arguments.preset:
            raise UsageError("repository and --preset are required")
        run_cmake_command(
            config,
            runner,
            arguments.command,
            arguments.repository,
            arguments.preset,
        )
        return 0
    except UsageError as error:
        print(f"asc: error: {error}\n", file=error_output)
        print(GLOBAL_USAGE, end="", file=error_output)
        return 2
    except ProcessError as error:
        print(f"asc: error: {error}", file=error_output)
        if raw_arguments and raw_arguments[0] in {"configure", "build", "test"}:
            return error.return_code if 1 <= error.return_code <= 125 else 1
        return 1
    except AscError as error:
        print(f"asc: error: {error}", file=error_output)
        return 1
    except KeyboardInterrupt:
        print("asc: interrupted", file=error_output)
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
