#!/usr/bin/env python3
"""Focused tests for portable browser-runner discovery."""

import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
RUNNER = ROOT / "test/docs/docs_visual_browser_test.py"
SPEC = importlib.util.spec_from_file_location("docs_visual_browser_test", RUNNER)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError(f"cannot load browser runner: {RUNNER}")
BROWSER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BROWSER)


class ChromeDiscoveryTest(unittest.TestCase):
    def test_accepts_executable_override(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            chrome = Path(temporary) / "chrome"
            chrome.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
            chrome.chmod(0o700)
            with patch.dict(os.environ, {"CHROME_BIN": str(chrome)}):
                self.assertEqual(BROWSER.find_chrome(), chrome)

    def test_rejects_missing_override_with_action(self) -> None:
        missing = Path(tempfile.gettempdir()) / "kollect-missing-chrome"
        with patch.dict(os.environ, {"CHROME_BIN": str(missing)}):
            with self.assertRaisesRegex(SystemExit, "executable regular file.*execute permissions"):
                BROWSER.find_chrome()

    def test_rejects_non_executable_regular_file_with_action(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            chrome = Path(temporary) / "chrome"
            chrome.write_text("not executable\n", encoding="utf-8")
            chrome.chmod(0o600)
            with patch.dict(os.environ, {"CHROME_BIN": str(chrome)}):
                with self.assertRaisesRegex(SystemExit, "executable regular file.*execute permissions"):
                    BROWSER.find_chrome()


if __name__ == "__main__":
    unittest.main()
