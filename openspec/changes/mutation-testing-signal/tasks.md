# Tasks

## 1. Measurement (MUT-1 to MUT-3)

- [ ] 1.1 Probe gremlins on the three packages locally: version, runtime, report format; record numbers
- [ ] 1.2 Write a test for `hack/ci/mutation-summary.sh` on a recorded report: counts per package, empty or broken report exits non-zero, threshold flag rejected; watch it fail on the assertion
- [ ] 1.3 Implement it; add job `mutation-report` to `e2e-nightly.yaml` with `GREMLINS_VERSION` single-sourced in `Taskfile.yml`

## 2. Land

- [ ] 2.1 Archive the change (`openspec archive`) as the last commit of the PR, after review and green CI; the review record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| MUT-1 | formatter test; one real nightly or dispatch run | counts per package; artifact present | not-run | |
| MUT-2 | formatter tests (broken report, threshold flag); run with a surviving mutant | tool failure red; low score green | not-run | |
| MUT-3 | formatter test over three recorded nights | night 2 red and distinguishable | not-run | |
| all | CI on the PR head; independent review | green; APPROVE; both revisions recorded | not-run | |
