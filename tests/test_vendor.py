"""Local asc-cmake vendoring safety tests."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from asc_devtools.config import Config
from asc_devtools.errors import RepositoryError
from asc_devtools.process import CommandRunner
from asc_devtools.vendor import (
    _decode_manifest,
    _manifest_bytes,
    vendor_apply,
    vendor_plan,
    vendor_status,
)
from tests.support import git, initialize_repository


class VendorTest(unittest.TestCase):
    def setup_workspace(self, directory: str) -> tuple[Config, Path, Path]:
        workspace = Path(directory) / "workspace with spaces"
        workspace.mkdir()
        source = workspace / "asc-cmake"
        consumer = workspace / "asc-cpp"
        initialize_repository(source)
        initialize_repository(consumer)
        (source / "modules").mkdir()
        (source / "modules" / "ASCWarnings.cmake").write_text(
            "message(STATUS warnings)\n", encoding="utf-8"
        )
        (source / "LICENSE").write_text("Apache-2.0\n", encoding="utf-8")
        (source / "VERSION").write_text("1.2.3\n", encoding="utf-8")
        git(source, "add", ".")
        git(source, "commit", "-qm", "distribution")
        return (
            Config(
                Path("/config"),
                "AI4SciComp",
                workspace,
                "asc-",
                True,
                "ssh",
                "origin",
            ),
            source,
            consumer,
        )

    def test_manifest_is_deterministic_and_strict(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            config, _, _ = self.setup_workspace(directory)
            runner = CommandRunner()
            plan = vendor_plan(config, runner, "asc-cpp")
            data = _manifest_bytes(plan.manifest)
            decoded = _decode_manifest(data)
            self.assertEqual(decoded.files[0].path, "LICENSE")
            self.assertTrue(data.endswith(b"\n"))
            with self.assertRaises(RepositoryError):
                _decode_manifest(data.rstrip() + b',"unknown":true}')

    def test_plan_apply_status_and_local_modification(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            config, source, consumer = self.setup_workspace(directory)
            runner = CommandRunner()
            self.assertEqual(
                vendor_status(config, runner, "asc-cpp").state, "not-vendored"
            )
            plan = vendor_plan(config, runner, "asc-cpp")
            self.assertEqual(
                [(action.action, action.path) for action in plan.actions],
                [("add", "LICENSE"), ("add", "modules/ASCWarnings.cmake")],
            )
            vendor_apply(config, runner, plan)
            self.assertEqual(vendor_status(config, runner, "asc-cpp").state, "current")
            target = consumer / "cmake" / "asc"
            (target / "notes.txt").write_text("unmanaged\n", encoding="utf-8")
            status = vendor_status(config, runner, "asc-cpp")
            self.assertEqual(status.extra_files, ("notes.txt",))
            managed = target / "modules" / "ASCWarnings.cmake"
            managed.write_text("local edit\n", encoding="utf-8")
            self.assertEqual(
                vendor_status(config, runner, "asc-cpp").state, "locally-modified"
            )
            with self.assertRaisesRegex(RepositoryError, "locally modified"):
                vendor_plan(config, runner, "asc-cpp")
            managed.write_text("message(STATUS warnings)\n", encoding="utf-8")

            (source / "modules" / "ASCWarnings.cmake").write_text(
                "message(STATUS newer)\n", encoding="utf-8"
            )
            git(source, "add", ".")
            git(source, "commit", "-qm", "new source")
            self.assertEqual(
                vendor_status(config, runner, "asc-cpp").state, "source-newer"
            )
            updated = vendor_plan(config, runner, "asc-cpp")
            self.assertIn(
                ("replace", "modules/ASCWarnings.cmake"),
                [(action.action, action.path) for action in updated.actions],
            )
            vendor_apply(config, runner, updated)
            self.assertTrue((target / "notes.txt").exists())

            (source / "modules" / "ASCWarnings.cmake").unlink()
            git(source, "add", "-A")
            git(source, "commit", "-qm", "remove source file")
            removal = vendor_plan(config, runner, "asc-cpp")
            self.assertIn(
                ("remove", "modules/ASCWarnings.cmake"),
                [(action.action, action.path) for action in removal.actions],
            )
            vendor_apply(config, runner, removal)
            self.assertFalse(managed.exists())
            self.assertTrue((target / "notes.txt").exists())

    def test_apply_refuses_dirty_source_and_symlinks(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            config, source, _ = self.setup_workspace(directory)
            runner = CommandRunner()
            plan = vendor_plan(config, runner, "asc-cpp")
            (source / "VERSION").write_text("dirty\n", encoding="utf-8")
            with self.assertRaises(RepositoryError):
                vendor_apply(config, runner, plan)
            git(source, "restore", "VERSION")
            (source / "modules" / "linked.cmake").symlink_to(source / "LICENSE")
            with self.assertRaisesRegex(RepositoryError, "symlink"):
                vendor_plan(config, runner, "asc-cpp")


if __name__ == "__main__":
    unittest.main()
