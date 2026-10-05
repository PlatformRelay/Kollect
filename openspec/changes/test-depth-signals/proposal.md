# Proposal

## Why

Three weaknesses in how the suite proves itself, each seen in the tree:

- The fuzz job retries any failure once when no `testdata/fuzz/` file appeared
  (`.github/workflows/ci.yaml`, job `fuzz`, the `for attempt in 1 2` loop). A panic Go fails to
  persist, an out-of-memory kill or a real hang is retried and, if it does not recur, reported
  green. A matrix leg that fuzzes nothing also reports green.
- Tests that pass only in source order are invisible: nothing runs the unit suites with `-shuffle`.
- The README quick start (`README.md:106`) is a claim to users. `hack/test/demo_task_aliases_test.sh`
  checks that `demo-up` and `demo-down` exist and delegate, and the `hero-demo-smoke` job exercises
  the path, but nothing ties the README's other commands (the `kubectl get` short names, the
  `kubectl apply -k` path) to what exists.

## What Changes

- TDS-1: the fuzz retry moves to `hack/ci/fuzz-retry.sh`: 30 seconds per target, one retry only
  for the known Go engine deadline flake (golang/go#75804), zero fuzz targets run is red.
- TDS-2: a separate nightly workflow `test-shuffle.yaml` runs the unit packages with
  `-shuffle=on -count=3`, printing the seed. Its own workflow, so its failures get their own issue
  from the CI-failure reporter and cannot mask the nightly suite.
- TDS-3: `hack/test/readme_quickstart_test.sh` checks the README quick-start commands against the
  Taskfile, the sample path and the CRD short names.

## Capabilities

### New Capabilities

- `test-depth`: refusals beyond "tests pass once, in source order".

### Modified Capabilities

None.

## Impact

- Entry points: job `fuzz` in `ci.yaml` calling `hack/ci/fuzz-retry.sh`; `.github/workflows/test-shuffle.yaml`
  (added to the reported set of `ci-failure-report.yaml`); the `lint` job running the README test
  and the fuzz-runner test.

## Dependencies

Landing order across the ten proposed changes: (1) ci-workflow-hardening, (2) task-check-entrypoint,
(3) golangci-lint-bump, (4) cross-file-consistency-gates, (5) developer-toolchain-pins,
(6) dependency-update-automation, (7) ci-failure-reporting, (8) test-depth-signals,
(9) mutation-testing-signal, (10) public-agent-contract.
Needs 7 (the shuffle failures must reach an issue) and 1 (guards block merges). New workflows
satisfy the zizmor gate in their own tasks.

## Non-goals

- Mutation testing (own change); new fuzz targets; repeated golden-test runs (the goldens already
  compare to a fixed file and `encoding/json` sorts map keys, so repeating them adds nothing).
- Executing `task demo-up` (needs kind and Docker; `hero-demo-smoke` does).
- Re-checking what `demo_task_aliases_test.sh` already asserts.

## Assumptions

- golang/go#75804 is the upstream report for the fuzz engine's "context deadline exceeded" at the
  `-fuzztime` boundary. This comes from the survey and is not yet verified; task 1.1 reproduces
  the exact output from a real CI log as a fixture. The genuine flake output contains
  `--- FAIL: FuzzX` (every failing `go test -fuzz` prints it), so that line is not a deny marker.
- `-shuffle=on` prints the seed at the start; probe in task 2.1. Packages needing Docker, envtest
  or kind are excluded by a listed reason; the list is fixed in task 2.1, with the measured cost.
