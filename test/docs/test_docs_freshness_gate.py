#!/usr/bin/env python3
"""Contracts for the single, reproducible documentation verification gate."""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]


class DocsFreshnessGateTest(unittest.TestCase):
    def test_task_composes_every_docs_contract_once(self) -> None:
        taskfile = (ROOT / "Taskfile.yml").read_text(encoding="utf-8")
        verifier = (ROOT / "hack/docs/verify.sh").read_text(encoding="utf-8")

        self.assertIn("docs:verify:", taskfile)
        self.assertIn("bash hack/docs/verify.sh", taskfile)
        self.assertIn("git ls-files -z", taskfile)
        self.assertIn("docs_launch_truth_test.sh", verifier)
        self.assertIn("security_architecture_docs_test.sh", verifier)
        self.assertIn("test/docs", verifier)
        self.assertIn("./test/samples", verifier)
        self.assertIn("mkdocs build --strict", verifier)
        self.assertIn("docs_visual_browser_test.py", verifier)

    def test_docs_workflow_calls_only_the_composed_gate(self) -> None:
        workflow = (ROOT / ".github/workflows/docs.yaml").read_text(encoding="utf-8")
        self.assertEqual(workflow.count("task docs:verify"), 1)
        self.assertNotIn("task lint:markdown", workflow)
        self.assertNotIn("mkdocs build --strict", workflow)
        self.assertNotIn("bash hack/test/security_architecture_docs_test.sh", workflow)
        self.assertIn("DOCS_REQUIRE_CHROME: \"1\"", workflow)

    def test_reproduction_is_documented(self) -> None:
        testing = (ROOT / "docs/development/testing.md").read_text(encoding="utf-8")
        self.assertIn("task docs:verify", testing)
        self.assertIn("DOCS_REQUIRE_CHROME=1", testing)

    def test_every_adr_is_indexed_and_declared(self) -> None:
        index = (ROOT / "docs/adr/README.md").read_text(encoding="utf-8")
        config = (ROOT / "mkdocs.yml").read_text(encoding="utf-8")
        for adr in (ROOT / "docs/adr").glob("[0-9][0-9][0-9][0-9]-*.md"):
            self.assertIn(adr.name, index, f"ADR missing from index: {adr.name}")
        self.assertIn("/adr/*.md", config)

    def test_every_crd_kind_has_a_reference_page(self) -> None:
        config = (ROOT / "mkdocs.yml").read_text(encoding="utf-8")
        kinds = set()
        for crd in (ROOT / "config/crd/bases").glob("*.yaml"):
            text = crd.read_text(encoding="utf-8")
            match = re.search(r"^\s{4}kind:\s*(Kollect\w+)$", text, flags=re.MULTILINE)
            self.assertIsNotNone(match, f"cannot read kind from {crd.name}")
            kinds.add(match.group(1))
        for kind in kinds:
            page = f"crds/{kind.lower()}.md"
            self.assertTrue((ROOT / "docs" / page).is_file(), f"missing CRD page: {page}")
            self.assertIn(page, config, f"CRD page missing from nav: {page}")

    def test_local_markdown_images_exist(self) -> None:
        image = re.compile(r"!\[[^]]*\]\(([^)\s]+)")
        for page in (ROOT / "docs").rglob("*.md"):
            if "node_modules" in page.parts:
                continue
            for target in image.findall(page.read_text(encoding="utf-8")):
                if target.startswith(("http://", "https://", "data:")):
                    continue
                # The demo recording guide names its generated, gitignored output.
                if target == "docs/assets/demo/hero-git-only.gif":
                    continue
                path = (page.parent / target.split("#", 1)[0]).resolve()
                self.assertTrue(path.is_file(), f"{page.relative_to(ROOT)}: missing image {target}")


if __name__ == "__main__":
    unittest.main()
