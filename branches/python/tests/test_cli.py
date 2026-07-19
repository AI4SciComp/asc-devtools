"""Black-box command contract tests."""

from __future__ import annotations

import io
import json
import os
import tempfile
import unittest
from collections.abc import Mapping
from pathlib import Path
from unittest import mock

from asc_devtools.cli import main
from asc_devtools.github import GitHubClient
from asc_devtools.process import CommandRunner
from tests.support import initialize_repository


def api_record(name: str) -> dict[str, object]:
    return {
        "name": name,
        "archived": False,
        "fork": False,
        "clone_url": f"https://github.com/AI4SciComp/{name}.git",
        "ssh_url": f"git@github.com:AI4SciComp/{name}.git",
        "default_branch": "main",
        "private": False,
    }


def client_factory(_token: str) -> GitHubClient:
    return GitHubClient(
        transport=lambda _url, _headers: (
            200,
            {},
            json.dumps([api_record("asc-one")]).encode(),
        )
    )


class CliTest(unittest.TestCase):
    def test_help_version_completion_and_usage_exit(self) -> None:
        for arguments, expected in (
            (["--help"], "repo sync"),
            (["--version"], "asc 0.1.0"),
            (["repo", "status", "--help"], "--json"),
            (["repo", "save", "--help"], "--message"),
            (["completion", "bash"], "complete -F"),
            (["cmake", "--help"], "workflow"),
            (["cmake", "vendor", "--help"], "status|plan|apply"),
            (["update", "--help"], "--check"),
        ):
            with self.subTest(arguments=arguments):
                output = io.StringIO()
                self.assertEqual(main(arguments, output=output), 0)
                self.assertIn(expected, output.getvalue())
        errors = io.StringIO()
        self.assertEqual(main(["build", "asc-cpp"], error_output=errors), 2)
        self.assertIn("--preset", errors.getvalue())
        errors = io.StringIO()
        self.assertEqual(main(["repo", "update"], error_output=errors), 2)
        self.assertIn("invalid choice", errors.getvalue())
        errors = io.StringIO()

        def missing_release(
            _url: str, _headers: Mapping[str, str], _limit: int
        ) -> tuple[int, Mapping[str, str], bytes]:
            return 404, {}, b""

        with mock.patch.dict(os.environ, {"ASC_GITHUB_TOKEN": "test"}):
            self.assertEqual(
                main(
                    ["update", "--check"],
                    error_output=errors,
                    update_transport=missing_release,
                ),
                1,
            )
        self.assertIn("no published asc release", errors.getvalue())

    def test_workspace_is_local_and_resolved(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            output = io.StringIO()
            code = main(["--workspace", directory, "workspace"], output=output)
        self.assertEqual(code, 0)
        self.assertEqual(output.getvalue().strip(), str(Path(directory).resolve()))

    def test_repo_list_json_matches_go_schema(self) -> None:
        output = io.StringIO()
        with mock.patch.dict(os.environ, {"ASC_GITHUB_TOKEN": "test"}):
            code = main(
                ["repo", "list", "--json"],
                output=output,
                github_client_factory=client_factory,
            )
        record = json.loads(output.getvalue())[0]
        self.assertEqual(code, 0)
        self.assertEqual(
            set(record),
            {
                "name",
                "archived",
                "fork",
                "clone_url",
                "ssh_url",
                "default_branch",
                "private",
            },
        )

    def test_status_json_continues_after_missing_repository(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            workspace = Path(directory)
            initialize_repository(workspace / "asc-good")
            output = io.StringIO()
            code = main(
                [
                    "--workspace",
                    directory,
                    "repo",
                    "status",
                    "asc-missing",
                    "asc-good",
                    "--json",
                ],
                output=output,
                runner=CommandRunner(),
            )
        statuses = json.loads(output.getvalue())
        self.assertEqual(code, 1)
        self.assertIn("error", statuses[0])
        self.assertTrue(statuses[1]["clean"])


if __name__ == "__main__":
    unittest.main()
