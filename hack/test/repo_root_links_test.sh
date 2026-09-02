#!/usr/bin/env bash
# Resolves every relative Markdown link in the repo-root Markdown files and fails on
# any target that does not exist on disk.
#
# DOCS-EXTLINK-01: `mkdocs build --strict` reads only `docs_dir: docs` (mkdocs.yml), and every
# contract under `test/docs/` is docs/-scoped, so README.md and CONTRIBUTING.md were invisible to
# the entire gate matrix. `task lint:markdown` checks style, not link targets. That is how three
# hard 404s (docs/QUICKSTART.md, docs/UNDERSTAND-THE-BASICS.md, docs/CR-REFERENCE.md -- all
# deleted in the docs restructure) survived on the top contributor page while every gate was
# green. mkdocs `redirects` do not help: they exist only on the BUILT site, and contributors read
# these files on GitHub, where a deleted target is a 404.
#
# Scope is the Git-TRACKED Markdown files at the repository root -- outside `docs_dir`, so no
# other gate covers them, and tracked rather than a `find` walk so gitignored scratch directories
# (docs/node_modules/, .claude/worktrees/) can never be mistaken for the repo. Deeper non-docs
# Markdown (charts/kollect/README.md, hack/demo/**, lab-evidence/**) is deliberately out of scope
# for now: those trees carry pre-existing broken links and generated content, and widening the
# scan before they are fixed would land this gate permanently red.
#
# DOCS-EXTLINK-03 F-6 (found independently as DOCS-MAPGATE-02 R2-02 in the sibling docs-map
# gate): reference-style links used to be invisible here. `[Quickstart][qs]` with a
# `[qs]: docs/DELETED.md` definition matches no `[text](target)` pattern, so a hard 404 written
# that way passed at exit 0 -- the same blind spot, in two gates, from the same inline-only
# regex. Link reference DEFINITIONS are now scanned too: the definition is where the target
# lives, so checking definitions covers every use of the label.
#
# Known boundary: raw HTML (`<a href>`, `<img src>`) is still NOT parsed, so README.md's HTML
# badge header contributes nothing to the scan. Every one of those is external today, so there
# is no live gap -- but do not read a GREEN here as "every link in README.md was checked".
# Widening to HTML attributes is a deliberate follow-up, not an accident.
#
# Code fences are NOT stripped: a relative link inside a fenced example is still checked. That is
# the conservative direction -- a documentation sample pointing at a missing file fails loudly
# instead of the gate quietly excusing whole regions of a file.
#
# The self-test at the bottom proves both directions on a throwaway tree every run, including
# that the RED names the offending file AND target, and that the non-empty/required-file guard
# actually fires. A gate that has never been watched failing is not a gate.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly ROOT

# Root-level Markdown that must always be in the scan set. A pathspec typo or a `git ls-files`
# that silently yields nothing would otherwise make this whole gate pass vacuously -- a failure
# mode this repo has hit before. Counting files is not enough; the names are asserted.
REQUIRED_ROOT_DOCS=(README.md CONTRIBUTING.md)
readonly REQUIRED_ROOT_DOCS

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() { echo "ok - $*"; }

# Emit one link target per line for a Markdown file: inline `[text](target)` destinations AND the
# targets of link reference definitions (`[label]: target "Title"`, up to three leading spaces,
# which is all Markdown allows), with any angle brackets and quoted title stripped. External,
# mail and pure-anchor links are dropped by the caller, which is also where fragments are split
# off.
#
# Each producer needs a guard: `grep` exits 1 on no match, and under `pipefail` + `set -e` an
# unguarded first producer would abort the whole function before the second ever ran -- silently
# dropping every reference definition in any file that happens to have no inline link.
#
# The guard is `[[ $? -eq 1 ]]`, NOT `|| true`. `|| true` cannot tell grep's exit 1 ("no match",
# expected and fine) from exit 2 ("unreadable file", "is a directory"), so an I/O error would
# become a confident empty target list and the file would be reported link-clean -- the same
# silent-drop shape this gate exists to remove.
#
# Anything other than "no match" is reported by EMITTING A SENTINEL rather than by calling `fail`.
# The caller reads this function through a process substitution, and a `fail` there exits only the
# substitution's subshell: the message is printed and the gate then reports the file as clean
# anyway, exit 0. Verified, not assumed -- that was the behaviour of the first version of this
# guard against a mode-000 GOVERNANCE.md. The sentinel travels back through the pipe and the
# caller, which runs in the gate's own shell, is what dies on it.
readonly EXTRACT_FAILED=$'\x1fextract-failed\x1f'

