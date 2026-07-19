"""Direct GitHub REST client tests."""

from __future__ import annotations

import json
import unittest

from asc_devtools.errors import AscError
from asc_devtools.github import GitHubClient


def record(name: str) -> dict[str, object]:
    return {
        "name": name,
        "archived": False,
        "fork": False,
        "clone_url": f"https://github.com/AI4SciComp/{name}.git",
        "ssh_url": f"git@github.com:AI4SciComp/{name}.git",
        "default_branch": "main",
        "private": False,
    }


class GitHubTest(unittest.TestCase):
    def test_headers_and_pagination(self) -> None:
        calls: list[tuple[str, dict[str, str]]] = []
        first = [record(f"asc-{index:03}") for index in range(100)]

        def transport(url: str, headers: object) -> tuple[int, dict[str, str], bytes]:
            copied = dict(headers)  # type: ignore[arg-type]
            calls.append((url, copied))
            if url.endswith("page=1"):
                return 200, {"Link": '<next>; rel="next"'}, json.dumps(first).encode()
            return 200, {}, json.dumps([record("asc-last")]).encode()

        repositories = GitHubClient(
            "secret", transport=transport
        ).list_organization_repositories("AI4SciComp")
        self.assertEqual(len(repositories), 101)
        self.assertEqual(calls[0][1]["Authorization"], "Bearer secret")
        self.assertIn("per_page=100&page=2", calls[1][0])

    def test_known_errors_do_not_echo_response_bodies(self) -> None:
        def transport(_url: str, _headers: object) -> tuple[int, dict[str, str], bytes]:
            return 401, {}, b"secret server body"

        with self.assertRaises(AscError) as context:
            GitHubClient(transport=transport).list_organization_repositories(
                "AI4SciComp"
            )
        self.assertIn("authentication failed", str(context.exception))
        self.assertNotIn("secret server body", str(context.exception))

    def test_malformed_and_oversized_responses(self) -> None:
        with self.assertRaisesRegex(AscError, "decode"):
            GitHubClient(
                transport=lambda _url, _headers: (200, {}, b"not-json")
            ).list_organization_repositories("AI4SciComp")


if __name__ == "__main__":
    unittest.main()
