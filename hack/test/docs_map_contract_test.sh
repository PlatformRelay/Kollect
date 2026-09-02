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
# DOCS-MAPGATE-02: the four rules above only bind what the PARSER CAN SEE, and the first version
# of this gate silently dropped everything else. It recognised a table row by the literal prefix
# `| ` and read links only from the second cell onwards, so SIX different spellings of a map entry
# -- every one of which the renderer turns into a live link, and none of which `markdownlint-cli2`
# reports -- slipped a planted `FAQ` label or a dead destination past it at exit 0:
#
#   |**Reference**| ...            (no space after the leading pipe)
#    | **Reference** | ...         (one space of indentation)
#   **Reference** | ... |          (no leading pipe at all -- GFM makes it optional)
#   | [Section](gone.md) | ...     (a link in the FIRST cell, which was cut before parsing)
#   [FAQ][fq]  + [fq]: page.md     (reference-style link)
#   <a href="page.md">FAQ</a>      (raw HTML; MD033 is disabled repo-wide)
#
# The fix is NOT a bigger floor -- a floor structurally cannot tell "20 rows parsed" from "30 rows
# present, 10 silently skipped". The invariant this parser now holds instead is:
#
#     every link the renderer produces from the map section is checked, and anything
#     link-shaped this parser cannot resolve is REPORTED rather than skipped.
#
# Concretely:
#   * the table is the run of consecutive non-blank lines in the section whose SECOND line is a
#     delimiter row -- which is how the renderer itself delimits a table. Leading pipes,
#     indentation and spacing are therefore all irrelevant, exactly as they are to Markdown;
#   * the header and delimiter rows are located POSITIONALLY inside that block rather than by
#     their text, so renaming the `Section` column or writing `|---|---|` cannot reclassify them;
#   * links are read from the WHOLE row, first cell included. The first cell is an editorial
#     grouping by convention, not by exemption -- a link there is still a link on the page;
#   * reference-style links are resolved against the file's `[label]: target` definitions; and
#   * what cannot be read is REPORTED: an entry row that yields no link, a reference with no
#     definition, any leftover bracketed construct, and any `<a`/`<img` tag are hard failures.
#     A parser that silently drops input it cannot parse is the very defect this gate exists to
#     catch in the map; it must not commit it itself.
#
# A blank line ends a Markdown table, so the section is swept a second time for anything
# link-shaped OUTSIDE the table: an entry row separated from the table by a blank line, or prose
# carrying a link, is reported rather than silently unread. That hole cost 5 of 30 links, a
# planted FAQ label and a duplicate destination, at exit 0, with `mkdocs build --strict` and
# `markdownlint-cli2` both silent on it.
#
# Direction of error is deliberate. A tab-indented row is an indented code block to the renderer
# but still a row to this parser -- stricter than the renderer, so a loud false positive rather
# than a silent pass. That is the safe side to be wrong on.
#
# Two residual boundaries, named here rather than left to be discovered. An autolink
# (`<https://example.com>`) in a cell renders as a link but is neither checked nor reported: its
# target is external, so none of the four rules could bind it -- it is the one link form the
# "anything link-shaped is REPORTED" clause does not cover. And a `[label]: target` definition
# outside the table is skipped by the sweep, because it renders nothing on its own; it is checked
# where it is USED, inside a row.
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

