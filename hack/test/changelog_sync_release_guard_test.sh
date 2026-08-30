#!/usr/bin/env bash
# Meta-test for hack/check-changelog-release-guard.sh.
# SPDX-License-Identifier: MIT
#
# Regression lock for the 2026-08-05 incident: github-actions commit fbb5196a3
# ("docs: sync CHANGELOG.md [skip ci]") rewrote the cut release header
# `## [0.17.0](...compare/v0.16.0..v0.17.0) - 2026-08-05` back to
# `## [Unreleased]`, because changelog-sync ran ~8 minutes BEFORE the v0.17.0
# tag was pushed and git-cliff therefore saw those commits as unreleased.
# docs launch-truth then resolved the released version as 0.16.0 and Docs CI
# went red on every open PR.
#
# The guard must be BOTH directions:
#   - loud + fatal when a released section vanishes while its tag is visible
#     (or when a non-top released section vanishes at all), and
#   - quiet (refuse-to-push, exit 2) on the ordinary release path, where the
#     hand-written header legitimately precedes the tag. Otherwise the guard
#     reds every release and re-creates the alarm fatigue it exists to prevent.
#
# Exercised entirely offline with fixture CHANGELOG content + throwaway git
# repos, so it needs no release to be cut.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GUARD="${ROOT}/hack/check-changelog-release-guard.sh"
WORKFLOW="${ROOT}/.github/workflows/changelog-sync.yaml"

failures=0

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  failures=$((failures + 1))
}

pass() {
  printf 'ok - %s\n' "$*"
}

TMPROOT="$(mktemp -d)"
trap 'rm -rf "${TMPROOT}"' EXIT

# ---------------------------------------------------------------------------
# fixture helpers
# ---------------------------------------------------------------------------

# changelog_header -- the immutable Keep a Changelog preamble git-cliff emits.
changelog_header() {
  cat <<'EOF'
# Changelog

All notable changes to this project are documented here.

EOF
}

# section VERSION [BODY] -- emit one `## [...]` section.
section() {
  local version="$1" body="${2:-- **sink:** Something happened}"
  if [[ "${version}" == "Unreleased" ]]; then
    printf '## [Unreleased]\n'
  else
    printf '## [%s](https://github.com/platformrelay/kollect/compare/vPREV..v%s) - 2026-08-05\n' \
      "${version}" "${version}"
  fi
  printf '\n### Bug Fixes\n\n%s\n\n' "${body}"
}

# make_repo NAME [TAG...] -- throwaway git repo whose visible tags are TAG...
# The guard resolves tag visibility from the working directory, so tag presence
# is controlled by which repo the guard is invoked in -- no test-only backdoor
# in the production script.
make_repo() {
  local name="$1"
  shift
  local dir="${TMPROOT}/${name}"
  mkdir -p "${dir}"
  git -C "${dir}" init -q
  git -C "${dir}" -c user.name=t -c user.email=t@e commit -q --allow-empty -m init
  local tag
  for tag in "$@"; do
    git -C "${dir}" tag "${tag}"
  done
  printf '%s\n' "${dir}"
}

# run_guard REPO OLD_CONTENT NEW_CONTENT -- returns the guard's exit code and
# leaves combined output in GUARD_OUT.
GUARD_OUT=""
run_guard() {
  local repo="$1" old_content="$2" new_content="$3"
  local old="${repo}/old.md" new="${repo}/new.md" rc=0
  printf '%s' "${old_content}" >"${old}"
  printf '%s' "${new_content}" >"${new}"
  GUARD_OUT="$(cd "${repo}" && bash "${GUARD}" "${old}" "${new}" 2>&1)" || rc=$?
  return "${rc}"
}

# expect NAME REPO OLD NEW WANT_RC
expect() {
  local name="$1" repo="$2" old="$3" new="$4" want="$5" rc=0
  run_guard "${repo}" "${old}" "${new}" || rc=$?
  if [[ "${rc}" -eq "${want}" ]]; then
    pass "${name} (exit ${rc})"
  else
    fail "${name}: want exit ${want}, got ${rc} -- output: ${GUARD_OUT}"
  fi
}

[[ -f "${GUARD}" ]] || {
  fail "missing ${GUARD}"
  exit 1
}
[[ -x "${GUARD}" ]] || fail "${GUARD} is not executable"

