## Verdict: CONCERNS

## Findings
- [WARNING] Nothing runs the real chain before merge: no task executes `task mutation` with real gremlins on the final tree. — `openspec/changes/mutation-testing-signal/tasks.md:177-181`
  Failure: R1–R7 only test against a fake gremlins, and 1.1 runs gremlins by hand from `<tmp>`. Integration faults are invisible until the first nightly. Examples: `GOBIN={{.BIN_DIR}}/…` is relative (the Taskfile's `BIN_DIR: bin`, `Taskfile.yml:18`, and `go install` refuses a relative GOBIN); a `--output=<dir>/./internal/aggregate.json` path needs a subdirectory that `os.Create` won't make (`report.go:190-193` logs the error and still exits 0, so R3 reds every night).
  Fix: add a 2.5 gate that runs `task mutation` locally (once plain, once with `MUTATION_FORCE_FAILURE=true`) and records the summary and exit codes. Also pin the report naming to `<short-pkg>.json` in R1.
  Confidence: 85
- [WARNING] The MUT-1 "report artifact exists" check can't work as specified. The runner writes to a `mktemp -d` path the workflow cannot know, and W1 forbids any channel that would tell the upload step where it is. — `tasks.md:120-128`, `tasks.md:167-169`; spec `spec.md:41-42`
  Failure: W1 allows no workflow or job `env:`, and the run step's `env:` must be exactly `MUTATION_FORCE_FAILURE`. So `actions/upload-artifact` (`if: always()`) has no `path:` matching the random directory. Its default `if-no-files-found: warn` keeps the run green with no artifact. W1 pins neither `path:` nor `if-no-files-found`.
  Fix: make the runner create the directory under a known parent (`mktemp -d "$RUNNER_TEMP/mutation.XXXX"`). Have W1 pin upload `path:` to that glob and `if-no-files-found: error`, with a mutant for each.
  Confidence: 75
- [WARNING] W3 can't detect gremlins being run in `lint` through Task, which MUT-2/MUT-3 forbid. — `tasks.md:140-145`; spec `spec.md:78-79,123-124`
  Failure: W3 only flags the token `go-gremlins` or the command word `gremlins`. A lint step `run: task mutation` (or the install task, or `bash hack/ci/mutation.sh …`) contains neither, so gremlins runs on every PR and every case passes. Its own mutant, "a `lint` step that installs gremlins", is only caught if it is written with the literal install line.
  Fix: W3 also rejects any reference to the install task, `mutation` or `hack/ci/mutation.sh` under `.github/`, in the `Makefile` and in included Taskfiles, except the `mutation.yaml` run step. Add the mutant `run: task mutation` in `ci.yaml` lint.
  Confidence: 70
- [WARNING] Several T1/T3 mutants also break W1/W2's exact-shape rules. If the guard stops at its first failure, they red with the W1/W2 message and the T check is never exercised. — `tasks.md:146-157,182-190`
  Failure: a threshold flag on a workflow step or the `mutation` task, or a workflow/job `env:`, already violates W1/W2 ("run exactly `task mutation`", "no workflow or job env", "cmds exactly"). The existing guard style exits on the first `fail` (`hack/test/ci_workflow_security_test.sh:56-59`). 2.6 says mutants are judged by their own case's message, but nothing requires the guard to keep going after a W1/W2 failure. A missing T1/T3 implementation then shows up as a failing self-test rather than as a gap.
  Fix: state in 2.3/2.4 that the guard collects every failure before exiting, or that T checks run before W1/W2, so each mutant can red with its own message.
  Confidence: 60
- [NOTE] The new `cfc_guards` entry has no CWS-7 "unwired" mutant. Every earlier entry has one (`hack/test/ci_workflow_security_test.sh:1102-1148`), so the list edit in 2.5 lands with no red test (TDD rule). — `tasks.md:174-175`
  Fix: add `mutant_rejected "CFC-6 mutation signal guard unwired" …`, as at `:1136`.
  Confidence: 70
- [NOTE] Spec and tasks give `--config` differently: the spec says `--config=hack/ci/gremlins.yaml` (relative), R1 says `<repo>/hack/ci/gremlins.yaml`. The fixture call runs from the fixture's own directory, and gremlins reads `--config` before it changes directory (`configuration.go:77-81`), so the relative form fails there. — `spec.md:26`, `tasks.md:92`
  Fix: make the spec say the path resolves from the repo root (absolute in argv).
  Confidence: 65
- [NOTE] T1's `--threshold.mutant-coverage` isn't a flag gremlins has (the real one is `threshold-mcover`, `cmd/unleash.go:65`), and no mutant uses the real spelling. The `--threshold[-_.]` regex still catches it. — `tasks.md:147`
  Fix: add `--threshold-mcover` as a T1 spelling.
  Confidence: 55
- [NOTE] 1.1 records the fixture control without saying which invocation produced it. The runner uses all eleven flags plus `--config`, and a control recorded under the default set could come out with a different identity. — `tasks.md:56-58`
  Fix: record the control from the runner's exact argv.
  Confidence: 45

Requirement status:
- MUT-1: partial (artifact path).
- MUT-2: partial (lint-runs-gremlins gap; the env, config-file and flag sources hold against `configuration.go:69-94` and `unleash.go:196-205`).
- MUT-3: holds (R4, plus `engine.go:140-158` and `coverage.go:166-207` confirm the NOT COVERED / LIVED split and package-relative `file_name` for `./internal/<pkg>`).
- MUT-4: holds (R5, R7, 3.1).
- Round-1 fixes V1–V6: checked against the gremlins source. They hold.

## Could not check
- I didn't run anything: no gremlins, no `go install` with a relative GOBIN (that claim is from memory of `cmd/go`), no guards.
- I didn't read the other legs' round-2 outputs, `loop.md`, `task-prompt.md`, `hack/task/Taskfile.test.yml`, `.github/actions/go-cache/action.yml` beyond one grep, or DTP-5's comment rules (`consistency_toolchain_pins_test.sh:522`).
- I didn't confirm #469 is merged (task 0.1) through git history. I only checked that `ci-failure-report.yaml:22-25` matches `ci_failure_report_test.sh:1549-1556`.
