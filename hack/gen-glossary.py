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
# SAFE / false-positive (SonarCloud pythonsecurity:S2083, KO-01, SEC-04a): this
# looks like a "path traversal" write target, but it is a module-level
# constant derived only from the script's own location (ROOT), never from
# sys.argv, os.environ, or os.getenv. There is no user-controlled input here.
# See hack/test/sonar_ko_01_glossary_path_constant_test.sh, which fails if
# this constant is ever changed away from this exact form. Reviewed-safe:
# do not "fix" this into a dynamic path.
GLOSSARY = ROOT / "docs" / "GLOSSARY.md"
BEGIN = "<!-- BEGIN AUTO-CRD -->"
END = "<!-- END AUTO-CRD -->"

# Number of spec fields rendered as table rows per kind. The remainder are
# named in a trailing sentence rather than dropped: a silently truncated tail
# meant a CRD could gain a ninth field and nothing — neither the docs nor a
# drift gate over them — would ever notice.
FIELD_TABLE_LIMIT = 8

# Curated field descriptions that deliberately override the CRD's own text,
# keyed by (kind, spec field).
#
# Why this map exists (decision, lane GLOSSARY-MARKER-01):
# hand-written prose used to live *inside* the BEGIN/END AUTO-CRD markers, so
# running the regeneration command CONTRIBUTING.md documents silently
# destroyed it. Three fixes were on the table:
#   A. move the curated note outside the markers — rejected: the auto row
#      would then assert something false ("http configures webhook snapshot
#      export"), with its correction exiled to another section.
#   B. add a drift gate and nothing else — rejected as actively harmful: a
#      gate over a block containing hand-written prose converts a silent loss
#      into a forced one, because deleting the prose becomes the way to go
#      green.
#   C. (chosen) teach the generator to carry curated overrides, then gate on
#      drift. Regeneration is idempotent, the curated text stays adjacent to
#      the row it qualifies, and hack/test/glossary_drift_test.sh proves both.
# The root cause — a Go doc comment in api/ that reads as if `type: http` were
# supported — is outside this file. When that text is corrected, delete the
# override and check_overrides() below will confirm nothing is left dangling.
#
# Every key must match a real CRD spec field that carries a description; a
# stale key is a hard error, so this map cannot rot unnoticed.
CURATED_DESCRIPTIONS: dict[tuple[str, str], str] = {
    # Admission rejects `type: http`. The CRD description reads as if webhook
    # export were available, and the field is routinely confused with
    # KollectInventory's optional HTTP read API.
    ("KollectSnapshotSink", "http"): (
        "Reserved snapshot type that is rejected by admission; do not confuse "
        "it with the optional Inventory HTTP read API."
    ),
}


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
                curated = CURATED_DESCRIPTIONS.get((kind, name))
                fields.append((name, curated or first_line(desc)))
        entries.append(
            {
                "kind": kind,
                "scope": scope,
                "description": first_line(root_desc) if root_desc else "",
                "fields": fields[:FIELD_TABLE_LIMIT],
                "other_fields": [name for name, _ in fields[FIELD_TABLE_LIMIT:]],
            }
        )
    return sorted(entries, key=lambda e: e["kind"])


def check_overrides(entries: list[dict]) -> None:
    """Fail loudly when a curated override no longer matches a real CRD field."""
    known = {
        (entry["kind"], name)
        for entry in entries
        for name in [n for n, _ in entry["fields"]] + entry["other_fields"]
    }
    stale = sorted(key for key in CURATED_DESCRIPTIONS if key not in known)
    if stale:
        listed = ", ".join(f"{kind}.{field}" for kind, field in stale)
        sys.exit(
            "CURATED_DESCRIPTIONS names spec fields that no longer exist or "
            f"lost their description: {listed}"
        )


def render_crd_section(entries: list[dict]) -> str:
    crd_pages = {p.stem for p in (ROOT / "docs" / "crds").glob("*.md")}
    lines = [
        BEGIN,
        "",
        "## Custom resources (from CRD schema)",
        "",
        "Auto-generated from `config/crd/bases/` OpenAPI descriptions. Regenerate with",
        "`python3 hack/gen-glossary.py`. Field-level detail: [CR reference](crds/index.md).",
        "",
        "Everything between the `AUTO-CRD` markers is rewritten on every run, so edits made",
        "here are lost. A row that must say something the CRD schema does not say belongs in",
        "the `CURATED_DESCRIPTIONS` map in `hack/gen-glossary.py`; prose that is not about a",
        "single field belongs outside the markers.",
        "",
    ]
    for entry in entries:
        kind = entry["kind"]
        slug = kind.lower()
        link = f"crds/{slug}.md" if slug in crd_pages else "crds/index.md#kinds"
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
        if entry["other_fields"]:
            listed = ", ".join(f"`{name}`" for name in entry["other_fields"])
            lines.append(f"Other spec fields: {listed}.")
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
    entries = load_crds()
    check_overrides(entries)
    patch_glossary(render_crd_section(entries))
    print(f"Updated {GLOSSARY.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
