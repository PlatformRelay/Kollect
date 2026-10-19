# Proposal

## Why

Three weaknesses in how the suite proves itself, each seen in the tree:

- The fuzz job retries any failure once when no `testdata/fuzz/` file appeared
  (`.github/workflows/ci.yaml:790`, job `fuzz`; step `Native Go fuzz (30s)` at `:817`, the
  `for attempt in 1 2` loop at `:828`). A panic Go fails to persist, an out-of-memory kill or a real
  hang is retried and, if it does not recur, reported green. The corpus check
  (`git status --porcelain | grep -q 'testdata/fuzz/'`, `:832`) is also blind in
  `internal/aggregate` and `internal/sink/git`, which track no `testdata/`: git collapses the new
  directory to `?? internal/aggregate/testdata/` (probe, 2026-10-18). A matrix leg that fuzzes
  nothing also reports green: a target name that matches nothing prints
  `testing: warning: no fuzz tests to fuzz` and exits 0 (verified locally 2026-10-18, go1.27.2,
  `go test -run='^$' -fuzz=FuzzNoSuch -fuzztime=1s ./internal/aggregate`).
- Tests that pass only in source order are invisible: nothing runs the unit suites with `-shuffle`.
- The README quick start (`README.md:106`, heading `## Quick start (MVP)`) is a claim to users.
  `hack/test/demo_task_aliases_test.sh` checks that `demo-up` and `demo-down` exist and delegate,
  and the `hero-demo-smoke` job exercises the path, but nothing ties the README's other commands
  (`task dev-up`, the `kubectl get` short names, the `kubectl apply -k` path) to what exists.

## What Changes

- TDS-1: the fuzz retry moves to `hack/ci/fuzz-retry.sh`: 30 seconds per target, one retry only
  when the failed attempt's output is exactly the known Go engine deadline flake (an allow-list of
  its lines, captured from real runs), fail closed on any other failure text (decision A,
  2026-10-16), a corpus check that sees untracked directories and fails closed when `git status`
  fails, and zero fuzz targets run is red.
- TDS-2: a separate nightly workflow `test-shuffle.yaml` runs `task test:shuffle`
  (`shuffle:` in `hack/task/Taskfile.test.yml`, the `test` include namespace, `Taskfile.yml:59`).
  The script chooses a seed, echoes it, and runs every unit package three times with
  `-shuffle=<seed>`: packages whose tests call Ginkgo's `RunSpecs` as three `-count=1`
  invocations (Ginkgo v2 refuses any other `-count`), every other package once with `-count=3`.
  The script provisions envtest itself as `hack/coverage.sh:23-30` does and excludes only e2e
  (decision A, 2026-10-16; no package is dropped by the Ginkgo split). Its own workflow, so its
  failures get their own issue from the CI-failure reporter and cannot mask the E2E nightly.
- TDS-3: `hack/test/readme_quickstart_test.sh` checks the README quick-start commands against the
  Taskfiles, the sample path and the CRD short names.

## Capabilities

### New Capabilities

- `test-depth`: refusals beyond "tests pass once, in source order".

### Modified Capabilities

None. The CI-failure reporter's spec (change 7, CFR-7) does not pin the list of reported
workflows; only its test does (`hack/test/ci_failure_report_test.sh:1547-1553`), and task 2.4
updates that test.

## Impact

- Entry points: job `fuzz` in `ci.yaml` calling `hack/ci/fuzz-retry.sh`;
  `.github/workflows/test-shuffle.yaml` (`name: Test shuffle`, `schedule:` cron `30 2 * * *`, free
  against the existing crons: e2e-nightly `0 3 * * *`, renovate `0 4 * * 1`, scorecard
  `22 5 * * 1`, codeql `18 6 * * 1`, go-patch-lag `23 6 * * 1`), added to
  `on.workflow_run.workflows` of `ci-failure-report.yaml:22-24`; the required `lint` job
  (`ci.yaml:279`) running the three new guards in both modes, each also named in the CFC-6
  `cfc_guards` list (`hack/test/ci_workflow_security_test.sh:603-604`). The lint wall-time delta is
  measured and recorded.
- `task check` runs every `hack/test/*_test.sh` bare and with `--self-test` when its code declares
  it (`hack/check.sh:88-111`, since change 7), so each new guard is swept locally too.

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
  `Test*` each, and Go's `-shuffle` does not control ordering inside `RunSpecs`. Ginkgo keeps its
  default randomization of top-level containers with its own time-based seed (v2.33.0
  `types/config.go:50`, `:287-288`); this change neither enables full spec randomization nor ties
  that seed to the Go shuffle seed, so reproducing a Ginkgo failure may need the Ginkgo seed the
  suite reports. A green shuffle nightly is not evidence that the controller and webhook specs are
  order-independent. Ginkgo's own `--randomize-all` is the follow-up for that, not this change.
- Executing `task demo-up` (needs kind and Docker; `hero-demo-smoke` does).
- Re-checking what `demo_task_aliases_test.sh` already asserts.
- Making the `fuzz` job required.

## Assumptions

- golang/go#75804 is the upstream report for the fuzz engine's "context deadline exceeded" at the
  `-fuzztime` boundary. This comes from the survey and is not yet verified; task 1.1 verifies the
  id before it goes into the warning text, or the warning names the flake without a number. The
  flake text is verified from two real runs (2026-10-18): run 37911142180
  (`FuzzValidateAttributePath`) and run 37288739312 (`FuzzContentHash`) each print
  `--- FAIL: FuzzX (3Ns)`, then an indented `context deadline exceeded`, `FAIL`, `exit status 1`,
  and pass on the retry. Every failing `go test -fuzz` prints `--- FAIL:`, so that line is not a
  deny marker. Log retention expires: task 1.1 captures both logs first.
- Go's fuzz output wording can change in a release. Fail closed (decision A) turns that into a
  visible red naming "unrecognised failure text" on a non-required job, not a hidden retry.
- `-shuffle=<seed>` seeds each test binary once: `M.Run` prints `-test.shuffle N` and shuffles the
  test list once (`src/testing/testing.go:2572-2587`, go1.27.2), and the `-count` loop at `:2714`
  reruns that same order. So each package runs one permutation, repeated; a failure reproduces
  with the same seed and repetition settings, and may also reproduce at `-count=1` unless it needs
  state from an earlier iteration. Task 2.1 probes this.
- Ginkgo v2.33.0 (`go.mod:18`) refuses `-count` other than 1 (`types/config.go:402-410`,
  "Only -count=1 is allowed", `types/errors.go:656`) and a second `RunSpecs` in one process
  (`core_dsl.go:260-262`), so a Ginkgo package is repeated by separate invocations.
- Envtest-backed packages in a nightly can fail for environmental reasons and file a reporter
  issue; that is the accepted cost of decision A, measured in task 2.1.
