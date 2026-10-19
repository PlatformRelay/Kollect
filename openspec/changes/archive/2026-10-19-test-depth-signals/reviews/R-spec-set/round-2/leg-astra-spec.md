## Verdict

**BLOCK** — The main round-1 corrections are present, but the revised guards still permit gaps in envtest provisioning and exact fuzz-output validation, and several test contracts need clarification before execution.

Read-only review of `9a42d7591`, including cached Go 1.27.2 and Ginkgo v2.33.0 sources. Offline `go list ./...` returned 47 packages, none e2e. No files changed; mutation tests and envtest suites were not executed.

## Round-1 closure

| id | status | one-line reason |
| --- | --- | --- |
| V1 | closed | Three separate `-count=1` invocations match the pinned Ginkgo count and rerun restrictions; S8 checks the split. |
| V2 | closed | Proposal, spec and probe now correctly describe one shuffle followed by repeated execution of that order. |
| V3 | closed | F5a isolates the corpus check without a deny marker, uses untracked `testdata/`, and requires `--untracked-files=all`. |
| V4 | partly | Provisioning/export are specified, but S9 checks only empty output; it cannot establish that setup ran or assets were exported. |
| V5 | partly | The ordered allow-list is specified, but no case rejects missing, duplicated or reordered otherwise-allowed lines. |
| V6 | partly | Corpus and retry-cap controls are repaired; marker-message assertions still need to distinguish runner diagnostics from replayed input. |
| V7 | closed | F1/F10 require exact argv; S10 compares discovered and executed package sets. |
| V8 | partly | Mutants are now enumerated, but the provisioning mutant is not reliably killed by S9. |
| V9 | closed | All three guards must enter `cfc_guards` and lint in both modes, with security-guard evidence. |
| V10 | closed | R8–R10 cover empty extraction, unknown namespaced tasks and out-of-scope commands. |
| V11 | closed | S11 specifies setup-task, checkout credentials and timeout; verification records guard timings and lint delta. |
| V12 | closed | Post-merge evidence requires completed workflow runs rather than an actual failure issue. |
| V13 | closed | Empty current exclusions match offline discovery; future exclusions require an e2e package and reason. |
| V14 | closed | F13 and its mutant explicitly pin the failure header to the selected target. |
| V15 | closed | Closed by the documented rejection: the non-duplication rule is explicitly held by review. |
| V16 | partly | Most citations are repaired; the reporter case and concurrency ranges still omit the operative lines. |

## Findings

### R2-1 CRITICAL: S9 does not prove envtest setup or export

- Where: `openspec/changes/test-depth-signals/tasks.md:136`
- Evidence: S9 supplies an empty `echo-kubebuilder-assets` result and requires failure before testing. Task 2.5 assigns “drop the envtest provisioning” to this case. The spec separately requires setup, resolution and export at `specs/test-depth/spec.md:90`.
- Why it matters: Removing only `make setup-envtest` leaves S9 passing because the empty-result check still fails as expected. Removing only `export KUBEBUILDER_ASSETS` also leaves S9 passing. A guard can therefore stay green while the provisioning contract is violated. This distinction matters against the actual Makefile: `echo-kubebuilder-assets` itself depends on `envtest` and invokes `setup-envtest use` (`Makefile:65-66`), so successful resolution does not prove the explicit setup call occurred.
- Fix: Add a successful-assets case with fake `make` recording setup before resolution and fake `go test` asserting the resolved value is exported. Add separate mutants deleting setup, deleting export and deleting the empty-assets rejection. Assign each to its corresponding assertion rather than one collective S9 mutant.

### R2-2 CRITICAL: The allow-list tests do not enforce the required complete sequence

- Where: `openspec/changes/test-depth-signals/specs/test-depth/spec.md:15`
- Evidence: The spec says “exactly the known deadline flake” and “matches, in order”; F7/F12/F13 exercise unknown text, an additional unknown line and a different target.
- Why it matters: A runner that requires the target header and deadline line, then accepts any sequence of allowed lines, passes the listed cases. It can retry truncated output, duplicate diagnostics or reordered lines despite the exact-flake contract. The substring mutant does not isolate this defect: F12 rejects unknown lines, not malformed sequences containing only allowed lines.
- Fix: State explicitly that all five non-progress records must occur exactly once in the specified order. Add cases deleting each required record, duplicating one and swapping two, plus a mutant replacing sequence validation with unordered allowed-line membership. Keep progress-line handling separately defined.

