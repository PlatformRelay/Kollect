# Proposal

## Why

golangci-lint is v2.11.4 in `Makefile:184` and, separately, in `hack/tooling/.custom-gcl.yml:6`
(two places, no check). The operator's baseline is v2.13.1. The next change moves `go.mod` to a
newer Go; golangci-lint refuses to lint a module whose `go` directive is newer than the Go that
built the linter, so the linter must move first. A bump that reveals findings must not loosen the
linter to absorb them.

## What Changes

- Raise golangci-lint to v2.13.1 in `Makefile` and `hack/tooling/.custom-gcl.yml` together
  (version-only commit).
- Fix each new finding, or justify it in `.golangci.yaml` with a reason, in separate commits.

## Capabilities

### New Capabilities

- `lint-toolchain`: the linter version rule and the rule that a bump does not weaken the linter.

### Modified Capabilities

None.

## Impact

- Entry points: `make golangci-lint` / `task lint` (`lint` job in `ci.yaml`),
  `hack/tooling/.custom-gcl.yml` (custom build with plugins).

## Dependencies

Landing order across the ten proposed changes: (1) ci-workflow-hardening, (2) task-check-entrypoint,
(3) golangci-lint-bump, (4) cross-file-consistency-gates, (5) developer-toolchain-pins,
(6) dependency-update-automation, (7) ci-failure-reporting, (8) test-depth-signals,
(9) mutation-testing-signal, (10) public-agent-contract. Must land before change 4.

## Non-goals

- Enabling new linters; the Go bump (change 4); drift guards for the pin (change 5 covers
  agreement and floor).

## Assumptions

- golangci-lint v2.13.1 exists (operator baseline). `.golangci.yaml` is `version: "2"`; v2.13.1
  may change defaults or deprecate options. Probe: run `task lint` after the bump (task 1.1) and
  record the findings count.
- `hack/tooling/.custom-gcl.yml` `version:` must equal the version it builds on; probe that a
  mismatch fails or misbehaves (task 1.1).
