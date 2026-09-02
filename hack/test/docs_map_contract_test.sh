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
# DOCS-MAPGATE-02: the four rules above only bind the rows the PARSER CAN SEE, and the first
# version of this gate silently dropped every row it could not read. It recognised a table row by
# the literal prefix `| `, so writing one row as `|**Reference**|` -- or merely indenting it by a
# single space -- dropped that row's links and let the gate's own headline defect, the `FAQ`
# label, through at exit 0. python-markdown renders all three spellings as the SAME table row and
# `markdownlint-cli2` flags none of them, so nothing else in the repo would have noticed. The
# same blind spot swallowed reference-style links (`[FAQ][fq]` plus a `[fq]: ...` definition),
# which the inline-only `[text](target)` pattern could not see at all.
#
# The fix is not a bigger floor -- a floor structurally cannot tell "20 rows parsed" from "30 rows
# present, 10 silently skipped". Instead the parser now:
#   * recognises a row by its first non-blank character being a pipe, so indentation and a missing
#     space after the pipe are irrelevant, exactly as they are to Markdown;
#   * locates the header and delimiter rows POSITIONALLY rather than by their text, so renaming
#     the `Section` column or writing `|---|---|` cannot reclassify them;
#   * resolves reference-style links against the file's `[label]: target` definitions; and
#   * REPORTS what it cannot read instead of skipping it -- an entry row that yields no link, a
#     reference with no definition, and any leftover bracketed construct are all hard failures.
#     A parser that silently drops input it cannot parse is the very defect this gate exists to
#     catch in the map; it must not commit it itself.
#
# Every parse step is guarded against a vacuous pass: an empty map, an entry row with no links, an
# empty nav index and an empty redirect map are hard failures, because a gate that parses nothing
# passes everything. The self-test at the bottom mutates a copy of the real index -- and of
# mkdocs.yml, for the two floors driven from it -- and proves each rule AND each vacuity floor
# rejects the shape it exists to catch. A gate that has never been watched failing is not a gate.
#
# Usage: docs_map_contract_test.sh [path/to/index.md]   (defaults to docs/index.md)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly ROOT
DOCS_DIR="${ROOT}/docs"
MKDOCS="${ROOT}/mkdocs.yml"
readonly DOCS_DIR MKDOCS
# The mkdocs.yml the parsers below read. Deliberately NOT readonly: the self-test points it at a
# mutated copy to prove the two vacuity floors driven from mkdocs.yml (the redirect-map floor and
# the nav floor) actually fire. Nothing outside the self-test reassigns it.
MKDOCS_FILE="${MKDOCS}"

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
  #
  # DOCS-MAPGATE-02 considered relabelling the map cell to the H1 to retire this exemption, and
  # kept the exemption: "Custom resources" is the term the rest of the docs and the CRD kinds use,
  # and a map is a list of destinations, not of page titles -- the word "reference" is already
  # carried by the section this row sits in. Editing the map to suit the gate rather than the
  # reader inverts the priority stated at the top of this file.
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

