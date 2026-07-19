"""Structured doctor tests."""

from __future__ import annotations

import io
import json
import tempfile
import unittest
from pathlib import Path

from asc_devtools.commands.doctor import run_doctor
from asc_devtools.config import Config
from asc_devtools.github import RemoteRepository
from tests.support import ScriptedRunner, result


class FakeClient:
    def list_organization_repositories(
        self, _organization: str
    ) -> tuple[RemoteRepository, ...]:
        return ()


class DoctorTest(unittest.TestCase):
    def test_json_schema_and_warning_policy(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            config = Config(
                Path(directory) / "config.json",
                "AI4SciComp",
                Path(directory),
                "asc-",
                True,
                "ssh",
                "origin",
                "token",
                "ASC_GITHUB_TOKEN",
            )
            runner = ScriptedRunner(
                [
                    result("git version 2.40\n"),
                    result("cmake version 3.28\n"),
                    result("ctest version 3.28\n"),
                    result(stderr="successfully authenticated", code=1),
                ]
            )
            output = io.StringIO()
            code = run_doctor(
                config,
                runner,  # type: ignore[arg-type]
                output,
                json_output=True,
                client=FakeClient(),  # type: ignore[arg-type]
            )
        checks = json.loads(output.getvalue())
        self.assertEqual(code, 0)
        self.assertTrue(
            all({"name", "status", "detail"} <= set(check) for check in checks)
        )
        self.assertEqual(
            next(check for check in checks if check["name"] == "api-authentication")[
                "status"
            ],
            "pass",
        )


if __name__ == "__main__":
    unittest.main()
