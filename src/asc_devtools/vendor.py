"""Safe local-only vendoring of pinned asc-cmake modules."""

from __future__ import annotations

import hashlib
import json
import os
import re
import shutil
import tempfile
from collections.abc import Mapping
from dataclasses import dataclass, field
from pathlib import Path, PurePosixPath

from asc_devtools.config import Config
from asc_devtools.errors import RepositoryError
from asc_devtools.process import CommandRunner
from asc_devtools.repositories import select_local_repositories

MANIFEST_NAME = "ASC_CMAKE_MANIFEST.json"
SOURCE_REPOSITORY = "AI4SciComp/asc-cmake"
_MANIFEST_KEYS = {"schemaVersion", "sourceRepository", "version", "commit", "files"}
_FILE_KEYS = {"path", "sha256"}
_VERSION_PATTERN = re.compile(
    r"project\s*\([^)]*\bVERSION\s+([0-9]+(?:\.[0-9]+){1,3}(?:[-+][A-Za-z0-9.-]+)?)",
    re.IGNORECASE | re.MULTILINE,
)


@dataclass(frozen=True)
class ManifestFile:
    path: str
    sha256: str

    def as_json(self) -> dict[str, str]:
        return {"path": self.path, "sha256": self.sha256}


@dataclass(frozen=True)
class Manifest:
    schema_version: int
    source_repository: str
    version: str
    commit: str
    files: tuple[ManifestFile, ...]

    def as_json(self) -> dict[str, object]:
        return {
            "schemaVersion": self.schema_version,
            "sourceRepository": self.source_repository,
            "version": self.version,
            "commit": self.commit,
            "files": [
                record.as_json()
                for record in sorted(self.files, key=lambda item: item.path)
            ],
        }


@dataclass(frozen=True)
class VendorStatus:
    repository: str
    state: str
    version: str = ""
    commit: str = ""
    extra_files: tuple[str, ...] = ()
    detail: str = ""

    def as_json(self) -> dict[str, object]:
        value: dict[str, object] = {"repository": self.repository, "state": self.state}
        if self.version:
            value["version"] = self.version
        if self.commit:
            value["commit"] = self.commit
        if self.extra_files:
            value["extraFiles"] = list(self.extra_files)
        if self.detail:
            value["detail"] = self.detail
        return value


@dataclass(frozen=True)
class VendorAction:
    path: str
    action: str

    def as_json(self) -> dict[str, str]:
        return {"path": self.path, "action": self.action}


@dataclass(frozen=True)
class VendorPlan:
    repository: str
    source: str
    version: str
    commit: str
    actions: tuple[VendorAction, ...]
    manifest: Manifest = field(repr=False, compare=False)
    files: Mapping[str, bytes] = field(repr=False, compare=False)
    target: Path = field(repr=False, compare=False)

    def as_json(self) -> dict[str, object]:
        return {
            "repository": self.repository,
            "source": self.source,
            "version": self.version,
            "commit": self.commit,
            "actions": [action.as_json() for action in self.actions],
        }


@dataclass(frozen=True)
class _Source:
    root: Path
    version: str
    commit: str
    dirty: bool
    files: Mapping[str, bytes]
    manifest: Manifest


def _managed_path(value: str) -> str:
    path = PurePosixPath(value)
    if (
        not value
        or value == MANIFEST_NAME
        or path.is_absolute()
        or path.as_posix() != value
        or value in {".", ".."}
        or ".." in path.parts
        or "\\" in value
    ):
        raise RepositoryError(f"unsafe managed path {value!r}")
    return value


def _manifest_bytes(manifest: Manifest) -> bytes:
    return (
        json.dumps(manifest.as_json(), indent=2, ensure_ascii=True, sort_keys=False)
        + "\n"
    ).encode()


