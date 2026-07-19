"""Verified updates for a managed asc installation."""

from __future__ import annotations

import hashlib
import io
import json
import os
import re
import stat
import subprocess
import sys
import tarfile
import tempfile
import urllib.error
import urllib.request
from collections.abc import Callable, Mapping, Sequence
from pathlib import Path, PurePosixPath
from typing import BinaryIO, TextIO

from asc_devtools.errors import AscError

REPOSITORY = "AI4SciComp/asc-devtools"
MAX_METADATA_SIZE = 4 << 20
MAX_CHECKSUM_SIZE = 1 << 20
MAX_ARCHIVE_SIZE = 128 << 20
MAX_EXTRACTED_SIZE = 256 << 20
ARCHIVE_NAME = "asc-devtools-python.tar.gz"
ARCHIVE_ROOT = "asc-devtools-python"
VERSION_PATTERN = re.compile(r"^[0-9]+(?:\.[0-9]+)*$")
PACKAGE_VERSION_PATTERN = re.compile(
    r'^__version__\s*=\s*["\']([0-9]+(?:\.[0-9]+)*)["\']$', re.MULTILINE
)

Transport = Callable[
    [str, Mapping[str, str], int], tuple[int, Mapping[str, str], bytes]
]
Command = Callable[[Sequence[str]], None]


def default_transport(
    url: str, headers: Mapping[str, str], limit: int
) -> tuple[int, Mapping[str, str], bytes]:
    request = urllib.request.Request(url, headers=dict(headers), method="GET")
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            return response.status, dict(response.headers), response.read(limit + 1)
    except urllib.error.HTTPError as error:
        error.read(8192)
        return error.code, dict(error.headers), b""
    except (urllib.error.URLError, TimeoutError, OSError) as error:
        raise AscError(f"GitHub release request failed: {error}") from error


