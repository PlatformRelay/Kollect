#!/usr/bin/env bash
# Locks hack/test/docs_map_contract_test.sh into a job that actually runs.
#
# DOCS-MAPGATE-02 R2-04: the docs-map gate shipped with no wiring self-lock, unlike its siblings
# repo_root_links_wiring_test.sh and dist_ci_wiring_test.sh. Its `ci.yaml` step could therefore
# be deleted in the same commit that weakened the map, and nothing in the repo would notice --
# the LAB-DEKIND / CI-CONTRIBFILTER-01 / CI-TIPGAP-01 shape this repo keeps re-filing. The gate
# is the ONLY thing that can see a Documentation map label that resolves and lies, so "green
# because it never ran" is the exact failure it must not have.
#
# Two reachability facts have to hold at once, and both are asserted here:
#   1. `ci.yaml`'s `lint` job runs the gate unconditionally. `hack/**` is in none of ci.yaml's
#      `paths-ignore` lists, so this is what covers a change to the gate SCRIPT.
#   2. `hack/docs/verify.sh` -- what `task docs:verify` runs, and what the path-filtered Docs
#      workflow reaches -- composes the gate. docs.yaml's `paths:` filter lists `docs/**`, so
#      this is what covers a change to the MAP itself in a docs-only PR.
# Dropping either one narrows the gate to half its change classes, so neither may go quietly.
#
# PURE BASH/GREP/AWK, NO yq -- CI-VERIFYCHAIN-01. This file is composed into hack/docs/verify.sh,
# which is `set -euo pipefail`, and the Docs workflow's `verify` job installs node, task, go,
# python, mkdocs and Chrome but NOT yq (nor does mise.toml list it). A `yq` dependency here would
# hard-fail nine lines into a ~25-line chain and starve every downstream docs gate --
# `mkdocs build --strict`, the Python test/docs contracts, `go test ./test/samples`,
# `go test ./test/docs`, the visual check -- and the Pages artifact with them. The constraint is
# recorded verbatim in hack/test/docs_pages_concurrency_test.sh: "Pure bash/grep -- no yq (Docs
# CI does not install yq)." The sibling repo_root_links_wiring_test.sh may use yq precisely
# because it is NOT in this chain; it has its own ci.yaml lint step instead.
#
# WHY THE WORKFLOW MATCHING IS DELIBERATELY NARROW. `.github/workflows/ci.yaml` is under active
# restructuring (a `changes` classifier job, a `test` -> `test-suite` rename with a report-only
# `test` job on `if: always()`, `needs:` gating, and `paths-ignore` dropped from `pull_request`
# entirely; `lint` is deliberately left ungated). A wiring lock that asserted line numbers, job
# ordering, neighbouring steps or the overall job set would red the moment that lands, and a
# wiring lock that reds for an unrelated reason gets deleted. So this file asserts exactly one
# thing about ci.yaml's shape: that SOMEWHERE in the `lint` job there is a bare, uncommented,
# unguarded `bash hack/test/docs_map_contract_test.sh` step that can fail the build. Adding jobs,
# renaming other jobs, gating other jobs on a classifier and removing a `paths-ignore` list are
# all invisible to it -- and the self-test asserts that, in the green direction.
#
# Matching follows the GATE-COMMENT-01 / GATE-SCOPE-01 lessons from dist_ci_wiring_test.sh: it is
# LINE-EXACT against a COMMENT-STRIPPED view of the `run:` body -- in ci.yaml AND in verify.sh --
# so `# bash ...`, `bash ... || true`, `bash ... &` and a narrowed lookalike are all rejected
# rather than counted as wiring. Both sides have a mutant proving it.
#
# KNOWN GAP, deliberate and recorded. This file is composed into `hack/docs/verify.sh` rather
# than given its own `ci.yaml` step, because the lane that added it was forbidden from touching
# `.github/workflows/**` while the restructure above was in flight. Two consequences:
#   * a PR that deletes ONLY the ci.yaml gate step triggers ci.yaml but not this file; and
#   * docs.yaml's `paths:` filter names its hack/test gates by hand and lists neither this file
#     nor the gate it locks, so a PR touching only THIS script triggers ci.yaml (which does not
#     run it) and not docs.yaml -- the self-lock cannot currently see edits to itself.
# The follow-up is one step in ci.yaml's `lint` job, next to the gate's own, plus two entries in
# BOTH of docs.yaml's `paths:` lists:
#
#     # .github/workflows/ci.yaml, jobs.lint.steps
#     - name: Verify the docs map gate stays wired into a job that runs
#       run: bash hack/test/docs_map_wiring_test.sh
#
#     # .github/workflows/docs.yaml, on.push.paths AND on.pull_request.paths
#     - "hack/test/docs_map_contract_test.sh"
#     - "hack/test/docs_map_wiring_test.sh"
#
# The self-test at the bottom mutates the real files every run and proves each assertion rejects
# the shape it exists to catch -- including a ci.yaml already carrying the restructure above, to
# prove this lock stays green through it. A gate that has never been watched failing is not a gate.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly ROOT
CI_WORKFLOW="${ROOT}/.github/workflows/ci.yaml"
VERIFY_SCRIPT="${ROOT}/hack/docs/verify.sh"
GATE_SCRIPT="hack/test/docs_map_contract_test.sh"
WIRING_SCRIPT="hack/test/docs_map_wiring_test.sh"
readonly CI_WORKFLOW VERIFY_SCRIPT GATE_SCRIPT WIRING_SCRIPT