# Records describing the Documentation map table, four Unit-Separator-delimited fields each. A
# non-whitespace delimiter is deliberate: with IFS=$'\t' bash's `read` collapses runs of tabs and
# would silently merge an empty field into its neighbour.
#
#   ROW   <n> <the row as written>   <empty>   -- one per table line, in file order
#   LINK  <n> <label>                <target>  -- one per resolved link in row <n>
#   UNDEF <n> <label>                <ref>     -- a reference-style link with no definition
#   STRAY <n> <what is left of row>  <empty>   -- a bracketed construct that is not a link
#
# The first cell of each row is the SECTION name (editorial grouping, not a link) and is dropped
# before links are read, so grouping stays a free choice while destinations do not.
#
# UNDEF and STRAY exist so that anything unreadable is REPORTED rather than skipped: silently
# dropping a row is what let `|**Reference**|` and `[FAQ][fq]` past the first version of this gate.
extract_map() {
  local index="$1"
  awk '
    function emit(kind, rowno, a, b) { print kind sep rowno sep a sep b }
    BEGIN { sep = sprintf("%c", 31) }
    {
      line = $0
      sub(/^[ \t]+/, "", line)
      sub(/[ \t]+$/, "", line)
    }
    # Link reference definitions may sit anywhere in a Markdown file, not just in the map
    # section, so they are collected from every line before the section state machine runs.
    line ~ /^\[[^]]+\][ \t]*:[ \t]*[^ \t]/ {
      cb = index(line, "]")
      lbl = tolower(substr(line, 2, cb - 2))
      val = substr(line, cb + 1)
      sub(/^[ \t]*:[ \t]*/, "", val)
      sub(/[ \t].*$/, "", val)
      defs[lbl] = val
      next
    }
    /^## Documentation map/ { inmap = 1; next }
    inmap && /^## / { inmap = 0 }
    # A table row is any line whose first non-blank character is a pipe. Matching `^\| ` instead
    # is what made `|**Reference**|` and a one-space indent invisible.
    inmap && line ~ /^\|/ {
      n++
      rowtext[n] = line
    }
    END {
      for (r = 1; r <= n; r++) {
        emit("ROW", r, rowtext[r], "")
        rest = rowtext[r]
        sub(/^\|[^|]*\|/, "", rest)
        # Inline `[label](target)` and reference-style `[label][ref]` / `[label][]`, in one
        # leftmost-longest scan. Each match is cut out of the row so that whatever brackets
        # remain afterwards can be reported rather than ignored.
        while (match(rest, /\[[^]]*\](\([^)]*\)|\[[^]]*\])/)) {
          link = substr(rest, RSTART, RLENGTH)
          rest = substr(rest, 1, RSTART - 1) substr(rest, RSTART + RLENGTH)
          cb = index(link, "]")
          label = substr(link, 2, cb - 2)
          tail = substr(link, cb + 1)
          if (substr(tail, 1, 1) == "(") {
            emit("LINK", r, label, substr(tail, 2, length(tail) - 2))
          } else {
            ref = tolower(substr(tail, 2, length(tail) - 2))
            if (ref == "") { ref = tolower(label) }
            if (ref in defs) { emit("LINK", r, label, defs[ref]) } else { emit("UNDEF", r, label, ref) }
          }
        }
        if (index(rest, "[") > 0) { emit("STRAY", r, rest, "") }
      }
    }
  ' "${index}"
}

# True for a Markdown table's delimiter line: pipes, dashes, colons and spaces only, with at
# least one dash. `| --- | --- |` and `|---|---|` are the same line to Markdown.
is_delimiter_row() {
  local row="$1"
  local delimiter_re='^[|[:space:]:-]+$'
  [[ "${row}" == *-* ]] || return 1
  [[ "${row}" =~ ${delimiter_re} ]]
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
  ' "${MKDOCS_FILE}"
}

# Count of nav entries overall -- a zero here would make every rule-4 lookup vacuously "no nav
# label", silently degrading rule 4 to an H1-only check.
nav_entry_count() {
  awk '
    /^nav:/ { innav = 1; next }
    innav && /^[a-zA-Z_]/ { innav = 0 }
    innav && /: *[^ ]+\.md$/ { n++ }
    END { print n + 0 }
  ' "${MKDOCS_FILE}"
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
  ' "${MKDOCS_FILE}"
}

# --- The contract ----------------------------------------------------------------------------

