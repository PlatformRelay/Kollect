# Proposal

## Why

Four weaknesses in how the test suite proves itself, each seen in the tree:

- Mutation testing is a process rule (`docs/development/spec-workflow.md:51`, risk table) with no
  measurement: nobody can say which packages' tests actually catch mutants.
- The fuzz job retries any failure once when no `testdata/fuzz/` file appeared
  (`.github/workflows/ci.yaml`, job `fuzz`, the `for attempt in 1 2` loop). A panic that Go
  fails to persist, an out-of-memory kill or a real hang is retried and, if it does not recur,
  reported green.
- Golden outputs (`test/schema/golden/export-envelope.json` via `TestMarshalExportEnvelopeGolden`,
  `TestMarshalEventEnvelopeGolden`) are run once per `go test`. Non-deterministic output (map
  order, time, state shared between runs) passes if it happens to match once.
- The README quick start (`README.md:106`) is a claim to users. The hero smoke
  (`hack/demo/hero/smoke.sh`, job `hero-demo-smoke` in `e2e-smoke.yaml`) tests the path, but
  nothing ties the README's commands to what the smoke runs.

The reference project attune covers these with nightly mutation reports, a narrowly scoped fuzz
retry, repeated runs of snapshot tests and a docs-truth check.

## What Changes

Four independent requirements, each with its own check:

- TDS-1: a nightly, report-only mutation run with `gremlins` on `internal/aggregate`,
  `internal/redact` and `internal/pathvalidate`.
- TDS-2: the fuzz retry moves to `hack/ci/fuzz-retry.sh`, which retries only when the output is
  the known Go engine deadline artifact (golang/go#75804) and nothing else.
- TDS-3: `task test:determinism` runs the named golden tests with `-count=2`; CI runs it.
- TDS-4: `hack/test/readme_quickstart_test.sh` ties the README quick-start commands to the
  tasks, scripts, CRD short names and smoke job that exist.

## Capabilities

### New Capabilities

- `test-depth`: what the suite must measure and refuse beyond "tests pass once".

### Modified Capabilities

None.

## Impact

- Entry points: nightly job `mutation-report` in `e2e-nightly.yaml` (TDS-1); job `fuzz` in
  `ci.yaml` calling `hack/ci/fuzz-retry.sh` (TDS-2); the `test-suite` job running
  `task test:determinism` (TDS-3); the `lint` job running the README test (TDS-4).
- If the change grows past review size, TDS-1 is the natural split into its own change.

## Non-goals

- Making mutation score a gate (explicitly not in this change: a threshold needs a baseline from
  at least several nightly reports; a follow-up change decides it).
- Mutation testing outside the three packages; fuzz targets changes; new golden tests.
- Executing `task demo-up` in the README test (needs kind and Docker; `hero-demo-smoke` already
  does). TDS-4 verifies the README names what the smoke runs, not a second cluster run.

## Assumptions

- gremlins (`github.com/go-gremlins/gremlins`) runs on the Go version in `go.mod` and reports
  killed/lived/not-covered counts. Probe in task 1.1 (`go run ...@<pinned>`, record the version).
  If it does not, the change picks go-mutesting and records why.
- The three packages are chosen because they are pure (no Docker, no network), small enough for
  a nightly budget, and security- or correctness-critical: `aggregate` (content hashing and
  dedupe, fuzzed by `FuzzContentHash`), `redact` (secret scrubbing), `pathvalidate` (path
  safety). The sink git prune path would qualify on value (spec-workflow requires mutation
  evidence for deletion changes) but needs git and is slow; revisit once timing is known.
- golang/go#75804 is the upstream report for the fuzz engine's "context deadline exceeded" at
  `-fuzztime`. The reference to attune's handling and the issue number come from the survey and
  are not yet verified here; task 2.1 reproduces the exact output from a real CI log and records
  it as a fixture.
- `-count=2` repeats a test in one process. It catches shared-state and ordering effects, not
  everything; Go randomizes map iteration per range, which is the main output-order hazard.
