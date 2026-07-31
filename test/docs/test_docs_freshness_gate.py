#!/usr/bin/env python3
"""Contracts for the single, reproducible documentation verification gate."""

from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]


class DocsFreshnessGateTest(unittest.TestCase):
    def test_task_composes_every_docs_contract_once(self) -> None:
        taskfile = (ROOT / "Taskfile.yml").read_text(encoding="utf-8")
        verifier = (ROOT / "hack/docs/verify.sh").read_text(encoding="utf-8")

        self.assertIn("docs:verify:", taskfile)
        self.assertIn("bash hack/docs/verify.sh", taskfile)
        self.assertIn("tracked-markdown.sh", taskfile)
        self.assertIn("--no-globs", taskfile)
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

    def test_tracked_markdown_selector_ignores_other_markdown(self) -> None:
        selector = ROOT / "hack/docs/tracked-markdown.sh"
        with tempfile.TemporaryDirectory() as temporary:
            repo = Path(temporary)
            subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
            (repo / ".gitignore").write_text("ignored.md\n", encoding="utf-8")
            (repo / "tracked.md").write_text("#tracked-invalid\n", encoding="utf-8")
            (repo / "ignored.md").write_text("#ignored-invalid\n", encoding="utf-8")
            (repo / "untracked.md").write_text("#untracked-invalid\n", encoding="utf-8")
            subprocess.run(["git", "add", ".gitignore", "tracked.md"], cwd=repo, check=True)
            selected = subprocess.check_output([selector], cwd=repo).split(b"\0")
            self.assertIn(b"tracked.md", selected)
            self.assertNotIn(b"ignored.md", selected)
            self.assertNotIn(b"untracked.md", selected)

    def test_workflow_watches_every_external_truth_input(self) -> None:
        workflow = (ROOT / ".github/workflows/docs.yaml").read_text(encoding="utf-8")
        for source in (
            "SECURITY.md",
            "osv-scanner.toml",
            "api/v1alpha1/**",
            "config/crd/bases/**",
            "config/samples/**",
            "charts/kollect/Chart.yaml",
            "CHANGELOG.md",
            "overrides/**",
        ):
            self.assertEqual(workflow.count(f'- "{source}"'), 2, source)

    def test_bigquery_adr_matches_the_registered_backend(self) -> None:
        registry = (ROOT / "internal/sink/registry.go").read_text(encoding="utf-8")
        config = (ROOT / "internal/sink/bigquery/config.go").read_text(encoding="utf-8")
        adr = (ROOT / "docs/adr/0420-bigquery-database-sink.md").read_text(encoding="utf-8")
        index = (ROOT / "docs/adr/README.md").read_text(encoding="utf-8")
        self.assertIn("Register(bigquery.TypeName, newBigQueryBackend)", registry)
        self.assertIn('TypeName = "bigquery"', config)
        self.assertIn("**Status:** Current", adr)
        self.assertNotRegex(adr.lower(), r"implementation pending|stub|re-enters|parallel change")
        self.assertRegex(index, r"\[0420\].*\| Current \|")

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

    def test_webhook_tls_docs_match_the_chart(self) -> None:
        install = (ROOT / "docs/getting-started/install.md").read_text(encoding="utf-8")
        manual = (ROOT / "docs/operator-manual/index.md").read_text(encoding="utf-8")
        public = "\n".join(
            path.read_text(encoding="utf-8")
            for base in (ROOT / "README.md", ROOT / "docs", ROOT / "charts/kollect/README.md")
            for path in ([base] if base.is_file() else base.rglob("*.md"))
        ).lower()
        self.assertIn("cert-manager", install.lower())
        self.assertIn("kubectl get crd certificates.cert-manager.io", install)
        self.assertIn("operator-provided", manual.lower())
        self.assertNotRegex(public, r"self-signed bootstrap|self signed bootstrap")
        self.assertNotRegex(public, r"certmanager\.create[^\n]{0,100}(selects|enables)[^\n]{0,40}(fallback|bootstrap)")

    def test_public_sink_contract_matches_admission_and_formats(self) -> None:
        validation = (ROOT / "internal/validation/family_sink.go").read_text(encoding="utf-8")
        source = (ROOT / "api/v1alpha1/sink_common_types.go").read_text(encoding="utf-8")
        pages = {
            path.relative_to(ROOT).as_posix(): path.read_text(encoding="utf-8")
            for base in (ROOT / "README.md", ROOT / "docs")
            for path in ([base] if base.is_file() else base.rglob("*.md"))
        }
        self.assertIn("validSnapshotSinkTypes", validation)
        self.assertNotIn("SnapshotSinkTypeHTTP,", validation)
        self.assertNotIn("SnapshotSinkTypeAzureBlob,", validation)
        self.assertIn("SerializationFormatParquet", (ROOT / "api/v1alpha1/constants.go").read_text(encoding="utf-8"))
        self.assertIn("SnapshotSinkTypeHTTP", source)  # reserved API constant, not admitted
        for name, text in pages.items():
            lowered = text.lower()
            self.assertNotRegex(lowered, r"parquet.{0,40}\*\*planned\*\*|\*\*planned\*\*.{0,40}parquet", name)
            self.assertNotRegex(lowered, r"stub backends?.{0,80}(pass admission|valid crd)", name)
        reference = (ROOT / "docs/crds/kollectsnapshotsink.md").read_text(encoding="utf-8").lower()
        for sink_type in ("git", "gitlab", "s3", "gcs"):
            self.assertIn(f"`{sink_type}`", reference)
        self.assertIn("parquet", reference)
        self.assertIn("not accepted by admission", reference)

    def test_current_adrs_do_not_publish_retired_architecture(self) -> None:
        adr0201 = (ROOT / "docs/adr/0201-crd-model.md").read_text(encoding="utf-8")
        adr0414 = (ROOT / "docs/adr/0414-sink-family-crds.md").read_text(encoding="utf-8")
        adr0801 = (ROOT / "docs/adr/0801-pipeline-cli-mode.md").read_text(encoding="utf-8")
        index = (ROOT / "docs/adr/README.md").read_text(encoding="utf-8")
        for text in (adr0201, adr0414):
            self.assertNotIn("KollectClusterSnapshotSink", text)
            self.assertNotIn("KollectClusterDatabaseSink", text)
            self.assertNotIn("KollectClusterEventSink", text)
        self.assertNotRegex(adr0414.lower(), r"stub backends?.*(azureblob|http)")
        self.assertNotIn("## Open questions", adr0801)
        self.assertIn("**Status:** Current", (ROOT / "docs/adr/0208-cluster-static-refs-via-namespace.md").read_text(encoding="utf-8"))
        self.assertRegex(index, r"\[0208\].*\| Current \|")

    def test_sink_configuration_adrs_describe_only_current_architecture(self) -> None:
        adr0416 = (ROOT / "docs/adr/0416-sink-config-layering.md").read_text(encoding="utf-8")
        adr0419 = (ROOT / "docs/adr/0419-git-export-serialization-layout.md").read_text(encoding="utf-8")
        for name, text in (("ADR-0416", adr0416), ("ADR-0419", adr0419)):
            lowered = text.lower()
            for retired in (
                "kollectclustersnapshotsink",
                "cluster-scoped",
                "azureblob",
                "azure blob",
                "http",
                "## open questions",
                "## implementation phases",
                "### crd shape (proposed)",
                "## deferred",
            ):
                self.assertNotIn(retired, lowered, f"{name}: {retired}")
        self.assertIn("`KollectSnapshotSink`", adr0416)
        self.assertIn("`git`, `gitlab`, `s3`, and `gcs`", adr0416)
        self.assertIn("**`yaml` (default), `json`, and `ndjson`**", adr0419)
        self.assertIn("**`json` (default), `parquet`, and `csv`**", adr0419)


if __name__ == "__main__":
    unittest.main()
