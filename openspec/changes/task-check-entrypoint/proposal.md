# Proposal

## Why

There is no single local command that equals CI. `task verify` means "generated artifacts are
fresh" (`Taskfile.yml:111`, `hack/verify.sh`), not "run every gate", and there is no `task check`.
The operator wants the same entry-point names in every project. Every guard added by the later
changes must also be wired into the local gate, or it can pass in CI and be forgotten locally.

## What Changes

- `task check` runs the full local gate: everything CI requires that can run locally (verify,
  lint, unit tests, vulncheck, scrub, shell and markdown lint, the `hack/test` guards in every
  mode, spec validation), and prints which required gates it leaves out and why (those that need
  Docker or kind: `test-integration`, `kind-smoke`, `docker-build`).
- `task verify` keeps its one meaning, generated-artifact drift, and is not redefined.
- A guard test fails when a required CI job is neither reachable from `check` nor listed as an
  exclusion with a reason.

## Capabilities

### New Capabilities

- `task-entrypoints`: the standard task names and what each promises.

### Modified Capabilities

None.

## Impact

- Entry point: `Taskfile.yml` (`check`), the `lint` job running `hack/test/task_check_test.sh`.
- Docs: `CONTRIBUTING.md`, `docs/COMMAND-REFERENCE.md`.

## Dependencies

Landing order across the ten proposed changes: (1) ci-workflow-hardening, (2) task-check-entrypoint,
(3) golangci-lint-bump, (4) cross-file-consistency-gates, (5) developer-toolchain-pins,
(6) dependency-update-automation, (7) ci-failure-reporting, (8) test-depth-signals,
(9) mutation-testing-signal, (10) public-agent-contract. Needs change 1 first so the new guard runs
in a required job.

## Non-goals

- Renaming `task verify`, or making it a fast subset.
- Running Docker or kind gates inside `check` (they stay separate, named tasks).

## Assumptions

- `verify-eligibility.sh` `required_checks` is the list of CI gates (`gitleaks verify audit-rbac
  vulncheck lint test build test-integration helm docker-build preflight kind-smoke
  pipeline-cli-smoke`). Source: `hack/release/verify-eligibility.sh:19-22`.
- `hack/test/*_test.sh` can be run by glob; each guard's modes are declared in the script header.
  Probe in task 1.1.