check_map() {
  local index="$1"
  local -a row_texts=() labels=() dests=() dest_paths=() link_rows=()
  local label dest basename_no_ext nav_label h1 matched
  local i j seen_dup=0 links=0 row_count=0 row_has_link
  local kind num field3 field4 rel

  [[ -f "${index}" ]] || fail "expected an index file at ${index}"
  rel="${index#"${ROOT}"/}"

  while IFS=$'\x1f' read -r kind num field3 field4; do
    case "${kind}" in
    ROW)
      row_texts+=("${field3}")
      row_count=$((row_count + 1))
      ;;
    LINK)
      labels+=("${field3}")
      dests+=("${field4}")
      # A destination may carry an anchor (`crds/index.md#kinds`). The page half is what exists
      # on disk, has a nav label and has an H1; the anchor half is the site's business.
      dest_paths+=("${field4%%#*}")
      link_rows+=("${num}")
      links=$((links + 1))
      ;;
    UNDEF)
      fail "${rel}'s Documentation map row ${num} uses the reference-style link '[${field3}][${field4}]', but the file defines no '[${field4}]: <target>' -- Markdown renders that as literal text rather than a link, and this gate cannot check where it claims to point"
      ;;
    STRAY)
      fail "${rel}'s Documentation map row ${num} carries a bracketed construct this gate cannot read as a link: '${field3}' -- write map entries as inline [label](destination) links, or as reference-style links with a matching '[ref]: <target>' definition. Reporting it is deliberate: a row the parser cannot read would otherwise be skipped in silence, which is the defect this gate exists to catch"
      ;;
    esac
  done < <(extract_map "${index}")

  # Vacuity guard: a renamed heading or a reshaped table must fail loudly, not silently pass.
  [[ "${row_count}" -gt 0 ]] ||
    fail "${rel} has no parseable '## Documentation map' table -- the rules below would pass vacuously"

  # A pipe table is a header row, a delimiter row, then the entries. Both are identified
  # POSITIONALLY: keying off their text (`| Section`, `| ---`) meant renaming the column or
  # writing `|---|---|` quietly reclassified a structural row as an entry, or an entry as
  # structure.
  [[ "${row_count}" -ge 3 ]] ||
    fail "${rel}'s Documentation map has only ${row_count} table line(s) -- a Markdown table needs a header row, a delimiter row and at least one entry"
  is_delimiter_row "${row_texts[1]}" ||
    fail "${rel}'s Documentation map has no delimiter row beneath its header -- expected '| --- | --- |' as the second table line, got: ${row_texts[1]}"
  for i in "${!link_rows[@]}"; do
    [[ "${link_rows[$i]}" -ge 3 ]] ||
      fail "${rel}'s Documentation map carries a link ('${labels[$i]}') in its header or delimiter row -- the table does not start where this gate thinks it does, so every row number below is wrong"
  done

  # The anti-silent-drop rule. Every entry row must yield at least one link: a row this parser
  # cannot read is REPORTED here rather than quietly contributing nothing, which is how
  # `|**Reference**|` hid four links and the `FAQ` defect behind an exit 0.
  for ((i = 3; i <= row_count; i++)); do
    row_has_link=0
    for j in "${!link_rows[@]}"; do
      [[ "${link_rows[$j]}" -eq "${i}" ]] && row_has_link=1
    done
    [[ "${row_has_link}" -eq 1 ]] ||
      fail "${rel}'s Documentation map row ${i} yields no [label](destination) link, so none of the rules below can see it: ${row_texts[$((i - 1))]}"
  done
  pass "every Documentation map entry row yields at least one parseable link"

  # A floor cannot tell a shrunken table from a silently skipped one -- that is what the per-row
  # rule above is for. It still catches the table being gutted or the heading being renamed onto
  # a different, smaller table.
  [[ "${links}" -ge 20 ]] ||
    fail "${rel}'s Documentation map parsed only ${links} link(s) -- the table shrank or the parser broke; either way the rules below are no longer covering the map"

  # Rule 1: every destination exists.
  for i in "${!dests[@]}"; do
    [[ -n "${dest_paths[$i]}" ]] ||
      fail "Documentation map link '${labels[$i]}' points at '${dests[$i]}', which names no page"
    [[ -e "${DOCS_DIR}/${dest_paths[$i]}" ]] ||
      fail "Documentation map link '${labels[$i]}' points at docs/${dest_paths[$i]}, which does not exist"
  done
  pass "all ${links} Documentation map destinations exist"

  # Rule 2: no two labels point at the same destination. This is the defect that shipped as
  # "Performance tuning" and "Scaling & fleet" both aiming at operator-manual/performance.md --
  # two names for one page teaches the reader the site has two pages. Compared on the PAGE, not
  # on the link as written: two labels deep-linking to different anchors of one page are still
  # two names for one page in a map.
  for i in "${!dest_paths[@]}"; do
    for ((j = i + 1; j < ${#dest_paths[@]}; j++)); do
      if [[ "${dest_paths[$i]}" == "${dest_paths[$j]}" ]]; then
        echo "FAIL: Documentation map points two labels at docs/${dest_paths[$i]}: '${labels[$i]}' (${dests[$i]}) and '${labels[$j]}' (${dests[$j]})" >&2
        seen_dup=1
      fi
    done
  done
  [[ "${seen_dup}" -eq 0 ]] ||
    fail "each page may appear in the Documentation map under exactly one label"
  pass "no two Documentation map labels point at the same page"

  # Rule 3: no label names a page that no longer exists.
  local -a gone_names=() gone_targets=()
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
    dest="${dest_paths[$i]}"
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
    dest="${dest_paths[$i]}"
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
# Self-test: mutate a copy of the real index -- and of mkdocs.yml, for the floors driven from it
# -- and prove each rule AND each vacuity floor rejects the shape it exists to catch, including
# the defects this gate was written in response to.
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

# The two vacuity floors below are driven from mkdocs.yml, not from the map, so their mutants are
# mkdocs.yml copies checked against the REAL, unmutated index.
mkdocs_mutant_rejected() {
  local mutant="$1" label="$2" expect="$3"
  local output status=0

  [[ -s "${mutant}" ]] ||
    fail "self-test: the mkdocs.yml mutant for '${label}' is missing or empty -- nothing was actually tested"
  if cmp -s "${mutant}" "${MKDOCS}"; then
    fail "self-test: the mkdocs.yml mutant for '${label}' is byte-identical to mkdocs.yml -- the mutation was a no-op, so nothing was actually tested"
  fi

  output="$( (
    MKDOCS_FILE="${mutant}"
    check_map "${DOCS_DIR}/index.md"
  ) 2>&1 )" || status=$?
  [[ "${status}" -ne 0 ]] ||
    fail "self-test: the gate still passed with a mkdocs.yml where ${label} -- that floor is inert"
  [[ "${output}" == *"${expect}"* ]] ||
    fail "self-test: the gate rejected '${label}' but not for the intended reason -- expected a mention of '${expect}', got: ${output}"
  pass "self-test: gate rejects a mkdocs.yml where ${label}"
}

# --- the four rules ---

sed 's#(reference/conditions.md)#(reference/conditions-that-never-existed.md)#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/dead-link.md"
mutant_rejected "${MUTANTS}/dead-link.md" \
  'a destination does not exist' 'does not exist'

# The exact shape that shipped: a second label aimed at a page the map already lists.
sed 's#\[Troubleshooting\](operator-manual/troubleshooting.md)#[Troubleshooting](operator-manual/troubleshooting.md) · [Common errors](operator-manual/troubleshooting.md)#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/two-labels-one-page.md"
mutant_rejected "${MUTANTS}/two-labels-one-page.md" \
  'two labels point at one page' 'two labels at'

# The same rule with rules 3 and 4 held out of the way. Assets have neither a nav entry nor an
# H1, so a second label on the package graph can be caught by NOTHING but rule 2 -- whereas the
# mutant above is also claimed by rule 3 ("Common errors" is itself a deleted page name), which
# means neutering rule 2 alone would not have shown up there.
sed 's#\[Metrics\](operator-manual/metrics.md)#[Metrics](operator-manual/metrics.md) · [Diagram](architecture-graph.svg)#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/two-labels-one-asset.md"
mutant_rejected "${MUTANTS}/two-labels-one-asset.md" \
  'two labels point at one asset, which no other rule covers' 'two labels at'

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

# --- the parser's own blind spots (DOCS-MAPGATE-02) ---
# Each of these carries the FAQ defect INSIDE a row written in a shape the first version of this
# gate could not read. Every one of them exited 0 before the parser was rewritten; each must now
# be rejected for the FAQ reason, which is only possible if the row was actually seen.

sed 's#^| \*\*Run in production\*\* | \[Performance and scaling\](operator-manual/performance.md) · \[Production checklist\](operator-manual/production-checklist.md) · \[Troubleshooting\](operator-manual/troubleshooting.md) |$#|**Run in production**| [Performance and scaling](operator-manual/performance.md) · [Production checklist](operator-manual/production-checklist.md) · [FAQ](operator-manual/troubleshooting.md) |#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/row-without-space-after-pipe.md"
mutant_rejected "${MUTANTS}/row-without-space-after-pipe.md" \
  'a row is written without a space after its leading pipe' 'removed in the docs restructure'

sed 's#^| \*\*Run in production\*\* | \[Performance and scaling\](operator-manual/performance.md) · \[Production checklist\](operator-manual/production-checklist.md) · \[Troubleshooting\](operator-manual/troubleshooting.md) |$# | **Run in production** | [Performance and scaling](operator-manual/performance.md) · [Production checklist](operator-manual/production-checklist.md) · [FAQ](operator-manual/troubleshooting.md) |#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/row-indented.md"
mutant_rejected "${MUTANTS}/row-indented.md" \
  'a row is indented by one space' 'removed in the docs restructure'

{
  sed 's#\[Troubleshooting\](operator-manual/troubleshooting.md)#[FAQ][fq]#' "${DOCS_DIR}/index.md"
  printf '\n[fq]: operator-manual/troubleshooting.md\n'
} >"${MUTANTS}/reference-style-link.md"
mutant_rejected "${MUTANTS}/reference-style-link.md" \
  'a link is written in reference style against a definition' 'removed in the docs restructure'

# A reference-style link with no definition renders as literal text. It must be reported, not
# treated as "this row simply has one link fewer".
sed 's#\[Troubleshooting\](operator-manual/troubleshooting.md)#[Troubleshooting][undefined-ref]#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/undefined-reference.md"
mutant_rejected "${MUTANTS}/undefined-reference.md" \
  'a reference-style link has no definition' 'defines no'

# The shortcut reference form, and every other bracketed construct the parser cannot read: the
# residue must be named rather than dropped.
sed 's#\[Troubleshooting\](operator-manual/troubleshooting.md)#[Troubleshooting]#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/stray-bracket.md"
mutant_rejected "${MUTANTS}/stray-bracket.md" \
  'a row carries a bracketed construct that is not a link' 'cannot read as a link'

# An entry row whose links are gone entirely: prose in a table cell is not a map entry.
sed 's#^| \*\*Design & internals\*\* | .*$#| **Design \& internals** | see the sidebar |#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/linkless-row.md"
mutant_rejected "${MUTANTS}/linkless-row.md" \
  'an entry row yields no link at all' 'yields no [label](destination) link'

# --- the vacuity floors themselves ---

sed 's/^## Documentation map$/## Where to start/' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/heading-renamed.md"
mutant_rejected "${MUTANTS}/heading-renamed.md" \
  'the map heading is renamed so nothing parses' 'would pass vacuously'

sed -e '/^| \*\*Run in production\*\* |/d' -e '/^| \*\*Reference\*\* |/d' \
  -e '/^| \*\*Design & internals\*\* |/d' -e '/^| \*\*Contributing\*\* |/d' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/table-gutted.md"
mutant_rejected "${MUTANTS}/table-gutted.md" \
  'half the table is deleted' 'the table shrank or the parser broke'

# The header and delimiter rows are located positionally, so losing the delimiter must be a hard
# failure rather than a silent off-by-one through every row number above.
sed '/^| --- | --- |$/d' "${DOCS_DIR}/index.md" >"${MUTANTS}/delimiter-row-removed.md"
mutant_rejected "${MUTANTS}/delimiter-row-removed.md" \
  'the table has no delimiter row beneath its header' 'no delimiter row beneath its header'

sed '/^| \*\*/d' "${DOCS_DIR}/index.md" >"${MUTANTS}/entry-rows-removed.md"
mutant_rejected "${MUTANTS}/entry-rows-removed.md" \
  'the table is left with a header and a delimiter and no entries' 'needs a header row, a delimiter row and at least one entry'

# A link in the header row means the entries do not start on the third table line, so every row
# number the messages above quote would be wrong.
sed 's#^| Section | Start here |$#| Section | [Start here](getting-started/install.md) |#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/link-in-header-row.md"
mutant_rejected "${MUTANTS}/link-in-header-row.md" \
  'the header row itself carries a link' 'in its header or delimiter row'

# An anchor-only destination names no page, so rule 1 has nothing to look for on disk.
sed 's|\[Glossary\](GLOSSARY.md)|[Glossary](#glossary)|' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/anchor-only-destination.md"
mutant_rejected "${MUTANTS}/anchor-only-destination.md" \
  'a destination is a bare anchor with no page' 'which names no page'

# The redirect-map floor: rule 3 has nothing to check against once redirect_maps is emptied out.
awk '
  /^[[:space:]]*redirect_maps:[[:space:]]*$/ { inmap = 1; kept = 0; print; next }
  inmap && !/^        [^ ]/ { inmap = 0 }
  inmap { kept++; if (kept > 5) next }
  { print }
' "${MKDOCS}" >"${MUTANTS}/redirects-gutted.yaml"
mkdocs_mutant_rejected "${MUTANTS}/redirects-gutted.yaml" \
  'redirect_maps lists too few deleted pages for rule 3 to mean anything' \
  'rule 3 has no deleted pages to check against'

# The nav floor: rule 4 degrades to an H1-only check once the nav index is empty. The one entry
# kept is concepts/architecture.md, whose H1 ("Kollect architecture") does NOT match the map
# label "Architecture" -- without it rule 3 would claim this mutant first, for its own reason.
awk '
  /^nav:/ { innav = 1; print; next }
  innav && /^[a-zA-Z_]/ { innav = 0 }
  innav && /: *[^ ]+\.md$/ && $0 !~ /concepts\/architecture\.md$/ { next }
  { print }
' "${MKDOCS}" >"${MUTANTS}/nav-gutted.yaml"
mkdocs_mutant_rejected "${MUTANTS}/nav-gutted.yaml" \
  'the nav index is emptied out' \
  'rule 4 would silently degrade to an H1-only check'

# --- guard rails ---

# Guard rail: the rules must not fire on the real map, or every "rejection" above is noise.
cp "${DOCS_DIR}/index.md" "${MUTANTS}/unmutated.md"
if ! (check_map "${MUTANTS}/unmutated.md") >/dev/null 2>&1; then
  fail "self-test: the gate rejects an unmutated copy of docs/index.md -- the rules above are firing on noise"
fi
pass "self-test: gate accepts an unmutated copy of the real map"

# The self-test's own guard rails: the degenerate "rejections" that an any-nonzero-exit check
# would have accepted as proof. mutant_rejected must refuse every one of them.
self_test_guard_holds() {
  local mutant="$1" label="$2"
  if (mutant_rejected "${mutant}" "${label}" 'unreachable-expected-message') >/dev/null 2>&1; then
    fail "self-test: mutant_rejected accepted ${label} as a genuine rejection -- it is tautological again"
  fi
  pass "self-test: mutant_rejected refuses to count ${label} as a rejection"
}

: >"${MUTANTS}/empty.md"
self_test_guard_holds "${MUTANTS}/empty.md" 'an empty file'
self_test_guard_holds "${MUTANTS}/does-not-exist.md" 'a nonexistent path'
self_test_guard_holds "${MUTANTS}/unmutated.md" 'an unmutated copy of the real map'

echo "All docs map contract tests passed."
