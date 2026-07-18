"""Environment diagnostics for asc."""

from __future__ import annotations

import json
import os
import sys
from pathlib import Path
from typing import TextIO

from asc_devtools.config import Config
from asc_devtools.errors import ProcessError
from asc_devtools.process import CommandRunner


def _line(output: TextIO, ok: bool, label: str, detail: str) -> None:
    marker = "OK" if ok else "FAIL"
    print(f"[{marker}] {label}: {detail}", file=output)


def _tool_version(
    runner: CommandRunner, output: TextIO, name: str, arguments: list[str]
) -> bool:
    try:
        result = runner.run(arguments)
        detail = (result.stdout or result.stderr).splitlines()[0]
        _line(output, True, name, detail)
        return True
    except (ProcessError, IndexError) as error:
        _line(output, False, name, f"{error}; install {name}")
        return False


def _writable_location(path: Path) -> bool:
    candidate = path
    while not candidate.exists() and candidate != candidate.parent:
        candidate = candidate.parent
    return candidate.is_dir() and os.access(candidate, os.W_OK)


def run_doctor(config: Config, runner: CommandRunner, output: TextIO) -> int:
    """Print diagnostics and return nonzero when a required check fails."""
    failures = 0
    print(f"Configuration: {config.config_path}", file=output)
    print(f"Organization:  {config.organization}", file=output)
    print(f"Workspace:     {config.workspace}", file=output)
    print(f"Clone protocol: {config.clone_protocol}", file=output)

    python_ok = sys.version_info >= (3, 11)
    _line(output, python_ok, "Python", sys.version.split()[0])
    failures += not python_ok
    for name, arguments in (
        ("Git", ["git", "--version"]),
        ("GitHub CLI", ["gh", "--version"]),
        ("CMake", ["cmake", "--version"]),
        ("CTest", ["ctest", "--version"]),
    ):
        failures += not _tool_version(runner, output, name, arguments)

    workspace_exists = config.workspace.is_dir()
    workspace_writable = _writable_location(config.workspace)
    detail = (
        "exists and is writable"
        if workspace_exists and workspace_writable
        else (
            "does not exist; parent is writable"
            if workspace_writable
            else "not writable; check directory ownership and permissions"
        )
    )
    _line(output, workspace_writable, "Workspace", detail)
    failures += not workspace_writable

    try:
        user_result = runner.run(["gh", "api", "user"])
        user_document = json.loads(user_result.stdout)
        username = (
            user_document.get("login") if isinstance(user_document, dict) else None
        )
        if not isinstance(username, str) or not username:
            raise ValueError("response did not contain a login")
        _line(output, True, "GitHub API authentication", f"authenticated as {username}")
    except (ProcessError, json.JSONDecodeError, ValueError) as error:
        failures += 1
        _line(
            output,
            False,
            "GitHub API authentication",
            f"{error}; run 'gh auth login'",
        )

    try:
        protocol = runner.run(["gh", "config", "get", "git_protocol"]).stdout.strip()
        if protocol not in {"ssh", "https"}:
            raise ValueError("GitHub CLI returned no recognized protocol")
        matches = protocol == config.clone_protocol
        detail = (
            protocol
            if matches
            else (f"{protocol}; asc is configured for {config.clone_protocol}")
        )
        _line(output, matches, "GitHub CLI Git protocol", detail)
        failures += not matches
    except (ProcessError, ValueError) as error:
        failures += 1
        _line(output, False, "GitHub CLI Git protocol", str(error))

    try:
        runner.run(
            [
                "gh",
                "repo",
                "list",
                config.organization,
                "--limit",
                "1",
                "--json",
                "name",
            ]
        )
        _line(output, True, "Organization access", f"can query {config.organization}")
    except ProcessError as error:
        failures += 1
        _line(
            output,
            False,
            "Organization access",
            f"cannot query {config.organization}: {error}",
        )

    if config.clone_protocol == "ssh":
        try:
            result = runner.run(
                [
                    "ssh",
                    "-T",
                    "-o",
                    "BatchMode=yes",
                    "-o",
                    "ConnectTimeout=5",
                    "git@github.com",
                ],
                check=False,
            )
            message = (result.stderr or result.stdout).strip()
            authenticated = "successfully authenticated" in message.lower()
            _line(
                output,
                authenticated,
                "Git SSH authentication",
                message or "no response; add an SSH key to GitHub",
            )
            failures += not authenticated
        except ProcessError as error:
            failures += 1
            _line(output, False, "Git SSH authentication", str(error))
    else:
        _line(
            output,
            True,
            "Git SSH authentication",
            "not used because clone_protocol is https",
        )

    return 1 if failures else 0
