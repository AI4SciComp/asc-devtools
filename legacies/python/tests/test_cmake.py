"""CMake wrapper tests."""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from asc_devtools.commands.cmake import run_cmake_command
from asc_devtools.config import Config
from asc_devtools.errors import ProcessError, RepositoryError
from tests.support import ScriptedRunner, result


def setup_repository(directory: str) -> tuple[Config, Path]:
    workspace = Path(directory)
    repository = workspace / "asc-cpp"
    (repository / ".git").mkdir(parents=True)
    (repository / "CMakePresets.json").write_text(
        json.dumps(
            {
                "version": 6,
                "configurePresets": [{"name": "dev"}],
                "buildPresets": [{"name": "dev"}],
                "testPresets": [{"name": "dev"}],
            }
        ),
        encoding="utf-8",
    )
    return Config(
        Path("/config"), "AI4SciComp", workspace, "asc-", True, "ssh"
    ), repository


class CMakeTest(unittest.TestCase):
    def test_correct_commands_and_working_directory(self) -> None:
        cases = {
            "configure": ("cmake", "--preset", "dev"),
            "build": ("cmake", "--build", "--preset", "dev", "--parallel"),
            "test": ("ctest", "--preset", "dev", "--output-on-failure"),
        }
        for operation, expected in cases.items():
            with (
                self.subTest(operation=operation),
                tempfile.TemporaryDirectory() as directory,
            ):
                config, repository = setup_repository(directory)
                runner = ScriptedRunner([result()])
                run_cmake_command(config, runner, operation, "asc-cpp", "dev")  # type: ignore[arg-type]
                self.assertEqual(runner.calls[0][0], expected)
                self.assertEqual(runner.calls[0][1], repository)
                self.assertFalse(runner.calls[0][2])

    def test_missing_repository(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            config = Config(
                Path("/config"), "AI4SciComp", Path(directory), "asc-", True, "ssh"
            )
            with self.assertRaisesRegex(RepositoryError, "not found"):
                run_cmake_command(config, ScriptedRunner([]), "build", "asc-cpp", "dev")  # type: ignore[arg-type]

    def test_missing_preset(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            config, _ = setup_repository(directory)
            with self.assertRaisesRegex(RepositoryError, "missing"):
                run_cmake_command(
                    config, ScriptedRunner([]), "build", "asc-cpp", "missing"
                )  # type: ignore[arg-type]

    def test_command_failure_propagates(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            config, _ = setup_repository(directory)
            with self.assertRaisesRegex(ProcessError, "build failed"):
                run_cmake_command(
                    config,
                    ScriptedRunner([ProcessError("build failed")]),  # type: ignore[arg-type]
                    "build",
                    "asc-cpp",
                    "dev",
                )


if __name__ == "__main__":
    unittest.main()
