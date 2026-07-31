#!/usr/bin/env python3
"""Contract checks for the launch documentation journey."""

from pathlib import Path
from fnmatch import fnmatch
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
            "UNDERSTAND-THE-BASICS.md": "concepts/resource-model.md",
            "examples/deployment-inventory.md": "getting-started/first-inventory.md",
            "FAQ.md": "operator-manual/troubleshooting.md",
            "ARCHITECTURE.md": "concepts/architecture.md",
            "DATA-FLOWS.md": "concepts/export-pipeline.md",
            "BEST-PRACTICES.md": "operator-manual/production-checklist.md",
            "TROUBLESHOOTING.md": "operator-manual/troubleshooting.md",
            "PERFORMANCE.md": "operator-manual/performance.md",
            "CR-REFERENCE.md": "crds/index.md",
            "OPERATOR-MANUAL.md": "operator-manual/index.md",
            "DEVELOPMENT.md": "development/setup.md",
            "operator-manual/common-errors.md": "operator-manual/troubleshooting.md",
            "operator-manual/scaling-and-fleet.md": "operator-manual/performance.md",
            "deployment/team-operator.md": "examples/team-operator.md",
            "examples/kind-local-lab.md": "getting-started/install.md",
            "examples/cert-manager-webhook.md": "operator-manual/index.md",
            "examples/kollect-inventory-demo.md": "examples/README.md",
        }
        for old, new in expected.items():
            self.assertIn(f"{old}: {new}", config)

        redirect_block = config.split("redirect_maps:\n", 1)[1].split("\n  - social:", 1)[0]
        self.assertEqual(len(re.findall(r"^        \S.*: \S", redirect_block, re.MULTILINE)), 18)

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
        self.assertIn("task build", install)
        self.assertIn("task kind-dev-up", install)
        self.assertNotIn("task dev-up", install)
        self.assertNotIn("kubectl apply -k config/samples/", install)

    def test_canonical_concepts_use_family_sink_model(self) -> None:
        export = (ROOT / "docs/concepts/export-pipeline.md").read_text(encoding="utf-8")
        self.assertNotIn("`sinkRefs`", export)
        self.assertNotIn("KollectSink", export)
        self.assertNotIn("hub", export.lower())
        self.assertIn("snapshotSinkRefs", export)
        self.assertIn("databaseSinkRefs", export)
        self.assertIn("eventSinkRefs", export)
        self.assertIn("nine", export.lower())

    def test_connection_probe_matrix_matches_wired_database_backends(self) -> None:
        page = (ROOT / "docs/examples/connection-test.md").read_text(encoding="utf-8")
        self.assertIn("`postgres`, `mongodb`, `bigquery`", page)
        self.assertIn("Reserved backend names (`azureblob`, `http`) are rejected by admission", page)
        self.assertNotRegex(page, r"Stub backends.*pass admission")

    def test_current_backend_examples_follow_task_template(self) -> None:
        pages = [
            "git-snapshot-sink.md",
            "object-store-sink.md",
            "mongodb-sink.md",
            "bigquery-sink.md",
            "multi-sink-fanout.md",
            "kafka-event-sink.md",
            "nats-event-sink.md",
            "connection-test.md",
            "team-operator.md",
            "cluster-rollup.md",
            "multi-tenant-watch-namespaces.md",
        ]
        for page in pages:
            text = (ROOT / "docs/examples" / page).read_text(encoding="utf-8")
            for heading in ("## Prerequisites", "## Apply", "## Verify", "## Cleanup", "## Further reading"):
                self.assertIn(heading, text, f"{page}: missing {heading}")
            for sample in re.findall(r"config/samples/[\w./-]+\.ya?ml", text):
                self.assertTrue((ROOT / sample).is_file(), f"{page}: missing {sample}")

    def test_every_page_is_navigated_or_declared(self) -> None:
        config = (ROOT / "mkdocs.yml").read_text(encoding="utf-8")
        nav = config.split("nav:\n", 1)[1].split("\nmarkdown_extensions:", 1)[0]
        navigated = set(re.findall(r": ([\w./-]+\.md)$", nav, re.MULTILINE))
        declared = ["DEMO-GIF-GUIDE.md", "adr/*.md", "rfc/*.md"]
        for page in (ROOT / "docs").rglob("*.md"):
            relative = page.relative_to(ROOT / "docs").as_posix()
            if relative.startswith(("_snippets/", "node_modules/")):
                continue
            self.assertTrue(
                relative in navigated or any(fnmatch(relative, pattern) for pattern in declared),
                f"orphan page: {relative}",
            )


if __name__ == "__main__":
    unittest.main()
