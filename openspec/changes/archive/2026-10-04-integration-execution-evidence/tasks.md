# Tasks

## 1. Discovery of expected tests (IEE-1)

- [x] 1.1 Write discovery tests on a testdata module (tagged, untagged, `!integration`, `TestMain`, `Testable`, benchmark, external test package, empty) and watch them fail on the assertion
- [x] 1.2 Implement discovery with `go list` + `go/build/constraint` + `go/parser`; tests pass

## 2. Analysis of results (IEE-2, IEE-3, IEE-4, IEE-6, IEE-7)

- [x] 2.1 Write analysis tests on recorded streams: all pass, all skipped, one skipped, one missing among passes, failed subtest, zero selected, malformed line, truncated test, truncated package, unreported package, non-zero exit with a clean stream, untagged skip, untagged failure, exploratory skip, exploratory failure; watch them fail
- [x] 2.2 Implement the analysis and the report; tests pass
- [x] 2.3 Defect controls: break each refusal in turn and confirm the matching test fails

## 3. Command and wiring (IEE-3, IEE-5, IEE-7)

- [x] 3.1 Test that the runner passes `-json -count=1 -tags=integration`, sets `KOLLECT_REQUIRE_DOCKER=true` in required mode only, and returns the process exit status
- [x] 3.2 Point `task test-integration` at the checker; add `task test-integration:explore`
- [x] 3.3 Update testing.md, COMMAND-REFERENCE.md and CONTRIBUTING.md; markdown lint passes

## 4. Real runs

- [x] 4.1 Real Docker run of `task test-integration` locally: expected = executed = passed
- [x] 4.2 Run without Docker (container without a socket): required mode fails, explore mode exits zero and says the run is incomplete
- [x] 4.3 CI `test-integration` job green on the PR head, with the checker's report in the log
- [x] 4.4 Independent review of the checker and its CI wiring

## Review rework

Independent review of ec8fe3db9: REQUEST CHANGES, no blocker. Each finding was taken as a spec change
first (scenarios added to IEE-1, IEE-2 and the new IEE-8), then a failing test, then the fix:

- F1: a name declared in both the internal and external test package ran twice and the last
  result won. Expected tests are now counted per declaration, every result is kept, and the worst
  one counts.
- F2/F3: the report kept the first 40 lines of a failing test (the failure message is last), and
  dropped compiler output and package-level failures. It now keeps the last lines, shows build
  output and names failed packages with their output. Package summaries are echoed to stderr
  while tests run.
- F4: a skipped subtest of an expected test now fails a required run.
- F5: integration-tagged tests outside the selected packages now fail a required run; an
  exploratory run lists them and is not complete.
- F6/F7: this table, and the change is archived in the same PR so the living spec exists at merge.

Round 2 (869a48517): APPROVE, four P3s, all taken:

- R1: with a name declared twice, the counts went per name and the totals per declaration
  ("expected 6, executed 5" on a passing run). Counts are now per declaration.
- R2: a result without a run event was accepted. It is now reported as a malformed stream.
- R3: IEE-8 wording now matches the narrowed live echo and the package-output rule.
- R4: testing.md and CONTRIBUTING.md say that a new integration-tagged package has to be added to
  `INTEGRATION_PACKAGES`.

## Verification

Code tested at `c7272e5b6`, then at the R1/R2 tree (final real run below). Go 1.26.6 (Taskfile pins GOTOOLCHAIN from go.mod), Docker 29.8.1,
umask 022.

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| IEE-1 | `TestDiscoverExpected_*`, `TestListPackages_*`, `TestOutsideSelection`, `TestAnalyze_noExpectedTestsFails`, `TestAnalyze_testsOutsideTheSelectionFail` | tagged tests found and counted per declaration; untagged, helpers, methods and benchmarks excluded; empty set, empty pattern and tests outside the selection refused | pass | `go test -race -count=1 ./hack/tools/integrationevidence/` |
| IEE-2 | `TestAnalyze_*` for all-pass, skipped, all-skipped, missing, failed subtest, zero selected, duplicate names, skipped subtest | only the complete all-pass streams succeed | pass | same command |
| IEE-3 | `TestAnalyze_nonZeroExitIsNeverMasked`, `TestExitCode_keepsTheProcessStatus` | exit 2 fails in both modes; a command that never started is an error | pass | same command |
| IEE-4 | `TestAnalyze_malformedLineFails`, `_truncatedTestFails`, `_truncatedUntaggedTestFails`, `_truncatedPackageFails`, `_unreportedPackageFails`, `_buildFailureFails` | each fails and is named | pass | same command |
| IEE-5 | `TestTestCommand_requiredModeDemandsDocker`; full run in `golang:1.26.6` without a Docker socket | env appended last; required run fails | pass | no-Docker required: exit 1, 35 tests fail with "docker required" (run from a `git archive` copy) |
| IEE-6 | `TestAnalyze_untaggedSkipIsReportedNotFailed`, `_untaggedFailureFails` | skip reported, failure fails | pass | unit tests; real run lists the 2 untagged pipeline envtest skips |
| IEE-7 | `TestAnalyze_explore*`; full run without Docker | exit 0, "NOT a completed integration verification"; failures fail | pass | no-Docker explore: exit 0, 22 passed, 35 skipped, INCOMPLETE |
| IEE-8 | `TestAnalyze_showsTheEndOfLongFailureOutput`, `_showsCompilerOutput`, `_namesFailedPackageWithItsOutput`, `TestOutputEcho_*` | failure message, compiler error and package output shown; package summaries echoed live | pass | unit tests; real run printed 42 live progress lines |
| all | defect controls: 42 mutations, one refusal disabled each | each turns a named test red | pass | 42/42 killed, judged by `go test` exit status; a no-op control survives. Harness note: an earlier verdict that grepped for `^ok` reported two false survivors |
| all | red before fix | new tests fail on assertions against stubs | pass | 21/23 initial tests red; rework tests red on the old code (one recovered after a `-run` pattern that matched nothing) |
| all | real Docker run, `task test-integration` | expected = executed = passed | pass | 57/57/57, 0 failed/skipped/missing, at c7272e5b6 and again on the R1/R2 tree. A umask-002 run was correctly refused on two untagged git-mirror tests |
| all | pinned golangci-lint v2.11.4, markdown lint, `task spec:validate` | clean | pass | 0 issues |
| all | CI on the PR head | required checks green, checker report in the job log | pass | 869a48517: 35 pass; `test-integration` log shows "expected 57, executed 57, passed 57 ... RESULT: PASS" ([run 37221341188](https://github.com/PlatformRelay/Kollect/actions/runs/37221341188)). The merged head is re-checked before merge |
| — | independent review | APPROVE | pass | round 1 (ec8fe3db9): REQUEST CHANGES; round 2 (869a48517): APPROVE, R1-R4 applied; the R1-R4 delta is re-checked before merge |
