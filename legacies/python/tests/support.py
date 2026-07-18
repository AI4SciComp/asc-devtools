"""Shared test doubles and Git helpers."""

from __future__ import annotations

import subprocess
from collections.abc import Sequence
from pathlib import Path

from asc_devtools.process import CommandResult


class ScriptedRunner:
    """Runner returning scripted results or exceptions in call order."""

    def __init__(self, responses: Sequence[CommandResult | Exception]) -> None:
        self.responses = list(responses)
        self.calls: list[tuple[tuple[str, ...], Path | None, bool, bool]] = []

    def run(
        self,
        arguments: Sequence[str],
        *,
        cwd: Path | None = None,
        capture_output: bool = True,
        check: bool = True,
    ) -> CommandResult:
        self.calls.append((tuple(arguments), cwd, capture_output, check))
        if not self.responses:
            raise AssertionError(f"unexpected command: {arguments}")
        response = self.responses.pop(0)
        if isinstance(response, Exception):
            raise response
        return response


def result(stdout: str = "", stderr: str = "", code: int = 0) -> CommandResult:
    """Create a simple command result for a scripted runner."""
    return CommandResult(("command",), code, stdout, stderr)


def git(path: Path, *arguments: str) -> str:
    """Run Git in a temporary test repository."""
    completed = subprocess.run(
        ["git", *arguments],
        cwd=path,
        check=True,
        capture_output=True,
        text=True,
    )
    return completed.stdout.strip()


def initialize_repository(path: Path) -> None:
    """Initialize a repository with one commit and a local test identity."""
    path.mkdir()
    git(path, "init", "-q")
    git(path, "config", "user.name", "Asc Tests")
    git(path, "config", "user.email", "asc-tests@example.invalid")
    (path / "tracked.txt").write_text("initial\n", encoding="utf-8")
    git(path, "add", "tracked.txt")
    git(path, "commit", "-qm", "initial")
