"""Safe local repository inspection, cloning, and synchronization."""

from __future__ import annotations

import re
from collections.abc import Iterable, Sequence
from dataclasses import dataclass
from pathlib import Path

from asc_devtools.config import Config
from asc_devtools.errors import ProcessError, RepositoryError
from asc_devtools.github import RemoteRepository
from asc_devtools.process import CommandRunner

_REPOSITORY_NAME_PATTERN = re.compile(r"^[A-Za-z0-9._-]+$")


@dataclass(frozen=True)
class ChangedFile:
    """Concise Git working-tree change information."""

    code: str
    path: str


@dataclass(frozen=True)
class RepositoryStatus:
    """Status details for one local Git repository."""

    name: str
    branch: str
    detached: bool
    clean: bool
    upstream: str | None
    ahead: int
    behind: int
    changes: tuple[ChangedFile, ...]


@dataclass(frozen=True)
class OperationResult:
    """Outcome of a clone or synchronization operation."""

    name: str
    outcome: str
    detail: str = ""


def validate_repository_name(name: str, config: Config) -> str:
    """Validate a repository name and ensure it is managed by asc."""
    if (
        not name
        or name in {".", ".."}
        or Path(name).is_absolute()
        or "/" in name
        or "\\" in name
        or not _REPOSITORY_NAME_PATTERN.fullmatch(name)
    ):
        raise RepositoryError(f"invalid repository name: {name!r}")
    if not (
        name.startswith(config.repository_prefix)
        or (config.include_dot_github and name == ".github")
    ):
        raise RepositoryError(f"repository is not managed by asc: {name}")
    return name


def is_git_repository(path: Path) -> bool:
    """Return whether a path has direct Git worktree metadata."""
    return path.is_dir() and (path / ".git").exists()


def discover_local_repositories(config: Config) -> tuple[Path, ...]:
    """Discover managed Git repositories that are direct workspace children."""
    if not config.workspace.is_dir():
        return ()
    repositories = []
    for child in config.workspace.iterdir():
        if not is_git_repository(child):
            continue
        if child.name.startswith(config.repository_prefix) or (
            config.include_dot_github and child.name == ".github"
        ):
            repositories.append(child)
    return tuple(sorted(repositories, key=lambda path: path.name.lower()))


def select_local_repositories(
    config: Config, requested_names: Sequence[str]
) -> tuple[Path, ...]:
    """Resolve requested managed names to existing direct-child repositories."""
    if not requested_names:
        return discover_local_repositories(config)
    selected: list[Path] = []
    for name in requested_names:
        validate_repository_name(name, config)
        path = config.workspace / name
        if not is_git_repository(path):
            raise RepositoryError(f"local Git repository not found: {name}")
        selected.append(path)
    return tuple(selected)


def _changed_file(line: str) -> ChangedFile | None:
    if line.startswith("? "):
        return ChangedFile("??", line[2:])
    if line.startswith("! "):
        return None
    if line.startswith(("1 ", "2 ")):
        parts = line.split(" ", 8)
        if len(parts) == 9:
            path = parts[8].split("\t")[-1]
            return ChangedFile(parts[1][:2], path)
    if line.startswith("u "):
        parts = line.split(" ", 10)
        if len(parts) == 11:
            return ChangedFile(parts[1][:2], parts[10])
    return None


def repository_status(path: Path, runner: CommandRunner) -> RepositoryStatus:
    """Inspect a repository without modifying it."""
    result = runner.run(["git", "status", "--porcelain=v2", "--branch"], cwd=path)
    branch = "unknown"
    detached = False
    upstream: str | None = None
    ahead = 0
    behind = 0
    changes: list[ChangedFile] = []
    for line in result.stdout.splitlines():
        if line.startswith("# branch.head "):
            head = line.removeprefix("# branch.head ")
            detached = head == "(detached)"
            branch = "detached HEAD" if detached else head
        elif line.startswith("# branch.upstream "):
            upstream = line.removeprefix("# branch.upstream ")
        elif line.startswith("# branch.ab "):
            fields = line.removeprefix("# branch.ab ").split()
            if len(fields) == 2:
                ahead = int(fields[0].lstrip("+"))
                behind = int(fields[1].lstrip("-"))
        elif not line.startswith("# "):
            change = _changed_file(line)
            if change is not None:
                changes.append(change)
    return RepositoryStatus(
        name=path.name,
        branch=branch,
        detached=detached,
        clean=not changes,
        upstream=upstream,
        ahead=ahead,
        behind=behind,
        changes=tuple(changes),
    )


def clone_repositories(
    config: Config,
    repositories: Iterable[RemoteRepository],
    requested_names: Sequence[str],
    runner: CommandRunner,
) -> tuple[OperationResult, ...]:
    """Clone selected missing repositories without overwriting local paths."""
    available = {repository.name: repository for repository in repositories}
    if requested_names:
        names = []
        for name in requested_names:
            validate_repository_name(name, config)
            if name not in available:
                raise RepositoryError(
                    f"repository not found in {config.organization}: {name}"
                )
            names.append(name)
    else:
        names = sorted(available, key=str.lower)

    for name in names:
        validate_repository_name(name, config)

    config.workspace.mkdir(parents=True, exist_ok=True)
    results: list[OperationResult] = []
    for name in names:
        destination = config.workspace / name
        if is_git_repository(destination):
            results.append(OperationResult(name, "skipped", "already cloned"))
            continue
        if destination.exists():
            results.append(
                OperationResult(name, "failed", "destination exists and is not Git")
            )
            continue
        repository = available[name]
        try:
            runner.run(
                ["git", "clone", repository.clone_url(config.clone_protocol), name],
                cwd=config.workspace,
                capture_output=False,
            )
            results.append(OperationResult(name, "cloned"))
        except ProcessError as error:
            results.append(OperationResult(name, "failed", str(error)))
    return tuple(results)


def sync_repositories(
    paths: Sequence[Path], runner: CommandRunner
) -> tuple[OperationResult, ...]:
    """Fast-forward clean repositories while continuing after failures."""
    results: list[OperationResult] = []
    for path in paths:
        try:
            dirty = runner.run(
                ["git", "status", "--porcelain"], cwd=path
            ).stdout.strip()
            if dirty:
                results.append(
                    OperationResult(path.name, "skipped", "working tree is dirty")
                )
                continue
            upstream = runner.run(
                ["git", "rev-parse", "--abbrev-ref", "@{upstream}"], cwd=path
            ).stdout.strip()
            before = runner.run(["git", "rev-parse", "HEAD"], cwd=path).stdout.strip()
            runner.run(["git", "fetch"], cwd=path, capture_output=False)
            runner.run(
                ["git", "merge", "--ff-only", upstream],
                cwd=path,
                capture_output=False,
            )
            after = runner.run(["git", "rev-parse", "HEAD"], cwd=path).stdout.strip()
            outcome = "updated" if before != after else "unchanged"
            results.append(OperationResult(path.name, outcome))
        except ProcessError as error:
            results.append(OperationResult(path.name, "failed", str(error)))
    return tuple(results)
