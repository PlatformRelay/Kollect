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
# shared workflow matchers
#
# Sections 6 and 7 both have to agree on WHICH step is the guard and WHICH steps
# push to main. A needle only one of them understands is a bug in both
# directions: an unrecognised spelling of the push is an invisible hole, and an
# honest rewrite of the push command turns into a gate failure. One matcher,
# imported by both.
# ---------------------------------------------------------------------------

PYLIB="${TMPROOT}/pylib"
mkdir -p "${PYLIB}"
export PYTHONPATH="${PYLIB}${PYTHONPATH:+:${PYTHONPATH}}"

cat >"${PYLIB}/syncwf.py" <<'PY'
"""Structural matchers for .github/workflows/changelog-sync.yaml."""

import re

# The guard is identified by the script it runs, not by its name or id:
# renaming the step is cosmetic, dropping the script is the defect.
GUARD_NEEDLE = "check-changelog-release-guard.sh"

_PUSH = re.compile(r"\bgit\s+push\b([^\n;&|)]*)")

_FALSY_WORDS = {"", "false", "0", "null"}


def code(step):
    """A step's `run:` body with comment-only lines dropped.

    A commented-out command is not a command, so `# git push origin HEAD:main`
    must not count as the push that self-healing depends on.
    """
    body = step.get("run") or ""
    return "\n".join(
        line for line in body.splitlines() if not line.lstrip().startswith("#")
    )


def _pushes_to_main(args):
    """True when the arguments of one `git push` invocation target main.

    Every spelling of the same push is recognised -- `main`, `HEAD:main`,
    `HEAD:refs/heads/main`, `+main`, `main:main` -- because they all put a
    demoted CHANGELOG.md on main under `permissions: contents: write`. Matching
    only the literal spelling in use today lets an appended step pick another
    one and stay invisible. A push aimed at any other branch (docs to gh-pages,
    say) is deliberately NOT matched.

    `git push` carrying no refspec is not matched: what it pushes depends on
    remote/push.default configuration, and treating it as a push to main would
    red honest probes such as `git push --dry-run origin`.
    """
    positional = [tok for tok in args.split() if not tok.startswith("-")]
    for spec in positional[1:]:  # positional[0] is the remote
        dst = spec.lstrip("+").split(":")[-1]
        if dst.startswith("refs/heads/"):
            dst = dst[len("refs/heads/") :]
        if dst == "main":
            return True
    return False


def push_step_indices(steps):
    """Indices of every step whose `run:` body pushes to main."""
    return [
        i
        for i, step in enumerate(steps)
        if any(_pushes_to_main(m.group(1)) for m in _PUSH.finditer(code(step)))
    ]


def guard_step_indices(steps):
    """Indices of every step that runs the release guard."""
    return [i for i, step in enumerate(steps) if GUARD_NEEDLE in code(step)]


def falsy_if(node):
    """repr() of the node's `if:` when it is a statically falsy literal, else None.

    `if: false` / `if: ${{ false }}` skip the step outright. Only literals are
    decidable here -- `if: ${{ github.actor == 'nobody' }}` is just as fatal and
    just as invisible to any static check -- so this is a floor, not a proof.
    An honest condition (a fork guard, say) is deliberately left alone.
    """
    if "if" not in node:
        return None
    raw = node["if"]
    if raw is None or raw is False:
        return repr(raw)
    if isinstance(raw, bool):
        return None
    if isinstance(raw, (int, float)):
        return None if raw else repr(raw)
    text = str(raw).strip()
    expr = re.fullmatch(r"\$\{\{(.*)\}\}", text, re.S)
    if expr:
        text = expr.group(1).strip()
    if text.strip("'\"").strip().lower() in _FALSY_WORDS:
        return repr(raw)
    return None
PY

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