def _request(
    url: str,
    token: str,
    current_version: str,
    accept: str,
    limit: int,
    transport: Transport,
) -> tuple[int, bytes]:
    headers = {
        "Accept": accept,
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": f"asc-devtools/{current_version or 'dev'}",
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"
    status, _response_headers, body = transport(url, headers, limit)
    if len(body) > limit:
        raise AscError(f"release response exceeded {limit} bytes")
    return status, body


def _latest_release(
    base_url: str, token: str, current_version: str, transport: Transport
) -> dict[str, object]:
    endpoint = f"{base_url.rstrip('/')}/repos/{REPOSITORY}/releases/latest"
    status, body = _request(
        endpoint,
        token,
        current_version,
        "application/vnd.github+json",
        MAX_METADATA_SIZE,
        transport,
    )
    if status == 404:
        raise AscError("no published asc release is available")
    if status < 200 or status >= 300:
        raise AscError(f"check latest release: GitHub API returned {status}")
    try:
        document = json.loads(body)
    except (UnicodeError, json.JSONDecodeError) as error:
        raise AscError(f"decode latest release: {error}") from error
    if not isinstance(document, dict):
        raise AscError("latest release response must be a JSON object")
    return document


def _version(tag: object) -> str:
    if not isinstance(tag, str):
        raise AscError("latest release has no tag")
    value = tag.removeprefix("v")
    if not VERSION_PATTERN.fullmatch(value):
        raise AscError(f"latest release has unsupported tag {tag!r}")
    return value


def _compare(current: str, latest: str) -> int:
    current = current.removeprefix("v")
    if not VERSION_PATTERN.fullmatch(current):
        return -1
    left = tuple(int(part) for part in current.split("."))
    right = tuple(int(part) for part in latest.split("."))
    width = max(len(left), len(right))
    left += (0,) * (width - len(left))
    right += (0,) * (width - len(right))
    return (left > right) - (left < right)


def _asset(document: dict[str, object], name: str) -> str:
    assets = document.get("assets")
    if not isinstance(assets, list):
        raise AscError("latest release has no asset list")
    for record in assets:
        if (
            isinstance(record, dict)
            and record.get("name") == name
            and isinstance(record.get("url"), str)
        ):
            return str(record["url"])
    raise AscError(f"latest release does not contain {name}")


def _download(
    url: str,
    token: str,
    current_version: str,
    limit: int,
    transport: Transport,
) -> bytes:
    status, body = _request(
        url,
        token,
        current_version,
        "application/octet-stream",
        limit,
        transport,
    )
    if status < 200 or status >= 300:
        raise AscError(f"download release asset: GitHub returned {status}")
    return body


def _expected_checksum(body: bytes, filename: str) -> str:
    try:
        lines = body.decode("ascii").splitlines()
    except UnicodeDecodeError as error:
        raise AscError("SHA256SUMS is not ASCII") from error
    for line in lines:
        fields = line.split()
        if len(fields) != 2 or fields[1].removeprefix("*") != filename:
            continue
        digest = fields[0].lower()
        if len(digest) == 64 and all(
            character in "0123456789abcdef" for character in digest
        ):
            return digest
        break
    raise AscError(f"SHA256SUMS has no valid entry for {filename}")


def _extract(archive: bytes, destination: Path) -> Path:
    destination.mkdir(mode=0o755)
    total = 0
    try:
        source: BinaryIO = io.BytesIO(archive)
        with tarfile.open(fileobj=source, mode="r:gz") as bundle:
            for member in bundle:
                relative = PurePosixPath(member.name)
                if (
                    relative.is_absolute()
                    or ".." in relative.parts
                    or not relative.parts
                    or relative.parts[0] != ARCHIVE_ROOT
                ):
                    raise AscError(f"unsafe or unexpected archive path {member.name!r}")
                target = destination.joinpath(*relative.parts)
                if member.isdir():
                    target.mkdir(parents=True, exist_ok=True, mode=0o755)
                    continue
                if not member.isfile():
                    raise AscError(f"unsupported archive entry {member.name!r}")
                total += member.size
                if member.size < 0 or total > MAX_EXTRACTED_SIZE:
                    raise AscError("archive exceeds extracted size limit")
                extracted = bundle.extractfile(member)
                if extracted is None:
                    raise AscError(f"cannot read archive entry {member.name!r}")
                target.parent.mkdir(parents=True, exist_ok=True, mode=0o755)
                flags = os.O_CREAT | os.O_EXCL | os.O_WRONLY
                mode = 0o755 if member.mode & 0o111 else 0o644
                descriptor = os.open(target, flags, mode)
                with os.fdopen(descriptor, "wb") as output:
                    while chunk := extracted.read(1024 * 1024):
                        output.write(chunk)
    except (tarfile.TarError, OSError) as error:
        raise AscError(f"extract release archive: {error}") from error
    return destination / ARCHIVE_ROOT


def _prefix(explicit: str, module_path: Path) -> Path:
    if explicit:
        prefix = Path(explicit).expanduser()
        if not prefix.is_absolute():
            raise AscError("update prefix must be an absolute path other than /")
    else:
        resolved = module_path.resolve()
        if (
            resolved.parent.name != "asc_devtools"
            or resolved.parent.parent.name != "asc-devtools"
            or resolved.parent.parent.parent.name != "lib"
        ):
            raise AscError(
                "cannot infer an installation prefix; run asc update "
                "--prefix /absolute/prefix"
            )
        prefix = resolved.parents[3]
    prefix = prefix.resolve()
    if not prefix.is_absolute() or prefix == Path(prefix.anchor):
        raise AscError("update prefix must be an absolute path other than /")
    return prefix


def _managed(prefix: Path) -> None:
    manifest = prefix / "share" / "asc-devtools" / "install-manifest.json"
    try:
        mode = manifest.lstat().st_mode
    except FileNotFoundError as error:
        raise AscError(
            f"refusing to update an unmanaged installation: {manifest} is missing"
        ) from error
    if not stat.S_ISREG(mode):
        raise AscError(f"refusing non-regular install manifest: {manifest}")


def _run_command(arguments: Sequence[str]) -> None:
    try:
        subprocess.run(arguments, check=True)
    except (OSError, subprocess.CalledProcessError) as error:
        raise AscError(f"install updated asc: {error}") from error


def _bundle_version(root: Path) -> str:
    source = root / "src" / "asc_devtools" / "__init__.py"
    if not source.is_file() or source.is_symlink() or source.stat().st_size > 65536:
        raise AscError("release bundle has no valid package version file")
    try:
        match = PACKAGE_VERSION_PATTERN.search(source.read_text(encoding="utf-8"))
    except (OSError, UnicodeError) as error:
        raise AscError(f"read release package version: {error}") from error
    if match is None:
        raise AscError("release bundle has no valid package version")
    return match.group(1)


def run_update(
    *,
    current_version: str,
    token: str = "",
    base_url: str = "https://api.github.com",
    prefix: str = "",
    check_only: bool = False,
    assume_yes: bool = False,
    input_stream: TextIO = sys.stdin,
    output: TextIO = sys.stdout,
    transport: Transport = default_transport,
    module_path: Path = Path(__file__),
    command: Command = _run_command,
) -> None:
    """Check for and optionally install the latest Python release bundle."""
    release = _latest_release(base_url, token, current_version, transport)
    latest = _version(release.get("tag_name"))
    comparison = _compare(current_version, latest)
    if comparison == 0:
        print(f"asc {current_version or 'dev'} is up to date.", file=output)
        return
    if comparison > 0:
        print(
            f"asc {current_version or 'dev'} is newer than the latest release "
            f"({latest}); no update performed.",
            file=output,
        )
        return
    archive_url = _asset(release, ARCHIVE_NAME)
    checksums_url = _asset(release, "SHA256SUMS")
    print(
        f"asc {latest} is available (current: {current_version or 'dev'}).",
        file=output,
    )
    if check_only:
        return
    install_prefix = _prefix(prefix, module_path)
    _managed(install_prefix)
    if not assume_yes:
        print(
            f"Update the managed installation in {install_prefix}? [y/N] ",
            end="",
            file=output,
        )
        if input_stream.readline().strip().lower() not in {"y", "yes"}:
            print("Update cancelled.", file=output)
            return
    checksums = _download(
        checksums_url, token, current_version, MAX_CHECKSUM_SIZE, transport
    )
    expected = _expected_checksum(checksums, ARCHIVE_NAME)
    archive = _download(
        archive_url, token, current_version, MAX_ARCHIVE_SIZE, transport
    )
    if hashlib.sha256(archive).hexdigest() != expected:
        raise AscError(f"checksum mismatch for {ARCHIVE_NAME}")
    with tempfile.TemporaryDirectory(prefix="asc-update-") as temporary:
        root = _extract(archive, Path(temporary) / "bundle")
        installer = root / "scripts" / "install.sh"
        if not installer.is_file() or installer.is_symlink():
            raise AscError("release bundle is missing a regular installer")
        bundle_version = _bundle_version(root)
        if bundle_version != latest:
            raise AscError(
                f"release package version {bundle_version!r} does not match "
                f"release {latest}"
            )
        command((str(installer), "--prefix", str(install_prefix)))
    print(f"Updated asc to {latest}.", file=output)
