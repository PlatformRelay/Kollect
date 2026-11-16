The daily standalone scan is a reasonable fix, but the tasks contain one direct contradiction and several gaps in verification and wording.

1. **CRITICAL — The SHA-only guard rejects the required local action.**  
   Evidence: `openspec/changes/scheduled-vulncheck/tasks.md:58` requires rejecting any `uses:` ref that is not a 40-hex SHA; `tasks.md:73` explicitly requires `uses: ./.github/actions/go-cache`. That local action cannot satisfy the stated rule, so implementing both instructions literally prevents the unmutated control from passing.  
   **Fix:** Require SHA pins for external actions, explicitly allow the exact local action path, and reject other local action paths. Clarify that the prohibition on `if:` applies to the workflow’s own steps: the required composite contains an optional envtest condition at `.github/actions/go-cache/action.yml:33`.

2. **WARNING — The mutation cases do not establish the central daily-schedule requirement.**  
   Evidence: `tasks.md:53` tests additional triggers and a second cron, but not removal of either required trigger or replacement of the daily cron with a weekly one. SVS-1 requires both triggers and daily execution (`specs/scheduled-vulnerability-scan/spec.md:7`). The existing mutation guard explicitly tests a daily-to-weekly cron mutation (`hack/test/mutation_signal_test.sh`, W1 self-tests).  
   **Fix:** Add separate mutants for missing `schedule`, missing `workflow_dispatch`, and a non-daily cron. Assert a concrete cron or a precisely defined once-per-day shape. Also reject nonpositive or invalid timeout values, rather than checking only presence.

3. **WARNING — The justification for excluding indirect Renovate updates is incorrect.**  
   Evidence: `proposal.md:57` says enabling indirect gomod updates would not have produced fix PRs. Disabled-by-default behavior does not establish that counterfactual: [Renovate’s gomod documentation](https://docs.renovatebot.com/modules/manager/gomod/#post-update-options) explicitly provides a rule enabling indirect updates. The repository already enables Go directive bumps at `renovate.json:172`. The narrower claim about OSV alerts covering direct Go dependencies is supported by [Renovate’s configuration documentation](https://docs.renovatebot.com/configuration-options/#osvvulnerabilityalerts).  
   **Fix:** Keep update automation out of scope because this change addresses independent detection, while acknowledging that indirect updates could supply fixes. Remove the Dependabot-only condition for revisiting that option.

4. **WARNING — The proposal overstates how existing PR checks become red.**  
   Evidence: `proposal.md:8` says a new advisory turns every open PR red “at once.” `.github/workflows/ci.yaml:34` declares push and pull-request triggers, with no database-update trigger; the scanner runs only when CI executes (`ci.yaml:273`). An already-completed green check does not rerun merely because the vulnerability database changes.  
   **Fix:** Say that subsequent or rerun code-affecting checks fail across affected branches. Preserve the historical “9 of 10” claim only with run evidence. The incident counts, Dependabot history, and last successful Go-patch-lag run remain evidence to collect in T0.1; repository configuration alone cannot substantiate them.

5. **WARNING — The listed security guard is not the security audit.**  
   Evidence: `tasks.md:20` labels `ci_workflow_security_test.sh` as covering pinned actions and zizmor, but that guard checks the audit’s configuration and wiring. The actual audit is the command at `.github/workflows/ci.yaml:196`, also run by `hack/check.sh:64`. The implementation introduces a composite containing shared Go caches (`.github/actions/go-cache/action.yml:22`).  
   **Fix:** Explicitly require the pinned offline zizmor invocation, shell lint, and `consistency_toolchain_pins_test.sh` after implementation—or explicitly require `task check` in the tasks. `loop.md` already promises that full gate, but the task acceptance criteria should stand alone. Do not add a cache suppression preemptively: a read-only scheduled scanner is not inherently a release-cache poisoning finding.

6. **WARNING — Post-merge acceptance misclassifies failures and never observes scheduled reporting.**  
   Evidence: `tasks.md:95` says a red run is a real advisory, while SVS-1 explicitly includes database and build failures (`spec.md:19`). The prescribed manual dispatch cannot exercise issue reporting because the reporter accepts only scheduled runs (`.github/workflows/ci-failure-report.yaml:37`).  
   **Fix:** Require inspecting and classifying any failure before reporting it as a vulnerability. Record the manual smoke run separately, then observe the first scheduled run and its reporter outcome before declaring operational verification complete. A successful manual run must not be expected to close an existing issue.

7. **WARNING — SLA wording could start or reset a vulnerability clock on an infrastructure failure.**  
   Evidence: `tasks.md:85` says a scheduled scan’s `ci-failure` issue starts the clock. Such issues also represent database, build, or runner failures. Existing policy starts the clock at the earliest reported finding (`docs/security/sca-remediation-policy.md:30`).  
   **Fix:** State that a vulnerability first reported by the scheduled scan starts its clock at that report, unless reported earlier elsewhere. Infrastructure-only issues do not start a vulnerability-remediation SLA.

8. **NOTE — Run state contains local identity and review-harness references.**  
   Evidence: `openspec/changes/scheduled-vulncheck/loop.md:3` contains an absolute home-directory path and “two-model duo”; line 5 references `agent-context`. I found no internal hostnames, private company names, or agent/model references in the three normative review files.  
   **Fix:** Keep this run state local, or remove the personal path and harness metadata before publishing it.

The standalone scope is sound: adding `schedule:` to the current CI workflow would activate substantially more work because non-PR events classify as code changes (`ci.yaml:62`). An existing nightly workflow could host the scan, but would combine its issue and recovery state with unrelated failures. Renovate post-upgrade scanning would remain conditional on update activity. Image scanning and auto-fixing can reasonably remain separate.

CFR-6 and CFR-1.8 are otherwise respected. Reporter concurrency already serializes issue updates; scan concurrency is optional here, and any added group should separate scheduled and manual runs. T0.1 correctly calls out registry discovery: specifically extend CWS’s `cfc_guards` at `hack/test/ci_workflow_security_test.sh:617`. Toolchain pin sites are discovered dynamically; I found no fixed setup-task count requiring adjustment.

This was a read-only review; I did not execute mutation suites or validate live incident history.

## Verdict

REQUEST_CHANGES
