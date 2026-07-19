"""Verified self-update tests with no live network or installation writes."""

from __future__ import annotations

import hashlib
import io
import json
import tarfile
import tempfile
import unittest
from collections.abc import Mapping, Sequence
from pathlib import Path

from asc_devtools.errors import AscError
from asc_devtools.selfupdate import ARCHIVE_NAME, _extract, run_update


def archive(files: dict[str, tuple[bytes, int]]) -> bytes:
    result = io.BytesIO()
    with tarfile.open(fileobj=result, mode="w:gz") as bundle:
        for name, (body, mode) in files.items():
            member = tarfile.TarInfo(name)
            member.size = len(body)
            member.mode = mode
            bundle.addfile(member, io.BytesIO(body))
    return result.getvalue()


class SelfUpdateTest(unittest.TestCase):
    def test_check_and_verified_install(self) -> None:
        bundle = archive(
            {
                "asc-devtools-python/scripts/install.sh": (
                    b"#!/usr/bin/env bash\n",
                    0o755,
                ),
                "asc-devtools-python/src/asc_devtools/__init__.py": (
                    b'__version__ = "0.2.0"\n',
                    0o644,
                ),
            }
        )
        checksum = hashlib.sha256(bundle).hexdigest()
        release = json.dumps(
            {
                "tag_name": "v0.2.0",
                "assets": [
                    {"name": ARCHIVE_NAME, "url": "https://test/archive"},
                    {"name": "SHA256SUMS", "url": "https://test/sums"},
                ],
            }
        ).encode()

        def transport(
            url: str, _headers: Mapping[str, str], _limit: int
        ) -> tuple[int, Mapping[str, str], bytes]:
            if url.endswith("/releases/latest"):
                return 200, {}, release
            if url.endswith("/archive"):
                return 200, {}, bundle
            if url.endswith("/sums"):
                return 200, {}, f"{checksum}  {ARCHIVE_NAME}\n".encode()
            return 404, {}, b""

        output = io.StringIO()
        run_update(
            current_version="0.1.0",
            check_only=True,
            output=output,
            transport=transport,
        )
        self.assertIn("0.2.0 is available", output.getvalue())

        with tempfile.TemporaryDirectory() as directory:
            prefix = Path(directory)
            manifest = prefix / "share" / "asc-devtools" / "install-manifest.json"
            manifest.parent.mkdir(parents=True)
            manifest.write_text("{}\n", encoding="utf-8")
            calls: list[Sequence[str]] = []
            output = io.StringIO()
            run_update(
                current_version="0.1.0",
                prefix=str(prefix),
                assume_yes=True,
                output=output,
                transport=transport,
                command=calls.append,
            )
            self.assertEqual(len(calls), 1)
            self.assertEqual(calls[0][1:], ("--prefix", str(prefix)))
            self.assertIn("Updated asc to 0.2.0", output.getvalue())

    def test_no_release_and_unsafe_archive(self) -> None:
        def missing(
            _url: str, _headers: Mapping[str, str], _limit: int
        ) -> tuple[int, Mapping[str, str], bytes]:
            return 404, {}, b""

        with self.assertRaisesRegex(AscError, "no published asc release"):
            run_update(current_version="0.1.0", check_only=True, transport=missing)

        unsafe = archive({"../outside": (b"bad", 0o644)})
        with (
            tempfile.TemporaryDirectory() as directory,
            self.assertRaisesRegex(AscError, "unsafe or unexpected"),
        ):
            _extract(unsafe, Path(directory) / "bundle")


if __name__ == "__main__":
    unittest.main()
