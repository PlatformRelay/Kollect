# Tasks

## 1. Mutation report (TDS-1)

- [ ] 1.1 Probe gremlins on the three packages locally: version, runtime, report format; record numbers
- [ ] 1.2 Write a test for the summary formatter (`hack/ci/mutation-summary.sh`) on a recorded report: counts per package, tool-failure exit; watch it fail
- [ ] 1.3 Implement it; add job `mutation-report` to `e2e-nightly.yaml` (pinned gremlins version single-sourced in `Taskfile.yml`, Renovate annotation)

## 2. Fuzz retry (TDS-2)

- [ ] 2.1 Capture a real deadline-artifact log and a real crash log as fixtures; write `hack/test/fuzz_retry_test.sh` with a fake `go` for every scenario; watch the panic and unknown-text cases fail against the current loop's behaviour
- [ ] 2.2 Implement `hack/ci/fuzz-retry.sh`; point the `fuzz` job at it
- [ ] 2.3 Defect controls: drop each deny pattern in turn; each turns a case red; one no-op control survives

## 3. Determinism (TDS-3)

- [ ] 3.1 Write a test that a deliberately time-dependent fixture test fails under `-count=2` and a listed-but-missing name fails the task; watch both red
- [ ] 3.2 Add `task test:determinism`; run it in `test-suite`

## 4. README truth (TDS-4)

- [ ] 4.1 Write `readme_quickstart_test.sh` with mutants (renamed task, unknown short name, missing section, diverged smoke); watch them fail
- [ ] 4.2 Implement; wire into `lint`

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| TDS-1 | summary-formatter test; one real nightly run | counts per package; tool failure red; low score green | not-run | |
| TDS-2 | `fuzz_retry_test.sh` all scenarios, mutants judged by exit status | only the pure deadline case retries | not-run | |
| TDS-3 | determinism fixture test; `task test:determinism` with `-v` | `=== RUN` for both golden tests; fixture red | not-run | |
| TDS-4 | `readme_quickstart_test.sh` plus mutants | each mutant red, tree green | not-run | |
| all | CI on the PR head; independent review | green; APPROVE | not-run | |
