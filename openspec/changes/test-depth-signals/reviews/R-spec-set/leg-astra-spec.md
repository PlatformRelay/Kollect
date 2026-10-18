## Verdict

BLOCK — the shuffle command is incompatible with the pinned Ginkgo suites, and the fuzz contract and test cases do not establish the promised fail-closed behavior.

## Findings

### TDS-R1 CRITICAL: `-count=3` cannot run the included Ginkgo suites
- Where: `openspec/changes/test-depth-signals/specs/test-depth/spec.md:67`; `openspec/changes/test-depth-signals/tasks.md:68`; `internal/controller/suite_test.go:51`; `internal/webhook/v1alpha1/suite_test.go:32`; `go.mod:18`.
- Evidence: The spec requires all non-e2e packages with `-count=3`. Both suites call `RunSpecs`. The locally cached source of pinned Ginkgo v2.33.0 rejects this in `types/config.go:409`; `types/errors.go:656` states: “Only -count=1 is allowed.”
- Why it matters: Task 2.1 cannot obtain a successful timing baseline for the prescribed command. Task 2.3 would ship a permanently failing nightly. This is independent of envtest provisioning and the non-goal of reordering Ginkgo specs.
- Fix: Amend TDS-2 and tasks 2.1–2.3 to specify a compatible repetition strategy: ordinary Go packages use `-count=3`; Ginkgo packages run three separate processes with `-count=1`, retaining every non-e2e package. Add a case checking that dispatch and a real smoke run of both Ginkgo suites.

### TDS-R2 WARNING: The shuffle/repetition explanation is factually wrong
- Where: `openspec/changes/test-depth-signals/proposal.md:89`; `openspec/changes/test-depth-signals/specs/test-depth/spec.md:85`; `openspec/changes/test-depth-signals/tasks.md:68`.
- Evidence: Installed Go 1.27.2 source, `src/testing/testing.go:2585–2588`, prints the seed and shuffles the test list once. The `-count` loop starts later at line 2714 and reuses that list.
- Why it matters: Task 2.1 asks implementers to prove an incorrect premise. Matching `-count` remains useful for reproducing accumulated state, but repetitions do not consume successive shuffle permutations. A failure can also reproduce with another count or seed.
- Fix: Replace the RNG-stream explanation with: “Each package independently shuffles once using N and repeats that order. Reproduce with the same revision, toolchain, package, seed and repetition settings.” Replace “only” with “with.” Add a two-package probe showing both receive the same explicit numeric seed.

### TDS-R3 CRITICAL: Deadline substrings are insufficient to identify a pure deadline failure
- Where: `openspec/changes/test-depth-signals/specs/test-depth/spec.md:14`; `openspec/changes/test-depth-signals/tasks.md:38`.
- Evidence: Retry eligibility requires two matching lines and absence of four deny markers. It does not reject additional unknown diagnostics, incomplete output, an unexpected process status, or failure text belonging to another target.
- Why it matters: A truncated log containing the deadline pair, or a deadline pair plus an unlisted crash diagnostic, satisfies the written rule. A passing retry then hides the original failure despite the stated fail-closed decision.
- Fix: Define an accepted, complete deadline-attempt grammar based on the captured fixtures, including expected target/package identity, terminal failure records and process status. Unknown additional failure text or incomplete capture must reject retry. Add mixed deadline/unknown-error, truncated-log, wrong-target and abnormal-exit fixtures, all asserting exactly one attempt.

### TDS-R4 CRITICAL: The corpus check can miss a newly written finding
- Where: `openspec/changes/test-depth-signals/specs/test-depth/spec.md:16`; `openspec/changes/test-depth-signals/tasks.md:44`; `.github/workflows/ci.yaml:832`.
- Evidence: The spec prescribes `git status --porcelain` and “no new file.” Default status omits ignored files and can collapse untracked directories to a parent such as `?? internal/collect/testdata/`, which does not contain `testdata/fuzz/`.
- Why it matters: A new corpus file can exist while the prescribed check sees nothing. Modified tracked corpus files and status-command failure are also unspecified. F5 permits using the output marker instead of testing filesystem detection.
- Fix: Require before/after inspection of the selected package’s corpus directory, including ignored/untracked files and changed existing files; inspection errors must fail closed. Split F5 into independent filesystem-only and marker-only cases, including a previously absent parent directory and an ignored corpus file. Document that Go’s external fuzz cache contains interesting inputs and is distinct from the crash corpus; cache growth alone must not count as a crash.

