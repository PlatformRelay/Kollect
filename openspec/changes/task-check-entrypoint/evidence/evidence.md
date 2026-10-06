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
- Exclusions with reasons: test-integration (Docker), kind-smoke (kind cluster),
  docker-build (Docker daemon), pipeline-cli-smoke (kind cluster + CLI image),
  dependency-review (GitHub pull-request context). Guards whose header declares a Docker
  requirement (today `integration_no_docker_test.sh`) are skipped in the guard sweep with the
  same printed-reason contract.
- The `coverage: preflight=lint:markdown,go-mod,verify,guard-sweep` line maps the required
  preflight context to the gates above (preflight runs task lint:markdown + task verify + the
  commit-identity guard + the go mod tidy/verify half, all of which the check runs).

## hack/test/task_check_test.sh (TCE-1..TCE-4 meta-test)

Plain mode: green (task check exists, the gate list is pinned, the sweep is the glob,
aggregation exits non-zero, verify keeps its meaning, every required check is run or excluded
with a reason).

`--self-test`: no-op control green; 7 mutants each rejected with the exact message of the
assertion the mutation was built to trip:
- TCE-1 gate removed (`vulncheck` deleted from the run list) → the gate-list check reds
- TCE-1 gate command swapped for a no-op (`run_gate verify true`) → the exact-command pin reds
- TCE-1 guard sweep hard-coded (glob replaced by a single script) → the must-sweep-the-glob check reds
- TCE-1 Docker guard swept unconditionally → red (task check would exit 1 on exactly the
  no-Docker machine TCE-1 promises to serve; the guard's header declares the requirement and
  hack/check.sh skips it with a printed reason)
- TCE-2 `verify` redefined to `task lint` → red
- TCE-2 `verify` redefined as a superset (its own command plus `task lint`) → red (the
  exact-cmds-length pin; a superset is still a redefinition)
- TCE-3 exclusion without reason → red naming the missing reason

## The pre-wiring red (TCE-1's "watch it fail on the missing task")

Before the Taskfile had the `check` task, the guard reds with
`FAIL: TCE-1: Taskfile.yml has no check task -- there is no single local command that equals CI; add one that runs hack/check.sh`.

## Carried-over hardening from the merged ci-workflow-hardening change

This branch also carries the approval-round leftovers for change ci-workflow-hardening
(merged as PR #450): the CWS-2 policy-switch check (`disable: true` under a rule needs its
justification comment — the bypass the approval legs found, plus its mutant), the duplicated
yq type/flow-style block actually REMOVED this time (the previous claim was unbacked until
this branch: the duplicate survived the merge), and the TCE-1 wording above. Recorded here so
the reviewer reads both changes in one diff.
