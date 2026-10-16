#!/usr/bin/env bash
# Evidence under openspec/ is Markdown only (openspec change cross-file-consistency-gates, G9).
#
# A change's evidence/ directory records how each claim was checked. A probe script committed
# there is analysed by SonarCloud as new code (sonar.sources=.) although CI never runs it: one
# 60-line Python probe turned PR #463's quality gate red on coverage and two taint findings. The
# decision (2026-10-16, option B) was not to widen sonar.exclusions but to keep evidence as
# Markdown: a probe's source goes into the evidence .md as a fenced block with the command line
# and input revision, and a probe worth keeping becomes a guard under hack/.
#
# What it locks:
#   * every git-tracked file under openspec/changes/**/evidence/ -- the archive included -- ends in
#     .md; each one that does not is named;
#   * the scan is never vacuous: a tree with no tracked evidence file (no openspec/ at all, or a
#     root that is not the top of a git work tree) fails instead of passing.
# Untracked files are not seen (Sonar analyses what is committed).
#
# Usage:
#   hack/test/openspec_evidence_markdown_only_test.sh              # check the repository tree
#   hack/test/openspec_evidence_markdown_only_test.sh --self-test  # mutate copies, assert each check fails
#   hack/test/openspec_evidence_markdown_only_test.sh <tree-root>  # check an arbitrary git work tree
set -euo pipefail

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"

MODE="check"
if [[ "${1:-}" == "--self-test" ]]; then
  MODE="self-test"
  shift
fi

ROOT="${1:-}"
if [[ -z "${ROOT}" ]]; then
  ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fi
ROOT="$(cd "${ROOT}" && pwd -P)"

command -v git >/dev/null 2>&1 || fail "git not found; install it to run this test"

EVIDENCE_GLOB=':(glob)openspec/changes/**/evidence/**'

check_evidence() {
  local root="$1" top
  # The root must be the top of its own work tree: otherwise git would answer for an enclosing
  # repository (or none) and the scan would read the wrong tree.
  top="$(git -C "${root}" rev-parse --show-toplevel 2>/dev/null)" || {
    echo "FAIL: ${root} is not a git work tree; nothing was scanned" >&2
    return 1
  }
  top="$(cd "${top}" && pwd -P)"
  if [[ "${top}" != "${root}" ]]; then
    echo "FAIL: ${root} is not the top of its git work tree (${top}); nothing was scanned" >&2
    return 1
  fi
  local -a files=()
  mapfile -d '' -t files < <(git -C "${root}" ls-files -z -- "${EVIDENCE_GLOB}")
  if [[ "${#files[@]}" -eq 0 ]]; then
    echo "FAIL: no tracked file under openspec/changes/**/evidence/; the scan read nothing" >&2
    return 1
  fi
  local f bad=0
  for f in "${files[@]}"; do
    if [[ "${f}" != *.md ]]; then
      echo "FAIL: ${f} is not Markdown; evidence under openspec/ is .md only (inline the source as a fenced block, or make it a guard under hack/)" >&2
      bad=$((bad + 1))
    fi
  done
  [[ "${bad}" -eq 0 ]] || return 1
  echo "ok - ${#files[@]} tracked evidence files, all Markdown"
}

if [[ "${MODE}" == "check" ]]; then
  if check_evidence "${ROOT}"; then
    pass "openspec evidence is Markdown only"
    exit 0
  fi
  fail "openspec evidence holds non-Markdown or nothing to scan (listed above)"
fi

# ---------------------------------------------------------------------------
# Self-test (CFC-6 shape): copy the openspec tree into a fresh git work tree, break one invariant
# per copy, and assert the gate reds WITH THE MESSAGE of the assertion the mutant targets -- a
# non-zero exit alone is not evidence. One unmutated copy must pass.
# ---------------------------------------------------------------------------
SCRATCH=""
trap 'rm -rf ${SCRATCH} 2>/dev/null' EXIT