def _decode_manifest(data: bytes) -> Manifest:
    try:
        document = json.loads(data)
    except (UnicodeError, json.JSONDecodeError) as error:
        raise RepositoryError(f"decode vendor manifest: {error}") from error
    if not isinstance(document, dict) or set(document) != _MANIFEST_KEYS:
        raise RepositoryError("vendor manifest has missing or unknown fields")
    if document["schemaVersion"] != 1:
        raise RepositoryError(
            f"unsupported vendor manifest schema {document['schemaVersion']!r}"
        )
    if (
        document["sourceRepository"] != SOURCE_REPOSITORY
        or not isinstance(document["version"], str)
        or not document["version"]
        or not isinstance(document["commit"], str)
        or not document["commit"]
        or not isinstance(document["files"], list)
    ):
        raise RepositoryError(
            "vendor manifest source, version, commit, and files are invalid"
        )
    records: list[ManifestFile] = []
    seen: set[str] = set()
    for value in document["files"]:
        if not isinstance(value, dict) or set(value) != _FILE_KEYS:
            raise RepositoryError("vendor manifest file record is invalid")
        path = _managed_path(value["path"] if isinstance(value["path"], str) else "")
        digest = value["sha256"]
        if (
            path in seen
            or not isinstance(digest, str)
            or len(digest) != 64
            or any(character not in "0123456789abcdef" for character in digest)
        ):
            raise RepositoryError(
                f"vendor manifest hash or duplicate is invalid: {path}"
            )
        seen.add(path)
        records.append(ManifestFile(path, digest))
    return Manifest(
        1,
        SOURCE_REPOSITORY,
        document["version"],
        document["commit"],
        tuple(sorted(records, key=lambda item: item.path)),
    )


def _git(runner: CommandRunner, root: Path, *arguments: str) -> str:
    return runner.run(["git", "-C", str(root), *arguments]).stdout.strip()


def _origin_matches(origin: str) -> bool:
    normalized = origin.strip().removesuffix(".git")
    return normalized.endswith(
        "github.com/AI4SciComp/asc-cmake"
    ) or normalized.endswith("github.com:AI4SciComp/asc-cmake")


def _distribution_files(root: Path) -> dict[str, bytes]:
    contract = root / "distribution.json"
    paths: list[str] = []
    if contract.exists():
        if contract.is_symlink() or not contract.is_file():
            raise RepositoryError("distribution.json must be a regular nonsymlink file")
        try:
            document = json.loads(contract.read_text(encoding="utf-8"))
        except (OSError, UnicodeError, json.JSONDecodeError) as error:
            raise RepositoryError(f"parse distribution.json: {error}") from error
        if (
            not isinstance(document, dict)
            or set(document) != {"files"}
            or not isinstance(document["files"], list)
        ):
            raise RepositoryError("distribution.json must contain only a files array")
        if not all(isinstance(item, str) for item in document["files"]):
            raise RepositoryError("distribution.json files must be strings")
        paths.extend(document["files"])
    else:
        license_path = root / "LICENSE"
        if license_path.is_file() and not license_path.is_symlink():
            paths.append("LICENSE")
        modules = root / "modules"
        if modules.exists():
            for path in modules.rglob("*"):
                if path.is_symlink():
                    raise RepositoryError(f"distribution contains symlink: {path}")
                if path.is_file() and path.suffix.lower() == ".cmake":
                    paths.append(path.relative_to(root).as_posix())
    if not paths:
        raise RepositoryError("asc-cmake distribution contains no managed files")
    files: dict[str, bytes] = {}
    root_resolved = root.resolve()
    for value in paths:
        relative = _managed_path(value)
        if relative in files:
            raise RepositoryError(f"distribution contains duplicate path {relative!r}")
        path = root / relative
        if (
            path.is_symlink()
            or not path.is_file()
            or not path.resolve().is_relative_to(root_resolved)
        ):
            raise RepositoryError(
                f"distribution file is not safely contained: {relative}"
            )
        files[relative] = path.read_bytes()
    return files


def _source_version(runner: CommandRunner, root: Path) -> str:
    version_file = root / "VERSION"
    if version_file.is_file() and not version_file.is_symlink():
        version = version_file.read_text(encoding="utf-8").strip()
        if version:
            return version
    cmake_lists = root / "CMakeLists.txt"
    if cmake_lists.is_file() and not cmake_lists.is_symlink():
        match = _VERSION_PATTERN.search(cmake_lists.read_text(encoding="utf-8"))
        if match:
            return match.group(1)
    try:
        tag = _git(runner, root, "describe", "--tags", "--exact-match", "HEAD")
    except Exception as error:
        raise RepositoryError(
            "asc-cmake version not found in VERSION, CMake project(), or exact tag"
        ) from error
    if not tag:
        raise RepositoryError(
            "asc-cmake version not found in VERSION, CMake project(), or exact tag"
        )
    return tag.removeprefix("v")