# Unit Separator, the field delimiter of the parser records below. Not a tab: with IFS=$'\t'
# bash's `read` collapses runs of tabs and would silently merge an empty field into its
# neighbour. No 0x1f byte appears anywhere under docs/ or in mkdocs.yml.
readonly SEP=$'\x1f'

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
#   SECLINES <count>                 <empty>   -- non-blank lines in the map SECTION
#   BLOCKS   <count>                 <empty>   -- tables found in that section
#   ROW   <n> <the row as written>   <empty>   -- one per line of the table, in file order
#   LINK  <n> <label>                <target>  -- one per resolved link in row <n>
#   UNDEF <n> <label>                <ref>     -- a reference-style link with no definition
#   STRAY <n> <what is left of row>  <empty>   -- something link-shaped that is not a link
#
# The table is the run of consecutive non-blank lines whose SECOND line is a delimiter row -- how
# the renderer delimits a table, so leading pipes and indentation are irrelevant here as they are
# there. Links are read from the WHOLE row: the first cell is an editorial grouping by convention,
# and cutting it before parsing is what let a dead destination hide in it at exit 0.
#
# UNDEF and STRAY exist so that anything unreadable is REPORTED rather than skipped: silently
# dropping a row is what let `|**Reference**|` and `[FAQ][fq]` past the first version of this gate.
extract_map() {
  local index="$1"
  awk '
    function trim(s) { sub(/^[ \t]+/, "", s); sub(/[ \t]+$/, "", s); return s }
    function isdelim(s) { return (s ~ /-/ && s ~ /^[|[:space:]:-]+$/) }
    # Anything that renders as, or contains, a link. `<a`/`<img` are here because MD033 is
    # disabled repo-wide, so raw HTML links are invisible to markdownlint too.
    function islinkshaped(s,   lo) {
      lo = tolower(s)
      return (index(s, "[") > 0 || lo ~ "<a[ \t>/]" || lo ~ "<img[ \t>/]")
    }
    # A link reference definition renders nothing on its own; it is checked where it is USED.
    function isdef(s) { return (s ~ /^\[[^]]+\][ \t]*:[ \t]*[^ \t]/) }
    function emit(kind, rowno, a, b) { print kind sep rowno sep a sep b }
    BEGIN { sep = sprintf("%c", 31) }
    { line = trim($0) }
    # Link reference definitions may sit anywhere in a Markdown file, not just in the map
    # section, so they are collected from every line. Deliberately no `next`: a definition line
    # sitting INSIDE the table must still be buffered as a row, or removing it from the buffer
    # would silently splice two blocks together.
    isdef(line) {
      cb = index(line, "]")
      lbl = tolower(substr(line, 2, cb - 2))
      val = substr(line, cb + 1)
      sub(/^[ \t]*:[ \t]*/, "", val)
      sub(/[ \t].*$/, "", val)
      defs[lbl] = val
    }
    /^## Documentation map/ { inmap = 1; next }
    inmap && /^## / { inmap = 0 }
    inmap { sec[++sn] = line }
    END {
      nonblank = 0
      for (i = 1; i <= sn; i++) { if (sec[i] != "") { nonblank++ } }
      emit("SECLINES", nonblank, "", "")

      # Find the table blocks: a run of consecutive non-blank lines whose second line is a
      # delimiter. Prose paragraphs elsewhere in the section are separate runs; link-shaped ones
      # are reported as ORPHAN below rather than ignored.
      nblocks = 0
      i = 1
      while (i <= sn) {
        if (sec[i] == "") { i++; continue }
        j = i
        while (j <= sn && sec[j] != "") { j++ }
        if (j - i >= 2 && isdelim(sec[i + 1])) {
          nblocks++
          if (nblocks == 1) { bs = i; be = j - 1 }
        }
        i = j
      }
      emit("BLOCKS", nblocks, "", "")
      if (nblocks < 1) { exit }

      r = 0
      for (k = bs; k <= be; k++) {
        r++
        emit("ROW", r, sec[k], "")
        rest = sec[k]
        # Inline `[label](target)` and reference-style `[label][ref]` / `[label][]`, in one
        # leftmost-longest scan over the WHOLE row. Each match is cut out so that whatever is
        # left afterwards can be reported rather than ignored.
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
        # Residue that is still link-shaped: a shortcut reference or any other bracket form, and
        # raw HTML anchors/images -- which render as links while MD033 is disabled repo-wide, so
        # markdownlint says nothing about them either.
        if (islinkshaped(rest)) { emit("STRAY", r, rest, "") }
      }

      # Anything link-shaped in the SECTION but outside the table just read. A blank line ends a
      # Markdown table, so an entry row separated by one falls into a second paragraph that is
      # neither a block nor -- until this loop existed -- reported: 30 links rendered, 25
      # checked, exit 0, with a planted FAQ label and a duplicate destination among the 5 lost.
      # Prose carrying a link has the same effect. Reference definitions are skipped: they
      # render nothing on their own and are checked where they are used.
      for (k = 1; k <= sn; k++) {
        if (k >= bs && k <= be) { continue }
        if (sec[k] == "" || isdef(sec[k])) { continue }
        if (islinkshaped(sec[k])) { emit("ORPHAN", k, sec[k], "") }
      }
    }
  ' "${index}"
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
  local -a row_texts=() labels=() dests=() dest_paths=() link_rows=() unreadable=()
  local label dest basename_no_ext nav_label h1 matched
  local i j seen_dup=0 links=0 row_count=0 row_has_link
  local kind num field3 field4 rel sec_lines=0 block_count=0

  [[ -f "${index}" ]] || fail "expected an index file at ${index}"
  rel="${index#"${ROOT}"/}"

  while IFS="${SEP}" read -r kind num field3 field4; do
    case "${kind}" in
    SECLINES) sec_lines="${num}" ;;
    BLOCKS) block_count="${num}" ;;
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
    UNDEF | STRAY | ORPHAN)
      # Deferred, not failed on the spot: a structural problem (no table, two tables) explains
      # these and names the fix better, so the guards below get first refusal.
      unreadable+=("${kind}${SEP}${num}${SEP}${field3}${SEP}${field4}")
      ;;
    esac
  done < <(extract_map "${index}")

  # Vacuity guard: a renamed heading or a reshaped table must fail loudly, not silently pass.
  [[ "${sec_lines}" -gt 0 ]] ||
    fail "${rel} has no parseable '## Documentation map' table -- the rules below would pass vacuously"
  [[ "${block_count}" -ge 1 ]] ||
    fail "${rel}'s Documentation map has no delimiter row beneath its header -- expected a '| --- | --- |' line as the table's second line; without one the renderer produces no table at all, and this gate has no rows to check"
  [[ "${block_count}" -eq 1 ]] ||
    fail "${rel}'s '## Documentation map' section contains ${block_count} tables -- this gate reads the first, so the rest would go unchecked; keep the map in one table"

  # Everything the parser could not read, reported rather than skipped.
  for i in "${!unreadable[@]}"; do
    kind="${unreadable[$i]%%"${SEP}"*}"
    num="${unreadable[$i]#*"${SEP}"}"
    field3="${num#*"${SEP}"}"
    num="${num%%"${SEP}"*}"
    field4="${field3#*"${SEP}"}"
    field3="${field3%%"${SEP}"*}"
    case "${kind}" in
    UNDEF)
      fail "${rel}'s Documentation map row ${num} uses the reference-style link '[${field3}][${field4}]', but the file defines no '[${field4}]: <target>' -- Markdown renders that as literal text rather than a link, and this gate cannot check where it claims to point"
      ;;
    ORPHAN)
      fail "${rel}'s '## Documentation map' section carries a link OUTSIDE the table this gate reads, on section line ${num}: '${field3}' -- a blank line ends a Markdown table, so any entry row after one is a separate paragraph that none of the rules below can see, and a link in prose here is unchecked for the same reason. Keep every map entry in one unbroken table"
      ;;
    STRAY)
      fail "${rel}'s Documentation map row ${num} carries something link-shaped this gate cannot read as a link: '${field3}' -- write map entries as inline [label](destination) links, or as reference-style links with a matching '[ref]: <target>' definition. A raw <a>/<img> tag renders as a link but is invisible to this contract, and markdownlint says nothing (MD033 is disabled repo-wide). Reporting it is deliberate: what the parser cannot read would otherwise be skipped in silence, which is the defect this gate exists to catch"
      ;;
    esac
  done

  # A pipe table is a header row, a delimiter row, then the entries. Both are identified
  # POSITIONALLY inside the block located above: keying off their text (`| Section`, `| ---`)
  # meant renaming the column or writing `|---|---|` quietly reclassified a structural row as an
  # entry, or an entry as structure.
  [[ "${row_count}" -ge 3 ]] ||
    fail "${rel}'s Documentation map has only ${row_count} table line(s) -- a Markdown table needs a header row, a delimiter row and at least one entry"
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

