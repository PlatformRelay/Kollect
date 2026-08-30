#!/usr/bin/env bash
# DOC-MAP-01: locks the "Documentation map" table in docs/index.md to the pages it claims.
#
# The map is the front door: a first-time evaluator reads it before anything else. It rotted
# silently through the whole go-by-example docs restructure -- it still routed readers into raw
# ADRs instead of the concepts pages built for them, carried a "FAQ" entry for a page that had
# been dissolved, labelled `production-checklist.md` with its pre-redesign name "Best practices",
# and aimed TWO different labels ("Performance tuning", "Scaling & fleet") at ONE page -- while
# every gate in the repo stayed green.
#
# Nothing could have caught it. `mkdocs build --strict` promotes `unrecognized_links` to an error,
# but every one of those links RESOLVED; they just misinformed. A link that resolves and lies is
# invisible to a link checker, so the only enforceable contract is between the label and the
# identity of the page it points at. That is what this gate asserts:
#
#   1. every destination exists on disk;
#   2. no two labels point at the same destination (the two-labels-one-page defect);
#   3. no label names a page that no longer exists -- driven from `mkdocs.yml`'s `redirect_maps`
#      keys, which ARE precisely the set of pages the restructure deleted;
#   4. every label matches the destination's nav label in `mkdocs.yml` or its H1.
#
# Rule 4 is the strong one and rules 1-3 are sharper-message subsets of it; all four are kept
# because each names a different failure to the person who has to fix it.
#
# Deviations are ENCODED BELOW as explicit, commented exemptions rather than by loosening the
# matching -- a looser rule 4 would have re-admitted every defect above.
#
# Every parse step is guarded against a vacuous pass: an empty map, an empty nav index and an
# empty redirect map are hard failures, because a gate that parses nothing passes everything.
# The self-test at the bottom mutates a copy of the real index and proves each rule rejects the
# shape it exists to catch. A gate that has never been watched failing is not a gate.
#
# Usage: docs_map_contract_test.sh [path/to/index.md]   (defaults to docs/index.md)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly ROOT
DOCS_DIR="${ROOT}/docs"
MKDOCS="${ROOT}/mkdocs.yml"
readonly DOCS_DIR MKDOCS

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() { echo "ok - $*"; }

# Compare names by their letters and digits only. "Best practices", "best-practices" and
# "BEST-PRACTICES.md" are the same name to a reader, and the map must not be able to smuggle a
# stale label past this gate on punctuation or case alone.
normalize() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]//g'
}

# --- Declared exemptions to rule 4 -----------------------------------------------------------
# Each entry is "<destination>|<label>" and needs a reason. Adding one is a deliberate act:
# state why the map is MORE faithful to the reader than the nav leaf it deviates from.
readonly LABEL_EXEMPTIONS=(
  # The nav label is "Sink roles: state, snapshots, and streams". The map cell already carries
  # three sibling links, so it uses the unambiguous prefix. It names no other page.
  "concepts/sinks.md|Sink roles"
  # The nav LEAF for this page is "Overview" -- a navigation-local word that means nothing in a
  # cross-page map. "Custom resources" is this page's parent nav section and matches its H1
  # ("Custom resource reference") in substance.
  "crds/index.md|Custom resources"
)

is_exempt() {
  local dest="$1" label="$2" entry
  for entry in "${LABEL_EXEMPTIONS[@]}"; do
    [[ "${entry}" == "${dest}|${label}" ]] && return 0
  done
  return 1
}

# --- Parsers ---------------------------------------------------------------------------------

# "label<TAB>destination", one per link, for every row of the Documentation map table.
# The first cell of each row is the SECTION name (editorial grouping, not a link) and is dropped
# before links are read, so grouping stays a free choice while destinations do not.
extract_map() {
  local index="$1"
  awk '
    /^## Documentation map/ { inmap = 1; next }
    inmap && /^## / { inmap = 0 }
    inmap && /^\| / { print }
  ' "${index}" |
    grep -v '^| *---' |
    grep -v '^| Section' |
    sed 's/^| [^|]*| //' |
    grep -oE '\[[^]]*\]\([^)]*\)' |
    sed -E 's/^\[([^]]*)\]\((.*)\)$/\1\t\2/'
}

