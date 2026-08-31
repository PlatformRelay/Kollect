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
#
# The map is EMPTY on purpose, and that is the healthy state. Its only entry
# corrected the KollectSnapshotSink `http` row, whose CRD description asserted
# that webhook export was available when admission in fact rejects
# `type: http`. That root cause lived in a Go doc comment in
# api/v1alpha1/kollectsnapshotsink_types.go, so the override was treating a
# symptom: lane API-HTTPDOC-01 corrected the comment and deleted the entry.
# Fix the schema first; add an entry here only when the CRD genuinely cannot
# carry the wording, because an override that patches a fixable comment hides
# the defect in the published schema instead of closing it.
#
# An override can rot in THREE directions. check_overrides() below catches two
# of them and fails the run before writing:
#   1. STALE key — it names a field that no longer exists, or whose description
#      was removed. Caught: the key is absent from the schema.
#   2. REDUNDANT text — the CRD description has caught up and now says exactly
#      what the override says, so the override is dead weight that still reads
#      as load-bearing. Caught: exact string equality against first_line().
#   3. REWORDED schema — the CRD description is corrected or rephrased so that
#      it no longer says what the override contradicts, without becoming
#      identical to it. NOT CAUGHT, and it cannot be: the guard compares
#      strings, and only a human can judge whether the new wording still needs
#      qualifying. The run exits 0 and quietly renders the stale override.
# Direction 3 is not hypothetical. Lane API-HTTPDOC-01 rewrote the `http` doc
# comment in api/ from "configures webhook snapshot export" to a sentence that
# states the field is reserved and rejected by admission. Neither guard fired —
# the key still resolved and the texts still differed — so the override had to
# be deleted deliberately, in the same commit, rather than on a gate's prompt.
# If you reword a description an override qualifies, re-read the override.
# Directions 1 and 2 are exercised with synthetic overrides in
# hack/test/glossary_drift_test.sh, so both guards stay tested while the map is
# empty.
CURATED_DESCRIPTIONS: dict[tuple[str, str], str] = {}


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
        raw_descriptions: dict[str, str] = {}
        # A spec field with no description still gets named. Skipping it
        # outright would make a newly added, undocumented field invisible to
        # both this page and the drift gate over it.
        undocumented: list[str] = []
        for name, prop in sorted(spec_props.items()):
            desc = prop.get("description")
            if not desc:
                undocumented.append(name)
                continue
            raw_descriptions[name] = first_line(desc)
            curated = CURATED_DESCRIPTIONS.get((kind, name))
            fields.append((name, curated or raw_descriptions[name]))
        entries.append(
            {
                "kind": kind,
                "scope": scope,
                "description": first_line(root_desc) if root_desc else "",
                "fields": fields[:FIELD_TABLE_LIMIT],
                "other_fields": sorted(
                    [name for name, _ in fields[FIELD_TABLE_LIMIT:]] + undocumented
                ),
                "raw_descriptions": raw_descriptions,
            }
        )
    return sorted(entries, key=lambda e: e["kind"])


def check_overrides(entries: list[dict]) -> None:
    """Fail the run when a curated override has gone stale or become redundant."""
    schema_text = {
        (entry["kind"], name): text
        for entry in entries
        for name, text in entry["raw_descriptions"].items()
    }
    stale = sorted(key for key in CURATED_DESCRIPTIONS if key not in schema_text)
    if stale:
        listed = ", ".join(f"{kind}.{field}" for kind, field in stale)
        sys.exit(
            "CURATED_DESCRIPTIONS names spec fields that no longer exist or "
            f"lost their description: {listed}"
        )
    redundant = sorted(
        key for key, text in CURATED_DESCRIPTIONS.items() if schema_text[key] == text
    )
    if redundant:
        listed = ", ".join(f"{kind}.{field}" for kind, field in redundant)
        sys.exit(
            "CURATED_DESCRIPTIONS repeats what the CRD schema already says; "
            f"delete these overrides: {listed}"
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
        "here are lost. A row that says the wrong thing is usually a wrong Go doc comment",
        "in `api/`: fix it there and regenerate. Only a row the CRD schema genuinely cannot",
        "carry belongs in the `CURATED_DESCRIPTIONS` map in `hack/gen-glossary.py`; prose",
        "that is not about a single field belongs outside the markers.",
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
