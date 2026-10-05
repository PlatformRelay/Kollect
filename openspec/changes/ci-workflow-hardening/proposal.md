# Proposal

## Why

`ci.yaml` has no `concurrency:` block, so every push to a PR branch leaves the previous run
burning runner minutes (`codeql.yaml:18-21` and `e2e-smoke.yaml:30-33` already cancel superseded
PR runs). `e2e-smoke.yaml`, which produces the required `kind-smoke` context, groups pushes to
`main` by `github.ref`, which has a second defect (see design.md). No workflow audits the runners' network egress, no static analysis
looks at the workflows themselves, and nothing reviews dependency changes on a PR. A comparison
with the OSS project attune (`.github/workflows/ci.yaml`) showed that it closes these gaps with
`step-security/harden-runner`, `zizmor --offline` and `actions/dependency-review-action`.

## What Changes

- Every job in every workflow starts with `step-security/harden-runner` in `egress-policy: audit`,
  except the reporter and classifier jobs whose step count `hack/test/ci_docs_gate_test.sh` pins
  (`test` has exactly 1 step and `changes` exactly 2, in `ci.yaml` and `e2e-smoke.yaml`): a step
  there could write `$GITHUB_ENV` and turn a required context green.
- A new `workflow-security` job in `ci.yaml` runs `zizmor --offline` against `.github/` with a
  committed config, `.github/zizmor.yml`, in which every suppression carries its reason.
- A new `dependency-review` job (pull requests only) runs `actions/dependency-review-action`
  with `fail-on-severity: high`.
- `ci.yaml` gets `concurrency:` that cancels superseded pull-request runs and gives every push to
  `main` its own group (keyed by SHA). `e2e-smoke.yaml` gets the same fix to its group.
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

## Dependencies

Landing order across the eight proposed changes: developer-toolchain-consistency (with its Go bump), cross-file-consistency-gates, ci-workflow-hardening, dependency-update-automation, nightly-failure-reporting, test-depth-signals, mutation-testing-signal, public-agent-contract. This change and the
toolchain change both edit every job and both meta-tests parse the workflows, so they land
sequentially, not in parallel.

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
