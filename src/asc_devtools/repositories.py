"""Safe direct-child Git inspection, clone, and fast-forward synchronization."""

from __future__ import annotations

import re
import shlex
from collections.abc import Sequence
from dataclasses import dataclass
from pathlib import Path

from asc_devtools.config import Config
from asc_devtools.errors import ProcessError, RepositoryError
from asc_devtools.github import RemoteRepository
from asc_devtools.process import CommandRunner

_NAME_PATTERN = re.compile(r"^[A-Za-z0-9._-]+$")


@dataclass(frozen=True)
class RepositoryStatus:
    name: str
    branch: str = ""
    detached: bool = False
    upstream: str = ""
    ahead: int = 0
    behind: int = 0
    clean: bool = True
    changes: int = 0
    error: str = ""

    def as_json(self) -> dict[str, object]:
        value: dict[str, object] = {
            "name": self.name,
            "branch": self.branch,
            "detached": self.detached,
            "upstream": self.upstream,
            "ahead": self.ahead,
            "behind": self.behind,
            "clean": self.clean,
            "changes": self.changes,
        }
        if self.error:
            value["error"] = self.error
        return value


@dataclass(frozen=True)
class OperationResult:
    name: str
    outcome: str
    detail: str = ""
    plan: tuple[str, ...] = ()


def validate_repository_name(name: str, config: Config) -> str:
    if (
        not name
        or name in {".", ".."}
        or name.startswith("-")
        or not _NAME_PATTERN.fullmatch(name)
    ):
        raise RepositoryError(f"invalid repository name {name!r}")
    if not (
        name.startswith(config.repository_prefix)
        or (config.include_dot_github and name == ".github")
    ):
        raise RepositoryError(f"repository is not managed by asc: {name}")
    return name


def direct_child(config: Config, name: str) -> Path:
    validate_repository_name(name, config)
    path = config.workspace / name
    if path.parent != config.workspace:
        raise RepositoryError(f"repository is not a direct workspace child: {name}")
    return path


def _is_git_worktree(path: Path, runner: CommandRunner) -> bool:
    if path.is_symlink() or not path.is_dir():
        return False
    try:
        result = runner.run(
            ["git", "-C", str(path), "rev-parse", "--is-inside-work-tree"]
        )
    except ProcessError:
        return False
    return result.stdout.strip() == "true"


def discover_local_repositories(
    config: Config, runner: CommandRunner
) -> tuple[Path, ...]:
    if not config.workspace.is_dir():
        return ()
    repositories: list[Path] = []
    for child in config.workspace.iterdir():
        try:
            validate_repository_name(child.name, config)
        except RepositoryError:
            continue
        if _is_git_worktree(child, runner):
            repositories.append(child)
    return tuple(sorted(repositories, key=lambda path: path.name))


def select_local_repositories(
    config: Config, names: Sequence[str], runner: CommandRunner
) -> tuple[tuple[str, Path, str], ...]:
    selected: list[tuple[str, Path, str]] = []
    if not names:
        return tuple(
            (path.name, path, "")
            for path in discover_local_repositories(config, runner)
        )
    for name in names:
        try:
            path = direct_child(config, name)
            if path.is_symlink():
                raise RepositoryError("repository path is a symbolic link")
            if not _is_git_worktree(path, runner):
                raise RepositoryError(f"local Git working tree not found: {name}")
            selected.append((name, path, ""))
        except RepositoryError as error:
            selected.append((name, config.workspace / name, str(error)))
    return tuple(selected)


def parse_status(name: str, output: str) -> RepositoryStatus:
    status = RepositoryStatus(name=name)
    values = status.__dict__.copy()
    for line in output.splitlines():
        if line.startswith("# branch.head "):
            branch = line.removeprefix("# branch.head ")
            values["detached"] = branch == "(detached)"
            values["branch"] = "HEAD" if values["detached"] else branch
        elif line.startswith("# branch.upstream "):
            values["upstream"] = line.removeprefix("# branch.upstream ")
        elif line.startswith("# branch.ab "):
            fields = line.removeprefix("# branch.ab ").split()
            if len(fields) != 2:
                raise RepositoryError(f"invalid ahead/behind record: {line}")
            values["ahead"] = int(fields[0].removeprefix("+"))
            values["behind"] = int(fields[1].removeprefix("-"))
        elif line and not line.startswith("# "):
            values["clean"] = False
            values["changes"] = int(values["changes"]) + 1
    return RepositoryStatus(**values)


def inspect_repositories(
    config: Config, names: Sequence[str], runner: CommandRunner
) -> tuple[RepositoryStatus, ...]:
    statuses: list[RepositoryStatus] = []
    for name, path, selection_error in select_local_repositories(config, names, runner):
        if selection_error:
            statuses.append(
                RepositoryStatus(name=name, clean=False, error=selection_error)
            )
            continue
        try:
            result = runner.run(
                ["git", "-C", str(path), "status", "--porcelain=v2", "--branch"]
            )
            statuses.append(parse_status(name, result.stdout))
        except (ProcessError, RepositoryError, ValueError) as error:
            statuses.append(RepositoryStatus(name=name, clean=False, error=str(error)))
    return tuple(statuses)