# The R2-06 strengthening: deep links are compared on the PAGE, not on the link as written, so a
# second label aimed at an anchor of a page the map already lists is still two names for one page.
# Aimed at the ASSET again, for the same reason as the mutant above: on a .md destination rule 4
# would claim this mutant for the label, leaving the anchor-splitting untested.
sed 's|(\[package graph\](architecture-graph.svg))|([package graph](architecture-graph.svg)) · [Diagram](architecture-graph.svg#top)|' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/two-labels-two-anchors.md"
mutant_rejected "${MUTANTS}/two-labels-two-anchors.md" \
  'a second label deep-links to an anchor of a page the map already lists' 'two labels at'

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

# GFM makes the leading pipe optional, so this renders as the same row with the same links.
sed 's#^| \*\*Run in production\*\* | \[Performance and scaling\](operator-manual/performance.md) · \[Production checklist\](operator-manual/production-checklist.md) · \[Troubleshooting\](operator-manual/troubleshooting.md) |$#**Run in production** | [Performance and scaling](operator-manual/performance.md) · [Production checklist](operator-manual/production-checklist.md) · [FAQ](operator-manual/troubleshooting.md) |#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/row-without-leading-pipe.md"
mutant_rejected "${MUTANTS}/row-without-leading-pipe.md" \
  'a row is written with no leading pipe at all' 'removed in the docs restructure'

