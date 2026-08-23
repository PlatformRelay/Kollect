#!/usr/bin/env bash
# Locks dist_* meta-tests into CI's lint job, in an order that actually works.
#
# GATE-HARDEN-01: this gate used to compare FILE-GLOBAL line numbers and lock onto the first
# `head -1` hit. Moving the operator-sdk install step out of `lint` into the earlier `gitleaks`
# job therefore kept it green while the job that actually runs the dist_* glob had no binary --
# exactly the permanent red the gate exists to prevent. Every assertion below is anchored to
# `.jobs.lint.steps`, so a step in another job is not a match, and each anchor must resolve to
# exactly one step, so a duplicate fails loudly instead of silently resolving to the first.
# The self-test at the bottom re-runs that perturbation on a mutated copy every CI run.
#
# GATE-COMMENT-01: the step anchors used to `contains()` the RAW `run:` body, which cannot tell
# code from a comment. `run: "# bash hack/install-operator-sdk.sh ./bin"` satisfied the install
# anchor while the lint job installed nothing, and commenting out `bash "${script}"` inside the
# glob step's body ran no meta-tests at all -- both left this gate green while producing exactly
# the end state it exists to prevent. Every anchor now matches against a COMMENT-STRIPPED view of
# the body, and the glob step must be shown to actually invoke each script it expands to.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CI_WORKFLOW="${ROOT}/.github/workflows/ci.yaml"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "ok - $*"; }

command -v yq >/dev/null 2>&1 ||
  fail "yq (mikefarah/yq v4) is required to inspect the lint job's step list"

# GATE-COMMENT-01: the executable half of a step's `run:` body -- every line whose first
# non-blank character is `#` is dropped before matching, so a commented-out command is not a
# command. Used by every anchor below; `.value` is a `to_entries` entry.
RUN_CODE='((.value.run // "") | split("\n") | map(select(test("^[[:space:]]*#") | not)) | join("\n"))'

SDK_MATCH="${RUN_CODE} | contains(\"hack/install-operator-sdk.sh\")"
GLOB_MATCH="${RUN_CODE} | contains(\"hack/test/dist_*_test.sh\")"
LINT_MATCH="${RUN_CODE} | test(\"^task lint[[:space:]]*\$\")"

# The same comment-stripped view, for one step selected by index.
lint_step_run_code() {
  local workflow="$1" idx="$2"
  yq eval "(.jobs.lint.steps[${idx}].run // \"\") | split(\"\n\") | map(select(test(\"^[[:space:]]*#\") | not)) | join(\"\n\")" "${workflow}"
}

# 0-based index of the ONE step in .jobs.lint.steps satisfying a yq predicate. Zero matches
# and duplicates are both hard failures -- see the header.
lint_step_index() {
  local workflow="$1" predicate="$2" label="$3"
  local query count
  query="$(printf '[.jobs.lint.steps | to_entries[] | select(%s) | .key]' "${predicate}")"
  count="$(yq eval "${query} | length" "${workflow}")"
  case "${count}" in
  1) yq eval "${query} | .[0]" "${workflow}" ;;
  0) fail "the lint job has no ${label} step -- a matching step in another job does not count, and a commented-out command is not a command; the dist_* glob runs in lint" ;;
  *) fail "the lint job has ${count} ${label} steps -- keep exactly one so the ordering assertions cannot lock onto the first match" ;;
  esac
}