# ---------------------------------------------------------------------------
# fixtures
# ---------------------------------------------------------------------------

RELEASED_TOP="$(
  changelog_header
  section 0.17.0
  section 0.16.0
  section 0.15.0
)"

DEMOTED_TOP="$(
  changelog_header
  section Unreleased
  section 0.16.0
  section 0.15.0
)"

UNRELEASED_TOP="$(
  changelog_header
  section Unreleased
  section 0.17.0
  section 0.16.0
)"

UNRELEASED_TOP_EDITED="$(
  changelog_header
  section Unreleased "- **api:** A different, freshly re-rendered entry [abc1234]"
  section 0.17.0
  section 0.16.0
)"

MIDDLE_DROPPED="$(
  changelog_header
  section 0.17.0
  section 0.15.0
)"

NEW_RELEASE_ADDED="$(
  changelog_header
  section 0.18.0
  section 0.17.0
  section 0.16.0
  section 0.15.0
)"

RC_TOP="$(
  changelog_header
  section 0.18.0-rc.1
  section 0.17.0
)"

RC_DEMOTED="$(
  changelog_header
  section Unreleased
  section 0.17.0
)"

# ---------------------------------------------------------------------------
# repos differing only in which tags are visible
# ---------------------------------------------------------------------------

REPO_NO_TAG="$(make_repo no-tag v0.16.0 v0.15.0)"
REPO_TAGGED="$(make_repo tagged v0.17.0 v0.16.0 v0.15.0)"
REPO_RC_NO_TAG="$(make_repo rc-no-tag v0.17.0)"
REPO_RC_TAGGED="$(make_repo rc-tagged v0.18.0-rc.1 v0.17.0)"

# ---------------------------------------------------------------------------
# 1. the guard must NOT over-fire: self-healing has to keep working
# ---------------------------------------------------------------------------

expect "identical files pass" \
  "${REPO_TAGGED}" "${RELEASED_TOP}" "${RELEASED_TOP}" 0

expect "Unreleased body re-render passes (self-healing preserved)" \
  "${REPO_TAGGED}" "${UNRELEASED_TOP}" "${UNRELEASED_TOP_EDITED}" 0

expect "no Unreleased section on either side passes" \
  "${REPO_TAGGED}" "${RELEASED_TOP}" "${RELEASED_TOP}" 0

expect "gaining an Unreleased section above intact releases passes" \
  "${REPO_TAGGED}" "${RELEASED_TOP}" "$(
    changelog_header
    section Unreleased
    section 0.17.0
    section 0.16.0
    section 0.15.0
  )" 0

expect "a newly rendered release header passes" \
  "${REPO_TAGGED}" "${RELEASED_TOP}" "${NEW_RELEASE_ADDED}" 0

# ---------------------------------------------------------------------------
# 2. release-in-flight: hand-written header, tag not pushed yet -> refuse (2)
# ---------------------------------------------------------------------------

expect "top release demoted with tag NOT yet visible refuses to push" \
  "${REPO_NO_TAG}" "${RELEASED_TOP}" "${DEMOTED_TOP}" 2

expect "prerelease header demoted with tag NOT yet visible refuses to push" \
  "${REPO_RC_NO_TAG}" "${RC_TOP}" "${RC_DEMOTED}" 2

# The refusal must name the pending tag so the operator can act on it.
run_guard "${REPO_NO_TAG}" "${RELEASED_TOP}" "${DEMOTED_TOP}" || true
if [[ "${GUARD_OUT}" == *"v0.17.0"* ]]; then
  pass "refusal names the pending tag"
else
  fail "refusal must name the pending tag v0.17.0 -- output: ${GUARD_OUT}"
fi
if [[ "${GUARD_OUT}" == *"::warning"* ]]; then
  pass "refusal emits a ::warning annotation"
else
  fail "refusal must emit a ::warning annotation -- output: ${GUARD_OUT}"
fi

# ---------------------------------------------------------------------------
# 3. real regression -> fatal (1)
# ---------------------------------------------------------------------------

expect "top release demoted while its tag IS visible is fatal" \
  "${REPO_TAGGED}" "${RELEASED_TOP}" "${DEMOTED_TOP}" 1