### TDS-R5 WARNING: Several mandatory fuzz mutants have no isolating test
- Where: `openspec/changes/test-depth-signals/tasks.md:40`; `openspec/changes/test-depth-signals/tasks.md:62`.
- Evidence: F3 and F4 contain kill/fatal markers without requiring an otherwise valid deadline transcript. F5 says corpus file “or” output marker. Task 1.3 requires deleting each corresponding protection independently.
- Why it matters: Removing the kill or fatal deny marker still leaves those fixtures rejected as unknown failures. Removing the corpus check can survive when F5 uses the output marker. Thus the stated cases do not reliably kill the required mutants.
- Fix: Put each deny marker independently into an otherwise accepted deadline fixture. Separate corpus-only and marker-only cases. Supply a third passing response for the retry-cap mutant and assert exactly two calls. Require each mutant to fail its named assertion, with a bounded execution timeout.

### TDS-R6 CRITICAL: Tests do not prove that the runners execute the requested work
- Where: `openspec/changes/test-depth-signals/tasks.md:22`; `openspec/changes/test-depth-signals/tasks.md:55`; `openspec/changes/test-depth-signals/tasks.md:79`.
- Evidence: The fake fuzz command records calls, but the cases assert call counts rather than exact arguments. F11 checks workflow-to-script wiring. Shuffle cases check exclusion entries and an empty list, without comparing the executed package list with discovery.
- Why it matters: A fuzz script can ignore its arguments and repeatedly fuzz a different valid target while F1–F11 pass. A shuffle script can silently discard an envtest package outside the exclusion list while S1–S8 pass. S1’s failure on the first iteration also does not establish `-count=3`.
- Fix: Add argument assertions for the exact fuzz target, resolved package and `-fuzztime=30s`, plus a mismatch case. For shuffle, fake discovery with multiple ordinary packages, both envtest packages and e2e; assert the complete executed set, seed and repetition arguments. Add mutants that drop one package or reduce the repetition count.

### TDS-R7 WARNING: Cache restoration is treated as provisioning in the workflow assertion
- Where: `openspec/changes/test-depth-signals/tasks.md:92`; `openspec/changes/test-depth-signals/tasks.md:99`; `.github/actions/go-cache/action.yml:32`; `hack/coverage.sh:23`.
- Evidence: S8 checks only `envtest: true`. The composite action restores a cache; coverage separately runs `make setup-envtest` and resolves/exports `KUBEBUILDER_ASSETS` at lines 25–26.
- Why it matters: Removing installation can leave S8 green and fail the nightly on a cold cache. The suites can discover downloaded `bin/k8s` assets themselves, so lack of an export alone is not necessarily fatal; lack of installation is.
- Fix: Make task 2.3 explicitly invoke `make setup-envtest` before tests and resolve/export the selected assets as coverage does. Add a provisioning-removal mutant and record a cold-cache smoke run. Expand the citation to include the actual installation path through `task coverage`.

### TDS-R8 CRITICAL: Required guard registration remains conditional
- Where: `openspec/changes/test-depth-signals/tasks.md:9`; `hack/test/ci_workflow_security_test.sh:597`.
- Evidence: Tasks require CFC-6 registration only for “a guard that checks its own wiring.” None of the new case lists explicitly requires checking its own lint invocation. The existing CFC-6 comment explains that deleting all invocations otherwise makes a guard disappear from CWS-6 discovery.
- Why it matters: Implementers can wire all three guards initially, omit unconditional registration, and still follow the tasks. Later deletion of both lint steps can pass the security guard while removing a required signal.
- Fix: Explicitly add all three new guard paths to `cfc_guards`. Require mutants removing their plain step, self-test step and both steps. Keep the existing `hack/check.sh` sweep unchanged; it already discovers the new scripts and their declared modes.