check_wiring() {
  local workflow="$1"
  local job_kind steps_kind sdk_idx glob_idx lint_idx glob_name glob_run

  [[ -f "${workflow}" ]] || fail "expected ${workflow}"

  # Without these two guards a renamed job or a stepless lint job makes every assertion
  # below select over an empty list, and the gate passes vacuously.
  job_kind="$(yq eval '.jobs.lint | type' "${workflow}")"
  [[ "${job_kind}" == "!!map" ]] ||
    fail "${workflow} declares no lint job -- the wiring assertions would pass vacuously"
  steps_kind="$(yq eval '.jobs.lint.steps | type' "${workflow}")"
  [[ "${steps_kind}" == "!!seq" ]] ||
    fail "${workflow} lint job declares no steps -- the wiring assertions would pass vacuously"

  glob_idx="$(lint_step_index "${workflow}" "${GLOB_MATCH}" 'hack/test/dist_*_test.sh glob')" || exit 1
  lint_idx="$(lint_step_index "${workflow}" "${LINT_MATCH}" 'task lint')" || exit 1

  glob_name="$(yq eval ".jobs.lint.steps[${glob_idx}].name" "${workflow}")"
  [[ "${glob_name}" == *"Hub distribution meta-tests"* ]] ||
    fail "the lint job's dist_* glob step must be named 'Hub distribution meta-tests', got '${glob_name}'"

  # GATE-COMMENT-01: matching the glob PATTERN is not the same as running the scripts it
  # expands to. The step's body mentions `hack/test/dist_*_test.sh` twice (the array assignment
  # and the empty-glob diagnostic), so commenting out the one line that actually executes a
  # script left this step anchored, ordered, correctly named -- and inert.
  glob_run="$(lint_step_run_code "${workflow}" "${glob_idx}")"
  # shellcheck disable=SC2016  # the single quotes are the point: this is a literal, not an expansion
  [[ "${glob_run}" == *'bash "${script}"'* ]] ||
    fail "the lint job's dist_* glob step must execute each matched script -- its run: body has no 'bash \"\${script}\"' invocation, so the step would expand the glob and run nothing"

  [[ "${glob_idx}" -lt "${lint_idx}" ]] ||
    fail "the dist_* glob (lint step ${glob_idx}) must come before the task lint step (lint step ${lint_idx})"
  pass "ci.yaml lint job globs dist_*_test.sh before task lint and runs each matched script"

  # DIST-OH-02: dist_olm_bundle_test.sh hard-fails without operator-sdk, so the SAME job must
  # install it BEFORE the dist_* glob step. Ordering is the whole point: an install step placed
  # after the glob -- or in another job entirely -- turns the OperatorHub validator gate into a
  # permanent red instead of a working gate.
  sdk_idx="$(lint_step_index "${workflow}" "${SDK_MATCH}" 'hack/install-operator-sdk.sh install')" || exit 1
  [[ "${sdk_idx}" -lt "${glob_idx}" ]] ||
    fail "the operator-sdk install (lint step ${sdk_idx}) must come before the dist_* glob (lint step ${glob_idx})"
  pass "ci.yaml lint job installs operator-sdk before the dist_* glob"
}

check_wiring "${CI_WORKFLOW}"

# Self-test: a gate that only passes on the happy path is not evidence. Mutate the real
# workflow with yq (structural, so innocent reformatting cannot spuriously red this) and
# assert the checks above reject each mutation. Mutant 1 is the reviewer's perturbation.
MUTANTS="$(mktemp -d)"
trap 'rm -rf "${MUTANTS}"' EXIT

# GATE-COMMENT-01: this used to accept ANY nonzero exit as proof of rejection, which made the
# self-test itself tautological -- an empty file, a syntactically broken file and a nonexistent
# path each exited nonzero and each printed `ok - self-test: gate rejects ...`. A mutant now has
# to be a real, different, non-empty workflow, AND the rejection has to carry the message of the
# assertion the mutation was built to trip. `${expect}` is the load-bearing half: `cmp` only
# rules out no-op mutations, it says nothing about WHICH check fired.
mutant_rejected() {
  local mutant="$1" label="$2" expect="$3"
  local output status=0

  [[ -s "${mutant}" ]] ||
    fail "self-test: the mutant for '${label}' is missing or empty -- the yq mutation step failed, so nothing was actually tested"
  if cmp -s "${mutant}" "${CI_WORKFLOW}"; then
    fail "self-test: the mutant for '${label}' is byte-identical to ${CI_WORKFLOW} -- the yq mutation was a no-op, so nothing was actually tested"
  fi

  # Subshell: fail's `exit 1` must not take the parent down -- rejection is the expectation.
  output="$( (check_wiring "${mutant}") 2>&1 )" || status=$?
  [[ "${status}" -ne 0 ]] ||
    fail "self-test: the gate still passed on a workflow where ${label} -- it is vacuous"
  [[ "${output}" == *"${expect}"* ]] ||
    fail "self-test: the gate rejected '${label}' but not for the intended reason -- expected the failure to mention '${expect}', got: ${output}"
  pass "self-test: gate rejects a workflow where ${label}"
}

