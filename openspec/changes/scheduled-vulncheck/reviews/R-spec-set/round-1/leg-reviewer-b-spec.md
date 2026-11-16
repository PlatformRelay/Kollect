## Verdict: CONCERNS

## Findings
- [WARNING] The tasks never name CWS-6's hard-coded guard list, so deleting both new `lint` steps would go unnoticed — `hack/test/ci_workflow_security_test.sh:617-623`
  Failure: the generic CWS-6 loop only checks guards CI already invokes (comment at `:605`). If both `vulncheck_schedule_test.sh` steps are deleted from ci.yaml, nothing reds. The named `cfc_guards` array exists to catch exactly this, and every sibling guard (go_patch_lag, mutation_signal, ci_failure_report) is in it. V-5 says "If CWS keeps a registry". It does, so the condition is already settled, but the task leaves the edit to T0.1's discovery.
  Fix: make V-5/1.2 explicit: add the script to `cfc_guards` and to the header list at `:27-38`, and add a CWS-7 "guard unwired" mutant modelled on `:1164-1167`.
  Confidence: 85
- [WARNING] The `ci-failure` issue for a red scan will not name the advisory — `hack/ci/ci-failure-issue-body.sh:132,142`
  Failure: the body builder excerpts only log lines containing `FAIL`. govulncheck's report prints `Vulnerability #1: GO-…` and has no such line, so the issue says "_No line containing FAIL…_". The proposal makes this issue the SLA "first reported" moment (proposal.md:53, tasks 1.3), yet the issue gives no GO-ID, module or fixed version. Meanwhile SVS-3/V-2 ("a second `run` step" fails) rules out adding a step-summary excerpt.
  Fix: choose one and record it in the spec. (a) Accept it: SVS-2 states the issue links the run and the advisory is read from the log. (b) Let the guard allow one `if: failure()` summary step. Option (b) conflicts with SVS-1's "no `if:`", so the decision has to be explicit.
  Confidence: 85
- [WARNING] No test checks that the schedule is daily — `specs/scheduled-vulnerability-scan/spec.md:7-8`, `tasks.md:53`
  Failure: SVS-1 says "daily `schedule` (one cron)". V-2 tests only for a second cron, so `"0 6 * * 1"` (weekly) or `"0 6 1 * *"` passes the guard and quietly drops coverage back to go-patch-lag's cadence.
  Fix: add a V-2 assertion and mutant that the cron's day-of-month, month and day-of-week fields are all `*`.
  Confidence: 75
- [WARNING] The V-2 "unpinned `uses:`" check, as worded, contradicts the step that 1.2 requires — `tasks.md:58,73`
  Failure: `./.github/actions/go-cache` has no `@ref`, so a literal "ref that is not a 40-hex sha" check reds the real workflow. The obvious workaround, exempting `./*`, accepts any local composite action.
  Fix: copy mutation_signal_test.sh W1's exact step-shape check (`hack/test/mutation_signal_test.sh:721-750`): checkout@sha, then exactly `./.github/actions/go-cache`, then setup-task@sha with only `version:`. Also add its "sha used by another workflow" check (`:742-750`) so the "same sha as other workflows" part of 1.2 is tested rather than just stated.
  Confidence: 70
- [NOTE] "`secrets.` anywhere in the file" can trip on the file's own comments and misses `github.token` — `tasks.md:59`, `spec.md:9`
  Failure: a header comment such as "reads no secrets." fails the guard. `${{ github.token }}` in an `env:` passes it, although SVS-1 says "no secrets".
  Fix: match `${{ … secrets. … }}` and `github.token` inside expressions only, ignoring comments.
  Confidence: 55
- [NOTE] SVS-1 does not say whether to set `concurrency:` — `spec.md:7-10`
  Failure: go-patch-lag sets a concurrency group (`go-patch-lag.yaml:16-18`), mutation does not. Without one, a dispatch that overlaps the cron runs twice, which is harmless. If one is added with `cancel-in-progress: true`, a scheduled run cancelled after its job started counts as a failure under CFR-1.3 and opens a spurious issue.
  Fix: say either "no concurrency" or "`cancel-in-progress: false`", and have the guard check it.
  Confidence: 50
- [NOTE] Failing closed on an unreachable vuln DB will open an issue on a transient vuln.go.dev or proxy outage — `spec.md:17-20`
  The next green day closes it (CFR-1.6). This is the right trade-off, but name it in the Impact section so a one-day issue is not read as an advisory.
  Confidence: 60
- [NOTE] Premises checked and holding:
  - the vulncheck trigger (`ci.yaml:273-287`);
  - the task command and pin (`Taskfile.yml:31,206-210`);
  - the reporter list and its schedule-only guard (`ci-failure-report.yaml:23-36`);
  - the go-patch-lag cron (`go-patch-lag.yaml:10`);
  - no `vulnerabilityAlerts`/`osv` key in `renovate.json`.

  The rename/unlist and continue-on-error scenarios are already enforced generically (`ci_failure_report_test.sh:1572-1594`). setup-task's version is compared across all sites, not counted, so a new site at 3.54.0 is safe. Scope is right: adding `schedule:` to ci.yaml would run the whole suite, and folding the scan into mutation.yaml would mix two report-only signals under one issue. No private hostnames, company names or agent references in the spec set.

## Could not check
- The Renovate docs claims about `osvVulnerabilityAlerts` and indirect deps, the Dependabot alert list, and recent run times/conclusions all need network or `gh` access, which I don't have.
- Whether zizmor's offline audit flags the new workflow (I did not run it). I also did not run any guard to confirm a red-first result.
- How the existing `vulncheck` job and other jobs share the `go-cache` key without a suffix. I only read `action.yml:28`.
