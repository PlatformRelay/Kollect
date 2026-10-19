# Stage B round 1: holistic run

Revision: `73e92c8c4` (branch `feat/public-agent-contract`), 2026-10-19.

## Base

- `git fetch origin`: `origin/main` did not move; it is still `e9a9779c5`, the merge base of the
  branch.
- `git merge-tree --write-tree origin/main HEAD`: clean (exit 0, no conflict).

## `task check`

Run with `umask 022`, a real `kubectl` binary first on `PATH` (not the shell's lazy wrapper
function) and `make operator-sdk` run first so `bin/operator-sdk` exists. Started in the
background and waited on by PID in the foreground.

| Item | Result |
|---|---|
| Gates (17, `hack/check.sh` `run_gate`) | all `ok`: verify, format:check, lint:shell, lint:markdown, lint, vulncheck, test, scrub, spec:validate, audit-rbac, build, helm, go-mod, gitleaks, gitleaks-detect, workflow-security, workflow-security-audit |
| Guard sweep | every `hack/test/*_test.sh` and `lab_harness_meta_suite.sh` passed, `--self-test` included where declared; `agent_contract_test.sh` ran plain and `--self-test`, both ok |
| Skipped guard | `integration_no_docker_test.sh` (its header declares Docker), with the printed reason |
| Exclusion table | test-integration, kind-smoke, docker-build, pipeline-cli-smoke, dependency-review, each with its reason |
| Summary line | `task check: all runnable gates passed` |
| `FAILED` lines in the log | 0 |
| Wall time | 2715 s |

`docs:verify` is not one of the `task check` gates, so the MkDocs-plugin caveat did not come up.

## Per-commit gate

`hack/test/agent_contract_test.sh` is absent on the first four commits of the branch and present
only on `73e92c8c4`, where the plain run is green (`ok - agent contract holds (PAC-1 to PAC-4,
PAC-6)`, 0.9 s, running-host check active). Verdict: absent or green on every commit.
