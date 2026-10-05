# Tasks

## 0. Prerequisite

- [ ] 0.1 Confirm the CI-failure reporter (change 7) is merged and its workflow list accepts a new entry

## 1. Measurement (MUT-1 to MUT-4)

- [ ] 1.1 Probe gremlins: choose the pinned version that supports the `go.mod` Go; run it on the three packages; record version, runtime, report format, and whether the fixture control reports LIVED
- [ ] 1.2 Write a test for `hack/ci/mutation-summary.sh` on a recorded report (counts per package, broken or empty report exits non-zero) and a guard that reads `.github/workflows/mutation.yaml` and `Taskfile.yml` for threshold options; add the fixture module with the executed-but-unasserted function; watch these fail on the assertions
- [ ] 1.3 Implement; add `.github/workflows/mutation.yaml` (nightly, `contents: read`, `GREMLINS_VERSION` from the Taskfile); add it to the reporter's list; the zizmor gate passes
- [ ] 1.4 Grep `hack/test/` for step-index assertions on workflows touched; run the guard in `lint` (plain and `--self-test`)

## 2. Land

- [ ] 2.1 Merge with the `post-merge` row open; archive in a follow-up evidence PR once it passes

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| MUT-1 | formatter test on a recorded report | counts per package | not-run | |
| MUT-1 | first real nightly run; forced failure reaches its own issue | artifact present; separate issue | post-merge | follow-up evidence PR |
| MUT-2 | formatter tests (broken report); threshold-option guard mutant | red; red | not-run | |
| MUT-3 | gremlins on the fixture | control LIVED, not NOT COVERED | not-run | |
| MUT-4 | formatter test over three recorded nights | night 2 red and distinguishable | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; reviewed and archive revisions recorded | not-run | |
