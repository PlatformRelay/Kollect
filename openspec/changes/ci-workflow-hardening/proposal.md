# Proposal

## Why

`ci.yaml` has no `concurrency:` block, so every push to a PR branch leaves the previous run
burning runner minutes (the other workflows already cancel superseded PR runs, for example
`e2e-nightly.yaml:17-20`). No workflow audits the runners' network egress, no static analysis
looks at the workflows themselves, and nothing reviews dependency changes on a PR. A comparison
with the OSS project attune (`.github/workflows/ci.yaml`) showed that it closes these gaps with
`step-security/harden-runner`, `zizmor --offline` and `actions/dependency-review-action`.

## What Changes

- Every job in every workflow starts with `step-security/harden-runner` in `egress-policy: audit`.
- A new `workflow-security` job in `ci.yaml` runs `zizmor --offline` against `.github/` with a
  committed config, `.github/zizmor.yml`, in which every suppression carries its reason.
- A new `dependency-review` job (pull requests only) runs `actions/dependency-review-action`
  with `fail-on-severity: high`.
- `ci.yaml` gets `concurrency:` that cancels superseded runs of pull-request refs only.
- `hack/test/ci_workflow_security_test.sh` proves each of the above stays wired and can fail.

## Capabilities

### New Capabilities

- `ci-workflow-security`: what the CI workflows must guarantee about their own supply chain,
  and which CI triggers must never be removed.

### Modified Capabilities

None.

## Impact

- Entry point: `.github/workflows/ci.yaml` (jobs `workflow-security`, `dependency-review`, the
  `concurrency:` block) and the `hack/test/ci_workflow_security_test.sh` meta-test, which runs
  in the existing `lint` job next to `ci_docs_gate_test.sh`.
- `hack/release/verify-eligibility.sh` `required_checks` gains `workflow-security`. It does NOT
  gain `dependency-review`, which only runs on pull requests and so has no exact-SHA result on
  main.
- Making the new jobs required status checks is a repository-ruleset change owned by the
  operator (see tasks 5.x).

## Non-goals

- Switching harden-runner to `block` mode. Block needs an allowed-endpoints list built from
  audit data; that is a follow-up once audit runs exist.
- Removing the push-to-main CI trigger (see design.md).
- Fixing findings zizmor reports beyond what task 3.2 plans.

## Assumptions

- `zizmor --offline` runs without network access and without a GitHub token, and exits non-zero
  on findings. Source: zizmor docs, "Usage"; probe in task 3.1 on the pinned release.
- `step-security/harden-runner` only supports GitHub-hosted Linux runners here; all jobs use
  `ubuntu-latest` (`ubuntu-latest-8-cores` in one dispatch-only job, which is also Linux).
  Probe: `grep -h "runs-on:" .github/workflows/*.yaml | sort -u` in task 1.1.
- `actions/dependency-review-action` needs the repository's dependency graph, which is on for
  GitHub-hosted public repositories. Probe in task 4.2.
- The required check `test` and `verify-eligibility.sh` read exact-SHA results on `main`
  (`hack/release/verify-eligibility.sh`, `required_checks`).
