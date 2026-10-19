## Verdict: CONCERNS

## Findings
- [WARNING] TDS-1's corpus check, as written, can't see a crash file in 2 of the 3 fuzz packages — `.github/workflows/ci.yaml:832`, spec `specs/test-depth/spec.md:17`
  Failure: `internal/aggregate/` and `internal/sink/git/` have no tracked `testdata/` dir (Glob: only `internal/collect/testdata/*.json` exists). A crash there writes `internal/aggregate/testdata/fuzz/FuzzContentHash/<h>`. Plain `git status --porcelain` collapses that to `?? internal/aggregate/testdata/`, so a `grep 'testdata/fuzz/'` misses it. The spec repeats the existing check word for word ("`git status --porcelain` shows no new file under `testdata/fuzz/`"), and F5 can pass on a fake-repo layout that doesn't match the real packages. Only the `Failing input written` marker would still catch it.
  Fix: in TDS-1 and 1.2, require `git status --porcelain --untracked-files=all` (or `git ls-files --others -- '*/testdata/fuzz/*'`). Add an F5 variant whose package has no tracked `testdata/`, plus a 1.3 mutant that drops `--untracked-files=all`.
  Confidence: 80
- [WARNING] TDS-2 says envtest packages "SHALL run" with `make setup-envtest`, but no task checks that — spec `:69-71`, `tasks.md:92-99`
  Failure: task 2.3 says "envtest provisioned as `ci.yaml:647-649` does". Those lines are only the `go-cache envtest: true` cache restore. CI's real provisioning is `make setup-envtest` plus `KUBEBUILDER_ASSETS` inside `hack/coverage.sh:23-30`. On a cache miss with no `setup-envtest`, `internal/pipeline/integration_test.go:45,132` call `t.Skip` and the nightly goes green without running the L2 envtest tests. S8 checks only `envtest: true`.
  Fix: in 2.3, have `hack/ci/test-shuffle.sh` run `make setup-envtest`, export `KUBEBUILDER_ASSETS` and fail when it's empty (as `coverage.sh:27-30` does). Add an S-case and a mutant for it.
  Confidence: 75
- [WARNING] Task 2.2 lists no `--self-test` mutants for `test_shuffle_test.sh`, though the preamble requires both modes — `tasks.md:9,75-95`
  Failure: 1.3 lists one mutant per defect for the fuzz guard, and 3.1 frames R1-R6 as mutants of a tree copy. 2.2 says neither. So the self-test can come out as a no-op, and the Verification row's "guard mutants" (`:134`) has nothing defined to check against. Nothing tests that `-count=3` is passed: the stub already passes it, and a script that drops it still reds S1.
  Fix: in 2.2, list the self-test mutants: drop `-shuffle`, drop `-count=3`, move the seed echo after the run, add a reasonless exclusion, add a non-e2e exclusion, empty the package list.
  Confidence: 70
- [NOTE] Nothing checks TDS-1's "SHALL run each target for 30 seconds" — `tasks.md:56-57`
  Failure: F11 checks the target and package wiring only. A runner passing `-fuzztime=1s` turns every case green.
  Fix: have F1/F10 assert that the recorded `go` argv contains `-fuzztime=30s`, and add a 1.3 mutant.
  Confidence: 70
- [NOTE] No case checks that the retry needs `--- FAIL: <target>` for the right target, or the line order ("followed by") — spec `:14-15`
  Failure: deadline text under a different `--- FAIL:` name, or printed before the FAIL line, is still retried. Low risk, because the deny markers still apply.
  Fix: add an F-case with `--- FAIL: FuzzOther` and the deadline text, expecting no retry.
  Confidence: 50
- [NOTE] TDS-3's "SHALL NOT duplicate the delegation checks of `demo_task_aliases_test.sh`" (spec `:112-113`) has no check and no Verification row. Review is the only thing holding it, and the plan doesn't say so.
  Confidence: 60
- [NOTE] TDS-2's "two separate issues" scenario is deferred to `post-merge` (`tasks.md:135`). It is plausibly held by design: one issue per workflow, and `concurrency` keyed on `workflow_id` at `ci-failure-report.yaml:38-40`. 2.3 correctly names the hard-coded list at `ci_failure_report_test.sh:1549-1553` and the W1 mutant at `:424`.
  Confidence: 60

Requirement status:
- **TDS-1**: partial. The retry logic, deny markers, zero-target, `fuzz: elapsed` and `unrecognised` cases are covered by F1-F10 and the 1.3 mutants. The corpus check and the 30s duration are weak.
- **TDS-2**: partial. Shuffle, seed and exclusion are held by S1-S8. Envtest provisioning and self-test mutants are missing.
- **TDS-3**: holds. R1-R7 map onto every scenario. `kinv`, `ktgt` and `ksnap` exist in `config/crd/bases/*:15`, `test:run` exists in `hack/task/Taskfile.test.yml:15`, and the README section is at `README.md:106-127`.

Work no requirement mentions:
- S7 (empty package list fails).
- R6 (namespaced include passes).
- The cron slot choice (`30 2 * * *`).

## Could not check
- I didn't run any test, `go list`, `task check` or zizmor. The porcelain-collapse claim comes from known git default behaviour, not a run here.
- I didn't read `openspec/changes/test-depth-signals/reviews/**` (the other reviewers' output), `loop.md` or `task-prompt.md`.
- I didn't verify golang/go#75804, or the run logs 37911142180 and 37288739312.
- I didn't fully read `hostEnvtestBinaryDir` in `internal/controller/suite_test.go`, so I don't know whether the Ginkgo suites fail or skip when assets are missing.
- I didn't read the CWS rules beyond CWS-6 for constraints a new workflow file must meet.
