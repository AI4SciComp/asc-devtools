"""Safe boundary for invoking external commands."""

from __future__ import annotations

import re
import subprocess
from collections.abc import Sequence
from dataclasses import dataclass
from pathlib import Path

from asc_devtools.errors import ProcessError

_SENSITIVE_OPTION = re.compile(
    r"^(--?(?:token|password|secret|authorization|auth-token))(?:=|$)",
    re.IGNORECASE,
)
_URL_CREDENTIALS = re.compile(r"(https?://)[^/@\s]+@")


@dataclass(frozen=True)
class CommandResult:
    """Result of a completed external command."""

    arguments: tuple[str, ...]
    return_code: int
    stdout: str
    stderr: str


def _display_arguments(arguments: Sequence[str]) -> str:
    """Render arguments for errors while concealing common credential forms."""
    displayed: list[str] = []
    conceal_next = False
    for argument in arguments:
        if conceal_next:
            displayed.append("<redacted>")
            conceal_next = False
            continue
        match = _SENSITIVE_OPTION.match(argument)
        if match:
            option = match.group(1)
            if "=" in argument:
                displayed.append(f"{option}=<redacted>")
            else:
                displayed.append(option)
                conceal_next = True
            continue
        displayed.append(_URL_CREDENTIALS.sub(r"\1<redacted>@", argument))
    return " ".join(displayed)


class CommandRunner:
    """Run argument arrays without a shell and normalize command failures."""

    def run(
        self,
        arguments: Sequence[str],
        *,
        cwd: Path | None = None,
        capture_output: bool = True,
        check: bool = True,
    ) -> CommandResult:
        """Run a command and return its result.

        Setting ``capture_output`` to false connects the child process to the
        current terminal, which is used for long-running builds and tests.
        """
        if not arguments:
            raise ValueError("arguments must not be empty")
        normalized = tuple(str(argument) for argument in arguments)
        try:
            completed = subprocess.run(
                normalized,
                cwd=cwd,
                check=False,
                capture_output=capture_output,
                text=True,
            )
        except FileNotFoundError as error:
            raise ProcessError(
                f"required executable not found: {normalized[0]}"
            ) from error
        except OSError as error:
            raise ProcessError(
                f"could not run {_display_arguments(normalized)}: {error}"
            ) from error

        result = CommandResult(
            arguments=normalized,
            return_code=completed.returncode,
            stdout=completed.stdout or "",
            stderr=completed.stderr or "",
        )
        if check and result.return_code != 0:
            detail = result.stderr.strip() or result.stdout.strip()
            message = (
                f"command failed ({result.return_code}): "
                f"{_display_arguments(normalized)}"
            )
            if detail:
                message += f"\n{detail}"
            raise ProcessError(message)
        return result
