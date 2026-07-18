"""GitHub discovery tests."""

from __future__ import annotations

import json
import unittest
from pathlib import Path

from asc_devtools.config import Config
from asc_devtools.errors import AscError, ProcessError
from asc_devtools.github import discover_repositories
from tests.support import ScriptedRunner, result


def configuration(include_dot_github: bool = True) -> Config:
    return Config(
        Path("/tmp/config.toml"),
        "AI4SciComp",
        Path("/tmp/workspace"),
        "asc-",
        include_dot_github,
        "ssh",
    )


class GithubTest(unittest.TestCase):
    def test_filters_and_sorts_repositories(self) -> None:
        records = [
            {"name": "other", "isArchived": False, "sshUrl": "s", "url": "u"},
            {"name": "asc-z", "isArchived": False, "sshUrl": "s", "url": "u"},
            {"name": ".github", "isArchived": False, "sshUrl": "s", "url": "u"},
            {"name": "asc-old", "isArchived": True, "sshUrl": "s", "url": "u"},
            {"name": "asc-a", "isArchived": False, "sshUrl": "s", "url": "u"},
        ]
        runner = ScriptedRunner([result(json.dumps(records))])
        repositories = discover_repositories(configuration(), runner)  # type: ignore[arg-type]
        self.assertEqual(
            [repository.name for repository in repositories],
            [".github", "asc-a", "asc-z"],
        )
        self.assertEqual(
            runner.calls[0][0][:5], ("gh", "repo", "list", "AI4SciComp", "--limit")
        )

    def test_excludes_dot_github_when_disabled(self) -> None:
        record = [{"name": ".github", "isArchived": False, "sshUrl": "s", "url": "u"}]
        runner = ScriptedRunner([result(json.dumps(record))])
        self.assertEqual(discover_repositories(configuration(False), runner), ())  # type: ignore[arg-type]

    def test_malformed_json(self) -> None:
        runner = ScriptedRunner([result("not json")])
        with self.assertRaisesRegex(AscError, "malformed"):
            discover_repositories(configuration(), runner)  # type: ignore[arg-type]

    def test_failed_gh_command_propagates(self) -> None:
        runner = ScriptedRunner([ProcessError("gh unavailable")])
        with self.assertRaisesRegex(ProcessError, "unavailable"):
            discover_repositories(configuration(), runner)  # type: ignore[arg-type]


if __name__ == "__main__":
    unittest.main()
