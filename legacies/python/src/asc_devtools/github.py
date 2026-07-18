"""GitHub organization repository discovery through GitHub CLI."""

from __future__ import annotations

import json
from dataclasses import dataclass

from asc_devtools.config import Config
from asc_devtools.errors import AscError
from asc_devtools.process import CommandRunner


@dataclass(frozen=True, order=True)
class RemoteRepository:
    """Repository metadata returned by GitHub organization discovery."""

    name: str
    is_archived: bool
    ssh_url: str
    url: str
    description: str = ""

    def clone_url(self, protocol: str) -> str:
        """Return the clone URL for a configured transport protocol."""
        return self.ssh_url if protocol == "ssh" else f"{self.url}.git"


def discover_repositories(
    config: Config, runner: CommandRunner
) -> tuple[RemoteRepository, ...]:
    """Discover active managed repositories in the configured organization."""
    result = runner.run(
        [
            "gh",
            "repo",
            "list",
            config.organization,
            "--limit",
            "1000",
            "--json",
            "name,isArchived,sshUrl,url,description",
        ]
    )
    try:
        records = json.loads(result.stdout)
    except json.JSONDecodeError as error:
        raise AscError(
            f"GitHub CLI returned malformed repository data: {error}"
        ) from error
    if not isinstance(records, list):
        raise AscError("GitHub CLI repository data must be a JSON array")

    repositories: list[RemoteRepository] = []
    for record in records:
        try:
            if not isinstance(record, dict):
                raise TypeError
            repository = RemoteRepository(
                name=str(record["name"]),
                is_archived=bool(record["isArchived"]),
                ssh_url=str(record["sshUrl"]),
                url=str(record["url"]),
                description=str(record.get("description") or ""),
            )
        except (KeyError, TypeError) as error:
            raise AscError("GitHub CLI returned incomplete repository data") from error
        managed = repository.name.startswith(config.repository_prefix)
        dot_github = config.include_dot_github and repository.name == ".github"
        if not repository.is_archived and (managed or dot_github):
            repositories.append(repository)
    return tuple(sorted(repositories, key=lambda repository: repository.name.lower()))
