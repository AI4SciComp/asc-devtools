"""Strict JSON configuration with CLI, environment, file, default precedence."""

from __future__ import annotations

import json
import os
import re
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path

from asc_devtools.errors import ConfigurationError

DEFAULT_CONFIG_PATH = Path("~/.config/asc/config.json")
DEFAULT_ORGANIZATION = "AI4SciComp"
DEFAULT_WORKSPACE = Path("~/projects/AI4SciComp")
DEFAULT_REPOSITORY_PREFIX = "asc-"
DEFAULT_INCLUDE_DOT_GITHUB = True
DEFAULT_CLONE_PROTOCOL = "ssh"
DEFAULT_REMOTE = "origin"
MAX_CONFIG_SIZE = 1 << 20

_OWNER_PATTERN = re.compile(r"^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$")
_SIMPLE_PATTERN = re.compile(r"^[A-Za-z0-9._-]+$")
_FILE_KEYS = {
    "organization",
    "workspace",
    "repositoryPrefix",
    "includeDotGitHub",
    "cloneProtocol",
    "remote",
}


@dataclass(frozen=True)
class ConfigOverrides:
    """Values explicitly supplied before the command."""

    organization: str | None = None
    workspace: str | None = None


@dataclass(frozen=True)
class Config:
    """Fully resolved application configuration."""

    config_path: Path
    organization: str
    workspace: Path
    repository_prefix: str
    include_dot_github: bool
    clone_protocol: str
    remote: str
    github_token: str = ""
    github_token_source: str = ""


def _expanded_path(value: str | Path, home: Path, cwd: Path) -> Path:
    raw = str(value)
    if raw == "~":
        raw = str(home)
    elif raw.startswith("~/"):
        raw = str(home / raw[2:])
    elif raw.startswith("~"):
        raise ConfigurationError("only '~' and '~/' home expansion are supported")
    path = Path(raw)
    if not path.is_absolute():
        path = cwd / path
    return Path(os.path.abspath(path))


def _read_file(path: Path) -> dict[str, object]:
    if not path.exists():
        return {}
    if path.is_symlink() or not path.is_file():
        raise ConfigurationError(f"configuration path is not a regular file: {path}")
    try:
        if path.stat().st_size > MAX_CONFIG_SIZE:
            raise ConfigurationError(f"configuration exceeds 1 MiB limit: {path}")
        document = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise ConfigurationError(
            f"could not parse configuration {path}: {error}"
        ) from error
    if not isinstance(document, dict):
        raise ConfigurationError(f"configuration must contain a JSON object: {path}")
    unknown = set(document) - _FILE_KEYS
    if unknown:
        raise ConfigurationError(f"unknown configuration field: {sorted(unknown)[0]}")
    return document


def _string(document: Mapping[str, object], key: str, default: str) -> str:
    value = document.get(key, default)
    if not isinstance(value, str):
        raise ConfigurationError(f"{key} must be a string")
    return value


def _boolean(document: Mapping[str, object], key: str, default: bool) -> bool:
    value = document.get(key, default)
    if not isinstance(value, bool):
        raise ConfigurationError(f"{key} must be true or false")
    return value


def _environment_boolean(environment: Mapping[str, str], default: bool) -> bool:
    value = environment.get("ASC_INCLUDE_DOT_GITHUB")
    if value is None:
        return default
    if value not in {"true", "false"}:
        raise ConfigurationError("ASC_INCLUDE_DOT_GITHUB must be 'true' or 'false'")
    return value == "true"


def _token(environment: Mapping[str, str]) -> tuple[str, str]:
    for name in ("ASC_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"):
        if environment.get(name):
            return environment[name], name
    return "", ""


def _validate(config: Config) -> Config:
    if not _OWNER_PATTERN.fullmatch(config.organization):
        raise ConfigurationError(
            "organization must be a valid GitHub organization name"
        )
    if not config.repository_prefix or not _SIMPLE_PATTERN.fullmatch(
        config.repository_prefix
    ):
        raise ConfigurationError("repository prefix contains invalid characters")
    if config.clone_protocol not in {"ssh", "https"}:
        raise ConfigurationError("clone protocol must be 'ssh' or 'https'")
    if (
        not config.remote
        or config.remote.startswith("-")
        or not _SIMPLE_PATTERN.fullmatch(config.remote)
    ):
        raise ConfigurationError("remote must be a simple Git remote name")
    if config.workspace == Path(config.workspace.anchor):
        raise ConfigurationError("workspace must not be a filesystem root")
    return config


def load_config(
    config_path: str | Path | None = None,
    *,
    environment: Mapping[str, str] | None = None,
    overrides: ConfigOverrides | None = None,
    home: Path | None = None,
    cwd: Path | None = None,
) -> Config:
    """Load and validate configuration using canonical precedence."""
    environment = os.environ if environment is None else environment
    overrides = overrides or ConfigOverrides()
    home = Path.home() if home is None else home
    cwd = Path.cwd() if cwd is None else cwd
    selected = config_path or environment.get("ASC_CONFIG") or DEFAULT_CONFIG_PATH
    path = _expanded_path(selected, home, cwd)
    document = _read_file(path)

    organization = environment.get(
        "ASC_ORGANIZATION", _string(document, "organization", DEFAULT_ORGANIZATION)
    )
    workspace = environment.get(
        "ASC_WORKSPACE", _string(document, "workspace", str(DEFAULT_WORKSPACE))
    )
    repository_prefix = environment.get(
        "ASC_REPOSITORY_PREFIX",
        _string(document, "repositoryPrefix", DEFAULT_REPOSITORY_PREFIX),
    )
    clone_protocol = environment.get(
        "ASC_CLONE_PROTOCOL",
        _string(document, "cloneProtocol", DEFAULT_CLONE_PROTOCOL),
    )
    remote = environment.get("ASC_REMOTE", _string(document, "remote", DEFAULT_REMOTE))
    include_dot_github = _environment_boolean(
        environment,
        _boolean(document, "includeDotGitHub", DEFAULT_INCLUDE_DOT_GITHUB),
    )
    if overrides.organization is not None:
        organization = overrides.organization
    if overrides.workspace is not None:
        workspace = overrides.workspace
    token, token_source = _token(environment)
    return _validate(
        Config(
            config_path=path,
            organization=organization,
            workspace=_expanded_path(workspace, home, cwd),
            repository_prefix=repository_prefix,
            include_dot_github=include_dot_github,
            clone_protocol=clone_protocol,
            remote=remote,
            github_token=token,
            github_token_source=token_source,
        )
    )
