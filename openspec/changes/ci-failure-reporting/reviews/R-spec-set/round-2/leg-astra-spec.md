Verdict: GO WITH FIXES

## Round-1 closure

Paths below are relative to `openspec/changes/ci-failure-reporting/` unless stated otherwise.

| ID | Status | Evidence |
| --- | --- | --- |
| R1 | CLOSED | `specs/ci-failure-reporting/spec.md:22` defines persistent ordering through the newest open or closed bot issue. `tasks.md:92` tests the closed-issue stale failure. |
| R2 | CLOSED | `specs/ci-failure-reporting/spec.md:12` requires reconciliation to the latest completed scheduled run. `tasks.md:122` tests disagreement between payload and listed run. |
| R3 | CLOSED | `specs/ci-failure-reporting/spec.md:105` defines equal-key suppression and the recovery-close exception. `tasks.md:133` tests retry after a failed close. |
| R4 | CLOSED | `specs/ci-failure-reporting/spec.md:151` restricts marker parsing to trusted first lines; `:174` requires protective fences and mention neutralisation. `tasks.md:67` and `:116` test log injection. See F3 for an additional coverage gap. |
| R5 | CLOSED | `specs/ci-failure-reporting/spec.md:147` defines the workflow key as `workflow_run.path`. `tasks.md:118` tests both workflows; `:190` adds the filter mutant. |
| R6 | CLOSED | `tasks.md:154` requires exactly the three agreed job permissions. `proposal.md:50` agrees. The unrelated Sonar-test coverage claim is gone. |
| R7 | CLOSED | `tasks.md:167` supplies the required suppression structure and both comments; `:177` prohibits the inline-ignore literal. `:199` tests removal of each comment. This matches repository-root `hack/test/ci_workflow_security_test.sh:267` and `:293`. |
| R8 | CLOSED | `specs/ci-failure-reporting/spec.md:26` supplies the decision table. `tasks.md:80` requires parameterisation from it; `:54`–`:60` assert body fields and bounds; `:130` injects errors across reporter calls. |
| R9 | CLOSED | `tasks.md:126` requires the fake to reject `run view` calls without the selected attempt and exercises attempt 2. |
| R10 | CLOSED | `tasks.md:39` requires the real bot-author spelling probe; `:120` binds the fake to its result, requires all issue states and prohibits search. This closes the planned fix; the probe remains unexecuted. |
| R11 | CLOSED | `tasks.md:44` replaces the write probe with documentation and post-merge proof. `proposal.md:109` records the source; `tasks.md:227` retains the live verification. |
| R12 | CLOSED | `tasks.md:12` prevents premature textual mode detection. `:202`–`:207` require self-wiring, its deletion mutant and both CI modes. |
| R13 | PARTIAL | `tasks.md:5` fixes no-write reds and `:7` permits missing-workflow diagnostics. However, `task-prompt.md:53` still expressly forbids a missing-file red. See F2. |
| R14 | CLOSED | `tasks.md:135` requires issue creation with job names and “logs unavailable”, followed by non-zero exit on fetch failure. |
| R15 | CLOSED | `tasks.md:151` pins the prefixed workflow-ID concurrency group exactly; `specs/ci-failure-reporting/spec.md:78` agrees. |

## New findings

### F1 · CRITICAL · The write assertion rejects the required close operation

Where: `tasks.md:128`

Evidence: Task 3.2 says **every write** goes through `--body-file`. Recovery explicitly requires a separate comment followed by close (`tasks.md:95`), including a close-only retry (`tasks.md:97`). The installed CLI’s read-only `gh issue close --help` lists no `--body-file` option. CFR-6 instead requires **bodies** to use `--body-file` (`specs/ci-failure-reporting/spec.md:209`).

Why it matters: A literal implementation of the task assertion rejects a valid close; a fake accepting `issue close --body-file` would conceal a real CLI failure. Either prevents the required CFR-3 recovery close.

Fix: Require `--body-file` on body-bearing create/comment calls. Explicitly permit bodyless `issue close <validated-number>` and make the fake reject unsupported close flags.

### F2 · WARNING · The dispatch instruction still prohibits G4’s authorised red

Where: `task-prompt.md:53`

Evidence: The dispatch-group line requires a red “not on a missing file or usage error”. Task 4.1 explicitly requires red on the absent workflow, with individual missing-assertion diagnostics (`tasks.md:142`); the preamble expressly permits this (`tasks.md:7`).

Why it matters: G4 cannot satisfy both instructions. Its intended red evidence can be rejected under the dispatch instruction even when it satisfies tasks.md.

Fix: Add the same explicit 4.1 exception to the dispatch-group line. The group membership itself is correct.

### F3 · WARNING · Two specified behaviours lack discriminating tests

Where: `tasks.md:67` and `tasks.md:135`

Evidence:

- CFR-5 requires neutralising `@` in run-supplied text **outside** fences (`specs/ci-failure-reporting/spec.md:175`). The hostile fixture puts `@user` inside an excerpt whose fence must contain it. No named case puts a mention in a rendered workflow, job or step name.
- CFR-6’s log-fetch scenario covers both an error and a successful call returning nothing (`specs/ci-failure-reporting/spec.md:224`). Task 3.2 tests only fetch failure. The body-builder’s absent-log case (`tasks.md:71`) does not prove the reporter exits non-zero for successful-but-empty output.

Why it matters: A builder that never neutralises unfenced mentions can pass the specified hostile-log test. A reporter that treats empty successful log output as normal can pass both the fetch-error and body-builder tests. Executing the named tests therefore does not prove the full CFR-5 SHALL or CFR-6’s specified empty-log behaviour.

Fix: Add an unfenced run-supplied name containing `@user`, asserting neutralisation while retaining the name. Add a fake log fetch returning exit 0 and empty stdout, asserting the issue/comment contains job names and “logs unavailable”, and the reporter exits non-zero.

## Could not check

- Live GitHub assumptions assigned to task 1.1: payload shape, run-list ordering after reruns, bot-author filtering and recorded run data.
- Actual implementation, mutation results and zizmor execution against the proposed workflow: those files do not exist yet.
- Post-merge label permissions and real scheduled failure/recovery behaviour.

The inspected CWS-2, CWS-6, CFC-6 and `hack/check.sh` references match the current tree. No additional gate conflict or stale cited location was found.
