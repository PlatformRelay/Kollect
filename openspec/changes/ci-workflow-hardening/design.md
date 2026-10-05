# Design

## Context

See proposal.md. The reference project attune removes CI on push to `main`.

## Decisions

- **Push-to-main CI stays. This is a deliberate counterpoint to the reference project.** attune
  squash-merges onto up-to-date branches, so a green PR head is the same tree as the merge
  result and a second run on `main` adds nothing. Kollect rebase-merges (workspace merge policy:
  linear history, no squash), so `main` can contain commits whose tree no PR run ever tested, and
  `hack/release/verify-eligibility.sh` requires exact-SHA green checks on the release commit.
  Dropping the trigger would make every release fail that check or force a manual re-run.
  The spec makes removal a forbidden outcome (CWS-6).
- **Group by PR number, and by SHA on main.** A group keeps one running and one pending run;
  a newer pending run replaces the older pending one even with `cancel-in-progress: false`.
  With a `github.ref` key, three quick merges to `main` would leave the middle SHA with no run,
  and `verify-eligibility.sh` needs a verdict for the exact release SHA (it would also serialize
  main runs). Group: `ci-${{ github.event.pull_request.number || github.sha }}`,
  `cancel-in-progress: ${{ github.event_name == 'pull_request' }}`. `e2e-smoke.yaml:31` has the
  `github.ref` key today and gets the same group shape (it produces the required `kind-smoke`).
  `codeql.yaml` and the e2e-extended/test-e2e groups do not produce required exact-SHA
  contexts and stay as they are.
- **Reporter and classifier jobs are exempt from harden-runner.** `ci_docs_gate_test.sh` pins
  `test` to one step and `changes` to two in both workflows, because an added third-party step
  could write `$GITHUB_ENV` and report a required context green. The exemption is by name, with
  that reason, in the meta-test.
- **harden-runner audit first.** Audit mode records egress and cannot break a job. A later
  block-mode change builds its allowlist from the audit data.
- **zizmor offline, suppressions in a file.** `--offline` avoids a token and network flakiness.
  Findings that need the network (for example impostor-commit checks) are out of reach; that is
  accepted. A suppression lives in `.github/zizmor.yml` with a comment naming the audit, the
  workflow and why it is safe, never as an inline ignore without a reason.
- **`dependency-review` is PR-only.** It compares base and head manifests, which does not exist
  on a push. It is therefore not an exact-SHA release check.

## Risks / Trade-offs

- [Cost: about 46 jobs each get an extra step, and audit mode sends telemetry (egress events) to
  StepSecurity's service from every runner] -> accepted for audit; the privacy side is the reason
  block mode is a separate, reviewed follow-up.
- [harden-runner is a third-party action with a network agent] -> pinned by digest like every
  other action (Renovate `github-actions` manager), audit mode only.
- [zizmor first run reports many findings] -> task 3.2 fixes or justifies each; the gate is not
  switched on until the tree is clean, so main never goes red.
- [Cancelling superseded PR runs hides a flaky failure of the older run] -> acceptable; the
  newest head is what merges. Main runs are never cancelled.
