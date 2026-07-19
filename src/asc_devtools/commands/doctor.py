"""Read-only structured environment diagnostics."""

from __future__ import annotations

import json
import os
import shutil
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import TextIO

from asc_devtools.config import Config
from asc_devtools.errors import AscError, ProcessError
from asc_devtools.github import GitHubClient
from asc_devtools.process import CommandRunner


@dataclass(frozen=True)
class Check:
    name: str
    status: str
    detail: str
    remedy: str = ""

    def as_json(self) -> dict[str, str]:
        value = {"name": self.name, "status": self.status, "detail": self.detail}
        if self.remedy:
            value["remedy"] = self.remedy
        return value


def _workspace_check(path: Path) -> Check:
    if path.exists():
        if path.is_dir():
            return Check("workspace", "pass", f"{path} exists")
        return Check("workspace", "failure", f"{path} is not a directory")
    candidate = path
    while not candidate.exists() and candidate != candidate.parent:
        candidate = candidate.parent
    if candidate.is_dir() and os.access(candidate, os.W_OK):
        return Check("workspace", "pass", f"{path} can be created")
    return Check("workspace", "failure", f"{path} cannot be created")


def _tool_check(runner: CommandRunner, name: str, required: bool) -> Check:
    try:
        result = runner.run([name, "--version"])
        detail = (result.stdout + result.stderr).strip().splitlines()[0]
        return Check(name, "pass", detail)
    except (ProcessError, IndexError) as error:
        return Check(
            name,
            "failure" if required else "warning",
            str(error),
            f"install {name}",
        )


def _vendor_source_check(config: Config, runner: CommandRunner) -> Check:
    path = config.workspace / config.cmake.source_repository
    if not path.exists():
        return Check(
            "asc-cmake-source",
            "warning",
            f"{path} is not checked out",
            "clone asc-cmake before using vendor commands",
        )
    if path.is_symlink() or not path.is_dir():
        return Check(
            "asc-cmake-source", "warning", f"{path} is not a regular directory"
        )
    try:
        result = runner.run(
            ["git", "-C", str(path), "rev-parse", "--is-inside-work-tree"]
        )
    except ProcessError as error:
        return Check("asc-cmake-source", "warning", str(error))
    if result.stdout.strip() != "true":
        return Check("asc-cmake-source", "warning", f"{path} is not a Git worktree")
    return Check("asc-cmake-source", "pass", f"{path} is available")


def run_doctor(
    config: Config,
    runner: CommandRunner,
    output: TextIO,
    *,
    json_output: bool = False,
    client: GitHubClient | None = None,
) -> int:
    """Run the same diagnostic categories as the Go implementation."""
    checks = [
        Check("configuration", "pass", str(config.config_path)),
        _workspace_check(config.workspace),
        Check("python", "pass", sys.version.split()[0]),
        _tool_check(runner, "git", True),
        _tool_check(runner, "cmake", False),
        _tool_check(runner, "ctest", False),
        _vendor_source_check(config, runner),
    ]
    if shutil.which("gh"):
        checks.append(Check("gh", "pass", "optional GitHub CLI is available"))
    else:
        checks.append(Check("gh", "warning", "optional GitHub CLI is not installed"))
    if config.github_token:
        checks.append(
            Check(
                "api-authentication",
                "pass",
                f"GitHub API token is available from {config.github_token_source}",
            )
        )
    else:
        checks.append(
            Check(
                "api-authentication",
                "warning",
                "using unauthenticated public GitHub API access",
                "set ASC_GITHUB_TOKEN for private repositories and higher rate limits",
            )
        )
    try:
        repositories = (
            client or GitHubClient(config.github_token)
        ).list_organization_repositories(config.organization)
        checks.append(
            Check(
                "github-api",
                "pass",
                f"can list {len(repositories)} repositories in {config.organization}",
            )
        )
    except AscError as error:
        checks.append(
            Check(
                "github-api",
                "failure",
                str(error),
                "check connectivity, organization, token permissions, and rate limits",
            )
        )
    if not shutil.which("ssh"):
        checks.append(
            Check(
                "ssh",
                "warning",
                "ssh is not installed",
                "install OpenSSH for SSH clones",
            )
        )
    else:
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
            detail = (result.stdout + result.stderr).lower()
        except ProcessError as error:
            detail = str(error).lower()
        if "successfully authenticated" in detail:
            checks.append(
                Check("ssh", "pass", "GitHub accepted the configured SSH key")
            )
        else:
            checks.append(
                Check(
                    "ssh",
                    "warning",
                    "GitHub SSH authentication was not confirmed",
                    "run ssh -T git@github.com",
                )
            )
    if shutil.which("asc"):
        checks.append(Check("path", "pass", "asc is available on PATH"))
    else:
        checks.append(
            Check(
                "path",
                "warning",
                "asc is not available on PATH",
                "install asc under /usr/local/bin",
            )
        )
    if json_output:
        json.dump([check.as_json() for check in checks], output, indent=2)
        print(file=output)
    else:
        for check in checks:
            suffix = f"; {check.remedy}" if check.remedy else ""
            print(
                f"{check.status.upper():<7} {check.name:<20} {check.detail}{suffix}",
                file=output,
            )
    return 1 if any(check.status == "failure" for check in checks) else 0