expect "prerelease demoted while its tag IS visible is fatal" \
  "${REPO_RC_TAGGED}" "${RC_TOP}" "${RC_DEMOTED}" 1

expect "a dropped middle release section is fatal (tag visible)" \
  "${REPO_TAGGED}" "${RELEASED_TOP}" "${MIDDLE_DROPPED}" 1

expect "a dropped middle release section is fatal even without its tag" \
  "${REPO_NO_TAG}" "${RELEASED_TOP}" "$(
    changelog_header
    section 0.17.0
    section 0.15.0
  )" 1

expect "top demotion PLUS a dropped middle section is fatal, not a refusal" \
  "${REPO_NO_TAG}" "${RELEASED_TOP}" "$(
    changelog_header
    section Unreleased
    section 0.15.0
  )" 1

expect "wiping every release section is fatal" \
  "${REPO_NO_TAG}" "${RELEASED_TOP}" "$(
    changelog_header
    section Unreleased
  )" 1

run_guard "${REPO_TAGGED}" "${RELEASED_TOP}" "${MIDDLE_DROPPED}" || true
if [[ "${GUARD_OUT}" == *"::error"* ]]; then
  pass "fatal path emits an ::error annotation"
else
  fail "fatal path must emit an ::error annotation -- output: ${GUARD_OUT}"
fi

# ---------------------------------------------------------------------------
# 4. version tokens are compared, not whole header lines
# ---------------------------------------------------------------------------

expect "a changed date/compare-link on an intact release is not a regression" \
  "${REPO_TAGGED}" "${RELEASED_TOP}" "$(
    changelog_header
    printf '## [0.17.0](https://github.com/platformrelay/kollect/compare/v0.16.0..v0.17.0) - 2026-08-06\n\n### Bug Fixes\n\n- **sink:** Something happened\n\n'
    section 0.16.0
    section 0.15.0
  )" 0

# ---------------------------------------------------------------------------
# 5. usage errors are distinguishable from findings
# ---------------------------------------------------------------------------

rc=0
(cd "${REPO_TAGGED}" && bash "${GUARD}") >/dev/null 2>&1 || rc=$?
if [[ "${rc}" -eq 3 ]]; then
  pass "missing arguments exit 3 (usage), not 0/1/2"
else
  fail "missing arguments: want exit 3, got ${rc}"
fi

rc=0
(cd "${REPO_TAGGED}" && bash "${GUARD}" /nonexistent/old.md /nonexistent/new.md) >/dev/null 2>&1 || rc=$?
if [[ "${rc}" -eq 3 ]]; then
  pass "unreadable inputs exit 3 (usage), not a silent pass"
else
  fail "unreadable inputs: want exit 3, got ${rc}"
fi

# ---------------------------------------------------------------------------
# 6. the workflow actually wires the guard in ahead of the push
# ---------------------------------------------------------------------------

if [[ -f "${WORKFLOW}" ]]; then
  # Parse the YAML rather than grepping raw lines: the header comment also
  # mentions the script path, so a line-based check would pass on the comment
  # alone and prove nothing about the actual step order.
  if python3 - "${WORKFLOW}" <<'PY'; then
import sys
import yaml

wf = yaml.safe_load(open(sys.argv[1]))
steps = wf["jobs"]["sync"]["steps"]

def indices_of(needle):
    return [i for i, s in enumerate(steps) if needle in (s.get("run") or "")]

def index_of(needle):
    found = indices_of(needle)
    return found[0] if found else -1

guard = index_of("check-changelog-release-guard.sh")
# Gating the FIRST push step proves nothing if a second one exists: an extra,
# ungated `git push origin HEAD:main` step would push the fbb5196a3 demotion
# straight to main under `permissions: contents: write`. Count them all.
pushes = indices_of("git push origin HEAD:main")
push = pushes[0] if pushes else -1

errs = []
if guard < 0:
    errs.append("no step in job 'sync' runs check-changelog-release-guard.sh")
if push < 0:
    errs.append("no step in job 'sync' runs 'git push origin HEAD:main'")
if len(pushes) > 1:
    errs.append(
        f"exactly one step may run 'git push origin HEAD:main'; found {len(pushes)} "
        f"at step indices {pushes} -- every push step beyond the first is UNGATED "
        "by the guard verdict and would push a demoted CHANGELOG.md to main"
    )
