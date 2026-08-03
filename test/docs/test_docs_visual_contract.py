#!/usr/bin/env python3
"""Visual-system contracts for the public documentation."""

from pathlib import Path
import re
import struct
import unittest


ROOT = Path(__file__).resolve().parents[2]


def png_dimensions(path: Path) -> tuple[int, int]:
    """Return the dimensions encoded in a PNG IHDR chunk."""
    with path.open("rb") as image:
        header = image.read(24)
    if header[:8] != b"\x89PNG\r\n\x1a\n":
        raise ValueError(f"not a PNG: {path}")
    return struct.unpack(">II", header[16:24])


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
            self.assertIn('data-concept="collector-aperture"', svg)
            self.assertIn(
                "M19 27h3.5v3.5h-3.5zm5.75 3.25h3.5v3.5h-3.5zM19 34h3.5v3.5h-3.5z",
                svg,
            )
        self.assertIn("logo: assets/branding/kollect-symbol-light.svg", config)
        self.assertIn("favicon: assets/branding/favicon.svg", config)
        self.assertIn("  - social:", config)
        self.assertIn('background_color: "#0F172A"', config)
        self.assertIn("kollect-wordmark-dark.svg", index)
        self.assertNotRegex(index, r"kollect-logo[^)]*\.png")

    def test_brand_asset_pack_has_platform_ready_rasters(self) -> None:
        branding = ROOT / "docs/assets/branding"
        expected_dimensions = {
            "favicon-32.png": (32, 32),
            "favicon-180.png": (180, 180),
            "logo-512.png": (512, 512),
            "kollect-app-icon-1024.png": (1024, 1024),
            "kollect-github-avatar.png": (500, 500),
            "og-image.png": (1200, 630),
            "social-preview.png": (1280, 640),
            "kollect-github-social-card.png": (1280, 640),
        }
        for name, dimensions in expected_dimensions.items():
            asset = branding / name
            self.assertTrue(asset.is_file(), name)
            self.assertEqual(png_dimensions(asset), dimensions, name)

        social_source = (branding / "kollect-social-card.svg").read_text(encoding="utf-8")
        self.assertIn('data-concept="collector-aperture"', social_source)
        self.assertIn("Collect once. Use everywhere.", social_source)
        self.assertIn("Git · Object storage · Databases · Event streams", social_source)

        readme = (ROOT / "README.md").read_text(encoding="utf-8")
        index = (ROOT / "docs/index.md").read_text(encoding="utf-8")
        config = (ROOT / "mkdocs.yml").read_text(encoding="utf-8")
        self.assertIn("Turn Kubernetes state into durable inventory.", readme)
        self.assertIn("Declare your inventory. Kollect keeps it current.", index)
        self.assertIn("site_description: Turn Kubernetes state into durable inventory.", config)

    def test_docs_workflow_requires_browser_layout_regression(self) -> None:
        workflow = (ROOT / ".github/workflows/docs.yaml").read_text(encoding="utf-8")
        self.assertIn("task docs:verify", workflow)
        self.assertIn("CHROME_BIN", workflow)
        self.assertIn('DOCS_REQUIRE_CHROME: "1"', workflow)


if __name__ == "__main__":
    unittest.main()