# static_wiring_check WORKFLOW -- the structural guard/push assertions, applied
# to any workflow file so the self-test in section 8 can run them against
# deliberately broken copies. Parses the YAML rather than grepping raw lines:
# the header comment also mentions the script path, so a line-based check would
# pass on the comment alone and prove nothing about the actual step order.
static_wiring_check() {
  python3 - "$1" <<'PY'
import sys

import yaml

from syncwf import falsy_if, guard_step_indices, push_step_indices

wf = yaml.safe_load(open(sys.argv[1]))
steps = wf["jobs"]["sync"]["steps"]

guards = guard_step_indices(steps)
guard = guards[0] if guards else -1
# Gating the FIRST push step proves nothing if a second one exists: an extra,
# ungated push step would put the fbb5196a3 demotion straight on main under
# `permissions: contents: write`. Count them all, in every spelling.
pushes = push_step_indices(steps)
push = pushes[0] if pushes else -1
cond = ""

errs = []
if guard < 0:
    errs.append("no step in job 'sync' runs check-changelog-release-guard.sh")
if push < 0:
    errs.append("no step in job 'sync' runs a 'git push' to main")
if len(pushes) > 1:
    errs.append(
        f"exactly one step may push to main; found {len(pushes)} at step indices "
        f"{pushes} -- every push step beyond the first is UNGATED by the guard "
        "verdict and would push a demoted CHANGELOG.md to main"
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
    # The other half of that class, and the louder failure: a falsy `if:` skips
    # the guard step outright. No verdict is emitted, so the push step is
    # skipped too, self-healing stops silently, and the job still reports green.
    skipped = falsy_if(steps[guard])
    if skipped is not None:
        errs.append(
            f"guard step must not declare a statically falsy 'if:' (found {skipped}) "
            "-- the step is SKIPPED, it emits no verdict, the push step is skipped "
            "with it, and CHANGELOG.md self-healing dies while the job stays green"
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
}

if [[ -f "${WORKFLOW}" ]]; then
  if static_wiring_check "${WORKFLOW}"; then
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

from syncwf import guard_step_indices, push_step_indices

wf = yaml.safe_load(open(sys.argv[1]))
steps = wf["jobs"]["sync"]["steps"]

guards = guard_step_indices(steps)
pushes = push_step_indices(steps)
if not guards or not pushes:
    print("guard or push step not found", file=sys.stderr)
    sys.exit(1)
# Evaluating the FIRST push step's `if:` says nothing about a second one. An
# appended, ungated push step slips past this whole section unless we count.
if len(pushes) > 1:
    print(
        f"exactly one step may push to main; found {len(pushes)} -- every push "
        "step beyond the first is UNGATED by the guard verdict",
        file=sys.stderr,
    )
    sys.exit(1)
guard = steps[guards[0]]
push = steps[pushes[0]]

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

# ---------------------------------------------------------------------------
# 8. self-test: the section 6 assertions actually reject each mutation
#
# A gate that only ever runs against the honest workflow is not evidence -- the
# needle it matches on may be one an attacker (or a careless edit) simply
# spells differently. Mutate a structural copy of the real workflow and assert
# section 6 rejects it FOR THE INTENDED REASON: "some check failed" says nothing
# about which. The two acceptance cases are just as load-bearing, because a
# matcher wide enough to catch every mutant reds honest workflows instead.
#
# Mutants are built from the parsed YAML, so innocent reformatting of
# changelog-sync.yaml cannot spuriously red this; comments are lost in the
# round-trip, which is why only static_wiring_check runs against them (the
# header-comment assertion above stays on the real file).
# ---------------------------------------------------------------------------

cat >"${PYLIB}/mkmutant.py" <<'PY'
"""Write a structurally mutated copy of changelog-sync.yaml for the self-test."""

import sys

import yaml

from syncwf import guard_step_indices, push_step_indices

source, target, kind = sys.argv[1], sys.argv[2], sys.argv[3]
wf = yaml.safe_load(open(source))
steps = wf["jobs"]["sync"]["steps"]

guards = guard_step_indices(steps)
pushes = push_step_indices(steps)
if not guards or not pushes:
    # Only reachable when the real workflow is already broken -- section 6 has
    # said so loudly by now; do not add a traceback on top of it.
    print("no guard/push step to mutate", file=sys.stderr)
    sys.exit(2)
guard = steps[guards[0]]
push = steps[pushes[0]]

if kind == "identity":
    pass  # normalisation baseline: proves the round-trip itself changes nothing
elif kind == "guard-if-false":
    guard["if"] = False
elif kind == "guard-if-expr-false":
    guard["if"] = "${{ false }}"
elif kind == "guard-if-fork-check":  # honest condition -- must NOT be rejected
    guard["if"] = "${{ github.repository == 'platformrelay/kollect' }}"
elif kind == "extra-push-plain":
    steps.append({"name": "Shadow push", "run": "git push origin main\n"})
elif kind == "extra-push-refs-heads":
    steps.append({"name": "Shadow push", "run": "git push origin HEAD:refs/heads/main\n"})
elif kind == "push-commented-out":
    push["run"] = (
        "\n".join(
            "# " + line if "git push" in line else line
            for line in push["run"].splitlines()
        )
        + "\n"
    )
elif kind == "guard-commented-out":
    guard["run"] = (
        "\n".join(
            "# " + line if "check-changelog-release-guard.sh" in line else line
            for line in guard["run"].splitlines()
        )
        + "\n"
    )
elif kind == "push-spelled-plain":  # honest rewrite -- must NOT be rejected
    push["run"] = push["run"].replace("git push origin HEAD:main", "git push origin main")
elif kind == "extra-push-other-branch":  # not main -- must NOT be rejected
    steps.append({"name": "Publish docs", "run": "git push origin HEAD:gh-pages\n"})
else:
    print(f"unknown mutation {kind!r}", file=sys.stderr)
    sys.exit(2)

yaml.safe_dump(wf, open(target, "w"), sort_keys=False, default_flow_style=False)
PY

# build_mutant KIND -- path of the mutated workflow, or empty on failure.
build_mutant() {
  local kind="$1"
  local mutant="${TMPROOT}/mutant-${kind}.yaml"
  python3 "${PYLIB}/mkmutant.py" "${WORKFLOW}" "${mutant}" "${kind}" >/dev/null || return 1
  [[ -s "${mutant}" ]] || return 1
  printf '%s\n' "${mutant}"
}

MUTANT_BASELINE="$(build_mutant identity)" ||
  fail "self-test: could not normalise ${WORKFLOW} -- no mutant can be trusted"

if [[ -n "${MUTANT_BASELINE}" ]]; then
  if static_wiring_check "${MUTANT_BASELINE}" >/dev/null 2>&1; then
    pass "self-test: the honest workflow still passes after a structural round-trip"
  else
    fail "self-test: the normalised honest workflow is rejected -- every mutant below would red for the wrong reason"
  fi
fi

# mutant_rejected KIND LABEL EXPECTED_SUBSTRING
mutant_rejected() {
  local kind="$1" label="$2" expect="$3" mutant output status=0
  mutant="$(build_mutant "${kind}")" || {
    fail "self-test: could not build the '${kind}' mutant -- nothing was tested"
    return
  }
  # `cmp` against an unset baseline errors out and would silently wave the
  # no-op check through, so the emptiness is tested first, not by cmp.
  if [[ -n "${MUTANT_BASELINE}" ]] && cmp -s "${mutant}" "${MUTANT_BASELINE}"; then
    fail "self-test: the '${kind}' mutant is identical to the baseline -- the mutation was a no-op, so nothing was tested"
    return
  fi
  output="$(static_wiring_check "${mutant}" 2>&1)" || status=$?
  if [[ "${status}" -eq 0 ]]; then
    fail "self-test: the gate still passed on a workflow where ${label} -- that assertion is vacuous"
  elif [[ "${output}" != *"${expect}"* ]]; then
    fail "self-test: the gate rejected '${label}' but not for the intended reason -- expected a message mentioning '${expect}', got: ${output}"
  else
    pass "self-test: gate rejects a workflow where ${label}"
  fi
}

# mutant_accepted KIND LABEL -- a shape that is honest, not a defect.
mutant_accepted() {
  local kind="$1" label="$2" mutant output status=0
  mutant="$(build_mutant "${kind}")" || {
    fail "self-test: could not build the '${kind}' variant -- nothing was tested"
    return
  }
  output="$(static_wiring_check "${mutant}" 2>&1)" || status=$?
  if [[ "${status}" -eq 0 ]]; then
    pass "self-test: gate accepts a workflow where ${label}"
  else
    fail "self-test: gate rejected an honest workflow where ${label} -- false positive: ${output}"
  fi
}

mutant_rejected guard-if-false \
  'the guard step is disabled with `if: false`' \
  'statically falsy'
mutant_rejected guard-if-expr-false \
  'the guard step is disabled with `if: ${{ false }}`' \
  'statically falsy'
mutant_rejected guard-commented-out \
  'the guard invocation is commented out' \
  'no step in job'
mutant_rejected extra-push-plain \
  "an extra ungated step pushes with 'git push origin main'" \
  'exactly one step may push to main'
mutant_rejected extra-push-refs-heads \
  "an extra ungated step pushes with 'git push origin HEAD:refs/heads/main'" \
  'exactly one step may push to main'
mutant_rejected push-commented-out \
  'the push invocation is commented out, so self-healing silently stops' \
  "runs a 'git push' to main"

mutant_accepted guard-if-fork-check \
  'the guard step carries an honest `if:` condition (a fork guard)'
mutant_accepted push-spelled-plain \
  "the one gated push is spelled 'git push origin main'"
mutant_accepted extra-push-other-branch \
  'another step pushes to a different branch (gh-pages)'

if [[ "${failures}" -ne 0 ]]; then
  printf '\nchangelog_sync_release_guard: %d check(s) failed\n' "${failures}" >&2
  exit 1
fi

printf '\nAll changelog_sync_release_guard tests passed.\n'
