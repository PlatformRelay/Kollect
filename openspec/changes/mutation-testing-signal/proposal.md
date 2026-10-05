# Proposal

## Why

Mutation testing is a process rule (`docs/development/spec-workflow.md:51`, the risk table:
deletion, tenancy, credentials need "a mutation or negative control") with no measurement: nobody
can say which packages' tests actually catch mutants. The reference project attune measures it
nightly (report only).

## What Changes

A nightly job `mutation-report` in `e2e-nightly.yaml` runs `gremlins` on `internal/aggregate`,
`internal/redact` and `internal/pathvalidate`, uploads the report and writes killed, lived and
not-covered counts per package to the job summary. It is a measurement, explicitly not a gate.

## Capabilities

### New Capabilities

- `mutation-testing`: what the nightly mutation measurement must report and must not do.

### Modified Capabilities

None.

## Impact

- Entry point: job `mutation-report` in `.github/workflows/e2e-nightly.yaml`, and
  `hack/ci/mutation-summary.sh` (formatter, tested on a recorded report).
- Tool version pinned once in `Taskfile.yml` (`GREMLINS_VERSION`, Renovate annotation).

## Dependencies

Landing order across the eight proposed changes: developer-toolchain-consistency (with its Go bump), cross-file-consistency-gates, ci-workflow-hardening, dependency-update-automation, nightly-failure-reporting, test-depth-signals, mutation-testing-signal, public-agent-contract.
(Pinned-tool annotations follow the toolchain change; the nightly gets reported by
nightly-failure-reporting.)

## Non-goals

- A mutation-score threshold or any gate. A threshold needs a baseline from several nightly
  reports; a later change decides it.
- Packages beyond the three; changing any test.

## Assumptions

- gremlins (`github.com/go-gremlins/gremlins`) runs on the Go version in `go.mod` and reports
  killed/lived/not-covered. Probe in task 1.1; if it does not, pick go-mutesting and record why.
- The three packages: pure (no Docker, no network), small enough for a nightly budget, and
  correctness- or security-critical: `aggregate` (content hashing and dedupe, fuzzed by
  `FuzzContentHash`), `redact` (secret scrubbing), `pathvalidate` (path safety). The git sink
  prune path qualifies on value but needs git and is slow; revisit once timings exist.
