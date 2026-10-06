# loop.md — ci-workflow-hardening (spec-loop run state)

- **Repo:** /Users/kheimel/.treehouse/kollect-79dca7/1/kollect (treehouse pool worktree; primary checkout untouched)
- **Branch:** `fm/kollect-dev-loop`, base `a1a300e1` (the spec-set commit; the worktree was
  detached there when the loop started).
- **Feature:** openspec change `ci-workflow-hardening` (Kollect has no `.specify/` tooling; the
  spec set lives in `openspec/changes/ci-workflow-hardening/`).
- **Reviewer availability:** `opencode` + `internal-ovhcloud` free legs available; `claude`
  available. Fan-out runs with claude legs — not single-model.
- **Operator steer (2026-10-05, mid-run):** ONE `claude/opus` leg per review point from here
  on. This round's two opus legs were already in flight when the steer arrived, so they run to
  completion under the `--claude-budget 3` cap; later gates (task reviews, round two) use a
  single opus leg.
- **Relaunch note:** the first worker session froze ~13 min into the implementation; this
  session resumed on GLM-5.3-Flash from its uncommitted worktree changes (zizmor hardening) and
  continued the loop per the firstmate brief.

## Stages

| Stage | State |
| --- | --- |
| P plan+tasks | present (spec.md, plan.md, tasks.md) |
| R spec-set review | done by earlier sessions; four review commits land on the spec set (886c5b98 independent review, b69f66ce round-two, 57981438 cold adversarial review, a1a300e1 post-merge verification status). Recorded per the skill's "already happened by other means" rule with commit hash a1a300e1; not repeated. |
| L task loop | implementation resumed and finished in this session (meta-test 1.1/1.2 was missing — written and wired; all agent-owned tasks closed; 4.2 and 5.x are throwaway-PR/operator rows left open) |
| B branch review | in progress — fanout-review register under `reviews/branch/` |
| hand-off | push + PR (non-draft, per the firstmate brief; the skill's default draft rule is overridden by the task contract) |

## Fitness functions (inventory at orient)

| Characteristic | Command | Type | Baseline |
| --- | --- | --- | --- |
| Go layering | `.go-arch-lint.yml` (arch lint) | triggered | allow-list |
| Go coverage floor | `task coverage` with `COVERAGE_MIN=90` (internal/, enforced by ci.yaml + codecov) | holistic, per change | ≥90% |
| Shell hygiene | `task lint:shell` (shellcheck --severity=warning over hack/**) | triggered | no warnings |
| Workflow supply chain | zizmor `--offline --min-severity=high` over `.github/` (new in this branch) | triggered | 0 findings at high |
| CI guard meta-tests | `hack/test/*_test.sh` run in the required lint job | triggered | green suite |
| Static Go analysis | golangci-lint + Sonar (required contexts `Analyze (Go)`, `vulncheck`) | holistic | green on main |

This branch touches no Go code, so the Go rows are unaffected by construction; the
triggered CI-workflow function (the new meta-test itself) runs green on the branch tip.

## Branch review (stage B)

- Fan-out: `GLM-5.3:spec,DeepSeek-V4.1-Flash:adversarial,Qwen3.8-Flash-Next:security,GLM-5.3:fitness`
  free legs + `claude/opus:spec` + `claude/opus:adversarial` (CI/protected paths trigger),
  `--leg-timeout 1800`.
- Target: `git range a1a300e1..HEAD`; spec lens on
  `openspec/changes/ci-workflow-hardening/specs/ci-workflow-security/spec.md`.
- Raw leg reports and the register live in `reviews/branch/` (`.err` files are never committed).

## Task log

| Task | Verdict | Note |
| --- | --- | --- |
| 1.1/1.2 meta-test + wiring | CLOSED | missing artifact written by this session; pre-wiring red observed (CWS-6 self-wiring assertion) |
| 2.1 concurrency | CLOSED | CWS-4 exact expressions, both workflows; dist_ci_wiring allowlist widened with justification |
| 3.1 installer + min-severity | CLOSED | 4-platform fail-closed SHA256 pins; offline probe green |
| 3.2 zizmor findings | CLOSED | 4 findings fixed (template-injection, github-app, cache-poisoning, artipacked), none suppressed; severity breakdown recorded in tasks.md |
| 3.3 workflow-security job + eligibility | CLOSED | verify-eligibility + its test updated |
| 3.4 rule adopted | CLOSED | enforced by the meta-test for later changes |
| 4.1 dependency-review | CLOSED | allow-licenses derived from go-licenses (141 Apache-2.0, 60 MIT, 42 BSD-3, 9 BSD-2, 1 ISC) |
| 4.2 throwaway-PR probe | OPEN | needs two real throwaway PRs — owner/operator action, not doable from this worker |
| 5.1/5.2 ruleset | OPEN | operator |
| 6.1 land | OPEN | merge authority decides |
