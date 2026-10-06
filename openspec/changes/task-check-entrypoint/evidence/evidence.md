# Evidence — task-check-entrypoint (implementation)

Recorded 2026-10-06 on the branch that produced the PR, against origin/main as the base.

## hack/check.sh + Taskfile check task (TCE-1..TCE-3)

- The Taskfile gains `check`, which runs `bash hack/check.sh` — the single local command that
  equals CI.
- hack/check.sh runs, from the repo root: `task verify`, `task format:check`, `task
  lint:shell`, `task lint:markdown`, `task lint` (golangci + arch-lint), `task vulncheck`,
  `task test` (make test self-sets envtest), `task scrub`, `task spec:validate`,
  `task audit:rbac`, `task build`, `task helm-test`, the pinned gitleaks install + the same
  `gitleaks detect --source . --config .github/gitleaks.toml --redact --no-git` invocation as
  CI's gitleaks job, the pinned zizmor + the same offline audit as ci.yaml's
  workflow-security job, and the guard sweep (every `hack/test/*_test.sh` plus
  `lab_harness_meta_suite.sh`, bare, plus `--self-test` where the script's code parses it —
  today only `ci_workflow_security_test.sh`).
- Exclusions with reasons: test-integration (Docker), kind-smoke (kind cluster),
  docker-build (Docker daemon), pipeline-cli-smoke (kind cluster + CLI image),
  dependency-review (GitHub pull-request context).
- The `coverage: preflight=lint:markdown,verify,guard-sweep` line maps the required preflight
  context to the gates above (preflight runs task lint:markdown + task verify + the
  commit-identity guard, all of which the check runs).

## hack/test/task_check_test.sh (TCE-1..TCE-4 meta-test)

Plain mode: green (task check exists, the gate list is pinned, the sweep is the glob,
aggregation exits non-zero, verify keeps its meaning, every required check is run or excluded
with a reason).

`--self-test`: no-op control green; 4 mutants each rejected with the exact message of the
assertion the mutation was built to trip:
- TCE-1 gate removed (`vulncheck` deleted from the run list) → `does not run the 'vulncheck' gate`
- TCE-1 guard sweep hard-coded (glob replaced by a single script) → rejected with the
  must-sweep-the-glob message (a new guard would never be picked up)
- TCE-2 `verify` redefined to `task lint` in the Taskfile → rejected with the
  generated-artifact message
- TCE-3 exclusion without reason → red naming the missing reason

## The pre-wiring red (TCE-1's "watch it fail on the missing task")

Before the Taskfile had the `check` task, the guard reds with
`FAIL: TCE-1: Taskfile.yml has no check task -- there is no single local command that equals CI; add one that runs hack/check.sh`.

## Carried-over hardening from the merged ci-workflow-hardening change

This branch also carries one commit of approval-round leftovers for change
ci-workflow-hardening (merged as PR #450): the CWS-2 policy-switch check
(`disable: true` under a rule needs its justification comment — the bypass the approval leg
found), the removed duplicate yq block, and the spec wording amendment. Recorded here so the
reviewer reads both changes in one diff.