### R2-3 WARNING: Marker text in captured output can satisfy the new diagnostic assertions

- Where: `openspec/changes/test-depth-signals/tasks.md:88`
- Evidence: F2–F4/F5b require that “the message names” the marker, and task 1.4 says dropping marker checks is killed “through the named marker.”
- Why it matters: Those markers already occur in the fake Go output. A runner that preserves that output and emits a generic `unrecognised failure text` diagnostic still exposes the marker after its dedicated check is deleted. A whole-output substring assertion would leave the mutant alive. The applied verification note recognizes the allow-list interaction but does not fix the diagnostic boundary.
- Fix: Define a distinct runner diagnostic, for example `::error::fuzz-retry: denied marker: <marker>`, and require F2–F4/F5b to match that diagnostic line specifically. Explicitly exclude replayed Go output from this assertion. Apply the same distinction to F8’s zero-target diagnostic.

### R2-4 WARNING: Shuffle cases assume an unspecified seed and fixture-selection interface

- Where: `openspec/changes/test-depth-signals/tasks.md:118`
- Evidence: S1 runs “on the fixture with that seed,” while the named stub runs `go test -count=3` on `go list ./...`. The fixture is deliberately excluded from that discovery. The spec only says the script chooses a seed.
- Why it matters: Neither the stub nor the implementation contract says how S1 selects the hidden fixture or supplies the seed. Running the stub literally discovers real packages and encounters Ginkgo’s count refusal, rather than demonstrating the intended fixture-order assertion. An implementer must invent a test seam before the prescribed red is meaningful.
- Fix: Specify one mechanism consistently in the spec and tasks. For example, define a `SHUFFLE_SEED` override and have the test’s fake discovery return only the real fixture’s import path while forwarding test execution to real Go. Require the stub to honor that same seed input and discovery mechanism. State whether S2 calls Go directly or uses a supported script option.

### R2-5 WARNING: The new Ginkgo explanation overstates what remains unshuffled

- Where: `openspec/changes/test-depth-signals/proposal.md:79`
- Evidence: The proposal says Ginkgo specs remain in their “default order” and “their order dependence stays hidden.” Pinned Ginkgo `types/config.go:50` initializes a time-based seed; `types/config.go:287-288` and `internal/ordering.go` show that default execution randomizes top-level containers.
- Why it matters: Separate Ginkgo invocations can already vary container order independently of the printed Go shuffle seed. The task’s split is valid, but the explanation misstates its coverage and can lead implementers to treat that seed as sufficient to reproduce a Ginkgo failure.
- Fix: Replace the paragraph with: “Go `-shuffle` does not control ordering inside `RunSpecs`. Ginkgo retains its default randomization of top-level containers using its own seed; this change neither enables full spec randomization nor ties that seed to the Go shuffle seed. Reproducing a Ginkgo failure may require its reported Ginkgo seed.” Preserve the accepted non-goal.

### R2-6 NOTE: Reporter citations stop before their assertions

- Where: `openspec/changes/test-depth-signals/specs/test-depth/spec.md:140`
- Evidence: The cited case range ends at `report`; its `expect` is the following line. The concurrency range ends at the `concurrency:` key, before `workflow_id`.
- Why it matters: These references are offered as existing evidence for the two-workflow scenario but omit the actual checks supporting that claim.
- Fix: Cite `hack/test/ci_failure_report_test.sh:1226-1233` and `.github/workflows/ci-failure-report.yaml:40-42`.

At case/mutant granularity, the split tasks are reasonably bounded: task 1.4 has fourteen mutants plus its no-op control. The principal remaining problem is assertion coverage and executable test setup, rather than task size. Executing the listed cases alone would not yet establish the whole spec.

## Stale file:line claims

| Claim | Cited | Actual |
| --- | --- | --- |
| Reporter “both open, nightly success” case including its assertion | `hack/test/ci_failure_report_test.sh:1225-1232` | Case starts at `:1226`; assertion is `:1233`. |
| Reporter concurrency keyed by `workflow_id` | `ci-failure-report.yaml:38-40` | Configuration is `:40-42`; `workflow_id` is at `:41`. |

The remaining explicit file:line citations in the three spec files match the inspected sources.
