#!/usr/bin/env python3
"""Install or remove the Python asc implementation with hash ownership checks."""

from __future__ import annotations

import argparse
import contextlib
import hashlib
import json
import stat
import sys
from pathlib import Path

MANIFEST_VERSION = 1
PACKAGE_FILES = (
    "asc_devtools/__init__.py",
    "asc_devtools/__main__.py",
    "asc_devtools/cli.py",
    "asc_devtools/completion.py",
    "asc_devtools/config.py",
    "asc_devtools/errors.py",
    "asc_devtools/github.py",
    "asc_devtools/process.py",
    "asc_devtools/repositories.py",
    "asc_devtools/selfupdate.py",
    "asc_devtools/vendor.py",
    "asc_devtools/commands/__init__.py",
    "asc_devtools/commands/cmake.py",
    "asc_devtools/commands/doctor.py",
)
WRAPPER = b"""#!/usr/bin/env bash
set -o errexit
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
export PYTHONPATH="${root}/lib/asc-devtools${PYTHONPATH:+:${PYTHONPATH}}"
exec python3 -m asc_devtools "$@"
"""


class InstallError(Exception):
    """A safe lifecycle operation could not be completed."""


def digest(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def regular_file(path: Path) -> bool:
    try:
        return stat.S_ISREG(path.lstat().st_mode)
    except FileNotFoundError:
        return False


def install_root(prefix: Path, destdir: Path | None) -> Path:
    if not prefix.is_absolute() or prefix == Path(prefix.anchor):
        raise InstallError("prefix must be an absolute path other than /")
    if destdir is not None and not destdir.is_absolute():
        raise InstallError("destdir must be an absolute path")
    return Path(f"{destdir or ''}{prefix}")


def sources(project: Path) -> dict[str, bytes]:
    files = {"bin/asc": WRAPPER}
    files["share/bash-completion/completions/asc"] = (
        project / "completions" / "asc.bash"
    ).read_bytes()
    for relative in PACKAGE_FILES:
        files[f"lib/asc-devtools/{relative}"] = (
            project / "src" / relative
        ).read_bytes()
    return files


def manifest_path(root: Path) -> Path:
    return root / "share" / "asc-devtools" / "install-manifest.json"


def load_manifest(root: Path, prefix: Path, expected: set[str]) -> dict[str, str]:
    path = manifest_path(root)
    if not regular_file(path):
        raise InstallError(f"invalid or missing install manifest: {path}")
    try:
        document = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise InstallError(f"invalid install manifest: {path}: {error}") from error
    if not isinstance(document, dict) or document.get("version") != MANIFEST_VERSION:
        raise InstallError(f"unsupported install manifest: {path}")
    if document.get("prefix") != str(prefix):
        raise InstallError(f"manifest prefix does not match {prefix}")
    hashes = document.get("files")
    if (
        not isinstance(hashes, dict)
        or not hashes
        or not set(hashes).issubset(expected)
    ):
        raise InstallError(f"manifest file set is not upgrade-compatible: {path}")
    if not all(
        isinstance(relative, str) and isinstance(value, str) and len(value) == 64
        for relative, value in hashes.items()
    ):
        raise InstallError(f"manifest contains invalid hashes: {path}")
    return hashes


def verify_existing(root: Path, hashes: dict[str, str]) -> None:
    for relative, expected_hash in hashes.items():
        target = root / relative
        if not target.exists() and not target.is_symlink():
            continue
        if not regular_file(target):
            raise InstallError(f"refusing non-regular managed path: {target}")
        if digest(target.read_bytes()) != expected_hash:
            raise InstallError(f"refusing modified managed file: {target}")


def install(project: Path, root: Path, prefix: Path) -> None:
    content = sources(project)
    manifest = manifest_path(root)
    managed_paths = [root / relative for relative in content]
    if any(path.exists() or path.is_symlink() for path in (*managed_paths, manifest)):
        hashes = load_manifest(root, prefix, set(content))
        verify_existing(root, hashes)
    for relative, data in content.items():
        target = root / relative
        if target.is_symlink():
            raise InstallError(f"refusing symbolic link: {target}")
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)
        target.chmod(0o755 if relative == "bin/asc" else 0o644)
        print(f"Installed {target}")
    document = {
        "version": MANIFEST_VERSION,
        "prefix": str(prefix),
        "files": {relative: digest(data) for relative, data in content.items()},
    }
    manifest.parent.mkdir(parents=True, exist_ok=True)
    manifest.write_text(json.dumps(document, indent=2) + "\n", encoding="utf-8")
    manifest.chmod(0o644)
    print(f"Installed {manifest}")


def uninstall(root: Path, prefix: Path, expected: set[str]) -> None:
    manifest = manifest_path(root)
    if not manifest.exists() and not manifest.is_symlink():
        print(f"asc-devtools is not installed under {root}")
        return
    hashes = load_manifest(root, prefix, expected)
    verify_existing(root, hashes)
    for relative in hashes:
        target = root / relative
        try:
            target.unlink()
            print(f"Removed {target}")
        except FileNotFoundError:
            pass
    manifest.unlink()
    print(f"Removed {manifest}")
    directories = sorted(
        {path.parent for relative in hashes for path in [root / relative]},
        key=lambda path: len(path.parts),
        reverse=True,
    )
    directories.extend(
        [manifest.parent, root / "share" / "bash-completion" / "completions"]
    )
    for directory in directories:
        with contextlib.suppress(OSError):
            directory.rmdir()


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser()
    result.add_argument("operation", choices=("install", "uninstall"))
    result.add_argument("--prefix", type=Path, default=Path("/usr/local"))
    result.add_argument("--destdir", type=Path)
    return result


def main() -> int:
    arguments = parser().parse_args()
    project = Path(__file__).resolve().parent.parent
    try:
        root = install_root(arguments.prefix, arguments.destdir)
        expected = set(sources(project))
        if arguments.operation == "install":
            install(project, root, arguments.prefix)
        else:
            uninstall(root, arguments.prefix, expected)
        return 0
    except (InstallError, OSError) as error:
        print(f"{arguments.operation}: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
