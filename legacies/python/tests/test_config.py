"""Configuration loading tests."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
from unittest import mock

from asc_devtools.config import ConfigOverrides, load_config
from asc_devtools.errors import ConfigurationError


class ConfigTest(unittest.TestCase):
    def test_defaults(self) -> None:
        with mock.patch.dict("os.environ", {"HOME": "/home/tester"}):
            config = load_config(environment={})
        self.assertEqual(config.organization, "AI4SciComp")
        self.assertEqual(config.workspace, Path("/home/tester/projects/AI4SciComp"))
        self.assertEqual(config.repository_prefix, "asc-")
        self.assertTrue(config.include_dot_github)
        self.assertEqual(config.clone_protocol, "ssh")

    def test_toml_and_tilde_expansion(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.toml"
            path.write_text(
                """[asc]
organization = "ExampleOrg"
workspace = "~/source"
repository_prefix = "project-"
include_dot_github = false
clone_protocol = "https"
""",
                encoding="utf-8",
            )
            with mock.patch.dict("os.environ", {"HOME": "/home/tester"}):
                config = load_config(path, environment={})
        self.assertEqual(config.organization, "ExampleOrg")
        self.assertEqual(config.workspace, Path("/home/tester/source"))
        self.assertFalse(config.include_dot_github)

    def test_environment_overrides_file(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.toml"
            path.write_text(
                '[asc]\norganization = "FromFile"\nclone_protocol = "ssh"\n',
                encoding="utf-8",
            )
            config = load_config(
                path,
                environment={
                    "ASC_ORGANIZATION": "FromEnvironment",
                    "ASC_CLONE_PROTOCOL": "https",
                },
            )
        self.assertEqual(config.organization, "FromEnvironment")
        self.assertEqual(config.clone_protocol, "https")

    def test_explicit_overrides_environment(self) -> None:
        config = load_config(
            environment={"ASC_ORGANIZATION": "EnvironmentOrg"},
            overrides=ConfigOverrides(organization="CommandOrg"),
        )
        self.assertEqual(config.organization, "CommandOrg")

    def test_environment_selects_configuration_file(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "selected.toml"
            path.write_text('[asc]\norganization = "SelectedOrg"\n', encoding="utf-8")
            config = load_config(environment={"ASC_CONFIG": str(path)})
        self.assertEqual(config.config_path, path.resolve())
        self.assertEqual(config.organization, "SelectedOrg")

    def test_invalid_protocol(self) -> None:
        with self.assertRaisesRegex(ConfigurationError, "clone_protocol"):
            load_config(environment={"ASC_CLONE_PROTOCOL": "ftp"})

    def test_empty_environment_override_is_rejected(self) -> None:
        with self.assertRaisesRegex(ConfigurationError, "organization"):
            load_config(environment={"ASC_ORGANIZATION": ""})

    def test_invalid_value_type(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.toml"
            path.write_text('[asc]\ninclude_dot_github = "yes"\n', encoding="utf-8")
            with self.assertRaisesRegex(ConfigurationError, "true or false"):
                load_config(path, environment={})

    def test_malformed_toml(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.toml"
            path.write_text("[asc\n", encoding="utf-8")
            with self.assertRaisesRegex(ConfigurationError, "could not read"):
                load_config(path, environment={})


if __name__ == "__main__":
    unittest.main()