def _load_source(
    config: Config,
    runner: CommandRunner,
    source_path: str = "",
    ref: str = "",
    *,
    allow_dirty: bool,
) -> _Source:
    root = (
        Path(source_path)
        if source_path
        else config.workspace / config.cmake.source_repository
    )
    root = Path(os.path.abspath(root))
    if root.is_symlink() or not root.is_dir():
        raise RepositoryError(f"asc-cmake source is not a regular directory: {root}")
    try:
        if _git(runner, root, "rev-parse", "--is-inside-work-tree") != "true":
            raise RepositoryError(f"asc-cmake source is not a Git working tree: {root}")
        commit = _git(runner, root, "rev-parse", "HEAD")
        if (
            ref
            and _git(runner, root, "rev-parse", "--verify", f"{ref}^{{commit}}")
            != commit
        ):
            raise RepositoryError(
                f"requested ref {ref!r} does not match checked-out source commit"
            )
        origin_result = runner.run(
            ["git", "-C", str(root), "remote", "get-url", "origin"], check=False
        )
        origin = origin_result.stdout.strip()
        if origin_result.return_code == 0 and origin and not _origin_matches(origin):
            raise RepositoryError(
                f"asc-cmake source origin is not {SOURCE_REPOSITORY}: {origin}"
            )
        dirty = bool(
            _git(runner, root, "status", "--porcelain", "--untracked-files=normal")
        )
    except RepositoryError:
        raise
    except Exception as error:
        raise RepositoryError(f"validate asc-cmake source: {error}") from error
    if dirty and not allow_dirty:
        raise RepositoryError("refusing to apply from a dirty asc-cmake source")
    files = _distribution_files(root)
    version = _source_version(runner, root)
    records = tuple(
        ManifestFile(path, hashlib.sha256(data).hexdigest())
        for path, data in sorted(files.items())
    )
    manifest = Manifest(1, SOURCE_REPOSITORY, version, commit, records)
    return _Source(root, version, commit, dirty, files, manifest)


def _target(config: Config, runner: CommandRunner, repository_name: str) -> Path:
    _, repository, error = select_local_repositories(config, [repository_name], runner)[
        0
    ]
    if error:
        raise RepositoryError(error)
    target = repository / config.cmake.vendor_directory
    if not target.resolve(strict=False).is_relative_to(repository.resolve()):
        raise RepositoryError("vendor directory escapes the consumer repository")
    current = repository
    for part in Path(config.cmake.vendor_directory).parts:
        current /= part
        if current.is_symlink():
            raise RepositoryError(f"refusing symlinked vendor path: {current}")
    return target


def _read_manifest(target: Path) -> Manifest | None:
    path = target / MANIFEST_NAME
    if not path.exists():
        return None
    if path.is_symlink() or not path.is_file():
        raise RepositoryError("vendor manifest is not a regular file")
    return _decode_manifest(path.read_bytes())


def _inspect(target: Path, manifest: Manifest) -> tuple[tuple[str, ...], bool]:
    managed = {record.path for record in manifest.files}
    modified = False
    for record in manifest.files:
        path = target / record.path
        if path.is_symlink() or not path.is_file():
            modified = True
            continue
        if hashlib.sha256(path.read_bytes()).hexdigest() != record.sha256:
            modified = True
    extras = (
        tuple(
            sorted(
                path.relative_to(target).as_posix()
                for path in target.rglob("*")
                if not path.is_dir()
                and path.relative_to(target).as_posix() not in managed | {MANIFEST_NAME}
            )
        )
        if target.exists()
        else ()
    )
    return extras, modified


def vendor_status(
    config: Config, runner: CommandRunner, repository_name: str
) -> VendorStatus:
    """Classify the current consumer without writing."""
    try:
        target = _target(config, runner, repository_name)
        manifest = _read_manifest(target)
    except RepositoryError as error:
        return VendorStatus(repository_name, "manifest-invalid", detail=str(error))
    if manifest is None:
        return VendorStatus(repository_name, "not-vendored")
    try:
        extras, modified = _inspect(target, manifest)
    except (OSError, RepositoryError) as error:
        return VendorStatus(repository_name, "manifest-invalid", detail=str(error))
    if modified:
        return VendorStatus(
            repository_name,
            "locally-modified",
            manifest.version,
            manifest.commit,
            extras,
            "managed files differ from the manifest",
        )
    try:
        source = _load_source(config, runner, allow_dirty=True)
    except RepositoryError as error:
        return VendorStatus(
            repository_name,
            "source-unavailable",
            manifest.version,
            manifest.commit,
            extras,
            str(error),
        )
    state = (
        "current"
        if _manifest_bytes(manifest) == _manifest_bytes(source.manifest)
        else "source-newer"
    )
    detail = "source checkout is dirty" if source.dirty else ""
    return VendorStatus(
        repository_name, state, manifest.version, manifest.commit, extras, detail
    )


