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
| B branch review | done — two rounds; round 1: 5/6 legs all BLOCK (one real CRITICAL, fixed + schema ratchet); round 2: 4/5 legs, both remaining CRITICALs fixed; rejections and deferrals recorded below |
| hand-off | done — pushed and opened as a ready-for-review PR (non-draft, per the firstmate brief): https://github.com/PlatformRelay/Kollect/pull/450 |

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

### Round one (6 legs; DeepSeek adversarial timed out, 5 counted) — all BLOCK

- CRITICAL (all 5 legs): dangling name-only step in ci.yaml — fixed + schema ratchet.
- Fixed: install-step/env/shell skip switches; dependency-review warn-only/continue-on-error
  holes; `# why:` anchoring; CWS-2 list-item semantics; composite-action guard hole; inline
  `# zizmor: ignore` policing; `base64 -w0`.
- Rejected after verification: "medium zizmor audits pass silently" claim about
  `unsound-inputs` (the tree audits clean at medium — measured).
- Round one raw legs + register: `reviews/branch/` (the DeepSeek leg timed out twice, its
  empty report is not committed).

### Round two (5 legs, DeepSeek adversarial timed out again, 4 counted)

- Verdicts: GLM-fitness BLOCK, Qwen3.8-security BLOCK, GLM-spec CONCERNS, Opus-spec CONCERNS.
- FIXED and re-verified: schema_steps now covers composite actions (the pass message had
  claimed it already — fitness CRITICAL); the workflow-security step LIST is pinned exactly
  (3 steps, names + shapes pinned) closing the stub-binary route Qwen found (CRITICAL); 15
  docs-side guards whose only CI invocation was inside `task docs:verify` are now wired into
  the required lint job as one grouped step (spec WARNING); CWS-2 moved to block-level
  justification (comment above the rule key; deeper lines inherit — the round-one and
  round-two Opus consensus); a job carrying a required context NAME in a PR-less workflow no
  longer counts as required coverage (+ mutant); evidence mutant counts and the zizmor
  baseline re-recorded against the branch tip.
- Rejected: "--min-severity=high filters medium findings silently" (the spec pins a
  `--min-severity` and tasks.md records the measured breakdown — that IS the recorded
  decision); "the release API publishes no asset digests" (false — verified against the live
  API, the darwin/arm64 digest matches exactly); "sonar_ko_*/dist_* loops share one step" (the
  required-job criterion is what the spec's scenarios test).
- Deferred: actionlint as a pinned second schema opinion (fitness leg, new dependency —
  follow-up change); docs.yaml Pages concurrency keyed on `github.ref` for push (same
  rationale as CWS-4, outside this spec's scope).
- Max two rounds at stage B per the skill; remaining NOTEs are documented limits, not open
  defects.

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