def _normalized_url(value: str) -> str:
    return value.strip().rstrip("/")


def clone_repositories(
    config: Config,
    repositories: Sequence[RemoteRepository],
    requested_names: Sequence[str],
    protocol: str,
    runner: CommandRunner,
) -> tuple[OperationResult, ...]:
    available = {repository.name: repository for repository in repositories}
    names = list(requested_names) if requested_names else sorted(available)
    try:
        config.workspace.mkdir(parents=True, exist_ok=True)
    except OSError as error:
        return (OperationResult("workspace", "failed", str(error)),)
    results: list[OperationResult] = []
    for name in names:
        try:
            destination = direct_child(config, name)
        except RepositoryError as error:
            results.append(OperationResult(name, "failed", str(error)))
            continue
        repository = available.get(name)
        if repository is None:
            results.append(
                OperationResult(
                    name,
                    "failed",
                    "repository was not returned by the organization API",
                )
            )
            continue
        if destination.is_symlink():
            results.append(
                OperationResult(name, "failed", "destination is a symbolic link")
            )
            continue
        if destination.exists():
            if not _is_git_worktree(destination, runner):
                results.append(
                    OperationResult(
                        name,
                        "failed",
                        "destination exists and is not a Git working tree",
                    )
                )
                continue
            try:
                actual = runner.run(
                    [
                        "git",
                        "-C",
                        str(destination),
                        "remote",
                        "get-url",
                        "--",
                        config.remote,
                    ]
                ).stdout
            except ProcessError:
                actual = ""
            if _normalized_url(actual) not in {
                _normalized_url(repository.ssh_url),
                _normalized_url(repository.clone_url),
            }:
                results.append(
                    OperationResult(
                        name,
                        "failed",
                        "existing working tree remote does not match the "
                        "organization repository",
                    )
                )
            else:
                results.append(OperationResult(name, "already-present"))
            continue
        clone_url = repository.url_for(protocol)
        if not clone_url:
            results.append(
                OperationResult(name, "failed", f"{protocol} clone URL is missing")
            )
            continue
        try:
            runner.run(
                ["git", "clone", "--", clone_url, str(destination)],
                capture_output=False,
            )
            results.append(OperationResult(name, "cloned"))
        except ProcessError as error:
            results.append(OperationResult(name, "failed", str(error)))
    return tuple(results)


def _git(runner: CommandRunner, path: Path, *arguments: str) -> str:
    return runner.run(["git", "-C", str(path), *arguments]).stdout.strip()


def sync_repositories(
    config: Config,
    names: Sequence[str],
    runner: CommandRunner,
    *,
    dry_run: bool = False,
) -> tuple[OperationResult, ...]:
    results: list[OperationResult] = []
    for name, path, selection_error in select_local_repositories(config, names, runner):
        if selection_error:
            results.append(OperationResult(name, "failed", selection_error))
            continue
        try:
            if _git(runner, path, "status", "--porcelain"):
                results.append(
                    OperationResult(name, "skipped", "working tree is dirty")
                )
                continue
            try:
                _git(runner, path, "remote", "get-url", "--", config.remote)
            except ProcessError:
                results.append(
                    OperationResult(
                        name,
                        "skipped",
                        f"configured remote is not available: {config.remote}",
                    )
                )
                continue
            try:
                branch = _git(
                    runner, path, "symbolic-ref", "--quiet", "--short", "HEAD"
                )
            except ProcessError:
                results.append(
                    OperationResult(
                        name, "skipped", "detached HEAD cannot be synchronized safely"
                    )
                )
                continue
            try:
                upstream = _git(
                    runner,
                    path,
                    "rev-parse",
                    "--abbrev-ref",
                    "--symbolic-full-name",
                    "@{upstream}",
                )
            except ProcessError:
                results.append(
                    OperationResult(name, "skipped", f"branch {branch} has no upstream")
                )
                continue
            if not upstream.startswith(f"{config.remote}/"):
                results.append(
                    OperationResult(
                        name,
                        "skipped",
                        f"upstream does not use configured remote: {upstream}",
                    )
                )
                continue
            fetch = ("git", "-C", str(path), "fetch", "--", config.remote)
            merge = ("git", "-C", str(path), "merge", "--ff-only", upstream)
            if dry_run:
                results.append(
                    OperationResult(
                        name, "planned", plan=(shlex.join(fetch), shlex.join(merge))
                    )
                )
                continue
            before = _git(runner, path, "rev-parse", "HEAD")
            try:
                runner.run(fetch, capture_output=False)
            except ProcessError as error:
                results.append(
                    OperationResult(name, "failed", f"fetch failed: {error}")
                )
                continue
            try:
                runner.run(merge, capture_output=False)
            except ProcessError as error:
                results.append(
                    OperationResult(name, "skipped", f"fast-forward refused: {error}")
                )
                continue
            after = _git(runner, path, "rev-parse", "HEAD")
            results.append(
                OperationResult(name, "unchanged" if before == after else "updated")
            )
        except ProcessError as error:
            results.append(OperationResult(name, "failed", str(error)))
    return tuple(results)


def operations_failed(results: Sequence[OperationResult]) -> bool:
    return any(result.outcome in {"failed", "skipped"} for result in results)
