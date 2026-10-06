## Verdict: BLOCK

## Findings
- [CRITICAL] The guard sweep runs a guard that hard-requires Docker, so `task check` reds on the exact no-Docker machine TCE-1 promises — `hack/check.sh:62` + `hack/test/integration_no_docker_test.sh:22`
  Failure: contributor without Docker runs `task check` → sweep reaches `integration_no_docker_test.sh` bare → `command -v docker … || fail "docker is needed on the host"` → `failures+=` → exit 1. It contradicts TCE-1 Scenario 1 ("on a machine without Docker … runs … and prints the Docker-dependent gates as not run") and the exclusion table's claim that Docker gates are the only ones left out. The guard's own header says its only entry point is `task test-integration:no-docker` (needs Docker); it is wired nowhere else.
  Fix: let guards declare a skip (e.g. a `# check: needs-docker` header the sweep honours) and skip `integration_no_docker_test.sh`; or drop it from the sweep and list it as an exclusion with its reason.
  Confidence: 95
- [WARNING] The gate pin is name-only: `c_tce1_gates`/`c_tce3_coverage` grep `run_gate <name> ` and never the command that follows — `hack/test/task_check_test.sh:88-92`, `:137-140`
  Failure: `sed -i 's/run_gate verify task verify/run_gate verify true/' hack/check.sh` survives plain mode AND every self-test mutant, so the "full local gate" can be gutted gate-by-gate while the guard stays green. Only the gitleaks and zizmor bodies are pinned (`:93-96`).
  Fix: assert the exact full line, e.g. `run_gate verify task verify`, per gate.
  Confidence: 90
- [WARNING] Commit `dd90e5a9` and `evidence.md` claim two changes the diff does not contain — `hack/test/ci_workflow_security_test.sh:253-265`
  Failure: the message says "removed the duplicated yq type/flow-style block in cws2_suppressions" and "the CWS-6 spec text is amended", but `git show --stat dd90e5a9` is one file with no such deletion, the duplicate block remains verbatim at lines 253-265, and no spec file appears in `origin/main..HEAD`. A reviewer trusting the evidence gets a false picture.
  Fix: either delete the duplicate block and land the spec amendment, or correct the commit/evidence text to what was actually done.
  Confidence: 95
- [NOTE] TCE-2's guard does not catch a superset redefinition — `hack/test/task_check_test.sh:121-127`
  Failure: `verify` changed to `bash hack/verify.sh && task lint` still greps as containing `bash hack/verify.sh` and keeps its desc, so it passes, though TCE-2 forbids a superset. Only removal/`task lint`-only trips it.
  Fix: assert the cmds list equals exactly `["bash hack/verify.sh"]`.
  Confidence: 80
- [NOTE] `--self-test` mode is detected by scanning non-comment code, not the "header declares" the spec names — `hack/check.sh:68` vs spec TCE-1:12
  Failure: a guard whose header declares `--self-test` but only in comments is never run with it; a guard that merely mentions the flag in code is invoked with it. Functionally fine for the two current guards, but the pinned contract (`:99`) does not constrain the mechanism.
  Fix: parse a declared mode from the header comment instead of grepping the body.
  Confidence: 70
- [NOTE] No mutant covers TCE-3 Scenario 1 (a required job neither run nor excluded) — `hack/test/task_check_test.sh:282-286` only mutates the reasonless-exclusion direction. TCE-4 asks for mutants "for TCE-1 to TCE-3".
  Fix: add a mutant that inserts a new name into `required_checks` and asserts the "neither run nor listed" red.
  Confidence: 75

## Could not check
- Did not execute `task check` end-to-end (it writes `coverage.out`, `bin/`), so the sweep's real runtime outcome beyond the Docker guard is unverified; I ran only `hack/test/task_check_test.sh` plain (green) — not `--self-test`.
- Read ~20 of the 73 swept guards; could not confirm none of the others needs a network or absent tool.
- Did not verify whether `dependency-review` is a branch-protected required check (it is excluded but absent from `required_checks`).
