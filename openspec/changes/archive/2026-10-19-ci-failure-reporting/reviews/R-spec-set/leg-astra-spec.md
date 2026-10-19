Verdict: NO-GO

### CFR-R1 · CRITICAL · Ordering state disappears when the issue closes
Where: openspec/changes/ci-failure-reporting/tasks.md:70; openspec/changes/ci-failure-reporting/specs/ci-failure-reporting/spec.md:66  
Evidence: Task 3.1 compares an event against “the key recorded on the open issue.” Neither the spec nor tasks defines the key’s storage format, update operation, or retrieval after closure.  
Why it matters: Process failure N, recovery N+1, then a delayed failure N. An open-issue-only lookup finds nothing and creates another issue, violating CFR-3’s stale-event SHALL. A sequence of increasing events does not expose this.  
Fix: Define a versioned machine-readable record containing repository, workflow identity, source run ID, run number, attempt and processing state. Specify where it is written and how the latest trusted record is retrieved across open and closed issues. Add tests for stale failures after closure, stale recoveries after a newer failure comment, and a delayed failure following a green run with no issue. Preserve the green-no-issue rule by explicitly defining how authoritative run history supplies ordering when no issue record exists.

### CFR-R2 · CRITICAL · Concurrency can discard the only recovery or failure report
Where: openspec/changes/ci-failure-reporting/proposal.md:98; openspec/changes/ci-failure-reporting/tasks.md:95; openspec/changes/ci-failure-reporting/tasks.md:142  
Evidence: The proposal acknowledges pending replacement but says “the ordering key lets the next report correct the state.” No task retrieves events whose reporters were cancelled.  
Why it matters: While failure N is being reported, recovery N+1 can wait and then be replaced by a late delivery of N−1. The stale-event check rejects N−1, leaving the issue open despite recovery. Alternatively, a pending failure can disappear entirely. Serialization prevents overlapping writers; it does not recover discarded events. GitHub documents pending replacement and does not guarantee source-event ordering. [GitHub concurrency documentation](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency)  
Fix: Add a reconciliation task that, inside the concurrency group, retrieves and processes unrecorded eligible completed runs/attempts in ordering-key order. Add replacement-sequence fixtures covering a lost recovery, a lost failure, and a stale event replacing a newer pending event. Specify pagination and the history boundary. Update the verification row to name these tests instead of treating a concurrency-group assertion as sufficient proof.

### CFR-R3 · WARNING · Reporter retries and partial writes have no defined behavior
Where: openspec/changes/ci-failure-reporting/tasks.md:52; openspec/changes/ci-failure-reporting/tasks.md:77; openspec/changes/ci-failure-reporting/specs/ci-failure-reporting/spec.md:51  
Evidence: Tests cover older keys and a newer source-run attempt, but not an equal key or a rerun of the reporter. Only lookup failure is injected.  
Why it matters: Rerunning a successful reporter can duplicate a failure comment. If recovery commenting succeeds but closing fails, recording the recovery key first can cause the retry to skip the unfinished close. An issue-create response lost after server-side success can also produce duplicates if the retry does not reconcile existing state.  
Fix: Define equal-key handling and resumable processing stages, distinguishing the triggering run’s attempt from the reporter’s attempt. Add tests for identical-event replay and failures after create, comment, state update and close. Require retries to complete unfinished actions without duplicate issues or comments.

### CFR-R4 · WARNING · Hostile text tests cover shell evaluation but not Markdown or argument injection
Where: openspec/changes/ci-failure-reporting/tasks.md:42; openspec/changes/ci-failure-reporting/tasks.md:101; openspec/changes/ci-failure-reporting/specs/ci-failure-reporting/spec.md:107  
Evidence: The hostile fixture contains only command substitution and backticks; event interpolation is checked only at the workflow `run:` boundary.  
Why it matters: Shell-inert logs can still inject mentions, misleading links, headings or forged state markers into a bot-authored issue. Job and step names receive no hostile-input coverage. A fake `gh` that records a flattened command string cannot prove argument boundaries. Passing values through `env:` does not establish safe use inside the scripts.  
Fix: Extend CFR-5 and tasks 2.1/3.1 to require inert rendering of all external names and log excerpts, including fence termination, HTML, mentions, links, newlines and reporter-metadata lookalikes. Keep state metadata separate from rendered excerpts. Require fixed `gh` options, quoted arguments, body files or stdin, explicit repository selection, and validated numeric identifiers. Record fake-`gh` arguments without losing boundaries and assert that hostile values cannot alter the operation or destination.

