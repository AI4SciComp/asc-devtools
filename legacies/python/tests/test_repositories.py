"""Local repository operation tests."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from asc_devtools.config import Config
from asc_devtools.errors import ProcessError, RepositoryError
from asc_devtools.github import RemoteRepository
from asc_devtools.process import CommandRunner
from asc_devtools.repositories import (
    clone_repositories,
    discover_local_repositories,
    repository_status,
    sync_repositories,
    validate_repository_name,
)
from tests.support import ScriptedRunner, initialize_repository, result


def configuration(workspace: Path, protocol: str = "ssh") -> Config:
    return Config(
        workspace / "config.toml",
        "AI4SciComp",
        workspace,
        "asc-",
        True,
        protocol,
    )


def remote(name: str) -> RemoteRepository:
    return RemoteRepository(
        name,
        False,
        f"git@github.com:AI4SciComp/{name}.git",
        f"https://github.com/AI4SciComp/{name}",
    )


class LocalDiscoveryTest(unittest.TestCase):
    def test_direct_child_discovery_only(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            workspace = Path(directory)
            initialize_repository(workspace / "asc-one")
            initialize_repository(workspace / ".github")
            initialize_repository(workspace / "unrelated")
            nested = workspace / "container"
            nested.mkdir()
            initialize_repository(nested / "asc-nested")
            (workspace / "asc-not-git").mkdir()
            found = discover_local_repositories(configuration(workspace))
        self.assertEqual([path.name for path in found], [".github", "asc-one"])

    def test_rejects_unsafe_or_unmanaged_names(self) -> None:
        config = configuration(Path("/tmp/workspace"))
        for name in ("../asc-other", "/tmp/asc-other", "asc/foo", "other"):
            with self.subTest(name=name), self.assertRaises(RepositoryError):
                validate_repository_name(name, config)


class StatusTest(unittest.TestCase):
    def test_clean_and_dirty_status(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "asc-one"
            initialize_repository(path)
            clean = repository_status(path, CommandRunner())
            self.assertTrue(clean.clean)
            self.assertIsNone(clean.upstream)
            (path / "tracked.txt").write_text("changed\n", encoding="utf-8")
            (path / "untracked.txt").write_text("new\n", encoding="utf-8")
            dirty = repository_status(path, CommandRunner())
        self.assertFalse(dirty.clean)
        self.assertEqual(len(dirty.changes), 2)

    def test_detached_head(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "asc-one"
            initialize_repository(path)
            from tests.support import git

            git(path, "checkout", "-q", "--detach")
            status = repository_status(path, CommandRunner())
        self.assertTrue(status.detached)
        self.assertEqual(status.branch, "detached HEAD")


class CloneTest(unittest.TestCase):
    def test_clones_missing_repository_with_ssh_url(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            workspace = Path(directory) / "workspace"
            runner = ScriptedRunner([result()])
            results = clone_repositories(
                configuration(workspace),
                [remote("asc-one")],
                [],
                runner,  # type: ignore[arg-type]
            )
        self.assertEqual(results[0].outcome, "cloned")
        self.assertEqual(
            runner.calls[0][0],
            ("git", "clone", "git@github.com:AI4SciComp/asc-one.git", "asc-one"),
        )

    def test_skips_existing_git_and_refuses_other_data(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            workspace = Path(directory)
            initialize_repository(workspace / "asc-git")
            (workspace / "asc-data").mkdir()
            runner = ScriptedRunner([])
            results = clone_repositories(
                configuration(workspace),
                [remote("asc-git"), remote("asc-data")],
                [],
                runner,  # type: ignore[arg-type]
            )
        self.assertEqual([item.outcome for item in results], ["failed", "skipped"])
        outcomes = {item.name: item.outcome for item in results}
        self.assertEqual(outcomes, {"asc-data": "failed", "asc-git": "skipped"})
        self.assertEqual(runner.calls, [])

    def test_requested_name_must_exist_remotely(self) -> None:
        with (
            tempfile.TemporaryDirectory() as directory,
            self.assertRaisesRegex(RepositoryError, "not found"),
        ):
            clone_repositories(
                configuration(Path(directory)),
                [remote("asc-one")],
                ["asc-two"],
                ScriptedRunner([]),  # type: ignore[arg-type]
            )

    def test_discovered_name_cannot_escape_workspace(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            unsafe = remote("asc-../outside")
            with self.assertRaisesRegex(RepositoryError, "invalid repository"):
                clone_repositories(
                    configuration(Path(directory)),
                    [unsafe],
                    [],
                    ScriptedRunner([]),  # type: ignore[arg-type]
                )

    def test_partial_clone_failure_is_recorded(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            runner = ScriptedRunner([ProcessError("clone failed"), result()])
            results = clone_repositories(
                configuration(Path(directory)),
                [remote("asc-a"), remote("asc-b")],
                [],
                runner,  # type: ignore[arg-type]
            )
        self.assertEqual([item.outcome for item in results], ["failed", "cloned"])


class SyncTest(unittest.TestCase):
    def test_update_and_unchanged_results(self) -> None:
        paths = [Path("/workspace/asc-a"), Path("/workspace/asc-b")]
        runner = ScriptedRunner(
            [
                result(""),
                result("origin/main\n"),
                result("old\n"),
                result(),
                result(),
                result("new\n"),
                result(""),
                result("origin/main\n"),
                result("same\n"),
                result(),
                result(),
                result("same\n"),
            ]
        )
        results = sync_repositories(paths, runner)  # type: ignore[arg-type]
        self.assertEqual([item.outcome for item in results], ["updated", "unchanged"])
        self.assertIn(
            ("git", "merge", "--ff-only", "origin/main"),
            [call[0] for call in runner.calls],
        )

    def test_dirty_repository_is_skipped(self) -> None:
        runner = ScriptedRunner([result(" M file\n")])
        results = sync_repositories([Path("/workspace/asc-one")], runner)  # type: ignore[arg-type]
        self.assertEqual(results[0].outcome, "skipped")
        self.assertEqual(len(runner.calls), 1)

    def test_divergence_failure_does_not_stop_next_repository(self) -> None:
        runner = ScriptedRunner(
            [
                result(""),
                result("origin/main\n"),
                result("old\n"),
                result(),
                ProcessError("not fast-forward"),
                result(""),
                result("origin/main\n"),
                result("same\n"),
                result(),
                result(),
                result("same\n"),
            ]
        )
        results = sync_repositories(
            [Path("/workspace/asc-bad"), Path("/workspace/asc-good")],
            runner,  # type: ignore[arg-type]
        )
        self.assertEqual([item.outcome for item in results], ["failed", "unchanged"])


if __name__ == "__main__":
    unittest.main()