# The copy holds the TRACKED files under openspec/ only (all the gate reads; an untracked local
# file must not red the no-op control), registered in a new index: ls-files reads the index, so
# no commit (and no git identity) is needed. `add -f` everywhere: a global core.excludesFile
# (say `*.log`) must not keep a mutant or control from landing.
make_copy() {
  local dst="$1"
  git -C "${dst}" init -q
  git -C "${ROOT}" ls-files -z -- openspec | tar -C "${ROOT}" --null -T - -cf - | tar -C "${dst}" -xf - ||
    fail "self-test: copying the tracked openspec/ files failed (a tracked file deleted from the working tree?) -- nothing was tested"
  git -C "${dst}" add -A -f
}

# Called directly (never as $(new_copy)), so SCRATCH survives in this shell for the EXIT trap.
COPY=""
new_copy() {
  COPY="$(mktemp -d)"
  SCRATCH="${SCRATCH} ${COPY}"
  make_copy "${COPY}"
}

# The probe proves the mutation landed in the index (what the gate reads) before the gate runs.
# PRESENT <path>: the path is tracked; ABSENT <path>: nothing under it is tracked.
landed() {
  local probe="$1" path="$2"
  if [[ "${probe}" == "PRESENT" ]]; then
    [[ -n "$(git -C "${COPY}" ls-files -- "${path}")" ]]
  else
    [[ -z "$(git -C "${COPY}" ls-files -- "${path}")" ]]
  fi
}

# mutant_rejected <label> <sentinel> <PRESENT|ABSENT> <probe-path> <command...>
mutant_rejected() {
  local label="$1" sentinel="$2" probe="$3" probe_path="$4"
  shift 4
  local out rc
  new_copy
  (cd "${COPY}" && "$@" && git add -A -f) ||
    fail "self-test: the mutation step for '${label}' failed -- nothing was tested"
  landed "${probe}" "${probe_path}" ||
    fail "self-test: the '${label}' mutant did not land (${probe} ${probe_path}) -- nothing was tested"
  out="$(bash "${SELF}" "${COPY}" 2>&1)" && rc=0 || rc=$?
  [[ "${rc}" -ne 0 ]] ||
    fail "self-test: the gate accepted the '${label}' mutant (exit 0) -- the assertion '${sentinel}' did not fire"
  grep -Fq -- "${sentinel}" <<<"${out}" ||
    fail "self-test: the '${label}' mutant red for the WRONG reason; expected '${sentinel}', got: $(grep FAIL <<<"${out}" | head -3 | tr '\n' ' ')"
  pass "self-test: ${label} rejected with '${sentinel}'"
}

# control_passes <label> <probe-path> <command...>: the change lands (PRESENT) and the gate passes.
control_passes() {
  local label="$1" probe_path="$2"
  shift 2
  local out
  new_copy
  (cd "${COPY}" && "$@" && git add -A -f) ||
    fail "self-test: the control step for '${label}' failed -- nothing was tested"
  landed PRESENT "${probe_path}" ||
    fail "self-test: the '${label}' control did not land (${probe_path} not tracked)"
  out="$(bash "${SELF}" "${COPY}" 2>&1)" ||
    fail "self-test: the gate red on the '${label}' control -- it must pass: $(grep FAIL <<<"${out}" | head -2 | tr '\n' ' ')"
  pass "self-test: ${label} control passes"
}

# No-op control: an unmutated copy passes. Also proves the gate reads the root it is given.
new_copy
if ! out="$(bash "${SELF}" "${COPY}" 2>&1)"; then
  fail "self-test: the no-op control failed on an unmutated copy -- fix the real tree (or the gate) first: $(grep FAIL <<<"${out}" | head -3 | tr '\n' ' ')"
fi
pass "self-test: no-op control passes on an unmutated copy"

