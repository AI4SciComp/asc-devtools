"""Canonical asc command parser and presentation."""

from __future__ import annotations

import argparse
import json
import sys
from collections import Counter
from collections.abc import Callable, Sequence
from datetime import datetime
from typing import NoReturn, TextIO

from asc_devtools import __version__
from asc_devtools.commands.cmake import list_presets, run_cmake_command, run_workflow
from asc_devtools.commands.doctor import run_doctor
from asc_devtools.completion import BASH_COMPLETION
from asc_devtools.config import Config, ConfigOverrides, load_config
from asc_devtools.errors import AscError, ProcessError
from asc_devtools.github import GitHubClient, discover_repositories
from asc_devtools.process import CommandRunner
from asc_devtools.repositories import (
    OperationResult,
    SavePlan,
    apply_save_repository,
    clone_repositories,
    inspect_repositories,
    operations_failed,
    plan_save_repository,
    sync_repositories,
)
from asc_devtools.selfupdate import (
    Transport as UpdateTransport,
    default_transport as default_update_transport,
    run_update,
)
from asc_devtools.vendor import VendorPlan, vendor_apply, vendor_plan, vendor_status

GLOBAL_USAGE = """Usage: asc [GLOBAL OPTIONS] COMMAND [ARGS]

Safely manage AI4SciComp repositories in one local workspace.

Commands:
  doctor [--json]                 Diagnose tools and GitHub access
  workspace                       Print the resolved workspace
  repo list [--json]              List organization repositories
  repo clone [NAME...]            Clone missing repositories
  repo status [NAME...] [--json]  Inspect local repositories
  repo sync [NAME...] [--dry-run] Download remote fast-forwards into clean repositories
  repo save NAME [--message TEXT] Commit local changes and push the tracked branch
  configure NAME --preset PRESET  Configure a CMake preset
  build NAME --preset PRESET      Build a CMake preset
  test NAME --preset PRESET       Run a CTest preset
  cmake COMMAND ...               CMake workflows and local vendoring
  update [--check] [--yes]        Update asc to the latest release
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
    "repo": "Usage: asc repo {list|clone|status|sync|save} [OPTIONS] [REPOSITORY...]\n",
    "repo list": "Usage: asc repo list [--json]\n",
    "repo clone": "Usage: asc repo clone [REPOSITORY...] [--protocol ssh|https]\n",
    "repo status": "Usage: asc repo status [REPOSITORY...] [--json]\n",
    "repo sync": "Usage: asc repo sync [REPOSITORY...] [--dry-run]\n",
    "repo save": (
        "Usage: asc repo save REPOSITORY [--message TEXT] [--dry-run] [--yes]\n"
    ),
    "update": "Usage: asc update [--check] [--yes] [--prefix PATH]\n",
    "completion": "Usage: asc completion bash\n",
    "configure": "Usage: asc configure REPOSITORY --preset PRESET\n",
    "build": "Usage: asc build REPOSITORY --preset PRESET\n",
    "test": "Usage: asc test REPOSITORY --preset PRESET\n",
    "cmake": "Usage: asc cmake {configure|build|test|workflow|presets|vendor} ...\n",
    "cmake configure": "Usage: asc cmake configure REPOSITORY --preset PRESET\n",
    "cmake build": "Usage: asc cmake build REPOSITORY --preset PRESET [--target TARGET]...\n",
    "cmake test": "Usage: asc cmake test REPOSITORY --preset PRESET [--label LABEL] [--output-on-failure]\n",
    "cmake workflow": "Usage: asc cmake workflow REPOSITORY --configure-preset PRESET --build-preset PRESET --test-preset PRESET\n",
    "cmake presets": "Usage: asc cmake presets REPOSITORY [--json]\n",
    "cmake vendor": "Usage: asc cmake vendor {status|plan|apply} ...\n",
    "cmake vendor status": "Usage: asc cmake vendor status REPOSITORY [--json]\n",
    "cmake vendor plan": "Usage: asc cmake vendor plan REPOSITORY [--source PATH] [--ref REF] [--json]\n",
    "cmake vendor apply": "Usage: asc cmake vendor apply REPOSITORY [--source PATH] [--ref REF] [--yes]\n",
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
    save = repo_commands.add_parser("save", add_help=False)
    save.add_argument("repository", nargs="?")
    save.add_argument("--message", default="")
    save.add_argument("--dry-run", action="store_true")
    save.add_argument("--yes", action="store_true")
    _help_flag(save)

    update = commands.add_parser("update", add_help=False)
    update.add_argument("--check", action="store_true")
    update.add_argument("--yes", action="store_true")
    update.add_argument("--prefix", default="")
    _help_flag(update)

    for name in ("configure", "build", "test"):
        child = commands.add_parser(name, add_help=False)
        child.add_argument("repository", nargs="?")
        child.add_argument("--preset")
        _help_flag(child)
    cmake = commands.add_parser("cmake", add_help=False)
    _help_flag(cmake)
    cmake_commands = cmake.add_subparsers(dest="cmake_command")
    for name in ("configure", "build", "test"):
        child = cmake_commands.add_parser(name, add_help=False)
        child.add_argument("repository", nargs="?")
        child.add_argument("--preset")
        if name == "build":
            child.add_argument("--target", action="append", default=[])
        if name == "test":
            child.add_argument("--label", default="")
            child.add_argument("--output-on-failure", action="store_true")
        _help_flag(child)
    workflow = cmake_commands.add_parser("workflow", add_help=False)
    workflow.add_argument("repository", nargs="?")
    workflow.add_argument("--configure-preset")
    workflow.add_argument("--build-preset")
    workflow.add_argument("--test-preset")
    _help_flag(workflow)
    presets = cmake_commands.add_parser("presets", add_help=False)
    presets.add_argument("repository", nargs="?")
    presets.add_argument("--json", action="store_true")
    _help_flag(presets)
    vendor = cmake_commands.add_parser("vendor", add_help=False)
    _help_flag(vendor)
    vendor_commands = vendor.add_subparsers(dest="vendor_command")
    vendor_status_parser = vendor_commands.add_parser("status", add_help=False)
    vendor_status_parser.add_argument("repository", nargs="?")
    vendor_status_parser.add_argument("--json", action="store_true")
    _help_flag(vendor_status_parser)
    for name in ("plan", "apply"):
        child = vendor_commands.add_parser(name, add_help=False)
        child.add_argument("repository", nargs="?")
        child.add_argument("--source", default="")
        child.add_argument("--ref", default="")
        if name == "plan":
            child.add_argument("--json", action="store_true")
        else:
            child.add_argument("--yes", action="store_true")
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
        if arguments.command == "cmake":
            if arguments.cmake_command == "vendor" and arguments.vendor_command:
                return USAGE[f"cmake vendor {arguments.vendor_command}"]
            if arguments.cmake_command:
                return USAGE[f"cmake {arguments.cmake_command}"]
        return USAGE[arguments.command]
    return None


def _resolved_client(
    config: Config,
    factory: Callable[[str], GitHubClient] | None,
) -> tuple[Config, GitHubClient]:
    token = config.github_token
    return config, (factory(token) if factory else GitHubClient(token))


def _save_message(message: str) -> str:
    return message or datetime.now().strftime("Updated at %Y-%m-%d %H:%M:%S")


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


def _print_vendor_plan(plan: VendorPlan, output: TextIO) -> None:
    print(f"asc-cmake {plan.version} ({plan.commit}) -> {plan.repository}", file=output)
    for action in plan.actions:
        print(f"  {action.action}\t{action.path}", file=output)


def _print_save_plan(plan: SavePlan, output: TextIO) -> None:
    print(
        f"{plan.operation.name}: planned: {plan.operation.detail}", file=output
    )
    for command in plan.operation.plan:
        print(f"  {command}", file=output)


def main(
    argv: Sequence[str] | None = None,
    *,
    output: TextIO | None = None,
    error_output: TextIO | None = None,
    input_stream: TextIO | None = None,
    runner: CommandRunner | None = None,
    github_client_factory: Callable[[str], GitHubClient] | None = None,
    update_transport: UpdateTransport | None = None,
) -> int:
    """Run asc and return its documented exit code."""
    output = output or sys.stdout
    error_output = error_output or sys.stderr
    input_stream = input_stream or sys.stdin
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
        if arguments.command == "cmake" and arguments.cmake_command is None:
            raise UsageError("a cmake command is required")
        if (
            arguments.command == "cmake"
            and arguments.cmake_command == "vendor"
            and arguments.vendor_command is None
        ):
            raise UsageError("a vendor command is required")
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
            config, client = _resolved_client(config, github_client_factory)
            return run_doctor(
                config,
                runner,
                output,
                json_output=arguments.json,
                client=client,
            )
        if arguments.command == "update":
            config, client = _resolved_client(config, github_client_factory)
            run_update(
                current_version=__version__,
                token=config.github_token,
                base_url=client.base_url,
                prefix=arguments.prefix,
                check_only=arguments.check,
                assume_yes=arguments.yes,
                input_stream=input_stream,
                output=output,
                transport=update_transport or default_update_transport,
            )
            return 0
        if arguments.command == "repo":
            if arguments.repo_command in {"list", "clone"}:
                config, client = _resolved_client(config, github_client_factory)
                repositories = discover_repositories(config, client=client)
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
            if arguments.repo_command == "save":
                if not arguments.repository:
                    raise UsageError("repo save requires one repository")
                message = _save_message(arguments.message)
                save_plan = plan_save_repository(
                    config, arguments.repository, message, runner
                )
                if arguments.dry_run:
                    _print_save_plan(save_plan, output)
                    return 0
                if not arguments.yes:
                    _print_save_plan(save_plan, error_output)
                    print(
                        "Commit all listed working-tree changes and push? [y/N] ",
                        end="",
                        file=error_output,
                    )
                    if input_stream.readline().strip().lower() not in {"y", "yes"}:
                        print("asc: repo save cancelled", file=error_output)
                        return 1
                result = apply_save_repository(config, save_plan, runner)
                _print_operations((result,), output)
                return 1 if operations_failed((result,)) else 0
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
        if arguments.command == "cmake":
            if not arguments.repository:
                raise UsageError(
                    f"cmake {arguments.cmake_command} requires a repository"
                )
            if arguments.cmake_command in {"configure", "build", "test"}:
                if not arguments.preset:
                    raise UsageError("repository and --preset are required")
                run_cmake_command(
                    config,
                    runner,
                    arguments.cmake_command,
                    arguments.repository,
                    arguments.preset,
                    targets=getattr(arguments, "target", ()),
                    label=getattr(arguments, "label", ""),
                    output_on_failure=getattr(arguments, "output_on_failure", False),
                )
                return 0
            if arguments.cmake_command == "workflow":
                if not all(
                    (
                        arguments.configure_preset,
                        arguments.build_preset,
                        arguments.test_preset,
                    )
                ):
                    raise UsageError(
                        "repository and all three workflow presets are required"
                    )
                run_workflow(
                    config,
                    runner,
                    arguments.repository,
                    arguments.configure_preset,
                    arguments.build_preset,
                    arguments.test_preset,
                    error_output,
                )
                return 0
            if arguments.cmake_command == "presets":
                presets_result = list_presets(config, runner, arguments.repository)
                if arguments.json:
                    json.dump(presets_result, output, indent=2)
                    print(file=output)
                else:
                    for group in ("configure", "build", "test"):
                        for preset in presets_result[group]:
                            print(f"{group}\t{preset}", file=output)
                return 0
            if arguments.vendor_command == "status":
                status_result = vendor_status(config, runner, arguments.repository)
                if arguments.json:
                    json.dump(status_result.as_json(), output, indent=2)
                    print(file=output)
                else:
                    detail = f": {status_result.detail}" if status_result.detail else ""
                    print(
                        f"{status_result.repository}: {status_result.state}{detail}",
                        file=output,
                    )
                    for extra in status_result.extra_files:
                        print(f"  extra: {extra}", file=output)
                return 0
            plan = vendor_plan(
                config,
                runner,
                arguments.repository,
                arguments.source,
                arguments.ref,
            )
            if arguments.vendor_command == "plan":
                if arguments.json:
                    json.dump(plan.as_json(), output, indent=2)
                    print(file=output)
                else:
                    _print_vendor_plan(plan, output)
                return 0
            if not arguments.yes:
                _print_vendor_plan(plan, error_output)
                print("Apply this exact plan? [y/N] ", end="", file=error_output)
                answer = input_stream.readline().strip().lower()
                if answer not in {"y", "yes"}:
                    print("asc: vendor apply cancelled", file=error_output)
                    return 1
            vendor_apply(config, runner, plan)
            _print_vendor_plan(plan, output)
            print(
                "Applied vendored asc-cmake files. Review with: git diff -- cmake/asc",
                file=output,
            )
            return 0
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
        if raw_arguments and raw_arguments[0] in {
            "configure",
            "build",
            "test",
            "cmake",
        }:
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
