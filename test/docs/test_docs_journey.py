#!/usr/bin/env python3
"""Contract checks for the launch documentation journey."""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]


class DocsJourneyTest(unittest.TestCase):
    def test_navigation_has_seven_task_first_groups(self) -> None:
        config = (ROOT / "mkdocs.yml").read_text(encoding="utf-8")
        nav = config.split("nav:\n", 1)[1].split("\nmarkdown_extensions:", 1)[0]
        groups = re.findall(r"^  - ([^:]+):$", nav, flags=re.MULTILINE)
        self.assertEqual(
            groups,
            [
                "Overview",
                "Getting started",
                "Examples",
                "Concepts",
                "Operate",
                "Reference",
                "Design & internals",
            ],
        )

    def test_moved_pages_keep_redirects(self) -> None:
        config = (ROOT / "mkdocs.yml").read_text(encoding="utf-8")
        expected = {
            "QUICKSTART.md": "getting-started/install.md",
            "examples/deployment-inventory.md": "getting-started/first-inventory.md",
            "FAQ.md": "operator-manual/troubleshooting.md",
            "operator-manual/common-errors.md": "operator-manual/troubleshooting.md",
            "examples/kind-local-lab.md": "getting-started/install.md",
        }
        for old, new in expected.items():
            self.assertIn(f"{old}: {new}", config)

    def test_readme_to_git_diff_journey(self) -> None:
        readme = (ROOT / "README.md").read_text(encoding="utf-8")
        install = (ROOT / "docs/getting-started/install.md").read_text(encoding="utf-8")
        first = (ROOT / "docs/getting-started/first-inventory.md").read_text(encoding="utf-8")
        self.assertIn("/getting-started/install/", readme)
        self.assertIn("first-inventory.md", install)
        self.assertIn("type: git", first)
        self.assertIn("git clone", first)
        self.assertIn("git diff", first)
        self.assertNotIn("Postgres backing store (required", install)


if __name__ == "__main__":
    unittest.main()
