"""External process runner tests."""

from __future__ import annotations

import sys
import unittest
from unittest import mock

from asc_devtools.errors import ProcessError
from asc_devtools.process import CommandRunner


class CommandRunnerTest(unittest.TestCase):
    def test_success_and_captured_output(self) -> None:
        result = CommandRunner().run([sys.executable, "-c", "print('hello')"])
        self.assertEqual(result.return_code, 0)
        self.assertEqual(result.stdout, "hello\n")

    def test_nonzero_exit_includes_stderr(self) -> None:
        with self.assertRaisesRegex(ProcessError, "specific failure"):
            CommandRunner().run(
                [
                    sys.executable,
                    "-c",
                    "import sys; print('specific failure', file=sys.stderr); "
                    "sys.exit(7)",
                ]
            )

    def test_nonzero_allowed(self) -> None:
        result = CommandRunner().run(
            [sys.executable, "-c", "raise SystemExit(3)"], check=False
        )
        self.assertEqual(result.return_code, 3)

    def test_missing_executable(self) -> None:
        with self.assertRaisesRegex(ProcessError, "required executable not found"):
            CommandRunner().run(["asc-test-command-that-does-not-exist"])

    def test_streaming_disables_capture(self) -> None:
        completed = mock.Mock(returncode=0, stdout=None, stderr=None)
        with mock.patch("subprocess.run", return_value=completed) as run:
            CommandRunner().run(["tool", "arg"], capture_output=False)
        self.assertFalse(run.call_args.kwargs["capture_output"])

    def test_error_redacts_sensitive_argument(self) -> None:
        with self.assertRaises(ProcessError) as context:
            CommandRunner().run(
                [sys.executable, "--token", "visible-secret", "missing.py"]
            )
        self.assertNotIn("visible-secret", str(context.exception))


if __name__ == "__main__":
    unittest.main()
