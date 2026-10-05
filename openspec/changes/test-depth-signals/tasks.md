# Tasks

## 1. Fuzz retry (TDS-1)

- [ ] 1.1 Capture a real deadline-artifact log (with its `--- FAIL: FuzzX` line) and a real crash log as fixtures; write `hack/test/fuzz_retry_test.sh` with a fake `go` for every scenario; watch the panic and unknown-text cases fail against the current loop's behaviour
- [ ] 1.2 Implement `hack/ci/fuzz-retry.sh`; point the `fuzz` job at it
- [ ] 1.3 Defect controls: drop each deny pattern in turn; each turns a case red; one no-op control survives

## 2. Shuffle (TDS-2)

- [ ] 2.1 Probe the seed output and fix the unit package list with exclusion reasons in `Taskfile.yml`
- [ ] 2.2 Write a test with a deliberately order-dependent fixture package that fails under `-shuffle` for a fixed seed (pass the seed explicitly so the red is deterministic) and a guard test for exclusions without a reason; watch both fail
- [ ] 2.3 Add `task test:shuffle`; run it in the nightly, recording the measured duration; fix any order dependence it finds in its own commits

## 3. README truth (TDS-3)

- [ ] 3.1 Write `readme_quickstart_test.sh` with mutants (renamed task, unknown short name, missing apply path, missing section); watch them fail
- [ ] 3.2 Implement; wire into `lint`

## 4. Land

- [ ] 4.1 Archive the change (`openspec archive`) as the last commit of the PR, after review and green CI; the review record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| TDS-1 | `fuzz_retry_test.sh` all scenarios, mutants judged by exit status | only the pure deadline case retries | not-run | |
| TDS-2 | order-dependent fixture with a fixed seed; `task test:shuffle -v` | fixture red; seed printed; tree green | not-run | |
| TDS-3 | `readme_quickstart_test.sh` plus mutants | each mutant red, tree green | not-run | |
| all | CI on the PR head; independent review | green; APPROVE; both revisions recorded | not-run | |
