"""Environment diagnostic tests."""

from __future__ import annotations

import io
import tempfile
import unittest
from pathlib import Path

from asc_devtools.commands.doctor import run_doctor
from asc_devtools.config import Config
from tests.support import ScriptedRunner, result


class DoctorTest(unittest.TestCase):
    def test_api_and_ssh_authentication_are_reported_separately(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            config = Config(
                Path(directory) / "config.toml",
                "AI4SciComp",
                Path(directory),
                "asc-",
                True,
                "ssh",
            )
            runner = ScriptedRunner(
                [
                    result("git version 2.40\n"),
                    result("gh version 2.40\n"),
                    result("cmake version 3.28\n"),
                    result("ctest version 3.28\n"),
                    result('{"login": "escapetiger"}\n'),
                    result("ssh\n"),
                    result("[]\n"),
                    result(
                        stderr=(
                            "Hi escapetiger! You've successfully authenticated, "
                            "but GitHub does not provide shell access.\n"
                        ),
                        code=1,
                    ),
                ]
            )
            output = io.StringIO()
            code = run_doctor(config, runner, output)  # type: ignore[arg-type]

        self.assertEqual(code, 0)
        report = output.getvalue()
        self.assertIn("GitHub API authentication", report)
        self.assertIn("authenticated as escapetiger", report)
        self.assertIn("Git SSH authentication", report)
        self.assertIn("successfully authenticated", report)
        self.assertEqual(runner.calls[6][0][:4], ("gh", "repo", "list", "AI4SciComp"))


if __name__ == "__main__":
    unittest.main()
