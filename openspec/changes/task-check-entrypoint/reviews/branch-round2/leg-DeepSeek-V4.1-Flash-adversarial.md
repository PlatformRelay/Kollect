## Verdict: BLOCK

## Findings

- [CRITICAL] `task check` skips `task_check_test.sh` itself via a false Docker match — `hack/check.sh:73` + `hack/test/task_check_test.sh:121,313`
  Failure: the sweep greps the *whole* guard file for `requires docker`; `task_check_test.sh` contains that literal in its own assertion (`:121`) and mutant (`:313`). So `task check` skips the very guard that pins TCE-1..TCE-4, printing a false "needs a Docker daemon on the host" reason. Contradicts TCE-1 ("every guard in every mode") and TCE-4's local verification; the meta-test's own `c_tce1_gates` never notices because it only checks the glob string, not which files match.
  Fix: match only a header marker (e.g. first ~15 lines, or an explicit `# check: needs-docker`), not the whole file.
  Confidence: 95

- [WARNING] Duplicate, unpinned `go-mod` gate — `hack/check.sh:50` and `hack/check.sh:62`
  Failure: `go-mod` is declared twice with different commands: `:50` `git diff --exit-code go.mod go.sum` (the pinned form) and `:62` `git diff --exit-code go.sum` (weaker, unpinned). `go mod tidy`/`go mod verify` run twice per `task check`; a future edit to `:62` (or its removal) is invisible to the guard.
  Fix: delete `:62` (and its orphaned comment) — `:50` already covers the preflight half.
  Confidence: 95

- [WARNING] `format:check` silently dropped from the pinned gate set — `hack/test/task_check_test.sh:71-86`
  Failure: round-one's `RUNNABLE` included `format:check`; the new `GATE_COMMAND` omits it although `hack/check.sh:37` still runs it and it is a step in the required `lint` job (`ci.yaml:284`). Deleting `run_gate format:check` leaves the guard green — a required CI step vanishes from the local gate. A coverage register shrank.
  Fix: add `[format:check]="task format:check"` to `GATE_COMMAND`.
  Confidence: 90

- [WARNING] TCE-3 Scenario 1 still has no mutant — `hack/test/task_check_test.sh:202-223,332-337`
  Failure: TCE-4 requires mutants "for TCE-1 to TCE-3"; the only TCE-3 mutant breaks the reasonless-exclusion direction. Deleting the "neither run nor listed as an exclusion" fail branch in `c_tce3_coverage` leaves `--self-test` green.
  Fix: add a mutant that inserts a new name into `required_checks` and asserts that red.
  Confidence: 80

- [NOTE] Docker-skip recognises only the literal `requires docker`, whole-file — `hack/check.sh:73`
  Failure: other host-tool requirements are not covered; `hack/test/demo_04_samples_kustomize_test.sh:21-28` hard-fails without `kubectl`/`kustomize`, so `task check` still reds on a machine lacking them despite TCE-1's "runs on a developer machine" promise.
  Fix: generalise the marker (e.g. `# check: needs=<tool>`) and skip with reason.
  Confidence: 75

- [NOTE] Evidence/tasks disagree on mutant count — `evidence/evidence.md:36` ("7 mutants") vs `tasks.md` TCE-4 row ("4 mutants + no-op")
  Failure: the recorded evidence is internally inconsistent; TCE-1 row names 4 and TCE-3 row asserts the unreachable-job direction is "covered by the required-checks sweep", which is not a mutant. Documentation only.
  Fix: reconcile to the actual seven mutants.
  Confidence: 90

## Could not check
- Did not execute `task check` or either guard's `--self-test` end-to-end (read-only; they write `coverage.out`, `bin/`, mktemp trees). The `task_check_test.sh` self-skip at `check.sh:73` was verified by running the exact `grep -qi 'requires docker'` over the sweep set, not by a full run.
- Did not confirm the ~70 other swept guards are Docker/network/tool-free; only the ones referencing docker/kind/kubectl were inspected.
- Did not verify the carried CWS-2 quoted-key bypass fix (Qwen round-one #5) beyond noting `rules_type=` appears once at HEAD; no zizmor binary available to confirm `disable:` semantics.
- Did not verify the `coverage: preflight=` mapping against a real preflight run (taken from `preflight.yaml` text).
