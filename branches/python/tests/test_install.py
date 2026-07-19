"""System-prefix lifecycle tests."""

from __future__ import annotations

import json
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
            manifest = (
                root
                / "usr"
                / "local"
                / "share"
                / "asc-devtools"
                / "install-manifest.json"
            )
            document = json.loads(manifest.read_text(encoding="utf-8"))
            prior_file = "lib/asc-devtools/asc_devtools/selfupdate.py"
            document["files"].pop(prior_file)
            manifest.write_text(json.dumps(document, indent=2) + "\n", encoding="utf-8")
            (root / "usr" / "local" / prior_file).unlink()
            subprocess.run(command, check=True, capture_output=True, text=True)
            self.assertTrue((root / "usr" / "local" / prior_file).is_file())
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
