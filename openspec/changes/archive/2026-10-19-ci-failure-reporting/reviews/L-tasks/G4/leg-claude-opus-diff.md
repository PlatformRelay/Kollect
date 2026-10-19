## Verdict: CONCERNS

## Findings
- [WARNING] The wiring test pins `on.workflow_run`, but it never checks that `on:` has no other trigger keys. The zizmor exception applies to the whole file. — `hack/test/ci_failure_report_test.sh:1033-1065`, `.github/zizmor.yml:18-21`
  Failure: someone adds `pull_request_target:` (or `workflow_dispatch:`) under `on:` in `ci-failure-report.yaml`. Every wiring check stays green, and the `dangerous-triggers` finding zizmor would raise is silenced by the file-name ignore. The pinned job `if:` would currently skip such runs, because `github.event.workflow_run` is null for them. So the damage is limited today, but the only thing preventing it is the `if:` pin, not the gate that is supposed to catch it.
  Fix: add one check, `wq '."on" | keys'` == `["workflow_run"]`, and a matching mutant in 5.1.
  Confidence: 85
- [NOTE] Values that enter through `env:` are not covered. The injection check only scans `run:` bodies for `${{ … github.event … }}`. — `hack/test/ci_failure_report_test.sh:1131`
  Failure: a later step with `env: SHA: ${{ github.event.workflow_run.head_sha }}` and `run: git checkout "$SHA"` passes every wiring check, including the checkout check, which only looks at `actions/checkout` `with:`. The schedule and same-repo guard means that SHA is a commit already in this repo, so this is not exploitable today. It is a gap in what the test claims to cover ("no checkout of the triggering run").
  Fix: either assert the step list is exactly checkout, label, report (three steps), or forbid any `github.event` in step `env:` and `with:` values.
  Confidence: 70
- [NOTE] The header comment in `.github/zizmor.yml` now names two changes (`ci-workflow-hardening` at line 1, `ci-failure-reporting` at line 12). That makes "the change's tasks.md" on line 13 ambiguous, and the breakdown it refers to lives in the hardening change. — `.github/zizmor.yml:12-14`
  Failure: a reader looks for the threshold breakdown in the wrong change's tasks.md.
  Fix: write "ci-workflow-hardening's tasks.md".
  Confidence: 75
- [NOTE] The label step runs `gh label create --force` on every guarded run, including successes where nothing needs reporting. Any manual edit to the label's colour or description is reverted each night. — `.github/workflows/ci-failure-report.yaml:55-59`
  Failure: none functionally. This is a cosmetic or ownership surprise, and the step comment says "a repeat change nothing", which holds only if nobody edited the label.
  Fix: no change needed. Optionally reword the comment to say the workflow owns the label's values.
  Confidence: 60

What I checked and found correct:
- **Guards:** The workflow's guards agree with the G3 reporter's own CFR-6 re-check (`hack/ci/ci-failure-report.sh:59-64`), which is defence in depth. The reporter derives `GH_REPO` from `GITHUB_REPOSITORY` (`:57`), so the reporter step needs no `GH_REPO`.
- **Permissions:** contents/actions read and issues write are the minimum that `gh run list/view --log-failed` and issue/label writes need.
- **Checkout:** it uses the same SHA pin as every other workflow, keeps no credentials, and sets no `ref:`.
- **Concurrency:** grouping per `workflow_id` with `cancel-in-progress: false` is right. The comment's explanation of why a dropped pending event is harmless matches the reporter's "latest completed scheduled run" logic (`:75`).
- **Workflow names:** both listed names match exactly one `name:` each (`e2e-nightly.yaml:2`, `go-patch-lag.yaml:6`).
- **Script injection:** no `run:` step interpolates event data.
- **CWS-2 self-test anchors:** I traced the new `s/^rules:[^\n]*\n/…/m` anchors against the awk walk (`ci_workflow_security_test.sh:256-301`). The inserted rule comes before the existing commented `dangerous-triggers` block, so each mutant still reds on its own message and the control's existing comments stay attached by line adjacency. This is not a weakened check. The flow-style mutant now replaces the whole block, which is fine on a throwaway copy.
- **Header comment:** it is accurate apart from the ambiguous reference above.
- **Evidence:** the G4.md matrix rows 1–12 match the assertions actually present in the diff.

## Could not check
- I did not run any sensor (the wiring test, zizmor 1.30.1, the CWS self-test, the mutant runner). I rely on the results recorded in `evidence/G4.md`, including the counts "38 ignored, 28 suppressed". I could not confirm independently that zizmor's basename `ignore:` selector matches only this file.
- Runtime behaviour on GitHub (row 21): whether `workflow_run` fires for both names and whether `issues: write` is enough for `gh label create --force` on this repo's settings.
- `tasks.md` 4.1–4.2 full text and `loop.md`: I relied on the quote in G4.md.
- The untracked `reviews/L-tasks/G4/` records: deliberately not read (other reviewers).
