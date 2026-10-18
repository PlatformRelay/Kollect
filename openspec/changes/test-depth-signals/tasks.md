# Tasks

Red first. Every test task comes before the implementation it drives and records its red in
evidence, failing on the target assertion (not on a missing file, a parse error or a usage error).
`task check` runs every `hack/test/*_test.sh` bare and, when its code declares it, with
`--self-test` (`hack/check.sh:88-108`, since change 7), so a red guard cannot be committed alone:
each test task closes in one commit with the implementation after it (1.1 with 1.2 and 1.3; 2.2
with 2.3; 3.1 with 3.2), and the tree is green at every commit. Each new guard has both modes from
its first commit, and that commit wires both into the required `lint` job (`ci.yaml:279`), as
CWS-6 (`hack/test/ci_workflow_security_test.sh:429`) demands for a guard CI runs; a guard that
checks its own wiring also joins the CFC-6 `cfc_guards` list there.

Fixtures live under `hack/test/testdata/<guard>/`, never under `openspec/changes/**/evidence/`
(Markdown only, `hack/test/openspec_evidence_markdown_only_test.sh`). Each fixture records its
source (run id and job, or the local command and Go version) in a comment or a sibling `README`.

Each case below reads: given / when / then · the failure it exposes.

## 1. Fuzz runner (TDS-1)

- [ ] 1.1 Capture fixtures first (log retention expires), then write `hack/test/fuzz_retry_test.sh`
  (plain and `--self-test`) with a fake `go` on `PATH` that replays one fixture per attempt and
  records each call.
  Fixtures in `hack/test/testdata/fuzz-retry/`:
  - deadline flake, trimmed to the failing step: run 37911142180, job
    `fuzz (FuzzValidateAttributePath, ./internal/collect/...)`, and run 37288739312, job
    `fuzz (FuzzContentHash, ./internal/aggregate/...)` (`gh run view <id> --log`; both show
    `--- FAIL: FuzzX`, an indented `context deadline exceeded`, `FAIL`, `exit status 1`, and a
    passing retry);
  - a passing attempt from the same runs (with `fuzz: elapsed` lines and `ok`);
  - zero targets: local `go test -run='^$' -fuzz=FuzzNoSuch -fuzztime=1s ./internal/aggregate`
    (`testing: warning: no fuzz tests to fuzz`, `ok`, exit 0);
  - a crash: a throwaway failing fuzz target run outside the repository, its output with
    `Failing input written` and a `panic:`; marked as locally produced.

  Cases, red against a stub `hack/ci/fuzz-retry.sh` that holds today's inline loop
  (`ci.yaml:826-838`) verbatim:
  - F1 given the deadline fixture then the pass fixture, when the runner runs, then exit 0, two
    `go` calls, and a warning naming golang/go#75804 · a retry removed, or a missing warning;
  - F2 given `panic:` alongside `context deadline exceeded`, then non-zero after one call · the
    blind retry (red today);
  - F3 given `signal: killed`, then non-zero after one call · a retried OOM kill (red today);
  - F4 given `fatal error:`, then non-zero after one call · a retried runtime crash (red today);
  - F5 given the deadline text plus a new file under `testdata/fuzz/` (or `Failing input written`),
    then non-zero after one call · a retried real finding;
  - F6 given the deadline fixture twice, then non-zero after two calls · an unbounded or
    swallowed retry;
  - F7 given empty output and given unknown failure text, then non-zero after one call and the
    message contains `unrecognised failure text` · fail open, and an undiagnosable Go wording
    change (red today);
  - F8 given the zero-target fixture (exit 0), then non-zero and the message says no fuzz target
    ran · a leg that fuzzes nothing reports green (red today);
  - F9 given exit 0 with no `fuzz: elapsed` line and no warning, then non-zero with
    `unrecognised` · a pass that fuzzed nothing for a reason the runner does not know;
  - F10 control: given the pass fixture, then exit 0 after one call · an over-strict runner;
  - F11 wiring: the `fuzz` job's fuzz step runs `hack/ci/fuzz-retry.sh` with the matrix target
    and package and has no inline `for attempt` loop · the job bypassing the runner.
- [ ] 1.2 Implement `hack/ci/fuzz-retry.sh` (`-fuzztime=30s`, retry rule and messages as TDS-1);
  point the `fuzz` job at it; wire `fuzz_retry_test.sh` into `lint` in both modes; zizmor gate
  passes. No existing guard asserts on the `fuzz` job's steps (`grep -n fuzz hack/test/*.sh`
  outside the new guard matched nothing on 2026-10-18), so none needs updating.
- [ ] 1.3 Defect controls in `--self-test`: drop each deny marker, the corpus check, the
  zero-target check, the `fuzz: elapsed` check, the `unrecognised failure text` message and the
  retry cap, one per mutant; each turns its case red; a no-op copy passes.

## 2. Shuffle (TDS-2)

