Confirmed a suspicious duplicate step. Now the rest of the diff:
Now the spec and the new 711-line test script:
Now the surrounding context — workflow headers, evidence, tasks, repo conventions:
Confirmed a parse-breaking step. Which commit introduced it, and does the repo's own gate catch it:
The gate passes a tree actionlint rejects — its own blind spot. Now verify the zizmor install actually works and the audit is green, using the repo's own installer into a temp dir:
zizmor confirms the audit is green and one digest pin verified by use. Run the repo's shell lint over the new scripts and check where `task lint:shell` runs in CI:
Shell lint clean. Now run the CWS-7 mutant self-test to verify the harness proof holds:
Self-test green (23 lines = 22 mutants + no-op). Now check how release eligibility handles a main SHA with no run (docs-only merge skips push runs):
`base64` portability question in the changelog-sync push — let me see the full push step and its guard:
The macOS `base64` doesn't wrap; GNU's does — checking the remaining diff file and where base64 runs (ubuntu runner):
## Verdict: BLOCK

Fitness lens, branch tip `cd990fe0`. Checked: full range diff; both new gates executed on the tree (`ci_workflow_security_test.sh` plain + `--self-test` green, 22 mutants + no-op rejected with pinned messages); zizmor installed via the repo's own pinned installer (darwin/arm64 digest verified by use) and its pinned invocation run — exit 0, 27 ignored/25 suppressed, matches evidence; actionlint, shellcheck, yq re-runs; spec CWS-1..CWS-7 traced to code. Characteristic moved: CI supply-chain determinism (pinned, offline, checksum-failed-closed) — direction up; gate coverage up (4 orphan guards moved into required `lint`). The mutant harness is the new fitness function and it holds — except against GitHub's own parser (finding 1), which today's measurement proves no function guards.

## Findings

- [CRITICAL] Duplicate step with `name` only and no `run`/`uses` makes `ci.yaml` invalid for GitHub Actions — `.github/workflows/ci.yaml:446`
  Failure: on the next push/PR, GitHub rejects the workflow at parse time ("Every step must have a run or an uses key"); no run starts, every required context never reports, and the merge gate is bricked for all PRs. Measured: `actionlint` errors at 446:9; both new gates pass the same tree (meta-test no-op control green; zizmor exit 0) — the gate cannot see its own host is broken. Introduced in 5b83c471.
  Fix: delete the stray line 446; add the missing ratchet — one step running actionlint over `.github/**` in `workflow-security` (tool already used locally; pin it like `install-zizmor.sh` or via mise) so parse-validity becomes a required verdict.
  Confidence: 92
- [WARNING] changelog-sync push will fail on ubuntu-latest: GNU `base64` wraps at 76 chars, splitting the `AUTHORIZATION` extraheader across lines — `.github/workflows/changelog-sync.yaml:136`
  Failure: App-token base64 (~88 chars for a typical token) contains an embedded newline in `GIT_CONFIG_VALUE_0`; git sends a malformed header, `git push` fails, the step degrades to a `::warning` and the job stays green — CHANGELOG.md drifts forever with no red anywhere. Local evidence is macOS (`base64` unwrapped there); the branch runs only on ubuntu.
  Fix: `| base64 | tr -d '\n'` in `GIT_CONFIG_VALUE_0`.
  Confidence: 80
- [NOTE] Evidence undercounts the mutant harness: 19 mutants claimed, 22 exist (`evidence.md:11` vs 22 `*_mutant_rejected` calls in `ci_workflow_security_test.sh`; tasks.md CWS-1 row says 8, there are 7) — `openspec/changes/ci-workflow-hardening/evidence/evidence.md:11`
  Failure: the next maintainer trusts a wrong baseline for the harness size and won't notice mutants silently dropped.
  Fix: correct the two counts.
  Confidence: 100
- [NOTE] Spec scenario CWS-1 "New unsafe pattern" (template-injection fixture) and CWS-3 "PR adds a vulnerable dependency" have no executed probe — deferred post-merge per tasks 4.2/verification table — `openspec/changes/ci-workflow-hardening/tasks.md:23`
  Failure: none yet; the detection ability of the gates (not their wiring) is unproven until then.
  Fix: keep the deferrals open in 6.1 as stated.
  Confidence: 90
- [NOTE] Allow-list widening flagged per lens: `WORKFLOW_KEY_ALLOWLIST` gains `concurrency` — `hack/test/dist_ci_wiring_test.sh:105`
  Failure: none found; the widening carries an in-file justification and is locked both directions by the CWS-4 exact-expression mutants, so the register did not silently shrink.
  Fix: none (this is the correct way to widen; recorded so the next widening is held to this standard).
  Confidence: 100

Spec compliance summary: CWS-1 holds (verified end-to-end: installer ran, digest matched real release, audit green at the exact pinned invocation); CWS-2 holds stronger (zero suppressions, bare-suppression mutant red); CWS-3 holds; CWS-4 holds exact expressions in both workflows; CWS-5 holds; CWS-6 holds (orphans wired); CWS-7 holds (self-test reproduced green). Beyond-spec work: release.yaml `cache: false` (cache-poisoning), kind-e2e-setup input→env binding (template injection), changelog-sync app-token `permission-contents` scoping — all tighten, none over-reach a caller.

## Could not check
- GitHub's own parser/branch-protection behaviour (ruleset required-check list, task 5.1) — needs repo settings API access, not in this tree.
- The three other platform digests in `hack/install-zizmor.sh` (only darwin/aarch64 exercised); GNU `base64` wrapping observed on real ubuntu runners (claimed from coreutils docs, not run).
- `dependency-review-action` licence/unknown semantics and `go-licenses` counts in the ci.yaml comment — not re-derived.
- Post-merge-only spec scenarios (live PR supersede, three-quick-merges, skipped-required-job probe) — recorded as open in tasks 5.x/6.1; not falsifiable from this tree.