# `continue-on-error: true` on the guard step swallows its fatal exit, so no
# verdict is emitted and the ::error is silenced. The push step is still skipped
# (bounded harm), but the guard stops being loud -- which is its whole job.
if guard >= 0:
    coe = steps[guard].get("continue-on-error")
    if coe is not None and coe is not False:
        errs.append(
            f"guard step must not declare continue-on-error (found {coe!r}) -- it "
            "silences the guard's ::error and leaves a lost release section unreported"
        )
if guard >= 0 and push >= 0:
    if guard >= push:
        errs.append(
            f"guard step (index {guard}) must run BEFORE the push step (index {push})"
        )
    # A verdict nobody consumes is a guard that does not guard: the push step
    # must be gated on the guard step's output.
    cond = str(steps[push].get("if") or "")
    if "verdict" not in cond:
        errs.append(
            f"push step must be gated on the guard verdict; its 'if:' is {cond!r}"
        )
    else:
        guard_id = steps[guard].get("id")
        if not guard_id:
            errs.append("guard step needs an 'id' for the push step to reference")
        elif f"steps.{guard_id}.outputs" not in cond:
            errs.append(
                f"push step 'if:' must reference steps.{guard_id}.outputs, got {cond!r}"
            )

if errs:
    for e in errs:
        print(e, file=sys.stderr)
    sys.exit(1)
print(f"guard step index {guard} < push step index {push}, push gated on {cond!r}")
PY
    pass "changelog-sync.yaml runs the guard before the push and gates the push on its verdict"
  else
    fail "changelog-sync.yaml guard/push wiring is wrong (see above)"
  fi

  # The header comment documents soft-fail behaviour; it must stay truthful now
  # that a fatal path exists.
  if grep -Fiq 'release guard' "${WORKFLOW}"; then
    pass "changelog-sync.yaml header documents the release guard"
  else
    fail "changelog-sync.yaml header comment must document the release guard"
  fi
else
  fail "missing ${WORKFLOW}"
fi

# ---------------------------------------------------------------------------
# 7. the workflow STEP BODY maps guard exit codes to the right job outcome
#
# Section 6 proves the guard step exists, runs before the push, and that the
# push is gated on its output -- but it says nothing about WHICH verdict each
# exit code produces. Flipping `2) verdict=hold` to `2) verdict=push` inside the
# step's inline script would push the exact fbb5196a3 demotion to main while
# every check above still reported green. So execute the real step body,
# extracted from the workflow YAML, against fixture repositories and assert the
# job-level outcome: does it push, and does it fail?
# ---------------------------------------------------------------------------

if [[ -f "${WORKFLOW}" ]] && command -v python3 >/dev/null 2>&1; then
  STEP_BODY="${TMPROOT}/guard-step.sh"

  # Extract the guard step's `run:` body and the literal verdict the push step
  # gates on. The push condition is required to be a plain equality against one
  # single-quoted literal so the comparison below is an evaluation, not a guess.
  if PUSH_VERDICT="$(python3 - "${WORKFLOW}" "${STEP_BODY}" <<'PY'
import re
import sys

import yaml

wf = yaml.safe_load(open(sys.argv[1]))
steps = wf["jobs"]["sync"]["steps"]

guard = next(
    (s for s in steps if "check-changelog-release-guard.sh" in (s.get("run") or "")),
    None,
)
pushes = [s for s in steps if "git push origin HEAD:main" in (s.get("run") or "")]
if guard is None or not pushes:
    print("guard or push step not found", file=sys.stderr)
    sys.exit(1)
# Evaluating the FIRST push step's `if:` says nothing about a second one. An
# appended, ungated push step slips past this whole section unless we count.
if len(pushes) > 1:
    print(
        f"exactly one step may run 'git push origin HEAD:main'; found {len(pushes)} "
        "-- every push step beyond the first is UNGATED by the guard verdict",
        file=sys.stderr,
    )
    sys.exit(1)
push = pushes[0]

