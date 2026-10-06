## Unified verdict: BLOCK   (legs ok: 4/4)

| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | CRIT↑ | evidence.md:47-51 + commit dd90e5a9 claim a removed duplicate yq block and a CWS-6 spec amendment; the duplicate survives verbatim and no spec file is in `origin/main..HEAD` | evidence.md:47-51; ci_workflow_security_test.sh:253-265 | GLM,DSV,QWN (3) | G-fit,D-adv,Q-sec | 100 |
| 2 | CRIT | guard sweep runs a Docker-hard-requiring guard unconditionally, so `task check` exits 1 on exactly the no-Docker machine TCE-1 Scenario 1 promises green | hack/check.sh:62; integration_no_docker_test.sh:22-24,43 | GLM,DSV (2) | G-fit,D-adv | 100 |
| 3 | WARN↑ | `--self-test` mode detected by grepping code, not header declaration as the spec words it; mis-invocation noise possible on future guards | hack/check.sh:66-68 | GLM,DSV,QWN (3) | G-spec,G-fit,D-adv,Q-sec | 100 |
| 4 | WARN↑ | TCE-2 superset redefinition of `verify` (verify.sh + extra cmds) stays green; guard only pins presence of `bash hack/verify.sh` | hack/test/task_check_test.sh:119-128 | GLM,DSV (2) | G-spec,G-fit,D-adv | 100 |
| 5 | WARN | CWS-2 comment-walk bypassed by flow-inline and quoted-key spellings of `disable: true` (awk pass verified by execution) | ci_workflow_security_test.sh:286-297 | QWN (1) | Q-sec | 90 |
| 6 | WARN | gate pins are name-only: `run_gate verify true` survives plain mode and all mutants; only gitleaks/zizmor bodies are pinned | task_check_test.sh:88-92,137-140 | DSV (1) | D-adv | 90 |
| 7 | WARN | CI preflight's `go mod tidy` + go.sum diff + `go mod verify` are neither run nor mapped by `task check`; guard can't see the drift | hack/check.sh:57; preflight.yaml:51-56 | GLM (1) | G-spec | 85 |

## Disagreements
- Verdict split 2-2 (BLOCK vs CONCERNS): G-fit/D-adv escalate the Docker guard to CRITICAL; G-spec ran the guard plain-mode green and judged TCE-1 "holds (partial)" without flagging it.
- G-spec judged TCE-3/TCE-4 fully holding and mutant-verified; D-adv says TCE-4's mutant requirement for TCE-1..TCE-3 is unmet (no mutant for a required check neither run nor excluded).

## Nobody could check
- Full `task check` end-to-end — no leg ran it (writes coverage.out/bin/); the dockerless path behind #2 was never executed by any leg or the author.
- Both guards' `--self-test` executions (mktemp writes) — mutant logic read, never run.
- Zizmor's real schema for rule-level `disable:` incl. flow/quoted forms — no binary available; the CWS-2 bypass claim rests on the branch's own approval-round text.
- Whether `dependency-review` is branch-protected (excluded but not required), and whether the `coverage: preflight=` mapping matches the real preflight job (no CI run visible).