### TDS-R9 WARNING: README extraction boundaries and empty content lack controls
- Where: `openspec/changes/test-depth-signals/specs/test-depth/spec.md:106`; `openspec/changes/test-depth-signals/specs/test-depth/spec.md:138`; `openspec/changes/test-depth-signals/tasks.md:109`.
- Evidence: R5 removes the heading, but no case retains the heading while removing all command blocks. No control places invalid commands outside the section or outside its `sh` fences. R6 tests a valid included task only.
- Why it matters: A parser that checks the heading and then accepts an empty extraction can pass R1–R7. A parser that accepts every namespaced task can also pass.
- Fix: Add cases for an existing section with empty extraction, an unknown included task, and deletion of an included task definition. Add green controls containing invalid commands in prose, another section and a non-`sh` fence.

### TDS-R10 WARNING: Workflow security and required-job runtime checks are underspecified
- Where: `openspec/changes/test-depth-signals/tasks.md:73`; `openspec/changes/test-depth-signals/tasks.md:92`; `openspec/changes/test-depth-signals/tasks.md:110`.
- Evidence: Tasks measure nightly runtime, but not the new guards in required lint. S8 omits checkout credential persistence, action pins, concurrency and timeout. CWS-4 at `hack/test/ci_workflow_security_test.sh:390` specifically checks `ci.yaml` and `e2e-smoke.yaml`; it does not impose concurrency on the new nightly.
- Why it matters: “Zizmor gate passes” at the configured high-severity threshold is not a complete workflow contract. Per-mutant tree copies and repeated Go fixture compilation can also add substantial required-job runtime without any task recording it.
- Fix: Specify pinned checkout/Task setup, `persist-credentials: false`, workflow-specific concurrency, and the measured timeout in task 2.3 and S8. Add a verification row measuring all new guards in both modes and total lint impact; require minimal fixture copies, per-mutant cleanup and reuse of compiled fixtures where possible.

### TDS-R11 WARNING: Post-merge verification depends on an unspecified scheduled failure
- Where: `openspec/changes/test-depth-signals/tasks.md:127`; `openspec/changes/test-depth-signals/tasks.md:135`.
- Evidence: Archival waits for “reporter issue on a scheduled failure,” while task 2.3 correctly notes that manual dispatch is not reported.
- Why it matters: A healthy nightly may never provide that evidence. The plan gives no deterministic test for simultaneous shuffle/E2E failures and no owner task for any live exercise requiring repository permissions.
- Fix: Add a deterministic reporter fixture covering two distinct scheduled workflow identities and asserting separate issues. Retain the first successful scheduled run as post-merge evidence. If a live failure exercise is required, park it explicitly as an owner task with permissions, procedure, cleanup and completion criteria. The reporter list/comment/message edit and W1-style mutant already specified are otherwise correctly targeted.

## Stale file:line claims

All explicit citations were checked. No numeric reference points to a moved or unrelated location; two citations overstate what their referenced lines establish.

| Claim | Cited | Actual |
|---|---|---|
| Envtest is “provisioned as” the cited CI lines do | `ci.yaml:647–649` | These lines restore the optional cache. `ci.yaml:654` invokes coverage; `hack/coverage.sh:23–29` installs and resolves assets. |
| Coverage line represents the unit package list minus e2e | `hack/coverage.sh:37` | This line additionally excludes `/internal/` and `/cmd$`; internal packages run separately at lines 43–45. `Makefile:70` is the accurate citation for the stated all-non-e2e selection. |
| Fuzz step, retry loop and extraction range | `ci.yaml:817`, `:828`, `:826–838` | Correct; line 826 is the YAML `run:` key, so only its shell body belongs in the extracted script. |
| Required lint job and CWS-6 | `ci.yaml:279`; `ci_workflow_security_test.sh:429` | Correct. The CFC-6 registry is at lines 603–604. |
| Reporter list, assertion and W1 mutant | `ci-failure-report.yaml:22–24`; `ci_failure_report_test.sh:1547–1553`, `:424` | Correct. |
| README heading, Task namespace and guard sweep | `README.md:106`; `Taskfile.yml:59`; `hack/check.sh:88–108` | Correct. |
| Envtest installation and Makefile package filter | `hack/coverage.sh:23`; `Makefile:70` | Correct. |