cond = str(push.get("if") or "")
m = re.fullmatch(
    r"\s*steps\.(?P<id>[A-Za-z0-9_-]+)\.outputs\.verdict\s*==\s*'(?P<lit>[^']+)'\s*",
    cond,
)
if not m:
    print(
        "push step 'if:' must be exactly steps.<id>.outputs.verdict == '<literal>', "
        f"got {cond!r}",
        file=sys.stderr,
    )
    sys.exit(1)
if m.group("id") != guard.get("id"):
    print(
        f"push step gates on steps.{m.group('id')} but the guard step id is "
        f"{guard.get('id')!r}",
        file=sys.stderr,
    )
    sys.exit(1)

open(sys.argv[2], "w").write(guard["run"])
print(m.group("lit"))
PY
  )"; then
    pass "push step gates on a single verdict literal (${PUSH_VERDICT}) emitted by the guard step"

    # make_sync_sandbox NAME COMMITTED REGENERATED [TAG...]
    # A repo whose HEAD commit is UNTAGGED (tags sit on the base commit, as on a
    # real main after a release) and whose CHANGELOG.md is dirty in the worktree,
    # exactly as `task changelog:write` leaves it before the guard step runs.
    make_sync_sandbox() {
      local name="$1" committed="$2" regenerated="$3"
      shift 3
      local dir="${TMPROOT}/sync-${name}" tag
      mkdir -p "${dir}/hack"
      cp "${GUARD}" "${dir}/hack/check-changelog-release-guard.sh"
      git -C "${dir}" init -q
      git -C "${dir}" -c user.name=t -c user.email=t@e commit -q --allow-empty -m base
      for tag in "$@"; do
        git -C "${dir}" tag "${tag}"
      done
      printf '%s' "${committed}" >"${dir}/CHANGELOG.md"
      git -C "${dir}" add CHANGELOG.md hack/check-changelog-release-guard.sh
      git -C "${dir}" -c user.name=t -c user.email=t@e commit -q -m "release prep"
      printf '%s' "${regenerated}" >"${dir}/CHANGELOG.md"
      printf '%s\n' "${dir}"
    }

    # run_sync_step DIR -- STEP_RC / STEP_VERDICT / STEP_OUT.
    # changelog-sync.yaml sets no `shell:` and no `defaults.run`, so GitHub runs
    # this body as `bash -e {0}`. `--noprofile --norc` keeps a developer's
    # dotfiles out of the result; `-o pipefail` is deliberately STRICTER than
    # production -- the body contains no pipelines today, so it cannot mask or
    # invent a verdict, it only catches a future edit that adds one.
    STEP_RC=0
    STEP_VERDICT=""
    STEP_OUT=""
    run_sync_step() {
      local dir="$1" rc=0
      : >"${dir}/github-output"
      STEP_OUT="$(
        cd "${dir}" &&
          RUNNER_TEMP="${dir}" GITHUB_OUTPUT="${dir}/github-output" \
            bash --noprofile --norc -eo pipefail "${STEP_BODY}" 2>&1
      )" || rc=$?
      STEP_RC="${rc}"
      STEP_VERDICT="$(sed -n 's/^verdict=//p' "${dir}/github-output" | tail -1)"
    }

    # -- 7a. release in flight: the demotion must NOT reach the push step ------
    dir="$(make_sync_sandbox in-flight "${RELEASED_TOP}" "${DEMOTED_TOP}" v0.16.0 v0.15.0)"
    run_sync_step "${dir}"
    if [[ "${STEP_VERDICT}" != "${PUSH_VERDICT}" ]]; then
      pass "release-in-flight demotion emits verdict '${STEP_VERDICT}' != '${PUSH_VERDICT}' -- push step is skipped"
    else
      fail "release-in-flight demotion emitted the push verdict '${STEP_VERDICT}' -- the fbb5196a3 demotion WOULD be pushed to main"
    fi
    if [[ "${STEP_RC}" -eq 0 ]]; then
      pass "release-in-flight demotion keeps the job green (no alarm fatigue on every release)"
    else
      fail "release-in-flight demotion must not fail the job, got rc ${STEP_RC} -- output: ${STEP_OUT}"
    fi
    if [[ "${STEP_OUT}" == *"::warning"* && "${STEP_OUT}" == *"v0.17.0"* ]]; then
      pass "release-in-flight demotion surfaces a ::warning naming v0.17.0"
    else
      fail "release-in-flight demotion must surface a ::warning naming v0.17.0 -- output: ${STEP_OUT}"
    fi

    # -- 7b. genuine regression: the job must FAIL, nothing pushed ------------
    dir="$(make_sync_sandbox regression "${RELEASED_TOP}" "${DEMOTED_TOP}" v0.17.0 v0.16.0 v0.15.0)"
    run_sync_step "${dir}"
    if [[ "${STEP_RC}" -ne 0 ]]; then
      pass "demotion of a section whose tag IS visible fails the job (rc ${STEP_RC})"
    else
      fail "demotion of a section whose tag IS visible must fail the job, got rc 0 -- output: ${STEP_OUT}"
    fi
    if [[ "${STEP_VERDICT}" != "${PUSH_VERDICT}" ]]; then
      pass "failing regression never emits the push verdict"
    else
      fail "failing regression emitted the push verdict '${STEP_VERDICT}'"
    fi

    # A dropped middle section must fail even while a release is in flight.
    dir="$(make_sync_sandbox regression-middle "${RELEASED_TOP}" "$(
      changelog_header
      section 0.17.0
      section 0.15.0
    )" v0.16.0 v0.15.0)"
    run_sync_step "${dir}"
    if [[ "${STEP_RC}" -ne 0 && "${STEP_VERDICT}" != "${PUSH_VERDICT}" ]]; then
      pass "a dropped middle section fails the job and never pushes"
    else
      fail "a dropped middle section must fail the job and never push, got rc ${STEP_RC} verdict '${STEP_VERDICT}'"
    fi

    # -- 7c. legitimate [Unreleased] append on an untagged tree still pushes ---
    dir="$(make_sync_sandbox legit-append "$(
      changelog_header
      section 0.17.0
      section 0.16.0
    )" "$(
      changelog_header
      section Unreleased "- **api:** A freshly merged fix [abc1234]"
      section 0.17.0
      section 0.16.0
    )" v0.17.0 v0.16.0)"
    run_sync_step "${dir}"
    if [[ "${STEP_VERDICT}" == "${PUSH_VERDICT}" && "${STEP_RC}" -eq 0 ]]; then
      pass "a legitimate [Unreleased] append on an untagged tree still pushes (self-healing preserved)"
    else
      fail "self-healing broke: legitimate [Unreleased] append gave rc ${STEP_RC} verdict '${STEP_VERDICT}' -- output: ${STEP_OUT}"
    fi

    # Re-rendering the [Unreleased] body (the drift this workflow exists to heal)
    # must also still push.
    dir="$(make_sync_sandbox legit-rerender "${UNRELEASED_TOP}" "${UNRELEASED_TOP_EDITED}" v0.17.0 v0.16.0)"
    run_sync_step "${dir}"
    if [[ "${STEP_VERDICT}" == "${PUSH_VERDICT}" && "${STEP_RC}" -eq 0 ]]; then
      pass "an [Unreleased] body re-render still pushes (drift-healing preserved)"
    else
      fail "drift-healing broke: [Unreleased] re-render gave rc ${STEP_RC} verdict '${STEP_VERDICT}' -- output: ${STEP_OUT}"
    fi

    # -- 7d. nothing to do must not push -------------------------------------
    dir="$(make_sync_sandbox in-sync "${RELEASED_TOP}" "${RELEASED_TOP}" v0.17.0 v0.16.0 v0.15.0)"
    run_sync_step "${dir}"
    if [[ "${STEP_RC}" -eq 0 && "${STEP_VERDICT}" != "${PUSH_VERDICT}" ]]; then
      pass "an already-in-sync CHANGELOG.md is green and does not push"
    else
      fail "in-sync run must be green without pushing, got rc ${STEP_RC} verdict '${STEP_VERDICT}'"
    fi
  else
    fail "could not extract the guard step body / push verdict literal from ${WORKFLOW}"
  fi
else
  fail "missing ${WORKFLOW} or python3 -- cannot verify the guard step verdict mapping"
fi

if [[ "${failures}" -ne 0 ]]; then
  printf '\nchangelog_sync_release_guard: %d check(s) failed\n' "${failures}" >&2
  exit 1
fi

printf '\nAll changelog_sync_release_guard tests passed.\n'