# Unit Separator. Records below carry free text (a `run:` line, a `paths-ignore` glob), and with
# IFS=$'\t' bash's `read` collapses runs of tabs, silently merging an empty field into its
# neighbour. No 0x1f byte exists in either file this gate reads.
readonly SEP=$'\x1f'

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() { echo "ok - $*"; }

# --- Workflow readers (pure awk; see the CI-VERIFYCHAIN-01 note above) ------------------------

# Records describing ONE job of a GitHub workflow. Structure is read from indentation, which is
# what YAML block mappings are, rather than from fixed column numbers:
#
#   JOBFOUND                        -- the job block exists at all
#   STEPSFOUND                      -- it declares a `steps:` key
#   JOBKEY   <key> <value>          -- a job-level scalar (if, continue-on-error, ...)
#   STEPKEY  <i> <key> <value>      -- a step-level scalar, steps numbered from 1
#   RUNLINE  <i> <line>             -- one comment-stripped, trimmed line of step i's run body
#   STEPS    <n>                    -- how many steps were found
#
# A `run:` body's lines are shell, not YAML: only indentation ends them. Lines whose first
# non-blank character is `#` are dropped; that is REDUNDANT against the line-exact `==` the
# caller uses (a `# bash x` line never equals `bash x`) and is kept as defence in depth for the
# day that comparison loosens -- it is not what makes the commented-out mutant red.
job_records() {
  local workflow="$1" job="$2"
  awk -v JOB="${job}" -v SEP="${SEP}" '
    function trim(s) { sub(/^[ \t]+/, "", s); sub(/[ \t]+$/, "", s); return s }
    function indent_of(s,   n) { n = match(s, /[^ ]/); return (n == 0) ? -1 : n - 1 }
    {
      raw = $0
      sub(/\r$/, "", raw)
      ind = indent_of(raw)
      body = trim(raw)
    }
    !injob && ind >= 0 && body == JOB ":" { injob = 1; jind = ind; print "JOBFOUND" SEP SEP SEP; next }
    !injob { next }
    # A run body may be indented arbitrarily deep, so it is consumed before the block-exit test.
    inrun {
      if (body == "") { next }
      if (ind > runind) { if (body !~ /^#/) { print "RUNLINE" SEP nsteps SEP body SEP }; next }
      inrun = 0
    }
    ind >= 0 && ind <= jind && body !~ /^#/ { injob = 0; next }
    body ~ /^#/ { next }
    body == "" { next }
    !insteps && ind == jind + 2 && body == "steps:" { insteps = 1; print "STEPSFOUND" SEP SEP SEP; next }
    !insteps {
      if (ind == jind + 2) {
        k = body; sub(/:.*$/, "", k)
        v = (index(body, ":") > 0) ? trim(substr(body, index(body, ":") + 1)) : ""
        print "JOBKEY" SEP k SEP v SEP
      }
      next
    }
    {
      if (stepind < 0 && body ~ /^- /) { stepind = ind }
      if (ind == stepind && body ~ /^- /) {
        nsteps++
        keyind = ind + 2
        body = trim(substr(body, 3))
        ind = keyind
      }
      if (nsteps == 0 || ind != keyind) { next }
      k = body; sub(/:.*$/, "", k)
      v = (index(body, ":") > 0) ? trim(substr(body, index(body, ":") + 1)) : ""
      print "STEPKEY" SEP nsteps SEP k SEP v
      if (k == "run") {
        if (v ~ /^[|>][-+]?$/) { inrun = 1; runind = keyind }
        else if (v != "" && v !~ /^#/) { print "RUNLINE" SEP nsteps SEP v SEP }
      }
    }
    BEGIN { stepind = -1 }
    END { print "STEPS" SEP nsteps + 0 SEP SEP }
  ' "${workflow}"
}

# Every entry of every `paths-ignore:` list under the workflow`s `on:` block. Which trigger owns
# an entry does not matter: any list naming `hack` at all is the failure.
paths_ignore_entries() {
  local workflow="$1"
  awk '
    function trim(s) { sub(/^[ \t]+/, "", s); sub(/[ \t]+$/, "", s); return s }
    function indent_of(s,   n) { n = match(s, /[^ ]/); return (n == 0) ? -1 : n - 1 }
    {
      raw = $0; sub(/\r$/, "", raw)
      ind = indent_of(raw); body = trim(raw)
    }
    !inon && ind == 0 && (body == "on:" || body == "\"on\":" || body == "true:") { inon = 1; next }
    !inon { next }
    ind == 0 && body !~ /^#/ && body != "" { inon = 0; next }
    body ~ /^#/ { next }
    body == "" { next }
    inpi && ind <= piind { inpi = 0 }
    !inpi && body == "paths-ignore:" { inpi = 1; piind = ind; next }
    inpi && body ~ /^- / {
      v = trim(substr(body, 3))
      gsub(/^"|"$/, "", v)
      gsub(/^'"'"'|'"'"'$/, "", v)
      print v
    }
  ' "${workflow}"
}

# --- The contract ----------------------------------------------------------------------------

# Reachability 1: ci.yaml's lint job runs the gate, and a change to the gate script itself
# triggers that job at all -- `hack/**` must stay out of every `paths-ignore` list.
check_ci_wiring() {
  local workflow="$1"
  local kind f2 f3 f4
  local job_found=0 steps_found=0 step_count=0
  local job_if="" job_coe="" gate_step="" gate_hits=0
  local -a step_if_keys=() step_coe_keys=()
  local entry i

  [[ -f "${workflow}" ]] || fail "expected ${workflow}"

  while IFS="${SEP}" read -r kind f2 f3 f4; do
    case "${kind}" in
    JOBFOUND) job_found=1 ;;
    STEPSFOUND) steps_found=1 ;;
    STEPS) step_count="${f2}" ;;
    JOBKEY)
      [[ "${f2}" == "if" ]] && job_if="${f3}"
      [[ "${f2}" == "continue-on-error" ]] && job_coe="${f3}"
      ;;
    STEPKEY)
      [[ "${f3}" == "if" ]] && step_if_keys+=("${f2}${SEP}${f4}")
      [[ "${f3}" == "continue-on-error" ]] && step_coe_keys+=("${f2}${SEP}${f4}")
      ;;
    RUNLINE)
      if [[ "${f3}" == "bash ${GATE_SCRIPT}" ]]; then
        gate_hits=$((gate_hits + 1))
        gate_step="${f2}"
      fi
      ;;
    esac
  done < <(job_records "${workflow}" lint)

  # Without these guards a renamed job or a stepless lint job makes every test below run over
  # nothing at all, and the assertions pass vacuously.
  [[ "${job_found}" -eq 1 ]] ||
    fail "${workflow##*/} declares no lint job -- the wiring assertions would pass vacuously"
  [[ "${steps_found}" -eq 1 && "${step_count}" -gt 0 ]] ||
    fail "${workflow##*/}'s lint job declares no steps -- the wiring assertions would pass vacuously"

  [[ -z "${job_if}" ]] ||
    fail "the lint job must run unconditionally, got 'if: ${job_if}' -- a skipped lint job never runs the docs map gate"
  [[ -z "${job_coe}" || "${job_coe}" == "false" ]] ||
    fail "the lint job must not declare 'continue-on-error: ${job_coe}' -- a soft-failed lint job turns the docs map gate into an advisory notice"

  # Zero matches and duplicates are both hard failures: zero means the wiring is gone, neutered
  # or commented out, and a duplicate would let the guards below lock onto the first hit.
  case "${gate_hits}" in
  1) ;;
  0) fail "the lint job in ${workflow##*/} has no step invoking ${GATE_SCRIPT} on a bare 'bash ${GATE_SCRIPT}' line -- a step in another job does not count, a commented-out command is not a command, and a neutered invocation (a trailing '|| true', '&', a redirect, or an 'echo' prefix) cannot fail the build" ;;
  *) fail "the lint job in ${workflow##*/} has ${gate_hits} steps invoking ${GATE_SCRIPT} -- keep exactly one so the guards below cannot lock onto the first match" ;;
  esac

  # A step that is skipped or soft-failed is wired in and still cannot fail the build -- the same
  # defect one level down.
  for i in "${step_if_keys[@]}"; do
    [[ "${i%%"${SEP}"*}" == "${gate_step}" ]] &&
      fail "the docs map gate step must run unconditionally, got 'if: ${i#*"${SEP}"}' -- a skipped step runs no gate"
  done
  for i in "${step_coe_keys[@]}"; do
    [[ "${i%%"${SEP}"*}" == "${gate_step}" ]] || continue
    [[ "${i#*"${SEP}"}" == "false" ]] ||
      fail "the docs map gate step must not declare 'continue-on-error: ${i#*"${SEP}"}' -- a soft-failed step reports a lying Documentation map as green"
  done

  pass "ci.yaml lint job runs the docs map gate unconditionally, on a line that can fail the build"

  # Literal, not glob: GitHub's `*` does not cross `/` but bash's does, so a hand-rolled matcher
  # would be wrong in the direction that passes. Any entry naming `hack` at all is a hard
  # failure -- that tree is what makes the step above reachable for a change to the gate script.
  # No paths-ignore list at all is FINE and must stay fine: dropping one means CI runs on more,
  # not less, and the restructure in flight does exactly that to `pull_request`.
  while IFS= read -r entry; do
    [[ -z "${entry}" ]] && continue
    case "${entry}" in
    hack | hack/*)
      fail "a paths-ignore list in ${workflow##*/} names '${entry}' -- a PR that weakens or deletes ${GATE_SCRIPT} would then trigger no CI, and the lint step above would never run on the change it guards"
      ;;
    esac
  done < <(paths_ignore_entries "${workflow}")
  pass "ci.yaml keeps hack/** out of every paths-ignore list, so a change to the gate itself reaches lint"
}

