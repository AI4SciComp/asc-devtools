"""CLI routing and presentation tests."""

from __future__ import annotations

import io
import json
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path

from asc_devtools.cli import _parser, main
from asc_devtools.errors import ProcessError
from tests.support import ScriptedRunner, result


class CliTest(unittest.TestCase):
    def test_help_and_version(self) -> None:
        for arguments, expected in (
            (["--help"], "Safely manage"),
            (["--version"], "asc 0.1.0"),
        ):
            with self.subTest(arguments=arguments):
                output = io.StringIO()
                with redirect_stdout(output), self.assertRaises(SystemExit) as context:
                    _parser().parse_args(arguments)
                self.assertEqual(context.exception.code, 0)
                self.assertIn(expected, output.getvalue())

    def test_workspace_routing(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            output = io.StringIO()
            code = main(["--workspace", directory, "workspace"], output=output)
        self.assertEqual(code, 0)
        self.assertEqual(output.getvalue().strip(), str(Path(directory).resolve()))

    def test_repo_list_json(self) -> None:
        records = [
            {
                "name": "asc-one",
                "isArchived": False,
                "sshUrl": "git@example",
                "url": "https://example",
                "description": "One",
            }
        ]
        output = io.StringIO()
        code = main(
            ["repo", "list", "--json"],
            output=output,
            runner=ScriptedRunner([result(json.dumps(records))]),  # type: ignore[arg-type]
        )
        self.assertEqual(code, 0)
        self.assertEqual(json.loads(output.getvalue())[0]["name"], "asc-one")

    def test_user_facing_error_has_no_traceback(self) -> None:
        errors = io.StringIO()
        code = main(
            ["repo", "list"],
            error_output=errors,
            runner=ScriptedRunner([ProcessError("gh failed")]),  # type: ignore[arg-type]
        )
        self.assertEqual(code, 1)
        self.assertEqual(errors.getvalue(), "asc: error: gh failed\n")

    def test_status_continues_after_missing_repository(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            workspace = Path(directory)
            repository = workspace / "asc-good"
            (repository / ".git").mkdir(parents=True)
            output = io.StringIO()
            runner = ScriptedRunner([result("# branch.head main\n")])
            code = main(
                ["--workspace", directory, "repo", "status", "asc-missing", "asc-good"],
                output=output,
                runner=runner,  # type: ignore[arg-type]
            )
        self.assertEqual(code, 1)
        self.assertIn("asc-missing: failed", output.getvalue())
        self.assertIn("asc-good: main; clean", output.getvalue())

    def test_sync_summary_and_nonzero_for_dirty_skip(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            repository = Path(directory) / "asc-dirty"
            (repository / ".git").mkdir(parents=True)
            output = io.StringIO()
            code = main(
                ["--workspace", directory, "repo", "sync"],
                output=output,
                runner=ScriptedRunner([result(" M tracked.txt\n")]),  # type: ignore[arg-type]
            )
        self.assertEqual(code, 1)
        self.assertIn("skipped=1", output.getvalue())


if __name__ == "__main__":
    unittest.main()
