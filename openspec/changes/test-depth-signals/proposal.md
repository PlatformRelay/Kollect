# Proposal

## Why

Three weaknesses in how the suite proves itself, each seen in the tree:

- The fuzz job retries any failure once when no `testdata/fuzz/` file appeared
  (`.github/workflows/ci.yaml:817`, job `fuzz`, the `for attempt in 1 2` loop at `:828`). A panic Go
  fails to persist, an out-of-memory kill or a real hang is retried and, if it does not recur,
  reported green. A matrix leg that fuzzes nothing also reports green: a target name that matches
  nothing prints `testing: warning: no fuzz tests to fuzz` and exits 0 (verified locally
  2026-10-18, go1.27.2, `go test -run='^$' -fuzz=FuzzNoSuch -fuzztime=1s ./internal/aggregate`).
- Tests that pass only in source order are invisible: nothing runs the unit suites with `-shuffle`.
- The README quick start (`README.md:106`, heading `## Quick start (MVP)`) is a claim to users.
  `hack/test/demo_task_aliases_test.sh` checks that `demo-up` and `demo-down` exist and delegate,
  and the `hero-demo-smoke` job exercises the path, but nothing ties the README's other commands
  (`task dev-up`, the `kubectl get` short names, the `kubectl apply -k` path) to what exists.

## What Changes

- TDS-1: the fuzz retry moves to `hack/ci/fuzz-retry.sh`: 30 seconds per target, one retry only
  for the known Go engine deadline flake (golang/go#75804), fail closed on any other failure text
  (decision A, 2026-10-16), and zero fuzz targets run is red.
- TDS-2: a separate nightly workflow `test-shuffle.yaml` runs `task test:shuffle`
  (`shuffle:` in `hack/task/Taskfile.test.yml`, the `test` include namespace, `Taskfile.yml:59`).
  The script chooses a seed, echoes it, and runs the unit packages with `-shuffle=<seed> -count=3`.
  The workflow provisions envtest as the CI test jobs do and excludes only e2e (decision A,
  2026-10-16). Its own workflow, so its failures get their own issue from the CI-failure reporter
  and cannot mask the E2E nightly.
- TDS-3: `hack/test/readme_quickstart_test.sh` checks the README quick-start commands against the
  Taskfiles, the sample path and the CRD short names.

## Capabilities

### New Capabilities

- `test-depth`: refusals beyond "tests pass once, in source order".

### Modified Capabilities

None. The CI-failure reporter's spec (change 7, CFR-7) does not pin the list of reported
workflows; only its test does (`hack/test/ci_failure_report_test.sh:1547-1553`), and task 2.3
updates that test.

## Impact

- Entry points: job `fuzz` in `ci.yaml` calling `hack/ci/fuzz-retry.sh`;
  `.github/workflows/test-shuffle.yaml` (`name: Test shuffle`, `schedule:` cron `30 2 * * *`, free
  against the existing crons: e2e-nightly `0 3 * * *`, renovate `0 4 * * 1`, scorecard
  `22 5 * * 1`, codeql `18 6 * * 1`, go-patch-lag `23 6 * * 1`), added to
  `on.workflow_run.workflows` of `ci-failure-report.yaml:22-24`; the required `lint` job
  (`ci.yaml:279`) running the three new guards in both modes.
- `task check` runs every `hack/test/*_test.sh` bare and with `--self-test` when its code declares
  it (`hack/check.sh:88-108`, since change 7), so each new guard is swept locally too.

## Dependencies

Landing order across the ten proposed changes: (1) ci-workflow-hardening, (2) task-check-entrypoint,
(3) golangci-lint-bump, (4) cross-file-consistency-gates, (5) developer-toolchain-pins,
(6) dependency-update-automation, (7) ci-failure-reporting, (8) test-depth-signals,
(9) mutation-testing-signal, (10) public-agent-contract.
Needs 7 (the shuffle failures must reach an issue) and 1 (guards block merges); both are merged
(change 7 as #469). New workflows satisfy the zizmor gate in their own tasks.

## Non-goals

- Mutation testing (own change); new fuzz targets; repeated golden-test runs (the goldens already
  compare to a fixed file and `encoding/json` sorts map keys, so repeating them adds nothing).
- Reordering Ginkgo specs. `-shuffle` reorders top-level `Test*` functions only; the Ginkgo suites
  (`internal/controller/suite_test.go`, `internal/webhook/v1alpha1/suite_test.go`) are one
  `Test*` each, so their specs still run in Ginkgo's default order and their order dependence stays
  hidden. A green shuffle nightly is not evidence that the controller and webhook specs are
  order-independent. Ginkgo's own `--randomize-all` is the follow-up for that, not this change.
- Executing `task demo-up` (needs kind and Docker; `hero-demo-smoke` does).
- Re-checking what `demo_task_aliases_test.sh` already asserts.
- Making the `fuzz` job required.

## Assumptions

- golang/go#75804 is the upstream report for the fuzz engine's "context deadline exceeded" at the
  `-fuzztime` boundary. This comes from the survey and is not yet verified. The flake text is
  verified from two real runs (2026-10-18): run 37911142180 (`FuzzValidateAttributePath`) and run
  37288739312 (`FuzzContentHash`) each print `--- FAIL: FuzzX (3Ns)`, then an indented
  `context deadline exceeded`, `FAIL`, `exit status 1`, and pass on the retry. Every failing
  `go test -fuzz` prints `--- FAIL:`, so that line is not a deny marker. Log retention expires:
  task 1.1 captures both logs first.
- Go's fuzz output wording can change in a release. Fail closed (decision A) turns that into a
  visible red naming "unrecognised failure text" on a non-required job, not a hidden retry.
- With `-shuffle=<seed> -count=N` one RNG stream serves all N iterations, so a failure reproduces
  only with the same seed and the same `-count`; task 2.1 probes this.
- Envtest-backed packages in a nightly can fail for environmental reasons and file a reporter
  issue; that is the accepted cost of decision A, measured in task 2.1.