# Fixture paths are created by the mutant, so the self-test does not depend on which changes
# exist or are archived in the real tree.
LIVE="openspec/changes/zz-selftest-change/evidence"
ARCH="openspec/changes/archive/2099-01-01-zz-selftest-change/evidence"

mutant_rejected "Python probe in a change's evidence" \
  "FAIL: ${LIVE}/probe.py is not Markdown" PRESENT "${LIVE}/probe.py" \
  bash -c 'mkdir -p "$1" && printf "print(1)\n" > "$1/probe.py" && printf "# E\n" > "$1/E.md"' _ "${LIVE}"

mutant_rejected "shell probe in an archived change's evidence" \
  "FAIL: ${ARCH}/probe.sh is not Markdown" PRESENT "${ARCH}/probe.sh" \
  bash -c 'mkdir -p "$1" && printf "echo 1\n" > "$1/probe.sh" && printf "# E\n" > "$1/E.md"' _ "${ARCH}"

# A file in a nested directory under evidence/ is still evidence.
mutant_rejected "probe in a nested evidence directory" \
  "FAIL: ${LIVE}/probes/run.sh is not Markdown" PRESENT "${LIVE}/probes/run.sh" \
  bash -c 'mkdir -p "$1/probes" && printf "echo 1\n" > "$1/probes/run.sh"' _ "${LIVE}"

# Every offender is named, not only the first.
mutant_rejected "second offender also named" \
  "FAIL: ${LIVE}/b.txt is not Markdown" PRESENT "${LIVE}/b.txt" \
  bash -c 'mkdir -p "$1" && printf "a\n" > "$1/a.sh" && printf "b\n" > "$1/b.txt"' _ "${LIVE}"

# The suffix must be .md exactly: a script named like Markdown is still a script.
mutant_rejected "Markdown-looking suffix" \
  "FAIL: ${LIVE}/G1.md.sh is not Markdown" PRESENT "${LIVE}/G1.md.sh" \
  bash -c 'mkdir -p "$1" && printf "echo 1\n" > "$1/G1.md.sh"' _ "${LIVE}"

# Empty scans: no openspec/ at all, or no tracked evidence file left.
mutant_rejected "no openspec directory" \
  "no tracked file under openspec/changes/**/evidence/" ABSENT "openspec" \
  rm -rf openspec

mutant_rejected "no evidence directory left" \
  "no tracked file under openspec/changes/**/evidence/" ABSENT "${EVIDENCE_GLOB}" \
  find openspec/changes -type d -name evidence -prune -exec rm -rf {} +

# The root must be the top of its own work tree: a subdirectory would let git answer for the
# enclosing repository, and a plain directory has no index to read.
root_rejected() {
  local label="$1" sentinel="$2" root="$3" out rc
  out="$(bash "${SELF}" "${root}" 2>&1)" && rc=0 || rc=$?
  [[ "${rc}" -ne 0 ]] ||
    fail "self-test: the gate accepted the '${label}' root (exit 0) -- the assertion '${sentinel}' did not fire"
  grep -Fq -- "${sentinel}" <<<"${out}" ||
    fail "self-test: the '${label}' root red for the WRONG reason; expected '${sentinel}', got: $(grep FAIL <<<"${out}" | head -3 | tr '\n' ' ')"
  pass "self-test: ${label} rejected with '${sentinel}'"
}

new_copy
root_rejected "root below the work-tree top" "is not the top of its git work tree" "${COPY}/openspec"
PLAIN="$(mktemp -d)"
SCRATCH="${SCRATCH} ${PLAIN}"
root_rejected "root outside any git work tree" "is not a git work tree" "${PLAIN}"

# Green control: a non-Markdown file outside evidence/ is out of scope.
control_passes "non-Markdown file outside evidence" "openspec/changes/zz-selftest-change/reviews/legs.log" \
  bash -c 'mkdir -p openspec/changes/zz-selftest-change/reviews && printf "log\n" > openspec/changes/zz-selftest-change/reviews/legs.log'

echo "All openspec evidence self-tests passed."
