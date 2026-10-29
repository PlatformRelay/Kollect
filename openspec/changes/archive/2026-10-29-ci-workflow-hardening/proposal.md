# Proposal

## Why

Probed on this tree:

- `ci.yaml` has no `concurrency:` block, so every push to a PR branch leaves the previous run
  burning runner minutes. `e2e-smoke.yaml:30-33`, which produces the required `kind-smoke`
  context, groups pushes to `main` by `github.ref`, so quick successive merges can lose the
  middle commit's run (see design.md).
- No static analysis looks at the workflows themselves, and nothing reviews dependency changes
  on a PR.
- The `protect-main` ruleset requires only `preflight`, `test`, `kind-smoke` and `Analyze (Go)`.
  `lint`, `vulncheck` and any job added here can be red on a merged PR (`test` needs only
  `changes` and `test-suite`), yet `hack/release/verify-eligibility.sh:19-22` demands them green
  on the release SHA. A guard that cannot block a merge is not a gate.

The reference project attune (`.github/workflows/ci.yaml`) runs `zizmor`, `dependency-review` and
cancels superseded PR runs.

## What Changes

- A new `workflow-security` job in `ci.yaml` runs `zizmor --offline` over `.github/` with a pinned
  version and `--min-severity`, and a committed config, `.github/zizmor.yml`, in which each
  suppression carries its reason. Existing findings are fixed, not suppressed.
- A new `dependency-review` job (pull requests only) runs `actions/dependency-review-action`
  with its default severity threshold, a licence policy through `allow-licenses`, and unknown
  licences reported, not failed.
- `concurrency:` in `ci.yaml`, and the group fix in `e2e-smoke.yaml`.
- The jobs `lint`, `vulncheck`, `workflow-security` and `dependency-review` become required
  checks (ruleset, operator-owned), so every guard in `lint` can block a merge.
- `hack/test/ci_workflow_security_test.sh` proves each of the above stays wired and can fail, and
  that each new guard script runs in a required job.

## Capabilities

### New Capabilities

- `ci-workflow-security`: the supply-chain guarantees of the CI workflows, which gates must be
  able to block a merge, and which triggers must never be removed.

### Modified Capabilities

None.

## Impact

- Entry points: `.github/workflows/ci.yaml` (jobs `workflow-security`, `dependency-review`, the
  `concurrency:` block), `.github/workflows/e2e-smoke.yaml`, the `lint` job running the meta-test,
  and the repository ruleset.
- `hack/release/verify-eligibility.sh` `required_checks` gains `workflow-security`, not
  `dependency-review` (PR-only, no exact-SHA result on main).
- Any later change that edits workflow steps must grep `hack/test/` for step-index assertions
  (`steps[0]`, `steps | length`, `steps[N]`; for example `hack/test/dist_operatorhub_pr_test.sh:146`
  and `ci_docs_gate_test.sh`) and update them in the same PR.

## Dependencies

Landing order across the ten proposed changes: (1) ci-workflow-hardening, (2) task-check-entrypoint,
(3) golangci-lint-bump, (4) cross-file-consistency-gates (with the Go bump),
(5) developer-toolchain-pins, (6) dependency-update-automation, (7) ci-failure-reporting,
(8) test-depth-signals, (9) mutation-testing-signal, (10) public-agent-contract. This change is
first because it makes the later guards blocking. Operator prerequisite: the ruleset edit in
tasks section 5.

## Deferred

`step-security/harden-runner` is deferred. In audit mode it gates nothing, adds a privileged
third-party agent to jobs that hold tokens, and its telemetry has no reader. Revival precondition:
a written block-mode egress allowlist plan.

## Non-goals

- Removing the push-to-main CI trigger (see design.md).
- Fixing more zizmor findings than task 3.2 plans.

## Assumptions

- `zizmor --offline` runs without network or token and exits non-zero on findings at or above
  `--min-severity`. Source: zizmor docs, "Usage"; probe in task 3.1 on the pinned release (version
  >= 1.30.1, exact pin chosen at implementation).
- `actions/dependency-review-action` needs the dependency graph, on for GitHub-hosted public
  repositories; `allow-licenses` and `allow-dependencies-licenses` are its supported licence
  inputs and `deny-licenses` is deprecated. Probe on a throwaway PR in task 4.2.
- A job skipped by its `if:` reports `skipped`, which a required check treats as passing
  (`vulncheck` is gated on `changes.code`). Probe in task 5.1.
- `hack/release/verify-eligibility.sh` reads exact-SHA results on `main` (`required_checks`).
- A rule `require_extra_approval_for_unattributed_changes` in the ruleset could change how bot PRs
  merge; probe in task 5.2 (read the ruleset JSON).