# Every nav label pointing at ${1}. A page may legitimately appear once; more than one entry is
# still fine here -- any of them satisfies rule 4.
nav_labels_for() {
  local target="$1"
  awk -v t="${target}" '
    /^nav:/ { innav = 1; next }
    innav && /^[a-zA-Z_]/ { innav = 0 }
    innav {
      line = $0
      # Leftmost-longest match of ": <no-spaces-to-EOL>" lands on the FINAL colon, so a quoted
      # label that itself contains a colon still splits at the right place.
      if (match(line, /: *[^ ]+$/)) {
        path = substr(line, RSTART + 1)
        gsub(/^[ \t]+|[ \t]+$/, "", path)
        if (path == t) {
          label = substr(line, 1, RSTART - 1)
          sub(/^[ \t]*-[ \t]*/, "", label)
          gsub(/^"|"$/, "", label)
          print label
        }
      }
    }
  ' "${MKDOCS}"
}

# Count of nav entries overall -- a zero here would make every rule-4 lookup vacuously "no nav
# label", silently degrading rule 4 to an H1-only check.
nav_entry_count() {
  awk '
    /^nav:/ { innav = 1; next }
    innav && /^[a-zA-Z_]/ { innav = 0 }
    innav && /: *[^ ]+\.md$/ { n++ }
    END { print n + 0 }
  ' "${MKDOCS}"
}

h1_for() {
  local path="$1"
  [[ -f "${DOCS_DIR}/${path}" ]] || return 0
  sed -n 's/^# //p' "${DOCS_DIR}/${path}" | head -n 1
}

# "deleted-page-path<TAB>redirect-target", from mkdocs.yml's redirect_maps. The keys are exactly
# the pages the restructure removed, which is why they are the authority for rule 3.
deleted_pages() {
  awk '
    /^[[:space:]]*redirect_maps:[[:space:]]*$/ { inmap = 1; next }
    inmap && !/^        [^ ]/ { inmap = 0 }
    inmap {
      line = $0
      sub(/^[[:space:]]+/, "", line)
      n = index(line, ": ")
      if (n > 0) { print substr(line, 1, n - 1) "\t" substr(line, n + 2) }
    }
  ' "${MKDOCS}"
}

# --- The contract ----------------------------------------------------------------------------

