#!/usr/bin/env python3
"""Behaviour of hack/test/docs_launch_truth_test.sh across the release-preparation window."""

from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = "hack/test/docs_launch_truth_test.sh"
TRUTH_FILES = (
    SCRIPT,
    "README.md",
    "CHANGELOG.md",
    "charts/kollect/Chart.yaml",
    "docs/index.md",
    "docs/ROADMAP.md",
    "docs/roadmap/planned-features.md",
    "docs/RELEASE.md",
    "docs/_snippets/pre-beta.md",
    "docs/adr/README.md",
    "overrides/main.html",
)


def released_version(changelog: str) -> str:
    match = re.search(r"^## \[(\d[^]-]*)\]", changelog, re.MULTILINE)
    assert match, "fixture changelog has no released heading"
    return match.group(1)


def next_minor(version: str) -> str:
    major, minor, _ = version.split(".")
    return f"{major}.{int(minor) + 1}.0"


class DocsLaunchTruthTest(unittest.TestCase):
    def setUp(self) -> None:
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.repo = Path(tmp.name)
        for rel in TRUTH_FILES:
            target = self.repo / rel
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(ROOT / rel, target)
        self.current = released_version((self.repo / "CHANGELOG.md").read_text(encoding="utf-8"))

    def run_check(self) -> subprocess.CompletedProcess:
        return subprocess.run(
            ["bash", str(self.repo / SCRIPT)], capture_output=True, text=True, check=False
        )

    def bump_chart(self, version: str) -> None:
        chart = self.repo / "charts/kollect/Chart.yaml"
        text = chart.read_text(encoding="utf-8")
        text = re.sub(r"(?m)^version: .*$", f"version: {version}", text)
        text = re.sub(r'(?m)^appVersion: .*$', f'appVersion: "{version}"', text)
        chart.write_text(text, encoding="utf-8")

    def bump_docs(self, old: str, new: str) -> None:
        for rel in ("docs/ROADMAP.md", "docs/roadmap/planned-features.md", "overrides/main.html"):
            path = self.repo / rel
            path.write_text(path.read_text(encoding="utf-8").replace(f"v{old}", f"v{new}"), encoding="utf-8")

    def test_current_tree_passes(self) -> None:
        result = self.run_check()
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_release_prep_with_stale_docs_fails(self) -> None:
        # Release prep bumps Chart.yaml; CHANGELOG.md gains its heading only after the tag, from
        # the post-merge sync bot under [skip ci]. Docs left at the previous release must fail
        # here, in the prep PR, not in whichever unrelated docs PR runs next.
        self.bump_chart(next_minor(self.current))

        result = self.run_check()

        self.assertNotEqual(result.returncode, 0, "stale docs passed during release prep")
        self.assertIn(f"v{next_minor(self.current)}", result.stderr)

    def test_release_prep_with_updated_docs_passes(self) -> None:
        candidate = next_minor(self.current)
        self.bump_chart(candidate)
        self.bump_docs(self.current, candidate)

        result = self.run_check()

        self.assertEqual(result.returncode, 0, result.stderr)

    def add_changelog_heading(self, version: str) -> None:
        changelog = self.repo / "CHANGELOG.md"
        text = changelog.read_text(encoding="utf-8")
        released = f"## [{self.current}]"
        text = text.replace(released, f"## [{version}] - 2026-10-05\n\n{released}", 1)
        changelog.write_text(text, encoding="utf-8")

    def test_chart_behind_changelog_fails(self) -> None:
        # Docs move down with the chart, so only the ordering guard can reject this state.
        major, minor, _ = self.current.split(".")
        older = f"{major}.{int(minor) - 1}.0"
        self.bump_chart(older)
        self.bump_docs(self.current, older)

        result = self.run_check()

        self.assertNotEqual(result.returncode, 0, "a chart older than the released version passed")
        self.assertIn("is older than released", result.stderr)

    def test_rc_of_released_version_fails(self) -> None:
        # sort -V ranks X-rc.1 above X, so an RC of an already-released version must be caught.
        self.bump_chart(f"{self.current}-rc.1")

        result = self.run_check()

        self.assertNotEqual(result.returncode, 0, "an RC of the released version passed")
        self.assertIn("is older than released", result.stderr)

    def test_rc_soak_keeps_docs_on_last_ga(self) -> None:
        # Public docs claim GA releases only: while the chart carries an RC, they stay put.
        rc = f"{next_minor(self.current)}-rc.1"
        self.bump_chart(rc)
        self.add_changelog_heading(rc)

        result = self.run_check()

        self.assertEqual(result.returncode, 0, result.stderr)

    def test_ga_prep_after_rc_heading_passes(self) -> None:
        # sort -V ranks 0.22.0-rc.1 above 0.22.0; a GA prep after an RC soak must still pass.
        candidate = next_minor(self.current)
        self.add_changelog_heading(f"{candidate}-rc.1")
        self.bump_chart(candidate)
        self.bump_docs(self.current, candidate)

        result = self.run_check()

        self.assertEqual(result.returncode, 0, result.stderr)

    def test_ga_prep_after_rc_with_stale_docs_fails(self) -> None:
        candidate = next_minor(self.current)
        self.add_changelog_heading(f"{candidate}-rc.1")
        self.bump_chart(candidate)

        result = self.run_check()

        self.assertNotEqual(result.returncode, 0, "stale docs passed at GA prep after an RC")

if __name__ == "__main__":
    unittest.main()
