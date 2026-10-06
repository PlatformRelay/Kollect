## Verdict: CONCERNS

Spec-lens review of `origin/main..HEAD` (2 commits) against `openspec/changes/task-check-entrypoint/specs/task-entrypoints/spec.md`. I read the spec, `hack/check.sh`, `hack/test/task_check_test.sh`, the ci.yaml/Taskfile/CONTRIBUTING/COMMAND-REFERENCE diffs, the carried CWS changes, `verify-eligibility.sh`, `preflight.yaml`, and the current `.github/zizmor.yml`; I ran both modes of the new guard (green: plain + 4 mutants + no-op control).

## Findings

- [WARNING] TCE-1's "every required CI gate that can run locally" is contradicted for two preflight steps — `hack/check.sh:57`
  Failure: `preflight.yaml:51-56` runs `go mod tidy` + `git diff --exit-code go.sum` + `go mod verify`; both are runnable on a dev machine, yet `task check` neither runs them nor lists them. The hand-written coverage mapping `coverage: preflight=lint:markdown,verify,guard-sweep` understates the real preflight job, and the TCE-3 guard validates only that mapping *components exist* (task_check_test.sh:154-169), never that the mapping covers the job — so the drift is invisible to the gate. A contributor gets green `task check` and a red CI preflight.
  Fix: add a `run_gate mod-tidy` (tidy + go.sum diff + `go mod verify`) to check.sh and add it to the coverage mapping; or honestly re-map and exclude the remainder with a reason.
  Confidence: 85
- [NOTE] TCE-2's "not a superset of check" direction is unenforced — `hack/test/task_check_test.sh:119-128`
  Failure: the guard only asserts `verify` still contains `bash hack/verify.sh`; appending `task lint` (or the whole check body) to `.tasks.verify.cmds` keeps the guard green, while the spec forbids verify becoming a superset of check. The mutant only tests redefinition to lint.
  Fix: pin the cmds list exactly (`yq '.tasks.verify.cmds | length' == 1` or equality with `[bash hack/verify.sh]`).
  Confidence: 90 (gap real; blast radius small)
- [NOTE] Mode detection is code-based, spec says "every mode its header declares" — `hack/check.sh:68`
  Failure: the sweep strips comments before grepping for `--self-test` (task_check_test.sh:60-62 feeds the same convention), so a guard that declares `--self-test` only in its header comment never gets that mode run. Today the two flag-parsing guards (task_check_test.sh:15, ci_workflow_security_test.sh:15) declare it in code, so it holds.
  Fix: one-line comment in check.sh recording that "declared" means "parsed in code", or extend detection to header declarations.
  Confidence: 70
- [NOTE] gitleaks invocation is not byte-identical to CI's: `--verbose` omitted — `hack/check.sh:50`
  Failure: none functionally (detection semantics identical); the comment claims "the same invocation shape as CI's gitleaks job" (ci.yaml:168 has `--verbose`), and the guard pins only the invocation prefix (task_check_test.sh:93), so the difference can grow.
  Fix: add `--verbose` or weaken the comment.
  Confidence: 60

## Requirement verdicts

- **TCE-1: holds (partial on preflight completeness).** All 14 `required_checks` (verify-eligibility.sh:19-22) are run/mapped/excluded; glob picks up new guards (check.sh:62); failure aggregation exits 1 (check.sh:93-96); helm gate = the exact CI invocation (`task helm-test`, no cluster — ci.yaml:620-641). Gap = finding 1.
- **TCE-2: holds for the tested directions; superset direction partial** (finding 2). `verify` still runs `hack/verify.sh` (Taskfile.yml:111-115).
- **TCE-3: holds.** Verified green by running the guard; mapping and exclusion reasons all parsed.
- **TCE-4: holds.** 4 mutants + no-op, each red with its intended assertion message (verified by execution); wired as two separate steps in the required `lint` job (ci.yaml:439-442), which runs on unfiltered `pull_request` (ci.yaml:43).

## What the target does that no requirement mentions

- Runs local-only gates beyond `required_checks`: `format:check`, `scrub`, `spec:validate`, `lint:shell`, and gitleaks/zizmor as split install+detect gates — stricter locally, harmless.
- Explicitly sweeps `hack/test/lab_harness_meta_suite.sh` (non-`*_test.sh` name) alongside the glob.
- The lint job also runs the TCE guard on `push` to main, stronger than TCE-4's pull_request requirement.
- Carried CWS leftovers: CWS-2 now demands comments on policy switches under rules (currently inert — `.github/zizmor.yml:14` is `rules: {}`, walk exits early; the mutant proves it fires when rules appear); CWS-3 drops `shell:` from the dependency-review step allow-list (current steps use only `uses/with`, no false red).

## Could not check

- A full `task check` run end-to-end on a Docker-less machine (compiles, envtest, downloads tooling) — not run; gate claims verified structurally, not by execution.
- The CI run of the new lint steps on the PR head — not available from this session.
- Whether the carried CWS-3 `shell:` tightening has a dedicated mutant among the 44 in ci_workflow_security_test.sh (diff adds none for it).