### CFR-R5 · WARNING · Workflow identity and foreign bot issues are not exercised
Where: openspec/changes/ci-failure-reporting/specs/ci-failure-reporting/spec.md:97; openspec/changes/ci-failure-reporting/tasks.md:74; openspec/changes/ci-failure-reporting/tasks.md:118  
Evidence: CFR-4 requires label, bot authorship and “the workflow’s key.” The only spoofing fixture is user-authored, and the mutant removes label/author filtering.  
Why it matters: An implementation matching only label plus bot author passes those fixtures while mixing the two workflows. A green nightly could close the Go patch lag issue. The workflow key and bot identity themselves are undefined.  
Fix: Define the key using repository identity and an allowlisted workflow ID or canonical path, and define the expected bot author independently of the triggering actor. Add simultaneous issues for both workflows, an unrelated bot issue, renamed titles, forged metadata in user comments, and a workflow-key-filter removal mutant. Specify behavior when multiple matching open issues already exist.

### CFR-R6 · WARNING · The planned permissions assertion removes checkout’s read grant
Where: openspec/changes/ci-failure-reporting/tasks.md:96; openspec/changes/ci-failure-reporting/proposal.md:44  
Evidence: Task 4.1 requires job permissions `issues: write` and `actions: read`, “nothing else,” while the proposal describes workflow-default `contents: read`.  
Why it matters: Job permissions do not add those two grants to the workflow map: unspecified job permissions become `none`. The reporter therefore lacks the intended authenticated contents access. Public-repository checkout behavior can conceal this mistake. [GitHub permissions documentation](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#jobsjob_idpermissions)  
Fix: Require exactly `contents: read`, `actions: read`, and `issues: write` at the reporter job. Keep workflow-level `contents: read`. Add assertions for `GH_TOKEN` wiring and explicit repository targeting. Do not treat `sonar_ko_08_workflow_permissions_test.sh` as coverage for this workflow: it checks only `changelog-sync.yaml`.

### CFR-R7 · WARNING · Suppression wording does not specify both comments CWS-2 requires
Where: openspec/changes/ci-failure-reporting/tasks.md:104; hack/test/ci_workflow_security_test.sh:266; hack/test/ci_workflow_security_test.sh:279  
Evidence: Task 4.2 asks for a comment “directly above it.” CWS-2 separately checks the rule key and every block-list suppression entry; encountering bare `ignore:` clears the preceding-comment state.  
Why it matters: A comment above `dangerous-triggers:` alone followed by an uncommented ignore entry fails the required gate. The task’s green-gate requirement catches this eventually, but the described shape is ambiguous.  
Fix: Include the exact shape in task 4.2:

```yaml
rules:
  # dangerous-triggers: default-branch code only; same-repo schedule guard; logs remain data.
  dangerous-triggers:
    ignore:
      # dangerous-triggers: exception limited to the guarded CI reporter.
      - ci-failure-report.yaml
```

Validate the filename selector with the pinned zizmor invocation. Add separate mutants removing the rule comment and the entry comment. Retain the ban on inline ignores.

### CFR-R8 · CRITICAL · The test tasks do not deliver CFR-7’s complete decision-table SHALL
Where: openspec/changes/ci-failure-reporting/specs/ci-failure-reporting/spec.md:149; openspec/changes/ci-failure-reporting/tasks.md:52; openspec/changes/ci-failure-reporting/tasks.md:33  
Evidence: CFR-7 requires “every row of (conclusion x open-issue x ordering).” Task 3.1 supplies examples, without defining the complete table or equal-key behavior. Task 2.1 never explicitly asserts the workflow, run URL, or bounded count of `FAIL` lines.  
Why it matters: The listed tests can pass while required decision rows and body fields are absent. Error propagation for job/log retrieval and issue mutations is also untested despite CFR-6 covering `gh` errors generally.  
Fix: Add an explicit table defining supported conclusions, issue states, and older/equal/newer keys; enumerate expected actions and parameterize task 3.1 from it. Include ignored conclusions and malformed inputs. Add body assertions for workflow, run URL, line-count bound and final size after metadata is included. Inject failure at every `gh` operation, not only lookup.

### CFR-R9 · WARNING · Task 1.1 instructs an agent to perform an operator-owned credentialed mutation
Where: openspec/changes/ci-failure-reporting/tasks.md:26; openspec/changes/ci-failure-reporting/proposal.md:96; openspec/changes/ci-failure-reporting/loop.md:127  
Evidence: Task 1.1 directs two label creations on a fork or throwaway repository. It is not marked as an owner task.  
Why it matters: This needs an account, an authorized repository and write credentials. A successful local `gh` operation also does not establish that a workflow token with only `issues: write` suffices. The review brief explicitly requires such work to be parked.  
Fix: Move this probe into a named operator-only task with an evidence template and explicitly prohibit agent execution. Keep agent work to recorded fixtures, documentation and fake-`gh` checks. If permission proof is required, ask the operator to capture it from a workflow with the exact proposed token grants. Give the post-merge verification rows explicit owner tasks and mark unavailable live evidence as pending.

### CFR-R10 · WARNING · Removing both CI invocations can silently remove the required test
Where: openspec/changes/ci-failure-reporting/tasks.md:129; hack/test/ci_workflow_security_test.sh:585; hack/test/ci_workflow_security_test.sh:602  
Evidence: CWS-6 checks modes of scripts already invoked by CI. CFC-6’s independently required set covers `consistency_*_test.sh`, `go_patch_lag_test.sh`, and `openspec_evidence_markdown_only_test.sh`; it does not include the proposed filename.  
Why it matters: The filename is valid, and plain mode now plus self-test in 5.1 is compatible with current enrollment. But deleting both reporter-test steps removes the script from CWS-6’s discovered set, leaving CFR-7’s CI requirement unprotected.  
Fix: In task 5.1, add an independent required-membership assertion for this script in both modes and a mutant deleting both invocations. Preserve the chosen filename. State that early versions must contain no non-comment `--self-test` token until that mode is implemented and wired, because mode detection is textual.

### CFR-R11 · NOTE · The red-first instructions cannot hold literally for no-op cases
Where: openspec/changes/ci-failure-reporting/tasks.md:3; openspec/changes/ci-failure-reporting/tasks.md:53; openspec/changes/ci-failure-reporting/tasks.md:87  
Evidence: Every test is supposed to record a target-assertion red against a stub that “exits 0 and does nothing,” but several expected outcomes are precisely no writes. Task 4.1 also explicitly uses an absent file despite the opening prohibition on missing-file reds.  
Why it matters: An implementer must either invent evidence or interpret exceptions that the tasks do not state.  
Fix: Require positive cases to fail against the empty stub; prove no-op cases against targeted faulty implementations that perform forbidden writes. Explicitly permit assertion-specific missing-workflow diagnostics in 4.1. Keep the paired test/implementation commits: `hack/check.sh:79` does enroll the new test immediately.

## Stale file:line claims

All explicit numeric file references in the three requested documents were checked. No materially stale reference was found.

| Claim location | Referenced location | Result |
|---|---|---|
| proposal.md:6 | `.github/workflows/e2e-nightly.yaml:6` | Holds: daily cron. |
| proposal.md:8 | `.github/workflows/go-patch-lag.yaml:10` | Holds: Monday cron. |
| proposal.md:9 | `hack/ci/go-patch-lag.sh:110` | Holds: lag branch exits 1. |
| proposal.md:56; spec.md:176 | `hack/test/ci_workflow_security_test.sh:303-308` | Holds: inline-ignore detection and rejection. |
| proposal.md:57; tasks.md:107 | `.github/zizmor.yml:10-14` | Holds as a containing range; precisely, header is lines 10–13, “no suppressions” is line 11, and line 14 is `rules: {}`. |
| proposal.md:71 | `.github/workflows/e2e-nightly.yaml:127-131` | Holds: race job and `continue-on-error: true`. |
| proposal.md:77 | `.github/workflows/e2e-nightly.yaml:220` | Holds: dispatch/input guard. |
| tasks.md:5 | `hack/check.sh:79` | Holds: test-file sweep; Docker-header exclusions are handled afterward. |
| tasks.md:8 | `.github/workflows/ci.yaml:279` | Holds: `lint` job begins here. |
| tasks.md:90 | `.github/workflows/e2e-nightly.yaml:2` | Holds: `name: E2E nightly`. |
| tasks.md:91 | `.github/workflows/go-patch-lag.yaml:6` | Holds: `name: Go patch lag`. |
| tasks.md:110; spec.md:176 | `hack/test/ci_workflow_security_test.sh:236` | Holds: `cws2_suppressions()` begins here. |
