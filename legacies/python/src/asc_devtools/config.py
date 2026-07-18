"""Loading and validation for asc configuration."""

from __future__ import annotations

import os
import re
import tomllib
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path

from asc_devtools.errors import ConfigurationError

DEFAULT_CONFIG_PATH = Path("~/.config/asc/config.toml")
DEFAULT_ORGANIZATION = "AI4SciComp"
DEFAULT_WORKSPACE = Path("~/projects/AI4SciComp")
DEFAULT_REPOSITORY_PREFIX = "asc-"
DEFAULT_INCLUDE_DOT_GITHUB = True
DEFAULT_CLONE_PROTOCOL = "ssh"

_GITHUB_OWNER_PATTERN = re.compile(r"^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$")
_PREFIX_PATTERN = re.compile(r"^[A-Za-z0-9._-]+$")


@dataclass(frozen=True)
class ConfigOverrides:
    """Optional command-line configuration overrides."""

    organization: str | None = None
    workspace: str | None = None
    repository_prefix: str | None = None
    include_dot_github: bool | None = None
    clone_protocol: str | None = None


@dataclass(frozen=True)
class Config:
    """Resolved and validated application configuration."""

    config_path: Path
    organization: str
    workspace: Path
    repository_prefix: str
    include_dot_github: bool
    clone_protocol: str


def _expanded_path(value: str | Path) -> Path:
    return Path(value).expanduser().resolve(strict=False)


def _configuration_path(
    explicit_path: str | Path | None, environment: Mapping[str, str]
) -> Path:
    selected = explicit_path or environment.get("ASC_CONFIG") or DEFAULT_CONFIG_PATH
    return _expanded_path(selected)


def _read_file(path: Path) -> dict[str, object]:
    if not path.exists():
        return {}
    if not path.is_file():
        raise ConfigurationError(f"configuration path is not a file: {path}")
    try:
        with path.open("rb") as config_file:
            document = tomllib.load(config_file)
    except (OSError, tomllib.TOMLDecodeError) as error:
        raise ConfigurationError(
            f"could not read configuration {path}: {error}"
        ) from error
    asc_section = document.get("asc", {})
    if not isinstance(asc_section, dict):
        raise ConfigurationError(f"[asc] must be a TOML table in {path}")
    return asc_section


def _string_value(
    values: Mapping[str, object], key: str, default: str, path: Path
) -> str:
    value = values.get(key, default)
    if not isinstance(value, str):
        raise ConfigurationError(f"asc.{key} must be a string in {path}")
    return value


def _boolean_value(
    values: Mapping[str, object], key: str, default: bool, path: Path
) -> bool:
    value = values.get(key, default)
    if not isinstance(value, bool):
        raise ConfigurationError(f"asc.{key} must be true or false in {path}")
    return value


def _validate(config: Config) -> Config:
    if not _GITHUB_OWNER_PATTERN.fullmatch(config.organization):
        raise ConfigurationError(
            "organization must be a valid GitHub organization name"
        )
    if not config.repository_prefix or not _PREFIX_PATTERN.fullmatch(
        config.repository_prefix
    ):
        raise ConfigurationError(
            "repository_prefix must contain only letters, digits, '.', '_', or '-'"
        )
    if config.clone_protocol not in {"ssh", "https"}:
        raise ConfigurationError("clone_protocol must be 'ssh' or 'https'")
    return config


def _selected_value(
    explicit: str | None,
    environment: Mapping[str, str],
    environment_name: str,
    file_value: str,
) -> str:
    if explicit is not None:
        return explicit
    if environment_name in environment:
        return environment[environment_name]
    return file_value


def load_config(
    config_path: str | Path | None = None,
    *,
    environment: Mapping[str, str] | None = None,
    overrides: ConfigOverrides | None = None,
) -> Config:
    """Load configuration using CLI, environment, file, and default precedence."""
    environment = os.environ if environment is None else environment
    overrides = overrides or ConfigOverrides()
    path = _configuration_path(config_path, environment)
    values = _read_file(path)

    organization = _selected_value(
        overrides.organization,
        environment,
        "ASC_ORGANIZATION",
        _string_value(values, "organization", DEFAULT_ORGANIZATION, path),
    )
    workspace_value = _selected_value(
        overrides.workspace,
        environment,
        "ASC_WORKSPACE",
        _string_value(values, "workspace", str(DEFAULT_WORKSPACE), path),
    )
    repository_prefix = _selected_value(
        overrides.repository_prefix,
        environment,
        "ASC_REPOSITORY_PREFIX",
        _string_value(values, "repository_prefix", DEFAULT_REPOSITORY_PREFIX, path),
    )
    clone_protocol = _selected_value(
        overrides.clone_protocol,
        environment,
        "ASC_CLONE_PROTOCOL",
        _string_value(values, "clone_protocol", DEFAULT_CLONE_PROTOCOL, path),
    ).lower()
    include_dot_github = (
        overrides.include_dot_github
        if overrides.include_dot_github is not None
        else _boolean_value(
            values, "include_dot_github", DEFAULT_INCLUDE_DOT_GITHUB, path
        )
    )
    if not workspace_value.strip():
        raise ConfigurationError("workspace must not be empty")
    return _validate(
        Config(
            config_path=path,
            organization=organization,
            workspace=_expanded_path(workspace_value),
            repository_prefix=repository_prefix,
            include_dot_github=include_dot_github,
            clone_protocol=clone_protocol,
        )
    )
