#!/usr/bin/env bash
# LAB-H-DESIGN: ADR-0707 lab harness must exist, stay Exploring until H01 lands,
# and be indexed under theme 07 in docs/adr/README.md.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ADR="${ROOT}/docs/adr/0707-lab-harness.md"
INDEX="${ROOT}/docs/adr/README.md"

fail() {
  printf 'lab adr 0707 indexed: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${ADR}" ]] || fail "${ADR} is missing"

grep -Eq '^# ADR-0707:.*[Ll]ab harness' "${ADR}" ||
  fail "ADR missing title/heading for ADR-0707 lab harness"

grep -Eqi 'Status:[[:space:]]*\*?Exploring\*?|\*\*Status:\*\*.*Exploring' "${ADR}" ||
  fail "ADR-0707 must document Status Exploring"

pass "ADR-0707 file and Exploring status present"

[[ -f "${INDEX}" ]] || fail "${INDEX} is missing"

# Theme 07 section must list 0707 with Exploring before the next theme heading.
theme07="$(
  awk '
    /^## 07 / {in07=1; next}
    /^## / {if (in07) exit}
    in07 {print}
  ' "${INDEX}"
)"
[[ -n "${theme07}" ]] || fail "theme 07 section missing from ${INDEX}"

printf '%s\n' "${theme07}" | grep -Eq '\[0707\]\(0707-lab-harness\.md\)' ||
  fail "docs/adr/README.md theme 07 must index [0707](0707-lab-harness.md)"

printf '%s\n' "${theme07}" | grep -E '\[0707\]\(0707-lab-harness\.md\)' | grep -Eq 'Exploring' ||
  fail "docs/adr/README.md 0707 row must list Exploring status"

pass "README indexes 0707 under theme 07 as Exploring"

echo "All lab ADR-0707 index tests passed."