# Which exit statuses a link producer may return. 0 is "matched", 1 is grep's "no match" and is
# the normal case for a file with no links; everything else is an I/O error -- EISDIR, EIO, a file
# that vanished mid-scan -- and must not be read as "this file has no links".
#
# A function rather than an inline `[[ $? -eq 1 ]]` so the self-test can exercise it directly.
# The `-r` precheck below masks the only failure this environment can construct on demand, so
# without a direct case, reverting either guard to `|| true` passed every test in this file --
# leaving the very class the guards exist for untested.
producer_ok() {
  [[ "$1" -eq 0 || "$1" -eq 1 ]]
}

extract_targets() {
  local file="$1"

  # REDUNDANT against the producer_ok guards below -- GNU grep exits 2 on an unreadable file, so
  # the sentinel would be emitted anyway -- and kept for the sharper diagnosis and because the
  # exit code a grep gives for an unreadable path is not the same everywhere (GNU grep 2, ugrep 1
  # for a directory). Ablating it survives the self-test; that is expected, not an oversight.
  if [[ ! -r "${file}" ]]; then
    printf '%s\n' "${EXTRACT_FAILED}"
    return 0
  fi

  {
    grep -oE '\]\([^)]*\)' "${file}" | sed -e 's/^](//' -e 's/)$//' ||
      producer_ok $? || printf '%s\n' "${EXTRACT_FAILED}"
    grep -oE '^ {0,3}\[[^]]+\]:[[:space:]]*[^[:space:]]+' "${file}" |
      sed -E 's/^ *\[[^]]+\]:[[:space:]]*//' ||
      producer_ok $? || printf '%s\n' "${EXTRACT_FAILED}"
  } |
    sed -e 's/[[:space:]]*"[^"]*"[[:space:]]*$//' \
      -e "s/[[:space:]]*'[^']*'[[:space:]]*\$//" \
      -e 's/^[[:space:]]*//' \
      -e 's/[[:space:]]*$//' \
      -e 's/^<//' \
      -e 's/>$//'
}