def vendor_plan(
    config: Config,
    runner: CommandRunner,
    repository_name: str,
    source_path: str = "",
    ref: str = "",
) -> VendorPlan:
    """Compute a deterministic read-only vendoring plan."""
    target = _target(config, runner, repository_name)
    source = _load_source(config, runner, source_path, ref, allow_dirty=True)
    old = _read_manifest(target)
    if old is not None:
        _, modified = _inspect(target, old)
        if modified:
            raise RepositoryError(
                "locally modified managed vendored files must be resolved before planning"
            )
    previous = {record.path: record for record in old.files} if old else {}
    actions: list[VendorAction] = []
    for record in source.manifest.files:
        prior = previous.pop(record.path, None)
        action = (
            "add"
            if prior is None
            else "preserve" if prior.sha256 == record.sha256 else "replace"
        )
        actions.append(VendorAction(record.path, action))
    actions.extend(VendorAction(path, "remove") for path in previous)
    return VendorPlan(
        repository_name,
        str(source.root),
        source.version,
        source.commit,
        tuple(sorted(actions, key=lambda item: item.path)),
        source.manifest,
        source.files,
        target,
    )


def _same_approved_plan(left: VendorPlan, right: VendorPlan) -> bool:
    return (
        left.repository,
        left.version,
        left.commit,
        left.actions,
    ) == (
        right.repository,
        right.version,
        right.commit,
        right.actions,
    )


def vendor_apply(config: Config, runner: CommandRunner, approved: VendorPlan) -> None:
    """Revalidate and conservatively apply an approved exact plan."""
    current = vendor_plan(
        config, runner, approved.repository, approved.source, approved.commit
    )
    if not _same_approved_plan(approved, current):
        raise RepositoryError("vendor plan changed during confirmation; run plan again")
    source = _load_source(
        config, runner, approved.source, approved.commit, allow_dirty=False
    )
    target = approved.target
    target.parent.mkdir(parents=True, exist_ok=True)
    stage = Path(tempfile.mkdtemp(prefix=".asc-vendor-stage-", dir=target.parent))
    backup = Path(tempfile.mkdtemp(prefix=".asc-vendor-backup-", dir=target.parent))
    changes: list[tuple[Path, Path | None, bool]] = []
    try:
        for relative, data in source.files.items():
            staged = stage / relative
            staged.parent.mkdir(parents=True, exist_ok=True)
            staged.write_bytes(data)
            staged.chmod(0o644)
        (stage / MANIFEST_NAME).write_bytes(_manifest_bytes(source.manifest))
        (stage / MANIFEST_NAME).chmod(0o644)
        target.mkdir(parents=True, exist_ok=True)
        for action in current.actions:
            if action.action == "preserve":
                continue
            destination = target / action.path
            saved: Path | None = None
            if destination.exists() or destination.is_symlink():
                saved = backup / action.path
                saved.parent.mkdir(parents=True, exist_ok=True)
                os.replace(destination, saved)
            changes.append((destination, saved, False))
            if action.action != "remove":
                destination.parent.mkdir(parents=True, exist_ok=True)
                os.replace(stage / action.path, destination)
                changes[-1] = (destination, saved, True)
        destination = target / MANIFEST_NAME
        saved = None
        if destination.exists() or destination.is_symlink():
            saved = backup / MANIFEST_NAME
            os.replace(destination, saved)
        changes.append((destination, saved, False))
        os.replace(stage / MANIFEST_NAME, destination)
        changes[-1] = (destination, saved, True)
    except Exception:
        for destination, saved, installed in reversed(changes):
            if installed:
                destination.unlink(missing_ok=True)
            if saved is not None and saved.exists():
                destination.parent.mkdir(parents=True, exist_ok=True)
                os.replace(saved, destination)
        raise
    finally:
        shutil.rmtree(stage, ignore_errors=True)
        shutil.rmtree(backup, ignore_errors=True)
