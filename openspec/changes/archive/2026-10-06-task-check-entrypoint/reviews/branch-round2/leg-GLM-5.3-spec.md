The round-one diff shows both a full and a partial `go-mod` gate were added. Now let me check the collateral effect of the whole-file Docker grep on the guard sweep, and the CI wiring.
Confirmed a self-reference issue: `task_check_test.sh:121` contains the literal string `requires docker`, which check.sh's whole-file grep matches. Let me verify the CI wiring and the remaining changed files.
The parent had no `go-mod` gate at all — the fix commit added it twice (full at line 50, weaker duplicate at line 62). Now let me confirm the CI job context for TCE-4 and the rewritten CWS-2 awk check.
The CWS-2 change is a de-duplication (parent had the yq rules check twice) — the check itself survives. Now the preflight job definition (to validate the `coverage: preflight=...` mapping's honesty), the guard list, and the openspec artifacts.
The tree has only an untracked round-2 review directory (orchestrator artifact, out of the range). Now the preflight job, which the coverage mapping claims to mirror:
Preflight's real module-graph step checks **go.sum only** — so check.sh line 50 is stronger than CI, and line 62 duplicates it as the exact CI-shape. Let me verify the false-skip empirically, check bash-version compatibility of the new guard, and read the openspec artifacts.
Evidence and tasks look consistent except for a duplicated paragraph and a stale "only ci_workflow_security parses --self-test" claim. Final empirical passes: syntax-check both scripts and run the meta-test in plain mode (read-only).
## Verdict: BLOCK
One CRITICAL: the round-one fix's Docker-skip mechanism contradicts TCE-1 for the very guard that pins the gate (verified by grep, not by reading prose). Everything else holds; two warnings are fix-round debris.

## Findings
- [CRITICAL] The guard sweep skips its own meta-test with a false reason — `hack/check.sh:73`
  Failure: the sweep greps each guard's whole file for `requires docker`; `task_check_test.sh:121` (and :313-314) contain that literal inside their own assertions, so `task check` prints `excluded guard task_check_test.sh ... (declared in its header)` and skips it — its header declares no Docker requirement. TCE-1's "runs every `hack/test` guard in every mode its code declares (including `--self-test`)" is contradicted for exactly the guard that pins TCE-1..TCE-3; the printed reason is false, so the omission reads as legitimate. Any future guard that merely mentions the phrase in test strings inherits the same silent skip, eroding the "new guard appears → it runs" scenario.
  Fix: match the declaration only in the guard's header block (first N lines or a strict `# Requires Docker:` marker), reword the self-referential literals at task_check_test.sh:121,313-314 so they cannot match, and update the mutant's `sed` at :313 to the new shape.
  Confidence: 97 (grep match reproduced; sweep list and skip path read in full)
- [WARNING] The `go-mod` gate is declared and executed twice — `hack/check.sh:62` duplicates `hack/check.sh:50`
  Failure: both lines were added by c186cd71 (parent 45f3f756 had neither); every `task check` runs `go mod tidy` twice, and on go.mod drift the same gate name prints FAILED (line 50) then `ok` (line 62) — two verdicts for one name. The weaker duplicate is pinned by nothing (the meta-test's exact-command pin matches only line 50).
  Fix: delete line 62; the `coverage: preflight=...,go-mod,...` comment stays valid via line 50.
  Confidence: 95
- [WARNING] The go-mod gate is stricter than the CI gate it claims to mirror — `hack/check.sh:49` vs `.github/workflows/preflight.yaml:54`
  Failure: the comment says "the same tidy + verify + go.sum drift check the Preflight workflow runs", but preflight checks `go.sum` only; a contributor with an untidy go.mod sees preflight green and `task check` red (stronger-than-spec; a legitimate caller passes CI but fails the "same" local gate).
  Fix: drop `go.mod` from the `git diff --exit-code` at check.sh:50 for parity, or reword the comment to say the local gate is deliberately stricter.
  Confidence: 90
- [NOTE] The round-one rewrite dropped `format:check` from the guard's pin — `hack/test/task_check_test.sh:71-86`
  Failure: the parent's RUNNABLE list pinned `format:check`; the new `GATE_COMMAND` does not, so removing `run_gate format:check task format:check` (check.sh:37, still present) no longer reds the meta-test. The spec scenario does not name format:check, so this is guard-strength shrinkage, not a spec violation.
  Fix: add `[format:check]="task format:check"` to GATE_COMMAND.
  Confidence: 85
- [NOTE] `task_check_test.sh:71` uses `declare -A` (needs bash ≥4); macOS's default bash 3.2 rejects it — first bash-4-only script in `hack/test`. Today masked by the CRITICAL skip; fixing the skip exposes `task check` to it on macOS.
  Fix: indexed parallel arrays, or an explicit bash-version precondition. Confidence: 80
- [NOTE] The change's own record is inaccurate in two trivial ways — `openspec/changes/task-check-entrypoint/evidence/evidence.md:17`
  Failure: the exclusions paragraph is duplicated verbatim; tasks.md:5-7 still claims "only `ci_workflow_security_test.sh` parses `--self-test`" — false on this branch (task_check_test.sh parses it too; verified the sweep's detection yields exactly two guards).
  Fix: dedupe the paragraph, update the note. Confidence: 90

Per-requirement verdicts against the spec: TCE-1 partial (gate list, exclusions-with-reason, failure aggregation all hold per check.sh:36-56,100-104,107-110; the guard-sweep promise is contradicted for one guard); TCE-2 holds (Taskfile verify untouched; pinned at task_check_test.sh:143-155); TCE-3 holds (all 14 `required_checks` from verify-eligibility.sh:19-22 accounted: run, excluded with reason, or mapped; mapping validated against preflight.yaml's real steps); TCE-4 holds (7 mutants + no-op control; wired plain and `--self-test` into the required, changes-ungated `lint` job at ci.yaml:440-447). Unrequested extras: format:check, audit-rbac, build, helm, gitleaks/zizmor installer gates beyond the scenario list (consistent with "every required gate"); an exclusion printed for `dependency-review`, which is not a required check.

## Could not check
- Did not run `task check` end-to-end (downloads pinned tools, envtest, helm; not read-only-safe) — the "everything runnable runs" scenario is verified statically (gate list vs `required_checks`, exclusion table, glob sweep over all 76 guards) plus `bash -n`, not by execution.
- Did not run `task_check_test.sh --self-test`: it mutates throwaway copies under `mktemp -d`, which crosses the read-only line I was given; mutant machinery was reviewed statically and the evidence claims it green locally.
- ShellCheck on the new scripts, and the actual execution of the two new ci.yaml lint-job steps (assumed sound from the identical CWS-7 pattern and the yq install above them; not run).
- Whether any guard needs Docker without declaring it (would red TCE-1 on a no-Docker machine) — would require running the sweep.
