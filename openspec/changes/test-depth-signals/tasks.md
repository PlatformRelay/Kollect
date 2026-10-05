# Tasks

## 1. Fuzz runner (TDS-1)

- [ ] 1.1 Capture a real deadline-artifact log (with its `--- FAIL: FuzzX` line), a real crash log and a "no fuzz tests" output as fixtures; write `hack/test/fuzz_retry_test.sh` (plain and `--self-test`) with a fake `go` for every scenario; watch the panic, unknown-text and zero-target cases fail against the current loop
- [ ] 1.2 Implement `hack/ci/fuzz-retry.sh`; point the `fuzz` job at it; grep `hack/test/` for assertions on the `fuzz` job's steps and update them
- [ ] 1.3 Defect controls: drop each deny pattern and the zero-target check in turn; each turns a case red; a no-op control survives

## 2. Shuffle (TDS-2)

- [ ] 2.1 Probe the seed output; fix the unit package list with exclusion reasons in `Taskfile.yml`; measure the cost of `-count=3`
- [ ] 2.2 Write a test with an order-dependent fixture package that fails under an explicitly passed seed (deterministic red) and a guard for reasonless exclusions; watch both fail
- [ ] 2.3 Add `task test:shuffle` and `.github/workflows/test-shuffle.yaml` (nightly, `contents: read`); add the workflow to the reporter's list in `ci-failure-report.yaml`; the zizmor gate passes; fix order dependence found, in its own commits

## 3. README truth (TDS-3)

- [ ] 3.1 Write `readme_quickstart_test.sh` with mutants (renamed task, unknown short name, missing apply path, missing section); watch them fail
- [ ] 3.2 Implement; wire into `lint` (plain and `--self-test`)

## 4. Land

- [ ] 4.1 Merge with the `post-merge` row open; archive in a follow-up evidence PR once it passes

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| TDS-1 | `fuzz_retry_test.sh` all scenarios; mutants by exit status | only the pure deadline case retries; zero targets red | not-run | |
| TDS-2 | order-dependent fixture with a fixed seed; guard mutants | fixture red; seed printed | not-run | |
| TDS-2 | first scheduled shuffle run; reporter issue on a forced failure | green; separate issue | post-merge | follow-up evidence PR |
| TDS-3 | `readme_quickstart_test.sh` plus mutants | each mutant red, tree green | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; reviewed and archive revisions recorded | not-run | |