# The strongest of the escapes and the one with NO compensating control: markdownlint reports
# nothing here. The first cell is an editorial grouping by CONVENTION, and cutting it before
# parsing turned that convention into an exemption a dead destination could hide behind.
sed 's#^| \*\*Run in production\*\* | \[Performance and scaling\]#| [Run in production](operator-manual/gone-forever.md) | [Performance and scaling]#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/link-in-section-cell.md"
mutant_rejected "${MUTANTS}/link-in-section-cell.md" \
  'the section cell of a row carries a link to a page that does not exist' 'does not exist'

# Raw HTML renders as a link, and MD033 is disabled repo-wide so markdownlint is silent.
sed 's#\[Troubleshooting\](operator-manual/troubleshooting.md)#<a href="operator-manual/does-not-exist.md">FAQ</a>#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/raw-html-anchor.md"
mutant_rejected "${MUTANTS}/raw-html-anchor.md" \
  'a map entry is written as a raw HTML anchor' 'cannot read as a link'

# A second table in the section: this gate reads the first, so the rest must not go unnoticed.
# Its rows are deliberately LINK-FREE, so the orphan sweep below cannot claim this mutant and the
# one-table assertion is the only thing that can reject it.
awk '
  { print }
  /^\| \*\*Contributing\*\* \|/ {
    print ""
    print "| Section | Notes |"
    print "| --- | --- |"
    print "| **Extra** | nothing to see here |"
  }
' "${DOCS_DIR}/index.md" >"${MUTANTS}/two-tables.md"
mutant_rejected "${MUTANTS}/two-tables.md" \
  'the map section carries a second table the gate would not read' 'contains 2 tables'

