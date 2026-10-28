I've finished reading and am writing the report now.

## Verdict: CONCERNS

## Findings
- [WARNING] A mixed group branch can pick up `rebaseWhen: behind-base-branch` from the digest automerge rule. That contradicts the design's claim that only auto-merge PRs are rebased when behind, and no fitness function can see it — `renovate.json:169`, `openspec/changes/dependency-update-automation/design.md:52`
  Failure: the `go toolchain` group holds the golang digest (from Dockerfile, Dockerfile.pipeline and devcontainer) plus a `go` directive bump. If the golang digest sorts first, the branch config is that upgrade's config (`evidence/2.4.md:124-130`). Branch `automerge` is recomputed to false, but `rebaseWhen` stays `behind-base-branch`. The branch is then force-rebased on every hourly run where `main` has moved. Each rebase re-runs the full required set (test-suite, kind-smoke-run, docker-build) and drops any review state, and the PR never merges. DUA-7 still holds (no wrong merge), and DUA-8 holds as written. So this is a CI-cost defect plus a design statement that is false. It is not a merge-safety defect. The 2.4 runs resolved to `conflicted` only because of the sort order, so nothing pins the behaviour.
  Fix: drop `rebaseWhen` from the automerge rule and set the top-level `rebaseWhen` to `"auto"`. In Renovate, `auto` means behind-base-branch when the branch-level, recomputed `automerge` is true, otherwise conflicted, which is exactly what the design states. The contract changes: T11 key set becomes `{automerge, matchManagers, matchUpdateTypes}`, T12 top level becomes `rebaseWhen="auto"` with no rule-level `rebaseWhen`, and the DUA-8 wording changes the same way. Before adopting it, check that `getBranchForceRebase` does not report strict up-to-date checks under the `protect-main` ruleset. If it does, `auto` rebases every PR when behind. If the contract is kept instead, at least correct `design.md:52` and `renovate.json:165` so they say mixed branches may be rebased.
  Confidence: 70
- [NOTE] Local/CI parity gap: the DUA-5 validator step runs in the required `lint` job but not in `task check`, and the exclusion table does not list it — `hack/check.sh:116-124`, `.github/workflows/ci.yaml:597-598`
  Failure: a contributor adds an unknown option or a migratable setting to `renovate.json`. `task check` passes, because guard T10–T15 checks shape, not schema. The PR then reds in the required `lint` job. `hack/test/task_check_test.sh` tracks jobs, not steps, so it cannot notice the gap.
  Fix: add `exclusion renovate-config-validate "needs a Docker daemon and the ghcr.io image; runs in the lint job"`, or call the script from `check.sh` when docker is present.
  Confidence: 85
- [NOTE] The required `lint` job now depends on pulling the Renovate image from ghcr.io on every PR, Renovate-related or not (T8 forbids an `if:`) — `hack/ci/renovate-config-validate.sh:39-41`
  Failure: a ghcr.io outage or rate limit reds `lint`, which is a release-eligibility check, on unrelated PRs. Every lint run also pays for a large image pull. This is a new external dependency in a merge gate, and nothing measures it.
  Fix: accept it and record it in `decisions.md`, or cache the image by digest (the digest is pinned, so a cache key on it is exact).
  Confidence: 60
- [NOTE] Built-in presets are resolved inside the pinned image, and no guard reads them: T10/T11 inspect only literal config — `hack/test/renovate_policy_test.sh:548`, `:572`
  Failure: a future Renovate whose `config:recommended` or a nested `group:`/`workarounds:` preset sets `automerge` widens the scope with no diff to `renovate.json`. The weekly, never-auto-merged, human-reviewed image PR (T14) is the only control.
  Fix (ratchet): have the image-bump review run `renovate-config-validator` plus a resolved-config dump (or the 2.4 dry run) and assert there is exactly one automerge source. Or add one line to the maintainer checklist in `tooling-setup.md`.
  Confidence: 55
- [NOTE] Out of scope, but worth an INBOX item: `CONTRIBUTING.md:139` allows "Squash and merge", while the brief says the ruleset is rebase-only and the workspace merge policy forbids squash. This branch does not touch that file.
  Confidence: 80

**Fitness summary (checked):**
- **Merge authority:** widened by design (settled). A bypass-actor token now merges digest-only Docker image PRs. It is held by T9/T11/T12/T13 (`renovate_policy_test.sh:451-668`) and by the pre-existing `ci_docs_gate_test.sh:248-266`, which keeps every required workflow's `pull_request` trigger unfiltered. That second guard matters because Renovate merges on the checks that are *reported*.
- **Supply-chain pinning:** improved. Floating fallback → digest-pinned `renovate-image` (T6) plus a digest-only validator image.
- **Token scope:** improved. Job token is `contents: write` → `read` (T3), the `github.token` fallback is removed (T1), and the token check fails closed (T2, run under the runner's real shell semantics).
- **CI gate set:** grown. Three lint steps were added, and CWS-6 enrolment grew (`ci_workflow_security_test.sh:623`, with its mutant at `:1172`).
- **Allow-lists, exclusions and baselines:** none widened. The `sonar_ko_08` residual shrank (`sonar_ko_08_workflow_permissions_test.sh:14-17`). The gitleaks, zizmor and coverage files are unchanged in the diff.
- **Determinism:** the `--self-test` uses a fixture that is independent of the real pins, with a bot-bump control.

## Could not check
- I ran nothing; I relied on the brief's account of the holistic `task check` (4 environment reds, all green after fixing the environment).
- I could not read Renovate's `generate.js` or `reuse` source, so the claims about the first-upgrade spread and how `auto` behaves are from memory and the branch's evidence.
- The live `protect-main` ruleset (rebase-only, strict up-to-date) is not in the repository.
- I read only the opening of the guard plus its T11/T12 lines and mutant list; I did not read the T14–T17 bodies, `tooling-setup.md`, or the round-2 review files, which belong to the other lenses.
