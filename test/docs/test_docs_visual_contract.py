#!/usr/bin/env python3
"""Visual-system contracts for the public documentation."""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]


def contrast_ratio(foreground: str, background: str) -> float:
    """Return WCAG relative-luminance contrast for two hex colors."""
    def luminance(value: str) -> float:
        channels = [int(value[index:index + 2], 16) / 255 for index in (1, 3, 5)]
        linear = [
            channel / 12.92 if channel <= 0.04045 else ((channel + 0.055) / 1.055) ** 2.4
            for channel in channels
        ]
        return sum(weight * channel for weight, channel in zip((0.2126, 0.7152, 0.0722), linear))

    light, dark = sorted((luminance(foreground), luminance(background)), reverse=True)
    return (light + 0.05) / (dark + 0.05)


class DocsVisualContractTest(unittest.TestCase):
    def test_diagrams_are_source_controlled_and_raster_free(self) -> None:
        public_text = "\n".join(
            path.read_text(encoding="utf-8")
            for path in [ROOT / "README.md", *(ROOT / "docs").rglob("*.md")]
        )
        self.assertNotRegex(public_text, r"assets/illustrations/[^)\s]+\.(?:webp|png)")
        self.assertFalse(list((ROOT / "docs/assets/illustrations").glob("*.webp")))

        architecture = (ROOT / "docs/concepts/architecture.md").read_text(encoding="utf-8")
        resource_model = (ROOT / "docs/concepts/resource-model.md").read_text(encoding="utf-8")
        tenancy = (ROOT / "docs/adr/0203-namespaced-multi-tenancy.md").read_text(encoding="utf-8")
        fleet = (ROOT / "docs/concepts/multi-cluster.md").read_text(encoding="utf-8")
        for page in (architecture, resource_model, tenancy, fleet):
            self.assertIn("```mermaid", page)
        for backend in ("Git", "GitLab", "S3", "GCS", "Postgres", "MongoDB", "BigQuery", "Kafka", "NATS"):
            self.assertIn(backend, resource_model)
        self.assertIn("Pipeline CLI", architecture)

    def test_theme_uses_complete_semantic_tokens(self) -> None:
        css = (ROOT / "docs/stylesheets/extra.css").read_text(encoding="utf-8")
        tokens = (
            "background", "surface", "text", "muted", "primary", "link", "focus",
            "success", "warning", "danger", "border", "code",
        )
        for scheme in ("default", "slate"):
            block = re.search(
                rf'\[data-md-color-scheme="{scheme}"\]\s*\{{(.*?)\n\}}',
                css,
                flags=re.DOTALL,
            )
            self.assertIsNotNone(block, scheme)
            for token in tokens:
                self.assertIn(f"--kollect-{token}:", block.group(1), f"{scheme}: {token}")
            colors = dict(re.findall(r"--kollect-([a-z-]+):\s*(#[0-9a-f]{6});", block.group(1)))
            for foreground in ("text", "muted", "link", "success", "warning", "danger"):
                self.assertGreaterEqual(
                    contrast_ratio(colors[foreground], colors["background"]),
                    4.5,
                    f"{scheme}: {foreground} on background",
                )
            self.assertGreaterEqual(contrast_ratio(colors["focus"], colors["background"]), 3.0)
            self.assertGreaterEqual(
                contrast_ratio(colors["on-primary"], colors["primary"]),
                4.5,
                f"{scheme}: primary button text",
            )
            self.assertGreaterEqual(
                contrast_ratio(colors["link"], colors["surface"]),
                4.5,
                f"{scheme}: outlined button text",
            )
            self.assertGreaterEqual(
                contrast_ratio(colors["link"], colors["surface"]),
                3.0,
                f"{scheme}: outlined button border",
            )
        self.assertIn(":focus-visible", css)
        self.assertIn(".kollect-hero .md-button:not(.md-button--primary)", css)
        self.assertNotIn(".kollect-illus", css)

    def test_brand_assets_are_canonical_transparent_vectors(self) -> None:
        config = (ROOT / "mkdocs.yml").read_text(encoding="utf-8")
        index = (ROOT / "docs/index.md").read_text(encoding="utf-8")
        expected = {
            "kollect-symbol-dark.svg",
            "kollect-symbol-light.svg",
            "kollect-wordmark-dark.svg",
            "kollect-wordmark-light.svg",
        }
        vectors = ROOT / "docs/assets/branding"
        self.assertTrue(expected.issubset({path.name for path in vectors.glob("*.svg")}))
        for name in expected:
            svg = (vectors / name).read_text(encoding="utf-8")
            self.assertIn("viewBox=", svg)
            self.assertNotIn("<rect", svg, f"{name} must keep a transparent canvas")
        self.assertIn("logo: assets/branding/kollect-symbol-light.svg", config)
        self.assertIn("favicon: assets/branding/favicon.svg", config)
        self.assertIn("  - social:", config)
        self.assertIn('background_color: "#0F172A"', config)
        self.assertIn("kollect-wordmark-dark.svg", index)
        self.assertNotRegex(index, r"kollect-logo[^)]*\.png")


if __name__ == "__main__":
    unittest.main()
