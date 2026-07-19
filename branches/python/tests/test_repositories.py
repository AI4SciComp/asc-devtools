"""Repository safety and Git behavior tests."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from asc_devtools.config import Config
from asc_devtools.github import RemoteRepository
from asc_devtools.process import CommandRunner
from asc_devtools.repositories import (
    apply_save_repository,
    clone_repositories,
    discover_local_repositories,
    inspect_repositories,
    plan_save_repository,
    sync_repositories,
)
from tests.support import ScriptedRunner, git, initialize_repository, result


def configuration(workspace: Path) -> Config:
    return Config(
        workspace / "config.json",
        "AI4SciComp",
        workspace,
        "asc-",
        True,
        "ssh",
        "origin",
    )


def remote(name: str) -> RemoteRepository:
    return RemoteRepository(
        name,
        False,
        False,
        f"https://github.com/AI4SciComp/{name}.git",
        f"git@github.com:AI4SciComp/{name}.git",
        "main",
        False,
    )


class RepositoryTest(unittest.TestCase):
    def test_direct_child_discovery_rejects_symlinks_and_nested_repositories(
        self,
    ) -> None:
        with tempfile.TemporaryDirectory() as directory:
            workspace = Path(directory)
            initialize_repository(workspace / "asc-one")
            nested = workspace / "container"
            nested.mkdir()
            initialize_repository(nested / "asc-nested")
            (workspace / "asc-link").symlink_to(
                workspace / "asc-one", target_is_directory=True
            )
            found = discover_local_repositories(
                configuration(workspace), CommandRunner()
            )
        self.assertEqual([path.name for path in found], ["asc-one"])

    def test_status_uses_porcelain_v2_and_counts_changes(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            workspace = Path(directory)
            repository = workspace / "asc-one"
            initialize_repository(repository)
            (repository / "untracked.txt").write_text("new\n", encoding="utf-8")
            statuses = inspect_repositories(
                configuration(workspace), [], CommandRunner()
            )
        self.assertEqual(statuses[0].name, "asc-one")
        self.assertFalse(statuses[0].clean)
        self.assertEqual(statuses[0].changes, 1)

    def test_clone_uses_api_url_and_absolute_destination(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            workspace = Path(directory) / "workspace"
            runner = ScriptedRunner([result()])
            results = clone_repositories(
                configuration(workspace),
                [remote("asc-one")],
                [],
                "ssh",
                runner,  # type: ignore[arg-type]
            )
        self.assertEqual(results[0].outcome, "cloned")
        self.assertEqual(
            runner.calls[0][0],
            (
                "git",
                "clone",
                "--",
                "git@github.com:AI4SciComp/asc-one.git",
                str(workspace / "asc-one"),
            ),
        )

    def test_dry_run_validates_but_does_not_fetch_or_merge(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            workspace = Path(directory)
            repository = workspace / "asc-one"
            initialize_repository(repository)
            branch = git(repository, "branch", "--show-current")
            head = git(repository, "rev-parse", "HEAD")
            git(
                repository,
                "remote",
                "add",
                "origin",
                "https://example.invalid/asc-one.git",
            )
            git(repository, "update-ref", f"refs/remotes/origin/{branch}", head)
            git(repository, "config", f"branch.{branch}.remote", "origin")
            git(repository, "config", f"branch.{branch}.merge", f"refs/heads/{branch}")
            results = sync_repositories(
                configuration(workspace), [], CommandRunner(), dry_run=True
            )
        self.assertEqual(results[0].outcome, "planned")
        self.assertIn("fetch -- origin", results[0].plan[0])
        self.assertIn("merge --ff-only", results[0].plan[1])

    def test_dirty_repository_is_skipped(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            workspace = Path(directory)
            repository = workspace / "asc-one"
            initialize_repository(repository)
            (repository / "dirty").write_text("dirty", encoding="utf-8")
            results = sync_repositories(configuration(workspace), [], CommandRunner())
        self.assertEqual(results[0].outcome, "skipped")

    def test_save_commits_pushes_and_refuses_remote_ahead(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bare = root / "remote.git"
            seed = root / "seed"
            workspace = root / "workspace"
            git(root, "init", "-q", "--bare", str(bare))
            initialize_repository(seed)
            git(seed, "remote", "add", "origin", str(bare))
            git(seed, "push", "-qu", "origin", "HEAD")
            workspace.mkdir()
            repository = workspace / "asc-one"
            git(root, "clone", "-q", str(bare), str(repository))
            git(repository, "config", "user.name", "Asc Tests")
            git(repository, "config", "user.email", "asc-tests@example.invalid")
            (repository / "saved.txt").write_text("saved\n", encoding="utf-8")
            config = configuration(workspace)
            runner = CommandRunner()
            plan = plan_save_repository(
                config, "asc-one", "Save local work", runner
            )
            self.assertEqual(plan.changes, 1)
            self.assertEqual(len(plan.operation.plan), 4)
            result = apply_save_repository(config, plan, runner)
            self.assertEqual(result.outcome, "saved")
            self.assertIn("committed and pushed", result.detail)
            self.assertEqual(git(bare, "log", "-1", "--format=%s"), "Save local work")

            git(seed, "pull", "-q", "--ff-only")
            (seed / "remote.txt").write_text("remote\n", encoding="utf-8")
            git(seed, "add", "remote.txt")
            git(seed, "commit", "-qm", "remote update")
            git(seed, "push", "-q")
            (repository / "not-saved.txt").write_text("local\n", encoding="utf-8")
            plan = plan_save_repository(config, "asc-one", "Must not commit", runner)
            result = apply_save_repository(config, plan, runner)
            self.assertEqual(result.outcome, "skipped")
            self.assertIn("reconcile them manually", result.detail)
            self.assertNotEqual(
                git(repository, "log", "-1", "--format=%s"), "Must not commit"
            )


if __name__ == "__main__":
    unittest.main()
