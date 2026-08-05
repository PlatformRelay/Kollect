#!/usr/bin/env bash
# DIST-ADR-01: ADR-0708 hub distribution must exist and be indexed under theme 07.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ADR="${ROOT}/docs/adr/0708-operator-distribution-hubs.md"
INDEX="${ROOT}/docs/adr/README.md"

fail() {
  printf 'dist adr 0708 indexed: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${ADR}" ]] || fail "${ADR} is missing"

grep -Eq '^# ADR-0708:.*[Oo]perator distribution' "${ADR}" ||
  fail "ADR missing title/heading for ADR-0708"

grep -Eqi 'Status:[[:space:]]*\*?Exploring\*?|\*\*Status:\*\*.*Exploring' "${ADR}" ||
  fail "ADR-0708 must document Status Exploring (Proposed)"

pass "ADR-0708 file and Exploring status present"

[[ -f "${INDEX}" ]] || fail "${INDEX} is missing"

theme07="$(
  awk '
    /^## 07 / {in07=1; next}
    /^## / {if (in07) exit}
    in07 {print}
  ' "${INDEX}"
)"
[[ -n "${theme07}" ]] || fail "theme 07 section missing from ${INDEX}"

printf '%s\n' "${theme07}" | grep -Eq '\[0708\]\(0708-operator-distribution-hubs\.md\)' ||
  fail "docs/adr/README.md theme 07 must index [0708](0708-operator-distribution-hubs.md)"

printf '%s\n' "${theme07}" | grep -E '\[0708\]\(0708-operator-distribution-hubs\.md\)' | grep -Eiq 'Exploring|Proposed' ||
  fail "docs/adr/README.md 0708 row must list Exploring/Proposed status"

pass "README indexes 0708 under theme 07 as Exploring/Proposed"

echo "All dist ADR-0708 index tests passed."
