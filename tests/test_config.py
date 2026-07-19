"""Canonical JSON configuration tests."""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from asc_devtools.config import ConfigOverrides, load_config
from asc_devtools.errors import ConfigurationError


class ConfigTest(unittest.TestCase):
    def test_defaults(self) -> None:
        config = load_config(
            environment={}, home=Path("/home/tester"), cwd=Path("/work")
        )
        self.assertEqual(
            config.config_path, Path("/home/tester/.config/asc/config.json")
        )
        self.assertEqual(config.workspace, Path("/home/tester/projects/AI4SciComp"))
        self.assertEqual(config.remote, "origin")
        self.assertEqual(config.cmake.vendor_directory, "cmake/asc")
        self.assertEqual(config.cmake.source_repository, "asc-cmake")

    def test_json_environment_and_cli_precedence(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.json"
            path.write_text(
                json.dumps(
                    {
                        "organization": "FileOrg",
                        "workspace": "~/file-workspace",
                        "repositoryPrefix": "project-",
                        "includeDotGitHub": False,
                        "cloneProtocol": "ssh",
                        "remote": "upstream",
                    }
                ),
                encoding="utf-8",
            )
            config = load_config(
                path,
                environment={
                    "ASC_ORGANIZATION": "EnvironmentOrg",
                    "ASC_CLONE_PROTOCOL": "https",
                    "ASC_INCLUDE_DOT_GITHUB": "true",
                },
                overrides=ConfigOverrides(
                    organization="CommandOrg", workspace="relative"
                ),
                home=Path("/home/tester"),
                cwd=Path("/work"),
            )
        self.assertEqual(config.organization, "CommandOrg")
        self.assertEqual(config.workspace, Path("/work/relative"))
        self.assertEqual(config.repository_prefix, "project-")
        self.assertTrue(config.include_dot_github)
        self.assertEqual(config.clone_protocol, "https")
        self.assertEqual(config.remote, "upstream")

    def test_token_precedence(self) -> None:
        config = load_config(
            environment={
                "ASC_GITHUB_TOKEN": "asc-token",
                "GH_TOKEN": "gh-token",
                "GITHUB_TOKEN": "github-token",
            },
            home=Path("/home/tester"),
        )
        self.assertEqual(config.github_token, "asc-token")
        self.assertEqual(config.github_token_source, "ASC_GITHUB_TOKEN")

    def test_unknown_field_and_invalid_types_are_rejected(self) -> None:
        for document, pattern in (
            ({"unknown": True}, "unknown configuration field"),
            ({"includeDotGitHub": "yes"}, "true or false"),
        ):
            with (
                self.subTest(document=document),
                tempfile.TemporaryDirectory() as directory,
            ):
                path = Path(directory) / "config.json"
                path.write_text(json.dumps(document), encoding="utf-8")
                with self.assertRaisesRegex(ConfigurationError, pattern):
                    load_config(path, environment={})

    def test_root_workspace_and_bad_environment_boolean_are_rejected(self) -> None:
        with self.assertRaisesRegex(ConfigurationError, "filesystem root"):
            load_config(environment={"ASC_WORKSPACE": "/"})
        with self.assertRaisesRegex(ConfigurationError, "must be 'true' or 'false'"):
            load_config(environment={"ASC_INCLUDE_DOT_GITHUB": "yes"})

    def test_cmake_configuration_is_nested_and_strict(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.json"
            path.write_text(
                json.dumps(
                    {
                        "cmake": {
                            "vendorDirectory": "vendor/asc",
                            "sourceRepository": "asc-cmake-local",
                        }
                    }
                ),
                encoding="utf-8",
            )
            config = load_config(path, environment={})
            self.assertEqual(config.cmake.vendor_directory, "vendor/asc")
            self.assertEqual(config.cmake.source_repository, "asc-cmake-local")
            for value in (
                {"cmake": {"unknown": True}},
                {"cmake": {"vendorDirectory": "../escape"}},
            ):
                path.write_text(json.dumps(value), encoding="utf-8")
                with self.assertRaises(ConfigurationError):
                    load_config(path, environment={})


if __name__ == "__main__":
    unittest.main()
