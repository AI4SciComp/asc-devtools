"""CMake parity tests."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from asc_devtools.commands.cmake import run_cmake_command
from asc_devtools.config import Config
from asc_devtools.errors import RepositoryError
from tests.support import ScriptedRunner, result


def setup(directory: str) -> tuple[Config, Path]:
    workspace = Path(directory)
    repository = workspace / "asc-cpp"
    (repository / ".git").mkdir(parents=True)
    (repository / "CMakePresets.json").write_text("{}", encoding="utf-8")
    return (
        Config(Path("/config"), "AI4SciComp", workspace, "asc-", True, "ssh", "origin"),
        repository,
    )


class CMakeTest(unittest.TestCase):
    def test_exact_commands(self) -> None:
        cases = {
            "configure": ("cmake", "--preset", "dev"),
            "build": ("cmake", "--build", "--preset", "dev"),
            "test": ("ctest", "--preset", "dev"),
        }
        for operation, expected in cases.items():
            with (
                self.subTest(operation=operation),
                tempfile.TemporaryDirectory() as directory,
            ):
                config, repository = setup(directory)
                runner = ScriptedRunner(
                    [result("true\n"), result('  "dev"\n'), result()]
                )
                run_cmake_command(config, runner, operation, "asc-cpp", "dev")  # type: ignore[arg-type]
                self.assertEqual(
                    runner.calls[1][0], ("cmake", f"--list-presets={operation}")
                )
                self.assertEqual(runner.calls[2][0], expected)
                self.assertEqual(runner.calls[2][1], repository)

    def test_preset_is_required(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            config, _ = setup(directory)
            with self.assertRaisesRegex(RepositoryError, "required"):
                run_cmake_command(config, ScriptedRunner([]), "build", "asc-cpp", "")  # type: ignore[arg-type]


if __name__ == "__main__":
    unittest.main()
