#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
"""Generate the CRD-derived section of docs/GLOSSARY.md from OpenAPI descriptions."""

from __future__ import annotations

import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
CRD_DIR = ROOT / "config" / "crd" / "bases"
GLOSSARY = ROOT / "docs" / "GLOSSARY.md"
BEGIN = "<!-- BEGIN AUTO-CRD -->"
END = "<!-- END AUTO-CRD -->"


def first_line(text: str) -> str:
    return text.strip().splitlines()[0].strip()


def load_crds() -> list[dict]:
    entries: list[dict] = []
    for path in sorted(CRD_DIR.glob("*.yaml")):
        with path.open(encoding="utf-8") as fh:
            doc = yaml.safe_load(fh)
        spec = doc["spec"]
        version = spec["versions"][0]
        schema = version["schema"]["openAPIV3Schema"]
        kind = spec["names"]["kind"]
        scope = spec["scope"]
        root_desc = schema.get("description", "")
        spec_props = schema.get("properties", {}).get("spec", {}).get("properties", {})
        fields: list[tuple[str, str]] = []
        for name, prop in sorted(spec_props.items()):
            desc = prop.get("description")
            if desc:
                fields.append((name, first_line(desc)))
        entries.append(
            {
                "kind": kind,
                "scope": scope,
                "description": first_line(root_desc) if root_desc else "",
                "fields": fields[:8],
            }
        )
    return sorted(entries, key=lambda e: e["kind"])


def render_crd_section(entries: list[dict]) -> str:
    crd_pages = {p.stem for p in (ROOT / "docs" / "crds").glob("*.md")}
    lines = [
        BEGIN,
        "",
        "## Custom resources (from CRD schema)",
        "",
        "Auto-generated from `config/crd/bases/` OpenAPI descriptions. Regenerate with",
        "`python3 hack/gen-glossary.py`. Field-level detail: [CR reference](CR-REFERENCE.md).",
        "",
    ]
    for entry in entries:
        kind = entry["kind"]
        slug = kind.lower()
        link = f"crds/{slug}.md" if slug in crd_pages else "CR-REFERENCE.md#kinds"
        lines.append(f"### `{entry['kind']}` ({entry['scope'].lower()})")
        lines.append("")
        if entry["description"]:
            lines.append(entry["description"])
            lines.append("")
        if entry["fields"]:
            lines.append("| Spec field | Description |")
            lines.append("| --- | --- |")
            for name, desc in entry["fields"]:
                lines.append(f"| `{name}` | {desc} |")
            lines.append("")
        lines.append(f"Full reference: [{entry['kind']}]({link}).")
        lines.append("")
    lines.append(END)
    return "\n".join(lines)


def patch_glossary(section: str) -> None:
    text = GLOSSARY.read_text(encoding="utf-8")
    if BEGIN not in text or END not in text:
        sys.exit(f"{GLOSSARY}: missing {BEGIN} / {END} markers")
    before, rest = text.split(BEGIN, 1)
    _, after = rest.split(END, 1)
    GLOSSARY.write_text(before + section + after, encoding="utf-8")


def main() -> None:
    patch_glossary(render_crd_section(load_crds()))
    print(f"Updated {GLOSSARY.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
