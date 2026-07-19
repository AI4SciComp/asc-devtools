"""System-prefix lifecycle tests."""

from __future__ import annotations

import subprocess
import tempfile
import unittest
from pathlib import Path


class InstallTest(unittest.TestCase):
    def test_install_upgrade_uninstall_and_modified_refusal(self) -> None:
        project = Path(__file__).resolve().parent.parent
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            command = [
                str(project / "scripts" / "install.sh"),
                "--destdir",
                str(root),
            ]
            subprocess.run(command, check=True, capture_output=True, text=True)
            subprocess.run(command, check=True, capture_output=True, text=True)
            asc = root / "usr" / "local" / "bin" / "asc"
            completed = subprocess.run(
                [str(asc), "--version"], check=True, capture_output=True, text=True
            )
            self.assertIn("asc 0.1.0", completed.stdout)
            asc.write_bytes(asc.read_bytes() + b"\nmodified\n")
            refused = subprocess.run(
                [str(project / "scripts" / "uninstall.sh"), "--destdir", str(root)],
                check=False,
                capture_output=True,
                text=True,
            )
            self.assertNotEqual(refused.returncode, 0)
            self.assertTrue(asc.exists())

    def test_clean_uninstall(self) -> None:
        project = Path(__file__).resolve().parent.parent
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            subprocess.run(
                [str(project / "scripts" / "install.sh"), "--destdir", str(root)],
                check=True,
                capture_output=True,
                text=True,
            )
            subprocess.run(
                [str(project / "scripts" / "uninstall.sh"), "--destdir", str(root)],
                check=True,
                capture_output=True,
                text=True,
            )
            self.assertFalse((root / "usr" / "local" / "bin" / "asc").exists())


if __name__ == "__main__":
    unittest.main()
