"""Bounded direct GitHub REST discovery."""

from __future__ import annotations

import json
import urllib.error
import urllib.parse
import urllib.request
from collections.abc import Callable, Mapping
from dataclasses import dataclass

from asc_devtools.config import Config
from asc_devtools.errors import AscError

MAX_RESPONSE_SIZE = 4 << 20


@dataclass(frozen=True, order=True)
class RemoteRepository:
    """Stable subset of GitHub repository metadata used by every variant."""

    name: str
    archived: bool
    fork: bool
    clone_url: str
    ssh_url: str
    default_branch: str
    private: bool

    def url_for(self, protocol: str) -> str:
        return self.ssh_url if protocol == "ssh" else self.clone_url

    def as_json(self) -> dict[str, object]:
        return {
            "name": self.name,
            "archived": self.archived,
            "fork": self.fork,
            "clone_url": self.clone_url,
            "ssh_url": self.ssh_url,
            "default_branch": self.default_branch,
            "private": self.private,
        }


Transport = Callable[[str, Mapping[str, str]], tuple[int, Mapping[str, str], bytes]]


def _default_transport(
    url: str, headers: Mapping[str, str]
) -> tuple[int, Mapping[str, str], bytes]:
    request = urllib.request.Request(url, headers=dict(headers), method="GET")
    try:
        with urllib.request.urlopen(request, timeout=15) as response:
            return (
                response.status,
                dict(response.headers),
                response.read(MAX_RESPONSE_SIZE + 1),
            )
    except urllib.error.HTTPError as error:
        error.read(8192)
        return error.code, dict(error.headers), b""
    except (urllib.error.URLError, TimeoutError, OSError) as error:
        raise AscError(f"GitHub API request failed: {error}") from error


class GitHubClient:
    """Small direct REST client with deterministic limits and pagination."""

    def __init__(
        self,
        token: str = "",
        *,
        base_url: str = "https://api.github.com",
        version: str = "0.1.0",
        transport: Transport | None = None,
    ) -> None:
        self.token = token
        self.base_url = base_url.rstrip("/")
        self.version = version
        self.transport = transport or _default_transport

    def list_organization_repositories(
        self, organization: str
    ) -> tuple[RemoteRepository, ...]:
        repositories: list[RemoteRepository] = []
        encoded = urllib.parse.quote(organization, safe="")
        page = 1
        while True:
            url = (
                f"{self.base_url}/orgs/{encoded}/repos"
                f"?type=all&per_page=100&page={page}"
            )
            headers = {
                "Accept": "application/vnd.github+json",
                "X-GitHub-Api-Version": "2022-11-28",
                "User-Agent": f"asc-devtools/{self.version}",
            }
            if self.token:
                headers["Authorization"] = f"Bearer {self.token}"
            status, response_headers, body = self.transport(url, headers)
            if status < 200 or status >= 300:
                raise AscError(_api_error(status, response_headers))
            if len(body) > MAX_RESPONSE_SIZE:
                raise AscError("GitHub API response exceeded size limit")
            try:
                records = json.loads(body)
            except (UnicodeError, json.JSONDecodeError) as error:
                raise AscError(f"decode GitHub API response: {error}") from error
            if not isinstance(records, list):
                raise AscError("GitHub API response must be a JSON array")
            for record in records:
                if not isinstance(record, dict):
                    raise AscError("GitHub API returned an invalid repository record")
                try:
                    repository = RemoteRepository(
                        name=_required_string(record, "name"),
                        archived=_required_bool(record, "archived"),
                        fork=_required_bool(record, "fork"),
                        clone_url=_required_string(record, "clone_url"),
                        ssh_url=_required_string(record, "ssh_url"),
                        default_branch=_required_string(record, "default_branch"),
                        private=_required_bool(record, "private"),
                    )
                except (KeyError, TypeError) as error:
                    raise AscError(
                        "GitHub API returned an incomplete repository record"
                    ) from error
                repositories.append(repository)
            link = response_headers.get("Link", response_headers.get("link", ""))
            if 'rel="next"' not in link and len(records) < 100:
                break
            if not records:
                break
            page += 1
        return tuple(repositories)


def _required_string(record: Mapping[str, object], key: str) -> str:
    value = record[key]
    if not isinstance(value, str):
        raise TypeError(key)
    return value


def _required_bool(record: Mapping[str, object], key: str) -> bool:
    value = record[key]
    if not isinstance(value, bool):
        raise TypeError(key)
    return value


def _api_error(status: int, headers: Mapping[str, str]) -> str:
    if status == 401:
        return "GitHub API authentication failed (401): configure ASC_GITHUB_TOKEN"
    if status == 403 and headers.get("X-RateLimit-Remaining") == "0":
        return "GitHub API rate limit exceeded (403): authenticate or wait for reset"
    if status == 403:
        return "GitHub API access forbidden (403): verify organization permissions"
    if status == 404:
        return (
            "GitHub organization or endpoint not found (404): verify "
            "organization and token access"
        )
    return f"GitHub API returned {status}"


def filter_managed(
    repositories: tuple[RemoteRepository, ...], config: Config
) -> tuple[RemoteRepository, ...]:
    selected = [
        repository
        for repository in repositories
        if not repository.archived
        and (
            repository.name.startswith(config.repository_prefix)
            or (config.include_dot_github and repository.name == ".github")
        )
    ]
    return tuple(sorted(selected, key=lambda repository: repository.name))


def discover_repositories(
    config: Config,
    *,
    client: GitHubClient | None = None,
) -> tuple[RemoteRepository, ...]:
    if client is None:
        client = GitHubClient(config.github_token)
    return filter_managed(
        client.list_organization_repositories(config.organization), config
    )
