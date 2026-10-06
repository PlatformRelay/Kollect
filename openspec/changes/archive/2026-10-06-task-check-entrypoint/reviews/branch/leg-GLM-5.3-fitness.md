## Verdict: BLOCK

## Findings

- [CRITICAL] `task check` contradicts TCE-1's no-Docker scenario: the guard sweep runs a Docker-required guard unconditionally — `hack/check.sh:62-67`
  Failure: contributor without Docker runs `task check` → the sweep reaches `hack/test/integration_no_docker_test.sh:24` (`command -v docker || fail`) and `:43` (real `docker run`, twice) → guard reds, `check` exits 1 although every local-runnable required gate passed; the exclusion table promises Docker-dependent gates "printed as not run", yet the run is unusable red on exactly the machine class the spec scenario names.
  Fix: in the sweep, condition the guard on docker presence and print it via `exclusion` with a reason (or drop it from the glob and exclude it explicitly); add a self-test direction pinning dockerless-green.
  Confidence: 95

- [WARNING] evidence claims carried CWS work that is not in this range — `openspec/changes/task-check-entrypoint/evidence/evidence.md:47-51`
  Failure: evidence says the branch carries "the removed duplicate yq block" and "the spec wording amendment"; no spec file changes in the range and the duplicated yq block survives verbatim in HEAD (`hack/test/ci_workflow_security_test.sh:240-252` is byte-identical to `:253-265`, present on origin/main too) — the approval-round finding reads as fixed when it is not.
  Fix: delete the duplicate block (`:253-265`) and correct or evidence the spec amendment claim.
  Confidence: 92

- [NOTE] exclusion register is born with an entry no requirement asks for — `hack/check.sh:90` vs `hack/release/verify-eligibility.sh:19-22`
  Failure: `dependency-review` is excluded but absent from `required_checks` (14 names, all else covered — I mapped each of the 14 to a run_gate, exclusion, or the `coverage: preflight=` line). Not a TCE-3 failure, but exclusion registers shrink only; an unrequired exclusion invites table rot.
  Fix: drop the line or cite the PR-context it guards in its reason.
  Confidence: 80

- [NOTE] TCE-2's "superset" half is unasserted — `hack/test/task_check_test.sh:121-127`
  Failure: the guard reds only when `bash hack/verify.sh` disappears from `verify.cmds`; a superset redefinition (`verify.sh` + extra cmds, which TCE-2 forbids) stays green, so the mutant suite overstates TCE-2 coverage.
  Fix: assert the cmd list has exactly one entry (`hack/verify.sh`).
  Confidence: 70

- [NOTE] sweep coverage rests on a filename convention with one hardcoded carve-out, and nothing fails when a new script misses it — `hack/check.sh:62`, `hack/test/task_check_test.sh:97`
  Failure: a future `hack/test/foo_suite.sh` (not `*_test.sh`, not `lab_harness_meta_suite.sh`) is silently never run by `task check` and no guard reds; the mutant only pins the glob string.
  Fix (smallest ratchet from today's inventory): add one assertion to `task_check_test.sh` failing when any executable `hack/test/*.sh` matches neither `*_test.sh` nor the sweep list.
  Confidence: 80

- [NOTE] `--self-test` mode detection is a grep heuristic — `hack/check.sh:68`
  Failure: any future guard that merely mentions the string in non-comment code (without parsing the flag) gets `--self-test` passed as `argv[1]`; currently correct for exactly the two parsers (I enumerated all 73 sweep members).
  Fix: keep a tiny registry of parsing guards or a `# self-test:` header convention instead of code-grep.
  Confidence: 55

Fitness direction, positively: the range tightens CI↔local coupling into an enforced ratchet — CWS-6 pins guards→required jobs, the new TCE-3 loop pins `verify-eligibility.sh:19-22`→`check.sh`/exclusions, and TCE-1's own mutants plus no-op control prove the guard can fail (plain mode green — I ran it; ShellCheck clean on both new scripts — I ran it; CWS guard plain green — I ran it). CWS-3's allow-list shrank (`shell` removed), correct direction.

## Could not check

- A full `task check` run (executes gates; not read-only here) — evidence itself defers it to post-merge, so the CRITICAL's dockerless path was never exercised by the author either.
- Either guard's `--self-test` (writes mktemp copies); I verified the mutant logic by reading and the plain modes by running.
- `task lint:markdown` / `task lint` on the edited `CONTRIBUTING.md`/`docs/COMMAND-REFERENCE.md` (mise/golangci toolchain unavailable in this environment).
- `openspec/changes/task-check-entrypoint/reviews/` contains only a `branch` file; no review record outside this repo was read.