# Root-level tracked Markdown, NUL-delimited exactly like hack/docs/tracked-markdown.sh so
# unusual file names survive. Paths containing `/` are skipped: `docs/` is covered by
# `mkdocs build --strict`, and the deeper trees are out of scope per the header.
collect_root_markdown() {
  local root="$1" path
  while IFS= read -r -d '' path; do
    [[ "${path}" == */* ]] && continue
    printf '%s\n' "${path}"
  done < <(git -C "${root}" ls-files -z -- '*.md')
}

# Resolve every relative link in a tree's root Markdown files. Prints one FAIL line per dead
# target (naming both the file and the target) and returns nonzero if any were found.
check_root_links() {
  local root="$1"
  local -a files=()
  local file required found dir raw target path resolved
  local broken=0

  while IFS= read -r file; do
    files+=("${file}")
  done < <(collect_root_markdown "${root}")

  [[ "${#files[@]}" -gt 0 ]] ||
    fail "no tracked root Markdown found under ${root} -- the scan set is empty, so this gate would pass without checking anything"

  for required in "${REQUIRED_ROOT_DOCS[@]}"; do
    found=0
    for file in "${files[@]}"; do
      if [[ "${file}" == "${required}" ]]; then
        found=1
        break
      fi
    done
    [[ "${found}" -eq 1 ]] ||
      fail "${required} is not in the tracked root Markdown scan set for ${root} -- the files this gate exists to cover are not being scanned"
  done

  for file in "${files[@]}"; do
    dir="$(dirname "${root}/${file}")"
    while IFS= read -r raw; do
      # This loop body runs in the gate's own shell, so `fail` here really does stop the gate --
      # which is the whole reason extract_targets reports through a sentinel instead of dying.
      if [[ "${raw}" == "${EXTRACT_FAILED}" ]]; then
        fail "could not read the links out of ${file} (unreadable, or grep failed with an I/O error) -- refusing to report an unreadable file as link-clean"
      fi
      target="${raw}"
      case "${target}" in
      http://* | https://* | mailto:* | tel:* | '#'* | '') continue ;;
      esac
      # `page.md#anchor` resolves on the file half; a bare `#anchor` was dropped above.
      path="${target%%#*}"
      [[ -z "${path}" ]] && continue
      case "${path}" in
      /*) resolved="${root}${path}" ;;
      *) resolved="${dir}/${path}" ;;
      esac
      if [[ ! -e "${resolved}" ]]; then
        echo "FAIL: ${file} links to '${target}', which does not exist (resolved to ${resolved#"${root}/"})" >&2
        broken=1
      fi
    done < <(extract_targets "${root}/${file}")
  done

  return "${broken}"
}

if check_root_links "${ROOT}"; then
  pass "every relative link in the tracked root Markdown resolves to an existing path"
else
  fail "repo-root Markdown has dead relative links (listed above) -- these are hard 404s on GitHub; mkdocs redirects only exist on the built site"
fi

# ---------------------------------------------------------------------------
# Self-test. Both directions, on a throwaway Git tree, every run.
# ---------------------------------------------------------------------------
FIXTURE="$(mktemp -d)"
trap 'rm -rf "${FIXTURE}"' EXIT

# `-c` flags rather than ambient config so a maintainer's global Git settings cannot change what
# this fixture does. No commit is needed: `git ls-files` reads the index.
init_fixture() {
  local dir="$1"
  mkdir -p "${dir}/docs/crds"
  : >"${dir}/docs/crds/index.md"
  : >"${dir}/GOVERNANCE.md"
  git -c init.defaultBranch=main init -q "${dir}"
}

stage_fixture() {
  git -C "$1" -c core.excludesFile=/dev/null add .
}

# GREEN direction: an honest tree passes.
HONEST="${FIXTURE}/honest"
init_fixture "${HONEST}"
printf '# R\n\nSee [contributing](CONTRIBUTING.md) and [crds](docs/crds/index.md#kinds).\n' \
  >"${HONEST}/README.md"
printf '# C\n\nSee [gov](GOVERNANCE.md "Governance"), [ext](https://example.com/missing.md) and [top](#c).\nAlso [crds][cr] and [home][hp].\n\n[cr]: docs/crds/index.md "CRDs"\n[hp]: https://example.com/\n' \
  >"${HONEST}/CONTRIBUTING.md"
stage_fixture "${HONEST}"
honest_status=0
# Subshell: an internal `fail` exits, and that must be reported here rather than taking the
# whole gate down with its own message.
honest_output="$( (check_root_links "${HONEST}") 2>&1 )" || honest_status=$?
[[ "${honest_status}" -eq 0 ]] ||
  fail "self-test: the gate rejected an honest tree whose every relative link resolves -- it would red the build on correct content; got: ${honest_output}"
pass "self-test: gate accepts a tree whose relative links all resolve"

# RED direction: a known-bad link fails, and the failure names the file and the target.
BROKEN_TREE="${FIXTURE}/broken"
init_fixture "${BROKEN_TREE}"
cp "${HONEST}/README.md" "${BROKEN_TREE}/README.md"
printf '# C\n\nSee [quickstart](docs/DEFINITELY-DELETED.md) for setup.\n' \
  >"${BROKEN_TREE}/CONTRIBUTING.md"
stage_fixture "${BROKEN_TREE}"
broken_status=0
broken_output="$( (check_root_links "${BROKEN_TREE}") 2>&1 )" || broken_status=$?
[[ "${broken_status}" -ne 0 ]] ||
  fail "self-test: the gate passed on a tree with a link to docs/DEFINITELY-DELETED.md -- it is vacuous"
[[ "${broken_output}" == *"CONTRIBUTING.md"* ]] ||
  fail "self-test: the gate rejected the broken tree without naming the offending file; got: ${broken_output}"
[[ "${broken_output}" == *"docs/DEFINITELY-DELETED.md"* ]] ||
  fail "self-test: the gate rejected the broken tree without naming the dead target; got: ${broken_output}"
pass "self-test: gate rejects a dead relative link and names both the file and the target"

# DOCS-EXTLINK-03 F-6: the same 404 written in reference style. This exact tree passed at exit 0
# before link reference definitions were scanned, which is the whole reason this case is here.
REF_TREE="${FIXTURE}/broken-reference-style"
init_fixture "${REF_TREE}"
cp "${HONEST}/README.md" "${REF_TREE}/README.md"
printf '# C\n\nSee [quickstart][qs] for setup.\n\n[qs]: docs/REFERENCE-STYLE-DELETED.md\n' \
  >"${REF_TREE}/CONTRIBUTING.md"
stage_fixture "${REF_TREE}"
ref_status=0
ref_output="$( (check_root_links "${REF_TREE}") 2>&1 )" || ref_status=$?
[[ "${ref_status}" -ne 0 ]] ||
  fail "self-test: the gate passed on a tree whose reference-style link resolves to docs/REFERENCE-STYLE-DELETED.md -- reference definitions are invisible to it again"
[[ "${ref_output}" == *"docs/REFERENCE-STYLE-DELETED.md"* ]] ||
  fail "self-test: the reference-style tree was rejected without naming the dead target; got: ${ref_output}"
pass "self-test: gate rejects a dead target reached through a link reference definition"

# An unreadable file must be a hard failure, not a file with "no links". The first version of
# this guard printed its complaint from inside a process substitution and the gate then reported
# the tree as clean at exit 0, so this case asserts the exit status, not just the message.
UNREADABLE="${FIXTURE}/unreadable"
init_fixture "${UNREADABLE}"
cp "${HONEST}/README.md" "${UNREADABLE}/README.md"
cp "${HONEST}/CONTRIBUTING.md" "${UNREADABLE}/CONTRIBUTING.md"
stage_fixture "${UNREADABLE}"
chmod 000 "${UNREADABLE}/CONTRIBUTING.md"
if [[ -r "${UNREADABLE}/CONTRIBUTING.md" ]]; then
  # Running as root, or on a filesystem that ignores the mode bits. Announced rather than
  # silently skipped: CI runs unprivileged, where this case does execute.
  pass "self-test: SKIPPED the unreadable-file case -- this user can read a mode-000 file"
else
  unreadable_status=0
  unreadable_output="$( (check_root_links "${UNREADABLE}") 2>&1 )" || unreadable_status=$?
  [[ "${unreadable_status}" -ne 0 ]] ||
    fail "self-test: the gate reported a tree containing an unreadable Markdown file as link-clean -- an I/O error is being read as 'this file has no links'; got: ${unreadable_output}"
  [[ "${unreadable_output}" == *"refusing to report an unreadable file as link-clean"* ]] ||
    fail "self-test: the unreadable-file tree was rejected for the wrong reason; got: ${unreadable_output}"
  pass "self-test: gate rejects a tree containing an unreadable Markdown file"
fi
chmod 644 "${UNREADABLE}/CONTRIBUTING.md"

# The I/O-error class the `-r` precheck above CANNOT cover: a path that is readable but that grep
# still fails on (EISDIR, EIO, a file truncated away mid-scan). Asserted directly on the status
# classifier, because no portable fixture produces that status on demand -- the exit code a
# directory yields is grep-implementation-specific (GNU grep 2, ugrep 1), so a fixture built on it
# would prove one thing on a developer's machine and another in CI.
producer_ok 0 ||
  fail "self-test: a link producer that matched (exit 0) is being treated as an I/O error"
producer_ok 1 ||
  fail "self-test: grep's 'no match' (exit 1) is being treated as an I/O error -- every file without links would red"
if producer_ok 2; then
  fail "self-test: a producer exit status of 2 (an I/O error such as EISDIR) is being accepted as 'no match' -- a readable file that cannot actually be read would be reported as link-clean, which is the silent drop this gate exists to remove"
fi
if producer_ok 141; then
  fail "self-test: a producer killed by SIGPIPE (141) is being accepted as 'no match' -- a truncated scan would be reported as link-clean"
fi
pass "self-test: only 0 and 1 are accepted from a link producer; 2 and 141 are I/O errors"

# The vacuity guard itself: a tree missing README.md must be a hard failure, not a quiet pass
# over whatever files happen to remain.
MISSING="${FIXTURE}/missing-readme"
init_fixture "${MISSING}"
cp "${HONEST}/CONTRIBUTING.md" "${MISSING}/CONTRIBUTING.md"
stage_fixture "${MISSING}"
missing_status=0
missing_output="$( (check_root_links "${MISSING}") 2>&1 )" || missing_status=$?
[[ "${missing_status}" -ne 0 ]] ||
  fail "self-test: the gate passed on a tree with no README.md -- the required-file guard is inert, so a pathspec typo would make this gate vacuous"
[[ "${missing_output}" == *"README.md is not in the tracked root Markdown scan set"* ]] ||
  fail "self-test: the tree with no README.md was rejected for the wrong reason; got: ${missing_output}"
pass "self-test: gate rejects a scan set that is missing a required root document"

# And the floor beneath that one: a scan set with nothing in it at all. A pathspec typo or a
# `git ls-files` that yields nothing would otherwise make this whole gate report success without
# opening a single file.
EMPTY_SET="${FIXTURE}/no-root-markdown"
mkdir -p "${EMPTY_SET}/docs/crds"
: >"${EMPTY_SET}/docs/crds/index.md"
git -c init.defaultBranch=main init -q "${EMPTY_SET}"
stage_fixture "${EMPTY_SET}"
empty_status=0
empty_output="$( (check_root_links "${EMPTY_SET}") 2>&1 )" || empty_status=$?
[[ "${empty_status}" -ne 0 ]] ||
  fail "self-test: the gate reported success on a tree with no tracked root Markdown at all -- it would pass without checking anything"
[[ "${empty_output}" == *"the scan set is empty"* ]] ||
  fail "self-test: the empty-scan-set tree was rejected for the wrong reason; got: ${empty_output}"
pass "self-test: gate rejects a tree with no tracked root Markdown at all"

echo "All repo-root Markdown link tests passed."