# A blank line ends a Markdown table. The rows after one still render as rows -- markdown-it
# produces all 30 links, every one resolves so `mkdocs build --strict` is silent, and
# markdownlint-cli2 reports 0 issues under this repo's own config -- but they are a second
# paragraph, not part of the table this gate reads.
#
# The split is placed before the LAST row on purpose, so that the table this gate does read still
# holds 25 links and clears the 20-link floor. Splitting earlier is caught by that floor, which
# would leave the sweep below untested -- the floor would be doing the work and an ablation of the
# sweep would still look dead. Here nothing but the sweep can see it: the orphaned row carries
# both a planted FAQ label (rule 3) and a second link to a page the map already lists (rule 2),
# and with the sweep ablated this file passes at exit 0 with both defects in it.
awk '
  /^\| \*\*Contributing\*\* \|/ { print "" }
  { print }
' "${DOCS_DIR}/index.md" |
  sed 's#\[Release process\](RELEASE.md)#[FAQ](operator-manual/troubleshooting.md)#' \
    >"${MUTANTS}/blank-line-in-table.md"
mutant_rejected "${MUTANTS}/blank-line-in-table.md" \
  'a blank line splits the table and orphans the rows after it' 'carries a link OUTSIDE the table'

# The same hole reached from the other side: prose in the section carrying a link nothing checks.
awk '
  { print }
  /^\| \*\*Contributing\*\* \|/ {
    print ""
    print "See also the [Glossary](GLOSSARY.md) for terminology."
  }
' "${DOCS_DIR}/index.md" >"${MUTANTS}/orphan-prose-link.md"
mutant_rejected "${MUTANTS}/orphan-prose-link.md" \
  'prose inside the map section carries an unchecked link' 'carries a link OUTSIDE the table'

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

# Green direction: normalize() is what lets the map differ from the nav in case and punctuation
# without differing in NAME. Nothing above needs it -- verified: reducing normalize() to the
# identity function survived every mutant in this file until this case existed -- so the rule that
# the map need not copy the nav byte for byte is asserted here rather than assumed.
sed 's#\[Production checklist\](operator-manual/production-checklist.md)#[PRODUCTION-CHECKLIST](operator-manual/production-checklist.md)#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/label-recased.md"
if cmp -s "${MUTANTS}/label-recased.md" "${DOCS_DIR}/index.md"; then
  fail "self-test: the recased-label case is byte-identical to docs/index.md -- it proves nothing"
fi
if ! (check_map "${MUTANTS}/label-recased.md") >/dev/null 2>&1; then
  fail "self-test: the gate rejects a map label differing from its nav label only in case and punctuation -- normalize() is inert, and the map would have to copy the nav byte for byte: $( (check_map "${MUTANTS}/label-recased.md") 2>&1 )"
fi
pass "self-test: gate accepts a label differing from its nav label only in case and punctuation"

# Green direction: rule 4 accepts the nav label OR the H1, and today every label in the real map
# matches a nav label -- so the H1 half was dead code that no mutant touched. concepts/
# architecture.md is nav "Architecture" but H1 "Kollect architecture", which makes it the one
# destination where the two differ and the fallback can be exercised.
sed 's#\[Architecture\](concepts/architecture.md)#[Kollect architecture](concepts/architecture.md)#' \
  "${DOCS_DIR}/index.md" >"${MUTANTS}/label-matches-h1-only.md"
if cmp -s "${MUTANTS}/label-matches-h1-only.md" "${DOCS_DIR}/index.md"; then
  fail "self-test: the H1-fallback case is byte-identical to docs/index.md -- it proves nothing"
fi
if ! (check_map "${MUTANTS}/label-matches-h1-only.md") >/dev/null 2>&1; then
  fail "self-test: the gate rejects a label that matches its destination's H1 but not its nav label -- rule 4's documented H1 fallback is inert: $( (check_map "${MUTANTS}/label-matches-h1-only.md") 2>&1 )"
fi
pass "self-test: gate accepts a label matching its destination's H1 rather than its nav label"

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