check_map() {
  local index="$1"
  local -a labels dests
  local label dest basename_no_ext nav_label h1 matched
  local i j seen_dup=0 rows=0

  [[ -f "${index}" ]] || fail "expected an index file at ${index}"

  while IFS=$'\t' read -r label dest; do
    [[ -z "${label}" && -z "${dest}" ]] && continue
    labels+=("${label}")
    dests+=("${dest}")
    rows=$((rows + 1))
  done < <(extract_map "${index}")

  # Vacuity guard: a renamed heading or a reshaped table must fail loudly, not silently pass.
  [[ "${rows}" -gt 0 ]] ||
    fail "${index#"${ROOT}"/} has no parseable '## Documentation map' table -- the rules below would pass vacuously"
  [[ "${rows}" -ge 20 ]] ||
    fail "${index#"${ROOT}"/}'s Documentation map parsed only ${rows} link(s) -- the table shrank or the parser broke; either way the rules below are no longer covering the map"

  # Rule 1: every destination exists.
  for i in "${!dests[@]}"; do
    [[ -e "${DOCS_DIR}/${dests[$i]}" ]] ||
      fail "Documentation map link '${labels[$i]}' points at docs/${dests[$i]}, which does not exist"
  done
  pass "all ${rows} Documentation map destinations exist"

  # Rule 2: no two labels point at the same destination. This is the defect that shipped as
  # "Performance tuning" and "Scaling & fleet" both aiming at operator-manual/performance.md --
  # two names for one page teaches the reader the site has two pages.
  for i in "${!dests[@]}"; do
    for ((j = i + 1; j < ${#dests[@]}; j++)); do
      if [[ "${dests[$i]}" == "${dests[$j]}" ]]; then
        echo "FAIL: Documentation map points two labels at docs/${dests[$i]}: '${labels[$i]}' and '${labels[$j]}'" >&2
        seen_dup=1
      fi
    done
  done
  [[ "${seen_dup}" -eq 0 ]] ||
    fail "each page may appear in the Documentation map under exactly one label"
  pass "no two Documentation map labels point at the same page"

  # Rule 3: no label names a page that no longer exists.
  local -a gone_names gone_targets
  local gone_count=0 gone_path gone_target
  while IFS=$'\t' read -r gone_path gone_target; do
    [[ -z "${gone_path}" ]] && continue
    basename_no_ext="${gone_path##*/}"
    basename_no_ext="${basename_no_ext%.md}"
    gone_names+=("$(normalize "${basename_no_ext}")")
    gone_targets+=("${gone_target}")
    gone_count=$((gone_count + 1))
  done < <(deleted_pages)

  [[ "${gone_count}" -ge 10 ]] ||
    fail "parsed only ${gone_count} entries from mkdocs.yml redirect_maps -- rule 3 has no deleted pages to check against and would pass vacuously"

  for i in "${!labels[@]}"; do
    label="${labels[$i]}"
    dest="${dests[$i]}"
    [[ "${dest}" == *.md ]] || continue
    nav_label="$(nav_labels_for "${dest}" | head -n 1)"
    h1="$(h1_for "${dest}")"
    for j in "${!gone_names[@]}"; do
      [[ "$(normalize "${label}")" == "${gone_names[$j]}" ]] || continue
      # A label may legitimately carry a deleted page's name when the destination genuinely
      # still goes by that name today (ARCHITECTURE.md -> concepts/architecture.md, nav label
      # "Architecture"). It may NOT when the destination has its own, different identity --
      # that is the "FAQ -> troubleshooting.md" defect.
      if [[ "$(normalize "${label}")" != "$(normalize "${nav_label}")" &&
        "$(normalize "${label}")" != "$(normalize "${h1}")" ]]; then
        fail "Documentation map label '${label}' names a page removed in the docs restructure (mkdocs.yml redirects it to ${gone_targets[$j]}), but points at docs/${dest}, whose own name is '${nav_label:-${h1}}' -- label the destination, not the page it replaced"
      fi
    done
  done
  pass "no Documentation map label names a page deleted by the docs restructure"

  # Rule 4: the label matches the destination's nav label or its H1.
  [[ "$(nav_entry_count)" -ge 40 ]] ||
    fail "parsed only $(nav_entry_count) page entries from mkdocs.yml nav -- rule 4 would silently degrade to an H1-only check"

  for i in "${!labels[@]}"; do
    label="${labels[$i]}"
    dest="${dests[$i]}"
    # Assets (the architecture package graph) have neither a nav entry nor an H1; rule 1 is the
    # whole contract available for them.
    [[ "${dest}" == *.md ]] || continue
    is_exempt "${dest}" "${label}" && continue

    matched=0
    while IFS= read -r nav_label; do
      [[ -z "${nav_label}" ]] && continue
      [[ "$(normalize "${label}")" == "$(normalize "${nav_label}")" ]] && matched=1
    done < <(nav_labels_for "${dest}")

    if [[ "${matched}" -eq 0 ]]; then
      h1="$(h1_for "${dest}")"
      [[ -n "${h1}" && "$(normalize "${label}")" == "$(normalize "${h1}")" ]] && matched=1
    fi

    [[ "${matched}" -eq 1 ]] || {
      nav_label="$(nav_labels_for "${dest}" | head -n 1)"
      h1="$(h1_for "${dest}")"
      fail "Documentation map labels docs/${dest} as '${label}', but that page is called '${nav_label:-<not in nav>}' in mkdocs.yml nav and '${h1:-<no H1>}' on the page -- rename the label, or add a commented exemption to LABEL_EXEMPTIONS saying why the map is more faithful than the nav"
    }
  done
  pass "every Documentation map label matches its destination's nav label or H1"
}

check_map "${1:-${DOCS_DIR}/index.md}"

# Only the default, whole-repo invocation runs the self-test; an explicit path argument means a
# caller (or the TDD red run) is checking one specific file.
[[ $# -eq 0 ]] || exit 0

# ---------------------------------------------------------------------------
# Self-test: mutate a copy of the real index and prove each rule rejects the shape it exists to
# catch -- including the defects this gate was written in response to.
# ---------------------------------------------------------------------------
MUTANTS="$(mktemp -d)"
trap 'rm -rf "${MUTANTS}"' EXIT

mutant_rejected() {
  local mutant="$1" label="$2" expect="$3"
  local output status=0

  [[ -s "${mutant}" ]] ||
    fail "self-test: the mutant for '${label}' is missing or empty -- nothing was actually tested"
  if cmp -s "${mutant}" "${DOCS_DIR}/index.md"; then
    fail "self-test: the mutant for '${label}' is byte-identical to docs/index.md -- the mutation was a no-op, so nothing was actually tested"
  fi

  output="$( (check_map "${mutant}") 2>&1 )" || status=$?
  [[ "${status}" -ne 0 ]] ||
    fail "self-test: the gate still passed on a map where ${label} -- it is vacuous"
  # An any-nonzero-exit check would count an unparseable file as proof; require the intended reason.
  [[ "${output}" == *"${expect}"* ]] ||
    fail "self-test: the gate rejected '${label}' but not for the intended reason -- expected a mention of '${expect}', got: ${output}"
  pass "self-test: gate rejects a map where ${label}"
}

sed 's#(reference/conditions.md)#(reference/conditions-that-never-existed.md)#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/dead-link.md"
mutant_rejected "${MUTANTS}/dead-link.md" \
  'a destination does not exist' 'does not exist'

# The exact shape that shipped: a second label aimed at a page the map already lists.
sed 's#\[Troubleshooting\](operator-manual/troubleshooting.md)#[Troubleshooting](operator-manual/troubleshooting.md) · [Common errors](operator-manual/troubleshooting.md)#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/two-labels-one-page.md"
mutant_rejected "${MUTANTS}/two-labels-one-page.md" \
  'two labels point at one page' 'two labels at'

# The dissolved-FAQ defect: a label naming a page mkdocs.yml records as deleted.
sed 's#\[Troubleshooting\](operator-manual/troubleshooting.md)#[FAQ](operator-manual/troubleshooting.md)#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/deleted-page-label.md"
mutant_rejected "${MUTANTS}/deleted-page-label.md" \
  'a label names a page deleted by the restructure' 'removed in the docs restructure'

# The pre-redesign-name defect, in its general form. The label here is deliberately NOT itself a
# deleted page name -- rule 3 runs first and would otherwise claim the mutant, leaving rule 4
# untested. ("Best practices" is exactly that case: it is covered by the rule 3 mutant above.)
sed 's#\[Conditions and status\](reference/conditions.md)#[Status reference](reference/conditions.md)#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/stale-label.md"
mutant_rejected "${MUTANTS}/stale-label.md" \
  'a label matches neither the nav label nor the H1' 'rename the label'

# Guard rail: the rules must not fire on the real map, or every "rejection" above is noise.
cp "${DOCS_DIR}/index.md" "${MUTANTS}/unmutated.md"
if ! (check_map "${MUTANTS}/unmutated.md") >/dev/null 2>&1; then
  fail "self-test: the gate rejects an unmutated copy of docs/index.md -- the rules above are firing on noise"
fi
pass "self-test: gate accepts an unmutated copy of the real map"

echo "All docs map contract tests passed."
