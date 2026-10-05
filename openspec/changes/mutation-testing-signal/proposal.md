# Proposal

## Why

Mutation testing is a process rule (`docs/development/spec-workflow.md:51`, the risk table:
deletion, tenancy, credentials need "a mutation or negative control") with no measurement: nobody
can say which packages' tests actually catch mutants. The reference project attune measures it
nightly, report only.

## What Changes

A nightly workflow `mutation.yaml` of its own runs `gremlins` on `internal/aggregate`,
`internal/redact` and `internal/pathvalidate`, uploads the report and writes killed, lived and
not-covered counts per package to the job summary. It is a measurement, explicitly not a gate, and
it is a separate workflow so its failures get their own issue and cannot mask the nightly suite.
It is deferred until the CI-failure reporter (change 7) has shipped.

## Capabilities

### New Capabilities

- `mutation-testing`: what the nightly mutation measurement must report and must not do.

### Modified Capabilities

None.

## Impact

- Entry point: `.github/workflows/mutation.yaml`, `hack/ci/mutation-summary.sh` (formatter, tested
  on a recorded report), `GREMLINS_VERSION` in `Taskfile.yml` with a Renovate annotation.
- The workflow is added to the reporter's list in `ci-failure-report.yaml` and satisfies the zizmor
  gate in its own tasks.

## Dependencies

Landing order across the ten proposed changes: (1) ci-workflow-hardening, (2) task-check-entrypoint,
(3) golangci-lint-bump, (4) cross-file-consistency-gates, (5) developer-toolchain-pins,
(6) dependency-update-automation, (7) ci-failure-reporting, (8) test-depth-signals,
(9) mutation-testing-signal, (10) public-agent-contract.
Hard dependency: change 7 must have shipped.

## Non-goals

- A mutation-score threshold or any gate (needs a baseline from several nightly reports; a later
  change decides it).
- Packages beyond the three; changing any test.

## Assumptions

- gremlins (`github.com/go-gremlins/gremlins`) at one pinned version that supports the Go version
  of `go.mod`; probe in task 1.1 (run it on the three packages: version, runtime, report format).
  If none does, pick go-mutesting and record why.
- Gremlins reports code that no test executes as NOT COVERED, never LIVED. The control therefore
  needs a function the test EXECUTES but does not assert on.
- The three packages: pure (no Docker, no network), small enough for a nightly budget, and
  correctness- or security-critical: `aggregate` (content hashing and dedupe, fuzzed by
  `FuzzContentHash`), `redact` (secret scrubbing), `pathvalidate` (path safety).
