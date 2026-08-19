#!/usr/bin/env bash
# DIST-AH-03: ADR-0709 (chart/image GHCR path separation) must exist, be indexed under
# theme 07, and keep its header status in sync with the index row.
#
# It also pins the two facts a future reader must not get wrong, because both were
# established the expensive way (see the ADR): the chart moves and the controller image
# does NOT. Reversing them would invalidate OLM bundles already merged upstream.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ADR="${ROOT}/docs/adr/0709-chart-image-oci-path-separation.md"
INDEX="${ROOT}/docs/adr/README.md"

fail() {
  printf 'dist adr 0709 indexed: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${ADR}" ]] || fail "${ADR} is missing"

grep -Eq '^# ADR-0709:' "${ADR}" || fail "ADR missing title/heading for ADR-0709"

# Status lives on the header line next to the theme. Match THAT line, not any occurrence
# of the word in the body -- a file-wide grep would pass on prose.
adr_status="$(grep -E '^\*\*Theme:\*\*.*\*\*Status:\*\*' "${ADR}" | head -1 |
  sed -E 's/^.*\*\*Status:\*\*[[:space:]]*//; s/[[:space:]]*$//')"
[[ -n "${adr_status}" ]] || fail "ADR-0709 header line carries no **Status:**"

# Deliberately not pinned to a literal: 0709 is expected to move Accepted -> Current once
# the migration ships. Assert it belongs to the repo's vocabulary
# (docs/development/adr-rfc-process.md) so a typo cannot slip through.
case "${adr_status%% *}" in
  Current|Accepted|Exploring|Parked|Dropped) ;;
  *) fail "ADR-0709 status '${adr_status}' is not part of the ADR status vocabulary" ;;
esac

pass "ADR-0709 file present with a valid status (${adr_status%% *})"

[[ -f "${INDEX}" ]] || fail "${INDEX} is missing"

theme07="$(
  awk '
    /^## 07 / {in07=1; next}
    /^## / {if (in07) exit}
    in07 {print}
  ' "${INDEX}"
)"
[[ -n "${theme07}" ]] || fail "theme 07 section missing from ${INDEX}"

printf '%s\n' "${theme07}" |
  grep -Eq '\[0709\]\(0709-chart-image-oci-path-separation\.md\)' ||
  fail "docs/adr/README.md theme 07 must index [0709](0709-chart-image-oci-path-separation.md)"

index_status="$(printf '%s\n' "${theme07}" |
  grep -E '\[0709\]\(0709-chart-image-oci-path-separation\.md\)' | head -1 |
  awk -F'|' '{gsub(/^[[:space:]]+|[[:space:]]+$/, "", $4); print $4}')"
[[ -n "${index_status}" ]] || fail "docs/adr/README.md 0709 row carries no status cell"

# The header may carry a parenthetical; the table cell is bare, so compare only the
# leading word. This is the ONLY status assertion on the index row -- pinning it to a
# literal as well would make the comparison unfalsifiable.
[[ "${index_status}" == "${adr_status%% *}" ]] ||
  fail "docs/adr/README.md 0709 row status '${index_status}' disagrees with the ADR header '${adr_status}'"

pass "README indexes 0709 under theme 07, in sync with the ADR header"

# --- the decision itself -------------------------------------------------------------
# Direction matters more than prose here. The chart is the artifact that moves.
grep -Fq 'ghcr.io/platformrelay/charts/kollect' "${ADR}" ||
  fail "ADR-0709 must name the accepted chart coordinate ghcr.io/platformrelay/charts/kollect"

grep -Eq 'controller image (stays|must not move)' "${ADR}" ||
  fail "ADR-0709 must state that the controller image does not move"

pass "ADR-0709 records the chart coordinate and that the image stays put"

echo "All dist ADR-0709 index tests passed."