# Reachability 2: the composition point the Docs workflow reaches. ci.yaml ignores `docs/**` on
# push, so for a docs-only change -- a change to the MAP -- `task docs:verify` is what runs this
# gate. The self-lock lives here too: the assertions above are only load-bearing if THIS script
# runs, so its own invocation is asserted with the same line-exact matching.
check_docs_composition() {
  local script="$1"
  local -a code_lines=()
  local line found script_path

  [[ -f "${script}" ]] || fail "expected ${script}"

  # Blank lines are not executable lines: counting them would let a verify.sh reduced to comments
  # and whitespace look non-vacuous to the guard below.
  while IFS= read -r line; do
    [[ -z "${line}" ]] && continue
    code_lines+=("${line}")
  done < <(grep -v '^[[:space:]]*#' "${script}" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')

  [[ "${#code_lines[@]}" -gt 0 ]] ||
    fail "${script} has no executable lines -- the composition assertions would pass vacuously"

  for script_path in "${GATE_SCRIPT}" "${WIRING_SCRIPT}"; do
    found=0
    for line in "${code_lines[@]}"; do
      [[ "${line}" == "bash ${script_path}" ]] && found=1
    done
    [[ "${found}" -eq 1 ]] ||
      fail "hack/docs/verify.sh does not invoke ${script_path} on a bare 'bash ${script_path}' line -- a commented-out command is not a command, and a neutered invocation (a trailing '|| true', '&', a redirect) cannot fail the chain, so the Docs workflow would trigger on a change to docs/index.md and still run no Documentation map check"
  done
  pass "task docs:verify composes the docs map gate and this wiring test on unguarded lines"
}

# The wiring above is worthless if the thing it wires in has been emptied out or deleted while
# its step was left in place. A function, not an inline test, so the self-test can watch it fire.
assert_gate_present() {
  local path="$1"
  [[ -s "${path}" ]] ||
    fail "${GATE_SCRIPT} is missing or empty -- there is nothing for this wiring test to lock in"
}

assert_gate_present "${ROOT}/${GATE_SCRIPT}"
check_ci_wiring "${CI_WORKFLOW}"
check_docs_composition "${VERIFY_SCRIPT}"

# ---------------------------------------------------------------------------
# Self-test: mutate the real files and prove each assertion rejects the shape it exists to
# catch. Mutations are sed/awk (no yq, see CI-VERIFYCHAIN-01) and every one of them is guarded
# three ways below: non-empty, not byte-identical to its original, and rejected for the INTENDED
# reason. A mutation that silently failed to apply cannot be counted as a kill.
# ---------------------------------------------------------------------------
MUTANTS="$(mktemp -d)"
trap 'rm -rf "${MUTANTS}"' EXIT

GATE_RUN_LINE="        run: bash ${GATE_SCRIPT}"
readonly GATE_RUN_LINE

mutant_rejected() {
  local checker="$1" mutant="$2" original="$3" label="$4" expect="$5"
  local output status=0

  [[ -s "${mutant}" ]] ||
    fail "self-test: the mutant for '${label}' is missing or empty -- the mutation step failed, so nothing was actually tested"
  if cmp -s "${mutant}" "${original}"; then
    fail "self-test: the mutant for '${label}' is byte-identical to ${original} -- the mutation was a no-op, so nothing was actually tested"
  fi

  # Subshell: fail's `exit 1` must not take the parent down -- rejection is the expectation.
  output="$( ("${checker}" "${mutant}") 2>&1 )" || status=$?
  [[ "${status}" -ne 0 ]] ||
    fail "self-test: the gate still passed on a file where ${label} -- it is vacuous"
  [[ "${output}" == *"${expect}"* ]] ||
    fail "self-test: the gate rejected '${label}' but not for the intended reason -- expected the failure to mention '${expect}', got: ${output}"
  pass "self-test: gate rejects a file where ${label}"
}

# The real ci.yaml must contain the anchor every mutation below keys off, or the mutations are
# no-ops and the whole self-test is theatre. `grep -c`, not `grep -q`: a `! producer | grep -q`
# shape inverts under pipefail when the producer is still writing as grep exits (GATE-SIGPIPE-01).
[[ "$(grep -cFx "${GATE_RUN_LINE}" "${CI_WORKFLOW}")" -eq 1 ]] ||
  fail "self-test: ci.yaml does not carry exactly one '${GATE_RUN_LINE}' line -- every mutation below would be a no-op"

# --- ci.yaml: the gate step itself ---
grep -vFx "${GATE_RUN_LINE}" "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-removed.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-removed.yaml" "${CI_WORKFLOW}" \
  'the docs map gate step is deleted from lint' \
  "has no step invoking ${GATE_SCRIPT}"

awk -v line="${GATE_RUN_LINE}" '
  { print }
  $0 == line { print "      - name: a second copy of the same gate"; print line }
' "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-duplicated.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-duplicated.yaml" "${CI_WORKFLOW}" \
  'the docs map gate step appears twice in lint' \
  "steps invoking ${GATE_SCRIPT}"

awk -v line="${GATE_RUN_LINE}" '{ if ($0 == line) { print $0 " || true" } else { print } }' \
  "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-or-true.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-or-true.yaml" "${CI_WORKFLOW}" \
  'the docs map gate step swallows failure with || true' \
  "has no step invoking ${GATE_SCRIPT}"

# A block scalar whose only line is commented out. This also exercises the block-scalar reader:
# a parser that read `run: |` as the command would count this as wiring.
awk -v line="${GATE_RUN_LINE}" -v gate="bash hack/test/docs_map_contract_test.sh" '
  { if ($0 == line) { print "        run: |"; print "          # " gate } else { print } }
' "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-commented-out.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-commented-out.yaml" "${CI_WORKFLOW}" \
  'the docs map gate step body is a commented-out block scalar' \
  "has no step invoking ${GATE_SCRIPT}"

awk -v line="${GATE_RUN_LINE}" '
  { print; if ($0 == line) { print "        continue-on-error: true" } }
' "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-soft-fail.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-soft-fail.yaml" "${CI_WORKFLOW}" \
  'the docs map gate step is soft-failed with continue-on-error' \
  'must not declare'

awk -v line="${GATE_RUN_LINE}" '
  { print; if ($0 == line) { print "        if: false" } }
' "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-disabled.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-disabled.yaml" "${CI_WORKFLOW}" \
  'the docs map gate step is disabled with if: false' \
  'must run unconditionally'

# --- ci.yaml: the lint job around it ---
awk '{ print; if ($0 == "  lint:") { print "    continue-on-error: true" } }' \
  "${CI_WORKFLOW}" >"${MUTANTS}/lint-job-soft-fail.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/lint-job-soft-fail.yaml" "${CI_WORKFLOW}" \
  'the whole lint job is soft-failed with continue-on-error' \
  'the lint job must not declare'

awk '{ print; if ($0 == "  lint:") { print "    if: false" } }' \
  "${CI_WORKFLOW}" >"${MUTANTS}/lint-job-disabled.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/lint-job-disabled.yaml" "${CI_WORKFLOW}" \
  'the whole lint job is disabled with if: false' \
  'the lint job must run unconditionally'

awk '{ if ($0 == "  lint:") { print "  linting:" } else { print } }' \
  "${CI_WORKFLOW}" >"${MUTANTS}/lint-job-renamed.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/lint-job-renamed.yaml" "${CI_WORKFLOW}" \
  'the lint job is renamed out from under this lock' \
  'declares no lint job'

# The steps key alone, so the orphaned step bodies stay in the file: a reader that scanned the
# whole job for a matching line rather than its steps would still find the gate here.
awk -v inlint=0 '
  $0 == "  lint:" { inlint = 1 }
  inlint && $0 == "    steps:" { inlint = 0; next }
  { print }
' "${CI_WORKFLOW}" >"${MUTANTS}/lint-job-stepless.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/lint-job-stepless.yaml" "${CI_WORKFLOW}" \
  'the lint job declares no steps at all' \
  'would pass vacuously'

# --- ci.yaml: the trigger that makes the lint step reachable at all ---
awk '{ print; if ($0 == "    paths-ignore:") { print "      - \"hack/test/**\"" } }' \
  "${CI_WORKFLOW}" >"${MUTANTS}/hack-path-ignored.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/hack-path-ignored.yaml" "${CI_WORKFLOW}" \
  'ci.yaml starts ignoring hack/test/**' \
  'would then trigger no CI'

# --- hack/docs/verify.sh: the composition point, and this file's self-lock ---
awk -v l="bash ${GATE_SCRIPT}" '{ if ($0 == l) { print "# " l } else { print } }' \
  "${VERIFY_SCRIPT}" >"${MUTANTS}/verify-gate-removed.sh"
mutant_rejected check_docs_composition "${MUTANTS}/verify-gate-removed.sh" "${VERIFY_SCRIPT}" \
  'task docs:verify no longer invokes the docs map gate' \
  "does not invoke ${GATE_SCRIPT}"

awk -v l="bash ${WIRING_SCRIPT}" '{ if ($0 == l) { print "# " l } else { print } }' \
  "${VERIFY_SCRIPT}" >"${MUTANTS}/verify-wiring-removed.sh"
mutant_rejected check_docs_composition "${MUTANTS}/verify-wiring-removed.sh" "${VERIFY_SCRIPT}" \
  'this wiring test is no longer run by task docs:verify' \
  "does not invoke ${WIRING_SCRIPT}"

# The verify.sh side of the line-exact claim, which the two mutants above do not prove: a
# neutered invocation is still an invocation to a substring matcher.
awk -v l="bash ${WIRING_SCRIPT}" '{ if ($0 == l) { print l " || true" } else { print } }' \
  "${VERIFY_SCRIPT}" >"${MUTANTS}/verify-wiring-or-true.sh"
mutant_rejected check_docs_composition "${MUTANTS}/verify-wiring-or-true.sh" "${VERIFY_SCRIPT}" \
  'this wiring test is invoked from task docs:verify with a trailing || true' \
  "does not invoke ${WIRING_SCRIPT}"

# A verify.sh with nothing executable in it would make the membership loop above vacuous.
sed 's/^\([^#]\)/# \1/' "${VERIFY_SCRIPT}" >"${MUTANTS}/verify-all-comments.sh"
mutant_rejected check_docs_composition "${MUTANTS}/verify-all-comments.sh" "${VERIFY_SCRIPT}" \
  'hack/docs/verify.sh has been reduced to comments' \
  'has no executable lines'

# The gate this file exists to lock could be emptied out while its wiring is left in place.
: >"${MUTANTS}/empty-gate.sh"
if (assert_gate_present "${MUTANTS}/empty-gate.sh") >/dev/null 2>&1; then
  fail "self-test: the gate accepts an empty ${GATE_SCRIPT} -- the wiring would then lock in nothing"
fi
pass "self-test: gate rejects an empty ${GATE_SCRIPT}"

# --- the GREEN direction that matters: the ci.yaml restructure in flight ---
# The shape of the lane landing alongside this one: a `changes` classifier job, `test` renamed to
# `test-suite`, a report-only `test` job on `if: always()`, `needs:`/`if:` gating on siblings, a
# soft-failed sibling job, and `paths-ignore` dropped from `pull_request` entirely -- with `lint`
# left ungated, exactly as that lane leaves it. If this lock reddened on any of that it would be
# deleted rather than fixed, so the pass below is an assertion, not a note.
#
# This fixture is written OUT IN FULL rather than derived from the real ci.yaml on purpose. A
# mutation that adds a `changes` job to the live file would start producing a DUPLICATE the day
# the restructure merges, and this gate would red on its own scaffolding -- the precise failure it
# exists to avoid. A standalone fixture is stable across that landing. The top-level run above is
# what checks the real file.
cat >"${MUTANTS}/ci-restructured.yaml" <<'RESTRUCTURED'
name: CI
on:
  push:
    branches: [main]
    paths-ignore:
      - "docs/**"
      - "CHANGELOG.md"
  pull_request:

permissions:
  contents: read

jobs:
  changes:
    name: changes
    runs-on: ubuntu-latest
    outputs:
      code: ${{ steps.filter.outputs.code }}
    steps:
      - id: filter
        run: echo "code=true" >> "${GITHUB_OUTPUT}"

  lint:
    name: lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
        with:
          persist-credentials: false
      - name: A neighbouring step whose body merely mentions the gate
        run: |
          # bash hack/test/docs_map_contract_test.sh
          echo "not the wiring"
      - name: Verify the docs map labels the pages it points at (DOC-MAP-01)
        run: bash hack/test/docs_map_contract_test.sh
      - name: Lint
        run: task lint

  test-suite:
    name: test-suite
    needs: [changes]
    if: needs.changes.outputs.code != 'false'
    runs-on: ubuntu-latest
    steps:
      - run: task test

  build:
    needs: [changes, test-suite]
    if: always()
    continue-on-error: true
    runs-on: ubuntu-latest
    steps:
      - run: task build

  test:
    name: test
    needs: [changes, test-suite]
    if: always()
    runs-on: ubuntu-latest
    steps:
      - name: Report the required test context
        run: |
          set -euo pipefail
          echo "report-only context"
RESTRUCTURED

# Assert the fixture really is the shape being guarded against, rather than a file that merely
# parses. Counts, never `grep -q` on a pipe (GATE-SIGPIPE-01).
[[ "$(grep -cFx '  changes:' "${MUTANTS}/ci-restructured.yaml")" -eq 1 ]] ||
  fail "self-test: the restructured fixture has no changes job -- it is not the shape being guarded against"
[[ "$(grep -cFx '  test-suite:' "${MUTANTS}/ci-restructured.yaml")" -eq 1 ]] ||
  fail "self-test: the restructured fixture has no renamed test-suite job"
[[ "$(grep -cFx '    if: always()' "${MUTANTS}/ci-restructured.yaml")" -eq 2 ]] ||
  fail "self-test: the restructured fixture has no if:-gated sibling jobs"
[[ "$(grep -cFx '    continue-on-error: true' "${MUTANTS}/ci-restructured.yaml")" -eq 1 ]] ||
  fail "self-test: the restructured fixture has no soft-failed sibling job"
[[ "$(grep -cFx '    paths-ignore:' "${MUTANTS}/ci-restructured.yaml")" -eq 1 ]] ||
  fail "self-test: the restructured fixture should keep paths-ignore on push and drop it on pull_request"
[[ "$(grep -cFx "${GATE_RUN_LINE}" "${MUTANTS}/ci-restructured.yaml")" -eq 1 ]] ||
  fail "self-test: the restructured fixture lost the gate step, so a green below would mean nothing"

if ! (check_ci_wiring "${MUTANTS}/ci-restructured.yaml") >/dev/null 2>&1; then
  fail "self-test: this wiring lock reds on a ci.yaml carrying the changes-classifier restructure -- it is coupled to job structure it must not read: $( (check_ci_wiring "${MUTANTS}/ci-restructured.yaml") 2>&1 )"
fi
pass "self-test: gate stays green on a ci.yaml with a changes job, a renamed test-suite, if:-gated and soft-failed siblings, and no pull_request paths-ignore"

# ...and the same fixture with the gate step removed must RED, or the green above only proves
# that check_ci_wiring is easy to please.
grep -vFx "${GATE_RUN_LINE}" "${MUTANTS}/ci-restructured.yaml" \
  >"${MUTANTS}/ci-restructured-unwired.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/ci-restructured-unwired.yaml" \
  "${MUTANTS}/ci-restructured.yaml" \
  'the restructured ci.yaml has no docs map gate step' \
  "has no step invoking ${GATE_SCRIPT}"

# The other green direction: the same wiring written as a `run: |` block scalar is still bare,
# uncommented and unguarded, and must be accepted. Without a case for it, a reader that ignored
# block scalars would look correct here and red the day someone reformats that step.
awk -v line="${GATE_RUN_LINE}" -v gate="bash ${GATE_SCRIPT}" '
  { if ($0 == line) { print "        run: |"; print "          " gate } else { print } }
' "${MUTANTS}/ci-restructured.yaml" >"${MUTANTS}/ci-block-scalar.yaml"
[[ "$(grep -cFx "          bash ${GATE_SCRIPT}" "${MUTANTS}/ci-block-scalar.yaml")" -eq 1 ]] &&
  [[ "$(grep -cFx "${GATE_RUN_LINE}" "${MUTANTS}/ci-block-scalar.yaml")" -eq 0 ]] ||
  fail "self-test: the block-scalar fixture was not built -- the check below proves nothing"
if ! (check_ci_wiring "${MUTANTS}/ci-block-scalar.yaml") >/dev/null 2>&1; then
  fail "self-test: this lock reds when the gate step is written as a 'run: |' block scalar, which is still bare, uncommented and unguarded wiring: $( (check_ci_wiring "${MUTANTS}/ci-block-scalar.yaml") 2>&1 )"
fi
pass "self-test: gate accepts the wiring written as a 'run: |' block scalar"

awk -v gate="          bash ${GATE_SCRIPT}" '
  { if ($0 == gate) { print "          echo skipped" } else { print } }
' "${MUTANTS}/ci-block-scalar.yaml" >"${MUTANTS}/ci-block-scalar-unwired.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/ci-block-scalar-unwired.yaml" \
  "${MUTANTS}/ci-block-scalar.yaml" \
  'the block-scalar wiring is replaced by an echo' \
  "has no step invoking ${GATE_SCRIPT}"

# --- guard rails ---
# The degenerate "rejections" that an any-nonzero-exit check would have accepted as proof.
self_test_guard_holds() {
  local checker="$1" mutant="$2" original="$3" label="$4"
  if (mutant_rejected "${checker}" "${mutant}" "${original}" "${label}" 'unreachable-expected-message') >/dev/null 2>&1; then
    fail "self-test: mutant_rejected accepted ${label} as a genuine rejection -- it is tautological again"
  fi
  pass "self-test: mutant_rejected refuses to count ${label} as a rejection"
}

: >"${MUTANTS}/empty.yaml"
self_test_guard_holds check_ci_wiring "${MUTANTS}/empty.yaml" "${CI_WORKFLOW}" 'an empty file'
printf 'jobs: [[[\n' >"${MUTANTS}/broken.yaml"
self_test_guard_holds check_ci_wiring "${MUTANTS}/broken.yaml" "${CI_WORKFLOW}" 'an unparseable file'
self_test_guard_holds check_ci_wiring "${MUTANTS}/does-not-exist.yaml" "${CI_WORKFLOW}" 'a nonexistent path'
cp "${CI_WORKFLOW}" "${MUTANTS}/unmutated.yaml"
self_test_guard_holds check_ci_wiring "${MUTANTS}/unmutated.yaml" "${CI_WORKFLOW}" \
  'an unmutated copy of the real workflow'

echo "All docs map wiring tests passed."