- [ ] 2.1 Probe, evidence only: how `-shuffle=<seed>` prints the seed (`-test.shuffle N`) and
  that a failure at `-count=3` reproduces only with the same seed and the same `-count`; that
  `go list ./...` skips a package under `hack/test/testdata/` while an explicit path still builds
  it (else the fixture goes behind a build tag); the unit package list (`go list ./...` minus
  `/e2e`, as `Makefile:70` and `hack/coverage.sh:37` filter); the wall time of `-count=3` with
  envtest assets provisioned (`make setup-envtest`), to set the job's `timeout-minutes`; a seed
  that runs the 2.2 fixture's dependent test first.
- [ ] 2.2 Write `hack/test/test_shuffle_test.sh` (plain and `--self-test`) and an order-dependent
  fixture package under `hack/test/testdata/shuffle/` (or behind a build tag, per 2.1), so
  `go list ./...` ignores it; wire both modes into `lint`. Cases, red against a stub
  `hack/ci/test-shuffle.sh` that runs `go test -count=3` without `-shuffle`:
  - S1 given the fixture and the 2.1 seed, when the script runs on the fixture with that seed,
    then non-zero and the output names the seed · the script does not pass `-shuffle` (red
    today);
  - S2 control: given the fixture with `-shuffle=off`, then it passes · a fixture red for a reason
    other than order;
  - S3 given a failing run's printed seed, when rerun with that seed and `-count=3`, then the same
    test fails · `-shuffle=on` with an uncontrolled seed, or a different `-count`;
  - S4 given any run, then the seed line precedes the first test output · a seed lost when the job
    times out;
  - S5 given an exclusion entry with no reason, then the guard fails · silent exclusions;
  - S6 given an exclusion entry for a non-e2e package, then the guard fails and names it · envtest
    packages quietly dropped against decision A;
  - S7 given an empty package list, then the script fails · a vacuous green;
  - S8 wiring: `test-shuffle.yaml` has `name: Test shuffle`, `schedule:` cron `30 2 * * *`,
    `permissions: contents: read`, `./.github/actions/go-cache` with `envtest: true`, and runs
    `task test:shuffle`; `ci-failure-report.yaml` lists `Test shuffle` · a nightly nobody hears
    from.
- [ ] 2.3 Implement `hack/ci/test-shuffle.sh` (chooses a seed, echoes it, passes
  `-shuffle=<seed> -count=3`; exclusion list with reasons, e2e only), `shuffle:` in
  `hack/task/Taskfile.test.yml` (so `task test:shuffle`), and `.github/workflows/test-shuffle.yaml`
  (`schedule:` cron `30 2 * * *`, `contents: read`, envtest provisioned as `ci.yaml:647-649` does;
  a `workflow_dispatch` run is never reported, CFR-1 acts on scheduled runs only). Add
  `Test shuffle` to `on.workflow_run.workflows` in `.github/workflows/ci-failure-report.yaml:22-24`
  and to the hard-coded list, its comment and its message in
  `hack/test/ci_failure_report_test.sh:1547-1553`, with a self-test mutant like W1 (`:424`) that
  drops it. The zizmor gate passes. Fix any order dependence found, each in its own commit before
  this one.

## 3. README truth (TDS-3)

- [ ] 3.1 Write `hack/test/readme_quickstart_test.sh` (plain and `--self-test`). Cases, each a
  mutant of a copy of the tree:
  - R1 given `demo-up` renamed in `Taskfile.yml`, then the test fails and names `demo-up` · a
    README naming a task that is gone;
  - R2 given `dev-up` renamed, then the test fails and names `dev-up` · commented alternatives
    skipped;
  - R3 given `kubectl get kinv,foo`, then the test fails and names `foo` · short names unchecked;
  - R4 given `kubectl apply -k config/samples/nosuch/`, then the test fails and names the path ·
    sample paths unchecked;
  - R5 given the heading renamed so no `##` heading starts with "Quick start", then the test fails
    · an empty extraction passing;
  - R6 given an included task (`task test:run`) in the section, then it passes · namespaced tasks
    reported as missing;
  - R7 control: the real tree passes.
- [ ] 3.2 Implement; wire into `lint` in both modes.

## 4. Land

- [ ] 4.1 Merge with the `post-merge` row open; archive in a follow-up evidence PR once it passes

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| TDS-1 | `fuzz_retry_test.sh` F1-F11, both modes; 1.3 mutants by exit status | only the pure deadline case retries; zero targets and unknown text red | not-run | |
| TDS-2 | `test_shuffle_test.sh` S1-S8 with the fixed seed; guard mutants | fixture red; seed printed; control green | not-run | |
| TDS-2 | first scheduled `Test shuffle` run; reporter issue on a scheduled failure | green; separate issue | post-merge | follow-up evidence PR |
| TDS-3 | `readme_quickstart_test.sh` R1-R7 | each mutant red, tree green | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; reviewed and archive revisions recorded | not-run | |
