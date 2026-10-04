# Tasks

## 1. Discovery of expected tests (IEE-1)

- [ ] 1.1 Write discovery tests on a testdata module (tagged, untagged, `!integration`, `TestMain`, `Testable`, benchmark, external test package, empty) and watch them fail on the assertion
- [ ] 1.2 Implement discovery with `go list` + `go/build/constraint` + `go/parser`; tests pass

## 2. Analysis of results (IEE-2, IEE-3, IEE-4, IEE-6, IEE-7)

- [ ] 2.1 Write analysis tests on recorded streams: all pass, all skipped, one skipped, one missing among passes, failed subtest, zero selected, malformed line, truncated test, truncated package, unreported package, non-zero exit with a clean stream, untagged skip, untagged failure, exploratory skip, exploratory failure; watch them fail
- [ ] 2.2 Implement the analysis and the report; tests pass
- [ ] 2.3 Defect controls: break each refusal in turn and confirm the matching test fails

## 3. Command and wiring (IEE-3, IEE-5, IEE-7)

- [ ] 3.1 Test that the runner passes `-json -count=1 -tags=integration`, sets `KOLLECT_REQUIRE_DOCKER=true` in required mode only, and returns the process exit status
- [ ] 3.2 Point `task test-integration` at the checker; add `task test-integration:explore`
- [ ] 3.3 Update testing.md, COMMAND-REFERENCE.md and CONTRIBUTING.md; markdown lint passes

## 4. Real runs

- [ ] 4.1 Real Docker run of `task test-integration` locally: expected = executed = passed
- [ ] 4.2 Run without Docker (container without a socket): required mode fails, explore mode exits zero and says the run is incomplete
- [ ] 4.3 CI `test-integration` job green on the PR head, with the checker's report in the log
- [ ] 4.4 Independent review of the checker and its CI wiring

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| IEE-1 | discovery tests (1.1) | tagged tests found; untagged, helpers and benchmarks excluded; empty set refused | not-run | |
| IEE-2 | analysis tests: all pass, one skipped, all skipped, missing, failed subtest, zero selected | only the all-pass stream succeeds | not-run | |
| IEE-3 | analysis test: clean stream with exit 1 | fails | not-run | |
| IEE-4 | analysis tests: malformed, truncated test, truncated package, unreported package | fail, named | not-run | |
| IEE-5 | runner test (3.1); run without Docker in required mode (4.2) | env set; run fails | not-run | |
| IEE-6 | analysis tests: untagged skip, untagged failure | skip reported, failure fails | not-run | |
| IEE-7 | analysis tests: exploratory skip and failure; run without Docker (4.2) | exit 0 with "not a completed integration verification"; failure fails | not-run | |
| all | defect controls (2.3) | each disabled refusal turns its test red | not-run | |
| all | real Docker run (4.1) and CI (4.3) | expected = executed = passed | not-run | |
| — | independent review (4.4) | APPROVE on the merged revision | not-run | |
