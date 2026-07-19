"""CMake parity tests."""

from __future__ import annotations

import tempfile
import unittest
from io import StringIO
from pathlib import Path

from asc_devtools.commands.cmake import list_presets, run_cmake_command, run_workflow
from asc_devtools.config import Config
from asc_devtools.errors import RepositoryError
from tests.support import ScriptedRunner, result


def setup(directory: str) -> tuple[Config, Path]:
    workspace = Path(directory)
    repository = workspace / "asc-cpp"
    (repository / ".git").mkdir(parents=True)
    (repository / "CMakePresets.json").write_text("{}", encoding="utf-8")
    (repository / "CMakeLists.txt").write_text(
        "cmake_minimum_required(VERSION 3.25)\n", encoding="utf-8"
    )
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
                expected_list = {
                    "configure": ("cmake", "--list-presets"),
                    "build": ("cmake", "--list-presets=build"),
                    "test": ("ctest", "--list-presets"),
                }[operation]
                self.assertEqual(runner.calls[1][0], expected_list)
                self.assertEqual(runner.calls[2][0], expected)
                self.assertEqual(runner.calls[2][1], repository)

    def test_preset_is_required(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            config, _ = setup(directory)
            with self.assertRaisesRegex(RepositoryError, "required"):
                run_cmake_command(config, ScriptedRunner([]), "build", "asc-cpp", "")  # type: ignore[arg-type]

    def test_grouped_arguments_workflow_and_listing(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            config, repository = setup(directory)
            runner = ScriptedRunner([result("true\n"), result('  "dev"\n'), result()])
            run_cmake_command(
                config,
                runner,  # type: ignore[arg-type]
                "test",
                "asc-cpp",
                "dev",
                label="unit-fast",
                output_on_failure=True,
            )
            self.assertEqual(
                runner.calls[-1][0],
                ("ctest", "--preset", "dev", "--output-on-failure", "-L", "unit-fast"),
            )
            self.assertEqual(runner.calls[-1][1], repository)

            workflow_runner = ScriptedRunner(
                [
                    result("true\n"),
                    result("true\n"),
                    result('"dev"\n'),
                    result(),
                    result("true\n"),
                    result('"dev"\n'),
                    result(),
                    result("true\n"),
                    result('"dev"\n'),
                    result(),
                ]
            )
            report = StringIO()
            run_workflow(
                config,
                workflow_runner,  # type: ignore[arg-type]
                "asc-cpp",
                "dev",
                "dev",
                "dev",
                report,
            )
            self.assertIn("configure: cmake --preset dev", report.getvalue())
            self.assertIn("test: ctest --preset dev", report.getvalue())

            list_runner = ScriptedRunner(
                [
                    result("true\n"),
                    result('"release"\n"dev"\n'),
                    result('"dev"\n'),
                    result('"test"\n'),
                ]
            )
            listed = list_presets(config, list_runner, "asc-cpp")  # type: ignore[arg-type]
            self.assertEqual(listed["configure"], ["dev", "release"])


if __name__ == "__main__":
    unittest.main()