yq eval '
  .jobs.gitleaks.steps += [.jobs.lint.steps[] | select((.run // "") | contains("hack/install-operator-sdk.sh"))] |
  .jobs.lint.steps = [.jobs.lint.steps[] | select(((.run // "") | contains("hack/install-operator-sdk.sh")) | not)]
' "${CI_WORKFLOW}" >"${MUTANTS}/moved-to-gitleaks.yaml"
mutant_rejected "${MUTANTS}/moved-to-gitleaks.yaml" \
  'the operator-sdk install moved out of lint into gitleaks' \
  'the lint job has no hack/install-operator-sdk.sh install step'

yq eval '
  .jobs.lint.steps += [.jobs.lint.steps[] | select((.run // "") | contains("hack/install-operator-sdk.sh"))]
' "${CI_WORKFLOW}" >"${MUTANTS}/duplicated.yaml"
mutant_rejected "${MUTANTS}/duplicated.yaml" \
  'the operator-sdk install appears twice in lint' \
  'hack/install-operator-sdk.sh install steps'

yq eval '
  .jobs.lint.steps = [.jobs.lint.steps[] | select(((.run // "") | contains("hack/install-operator-sdk.sh")) | not)] |
  .jobs.lint.steps += [{"name": "Install operator-sdk (OLM bundle validators)", "run": "bash hack/install-operator-sdk.sh ./bin"}]
' "${CI_WORKFLOW}" >"${MUTANTS}/after-glob.yaml"
mutant_rejected "${MUTANTS}/after-glob.yaml" \
  'the operator-sdk install sits after the dist_* glob' \
  'must come before the dist_* glob'

# GATE-COMMENT-01: the two comment-blindness shapes. Both passed before this change -- the first
# leaves the lint job with no operator-sdk, the second runs no dist_* meta-test at all.
yq eval '
  (.jobs.lint.steps[] | select((.run // "") | contains("hack/install-operator-sdk.sh")) | .run) |= "# " + .
' "${CI_WORKFLOW}" >"${MUTANTS}/sdk-commented-out.yaml"
mutant_rejected "${MUTANTS}/sdk-commented-out.yaml" \
  'the operator-sdk install body is commented out' \
  'the lint job has no hack/install-operator-sdk.sh install step'

yq eval '
  (.jobs.lint.steps[] | select((.run // "") | contains("hack/test/dist_*_test.sh")) | .run) |=
    sub("(?m)^[[:space:]]*bash \"", "# bash \"")
' "${CI_WORKFLOW}" >"${MUTANTS}/glob-invocation-commented-out.yaml"
mutant_rejected "${MUTANTS}/glob-invocation-commented-out.yaml" \
  'the dist_* glob step no longer invokes the scripts it expands' \
  'must execute each matched script'

# The self-test's own guard rails: these four are the degenerate "rejections" that the old
# any-nonzero-exit check accepted as proof. mutant_rejected must now refuse every one of them.
self_test_guard_holds() {
  local mutant="$1" label="$2"
  if (mutant_rejected "${mutant}" "${label}" 'unreachable-expected-message') >/dev/null 2>&1; then
    fail "self-test: mutant_rejected accepted ${label} as a genuine rejection -- it is tautological again"
  fi
  pass "self-test: mutant_rejected refuses to count ${label} as a rejection"
}

: >"${MUTANTS}/empty.yaml"
self_test_guard_holds "${MUTANTS}/empty.yaml" 'an empty file'
printf 'jobs: [[[\n' >"${MUTANTS}/broken.yaml"
self_test_guard_holds "${MUTANTS}/broken.yaml" 'an unparseable file'
self_test_guard_holds "${MUTANTS}/does-not-exist.yaml" 'a nonexistent path'
cp "${CI_WORKFLOW}" "${MUTANTS}/unmutated.yaml"
self_test_guard_holds "${MUTANTS}/unmutated.yaml" 'an unmutated copy of the real workflow'

echo "All dist CI wiring tests passed."
