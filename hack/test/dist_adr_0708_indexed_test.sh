#!/usr/bin/env bash
# DIST-ADR-01: ADR-0708 hub distribution must exist and be indexed under theme 07.
# DIST-OH-03: its status is resolved -- ADR header and index row must both read Current.
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

# The status lives on the header line, next to the theme. Match THAT line, not any
# occurrence of the word in the body -- a file-wide grep would pass on prose.
adr_status="$(grep -E '^\*\*Theme:\*\*.*\*\*Status:\*\*' "${ADR}" | head -1 |
  sed -E 's/^.*\*\*Status:\*\*[[:space:]]*//; s/[[:space:]]*$//')"
[[ -n "${adr_status}" ]] || fail "ADR-0708 header line carries no **Status:**"

# ADR-0708 was accepted by the operator on 2026-08-08 and shipped on 2026-08-18; the
# repo's status vocabulary spells an accepted decision **Current**
# (docs/development/adr-rfc-process.md). Anything still saying Exploring/Proposed means
# the header was never resolved.
[[ "${adr_status}" == Current* ]] ||
  fail "ADR-0708 header must document Status Current, found '${adr_status}'"

pass "ADR-0708 file and Current status present"

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

# The index row must agree with the ADR header, so the two can never drift apart.
index_status="$(printf '%s\n' "${theme07}" |
  grep -E '\[0708\]\(0708-operator-distribution-hubs\.md\)' | head -1 |
  awk -F'|' '{gsub(/^[[:space:]]+|[[:space:]]+$/, "", $4); print $4}')"
[[ -n "${index_status}" ]] || fail "docs/adr/README.md 0708 row carries no status cell"

# The header may carry a parenthetical ("Current (accepted ...)"); the table cell is bare,
# so compare only the leading word. This is the ONLY status assertion on the index row --
# pinning it to a literal as well would make this comparison unfalsifiable.
[[ "${index_status}" == "${adr_status%% *}" ]] ||
  fail "docs/adr/README.md 0708 row status '${index_status}' disagrees with the ADR header '${adr_status}'"

pass "README indexes 0708 under theme 07 as Current, in sync with the ADR header"

echo "All dist ADR-0708 index tests passed."
