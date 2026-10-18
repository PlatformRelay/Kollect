## Verdict

BLOCK — the README resource guard and shuffle workflow guard both accept concrete violations of their requirements.

## Answers

1. **Fuzz:** Replaying the real crash fixture, a prefixed panic, split panic text, and a nested corpus-status entry all failed without retry. A crash marker alongside the flake sequence is rejected before classification. Stdout and stderr are combined. Go rejects `-fuzz` with multiple packages; that output fails the classifier. A second attempt printing only `no fuzz tests to fuzz` fails. However, exact flake text with exit status **2 or 137** is retried and can subsequently pass (A3). I demonstrated this with mocked process outcomes, not a real crash escaping detection. Success requires an elapsed line; it does not independently verify an execution count.

2. **Shuffle:** The current runner behaves correctly in the tested cases. Empty and `off` seeds fail; every invocation receives the numeric seed. Failed or empty package discovery fails, all discovered packages enter one of the two arrays, and exclusions require an e2e path and reason. Empty assets and failed setup stop execution; assets are exported to tests. A failure in Ginkgo invocation 1 remains red after invocations 2 and 3 pass. The committed guard lacks that failure-sequence case (A4).

3. **Reproduction:** Correct for Go test ordering with the same package, test binary and repetition settings. Installed Go 1.27.2’s `testing.go:2570` shuffles `m.tests` once; `testing.go:2712` repeats that same slice for `-count`. It does not reshuffle each repetition. The Go seed alone does not reproduce Ginkgo ordering; the spec explicitly requires its separately reported seed where relevant. Neither seed guarantees reproduction of timing-dependent failures.

4. **README:** In-memory checks rejected unknown tasks after `;`, `||`, pipes, substitutions, `sudo`, and `env X=1`. Pre-verb kubectl flags fail closed. `kubectl get kinv/name` passes correctly, but an unknown second `TYPE/NAME` also passes (A1). `task -d=/nonexistent demo-up`, `kubectl apply -k README.md`, and `make nosuch` pass: flags are skipped, apply checks only existence, and make is outside the contract. A second Quick start section is ignored; changing the sole heading to `###` fails. CRLF causes false rejection through retained carriage returns. The continuation and indented-fence fixes are present and address their stated LF-input cases.

5. **CI wiring:** The current configuration is correct: fuzz inherits `contents: read`, retains five matrix entries, and downloads modules after go-cache and before the runner. All three guards run in both modes in required lint and are included in CWS-6; `hack/check.sh` discovers them automatically. Shuffle has SHA-pinned external actions, credential-free checkout, read-only permissions, a 45-minute timeout, and concurrency group `test-shuffle-${{ github.ref }}` with cancellation disabled. It runs on schedule and manual dispatch, not PRs, and sets no seed. Reporter and test both name **`Test shuffle`** exactly. The shuffle wiring guard nevertheless accepts disabled or non-blocking execution (A2).

6. **Reporting:** A scheduled failure opens a workflow-specific issue, or comments on its existing issue, subject to the reporter’s latest-completed-run logic. Ordinary test failures emit `FAILED with seed …`, eligible for the issue’s FAIL excerpt; truncation can omit it, but the issue links to the run containing the initial seed line. Setup failures still leave that initial line in the log. Workflow-path markers and concurrency keyed by workflow ID keep shuffle separate from E2E nightly. Manual runs are intentionally not reported.

7. **Go fixes:** Both are legitimate repetition fixes. They retain exact assertions of increments **1** and **1024**, instead of assuming process-global counters begin at zero. The collect test still checks which labeled series survive. I found no competing writer to the metrics test’s counter within that package’s tests. These changes do not weaken the asserted operation into “nonzero” or “at least.”

8. **Other gaps:** The retained findings below concern parser coverage, workflow enforcement, process-status classification, and failure propagation coverage. I am not re-reporting the deferred F16, setup diagnostic, missing F0/crash-fixture cases, or internal-task acceptance. The deferred items were not worse than described; notably, the real crash fixture was rejected during this review. The README check should not be described as proving that the entire quick start executes successfully.

## Findings

### A1 CRITICAL: README guard ignores resource types after the first positional argument

- Where: `hack/test/readme_quickstart_test.sh:168`
- Evidence: The parser assigns only the first `res`: `elif [[ -z "${res}" ]]; then res="${w[j]}"`. Running its actual checking functions with `kubectl get kinv/name foo/name` returned **0**, although `foo` is absent from the CRDs.
- Counterargument: The documented examples use comma-separated types; however, the parser explicitly supports `TYPE/NAME`, and silently accepts additional resource operands.
- Why it matters: A quick-start command naming a nonexistent resource passes TDS-3.
- Fix: Validate the type portion of every `TYPE/NAME` operand, or reject unsupported additional operands. Add a mutant containing a valid first operand and invalid second operand.

### A2 CRITICAL: Shuffle wiring guard accepts skipped or non-blocking test execution

- Where: `hack/test/test_shuffle_test.sh:411`
- Evidence: S11 locates the literal `run: task test:shuffle` but does not validate its execution conditions. Executing the actual S11 assertions against in-memory YAML variants returned **0** for step `if: false`, step `continue-on-error: true`, and job `if: false`.
- Counterargument: The committed workflow currently contains none of these switches; nevertheless, the guard explicitly claims to enforce that the nightly runs the shuffle task.
- Why it matters: CI can accept a workflow that skips all shuffle tests or reports success after they fail, preventing the scheduled failure report.
- Fix: Reject skip/error-suppression controls on the job and required steps, including shell overrides that bypass execution, and add corresponding self-test mutants.

### A3 WARNING: Fuzz retries unexpected process exit statuses

- Where: `hack/ci/fuzz-retry.sh:88`
- Evidence: Every nonzero status reaches `is_deadline_flake`. With the captured deadline output, first-attempt statuses **2** and **137**, followed by the passing fixture, both produced runner exit **0**.
- Counterargument: The exact output sequence is strong evidence of the known flake, and ordinary killed processes produce different output; it still does not explain an unexpected process termination after that output.
- Why it matters: The fail-closed decision ignores conflicting process-status evidence and can turn an abnormal termination into success.
- Fix: Require status **1** before allowing the deadline retry. Add cases for unexpected statuses followed by a passing second fixture, asserting no retry.

### A4 WARNING: Shuffle tests never exercise a failing Ginkgo invocation

- Where: `hack/test/test_shuffle_test.sh:126`
- Evidence: Non-forwarded fake tests always `exit 0`; forwarded failure cases use the plain Go fixture. S8 checks Ginkgo arguments and invocation counts, but never failure propagation.
- Counterargument: The current runner correctly preserves failures, which the review’s in-memory invocation sequence confirmed.
- Why it matters: Replacing the Ginkgo invocation’s `|| rc=1` with `|| true` would escape the committed cases while making failed suites green.
- Fix: Let the fake Go return per-invocation statuses. Test Ginkgo outcomes `1,0,0` and require a nonzero final result; add a mutant removing that failure accumulation.

## Could not check

No files were changed. Checks used source inspection, syntax validation, the real README plain check, and in-memory parser, workflow, and runner probes.

I did not run the full mutation suites, Go package tests, envtest, live fuzzing, or GitHub workflows: those require writable scratch/build storage or network access. I verified installed Go 1.27.2 source directly after the repository’s Go shim attempted an unavailable installation. Actual issue creation, remote action-pin validity, and historical fixture provenance were not independently verified.
