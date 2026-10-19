# PR description — test-depth-signals

**Create at:** <https://github.com/platformrelay/kollect/compare/main...feat/test-depth-signals?expand=1>

- **Source:** `feat/test-depth-signals`
- **Target:** `main`
- **Title:** `:white_check_mark: test(ci): strict fuzz retry, nightly shuffle, README quick-start guard`
- **Opens as:** ready

Paste everything below the line.

---

Closes #445

## TL;DR

- **What:** the fuzz job retries only the exact Go deadline flake and otherwise fails closed; a new
  nightly `Test shuffle` workflow runs every unit package three times under one `-shuffle` seed; a
  new guard checks the README quick start against the tree.
- **Why:** the old fuzz loop retried any failure once and could report a lost panic or an OOM kill
  green; nothing ran the suites out of source order; the README quick start was an unchecked claim.
- **Risk:** a Go release that rewords the fuzz deadline output turns the (non-required) fuzz job red
  as `unrecognised failure text` until the allow-list is updated.
- **Reviewer action:** check `hack/ci/fuzz-retry.sh`'s retry rule and the shuffle scope; approving
  also accepts the `lint` job at ~13-14 min and one more nightly that can file a `ci-failure` issue.

The fuzz retry moves out of an inline `for attempt in 1 2` loop in `ci.yaml` into
`hack/ci/fuzz-retry.sh`. `hack/ci/test-shuffle.sh` (`task test:shuffle`) runs nightly from its own
workflow, reported by the CI-failure reporter. `hack/test/readme_quickstart_test.sh` runs in `lint`.
The shuffle probe found two tests that fail when repeated; both are fixed here.

## Most important things

- **Strict fuzz retry, fail closed (decision A, operator-validated).** One retry, and only when the
  failed attempt exited **1** with exactly the captured flake sequence for that target
  (`--- FAIL: FuzzX`, indented `context deadline exceeded`, `FAIL`, `exit status 1`; from runs
  37911142180 and 37288739312, kept as fixtures) and wrote no crash corpus. A deny marker
  (`panic:`, `fatal error:`, `Failing input written`, `signal: killed`), any other text, any other
  exit status, a `testdata/fuzz/` path in `git status` (now read with untracked files listed, so
  the `internal/aggregate` and `internal/sink/git` blind spot is closed), a failing `git status`,
  or an exit 0 with no `fuzz: elapsed` line (zero targets fuzzed) is red. The trade: a Go wording
  change shows as a red, not as a hidden retry.
- **`go mod download` step in the fuzz job.** On a cold module cache `go test` prints
  `go: downloading …` lines into the captured output, so a real flake would no longer match the
  exact sequence. An uncaptured `go mod download` step runs first instead of loosening the matcher;
  `fuzz_retry_test.sh` F16 pins exactly one such step.
- **Nightly shuffle, scope decision A.** `test-shuffle.yaml` (cron `30 2 * * *`, free against the
  other crons) prints a fresh seed first, provisions envtest itself, and excludes only e2e.
  Ginkgo v2 refuses `-count≠1` and a second `RunSpecs` per process, so the Ginkgo packages run as
  three `-count=1` invocations and every other package once at `-count=3`. **Non-goal:** Go's
  `-shuffle` does not reorder specs inside `RunSpecs`, so a green nightly says nothing about
  controller and webhook spec order; Ginkgo `--randomize-all` is the named follow-up.
- **Two real test defects found and fixed.** The probe (`-count=3`) failed
  `TestRecordLabeledMetricSeries_CapsCardinalityDeterministically` (`internal/collect`) and
  `TestRegister` (`internal/metrics`): both read process-global counters as absolutes. They now
  assert their own increment (a52189aa9, 297952dee). Test-only changes, no product code.
- **README guard.** `readme_quickstart_test.sh` (bash/awk) reads the `## Quick start (MVP)` `sh`
  fences and checks every `task` name exists and is not internal, every `kubectl get` TYPE/NAME
  operand is a known CRD short name, and the `kubectl apply -k` path exists. The README had no
  drift; review rounds hardened the reader against `&`, `$(…)`, `\` continuations, quoted verbs,
  comma-joined operands and indented fences.
- **Test change on change 7's guard.** `ci-failure-report.yaml` gains `Test shuffle` in
  `on.workflow_run.workflows`, so `hack/test/ci_failure_report_test.sh`'s exact-list assertion,
  its comment and its mutant are updated. Extended, not weakened: the CFR spec does not pin the list.
- **`lint` job ~11 → ~13-14 min.** Three new guards, each plain and `--self-test`, also added to the
  CFC-6 `cfc_guards` list. Still below the slowest required job.

## Risk

- **What can break:** the fuzz job goes red on a Go release that changes the flake wording or adds
  output lines; the shuffle nightly files `ci-failure` issues for envtest environment failures as
  well as real order or repetition bugs.
- **Blast radius:** CI signals only. `fuzz` is not required; `Test shuffle` is a separate scheduled
  workflow, so it cannot mask `E2E nightly`. The `lint` job (required) gets slower and runs three
  more guards that can fail on a README or workflow edit.
- **Rollback:** revert the PR, or disable the `Test shuffle` workflow in the Actions tab. No state
  to undo besides any issue the reporter opened.
- **Residual risk accepted:** Ginkgo spec order unproven (non-goal above); the README guard checks
  command names and paths, not that `task demo-up` works (`hero-demo-smoke` does that); first
  scheduled run unobserved until after merge.

## Files to look at first

- `hack/ci/fuzz-retry.sh` — the retry decision; check that every exit path fails closed.
- `hack/ci/test-shuffle.sh`, `.github/workflows/test-shuffle.yaml` — seed printed first, envtest
  provisioned, Ginkgo packages split into three `-count=1` runs, failures aggregated.
- `internal/collect/metrics_snapshot_test.go`, `internal/metrics/metrics_test.go` — the two fixes.
- `.github/workflows/ci-failure-report.yaml`, `hack/test/ci_failure_report_test.sh` — `Test shuffle`
  in the reporter list.
- `.github/workflows/ci.yaml` — fuzz job (`go mod download` step, runner call), lint job (3 guards × 2).
- `hack/test/readme_quickstart_test.sh` — the README quick-start reader.

**Review this commit-by-commit, not as one flattened diff.** Each fix commit pairs with the review
finding it closes, and the two test fixes land before the shuffle workflow that needs them.

## Testing protocol

- [x] `task check` exit 0 on the branch (stage-B round 2, 2138 s).
- [x] Each new guard green plain and with `--self-test`; `task lint:shell`, `task lint:markdown`,
  `task spec:validate`, zizmor (high) clean.
- [ ] Every check on the PR head green.
- [ ] After merge, the first scheduled `Test shuffle` run (02:30 UTC) is green and its log starts
  with the seed line; a red run names the seed and the failing package.
- [ ] The `CI failure report` run triggered by that run concludes `success` with decision `none`
  (or `open` with a `ci-failure` issue titled for `Test shuffle` if the nightly was red).

---

## Optional reading — the long version

*Nothing above depends on this. Read it if you want the full story, the road not taken, or the
deployment fine print — skip it if the top half already told you what you needed.*

- **Why fail closed and not fail open on unknown text.** Fail open (retry anything without a crash
  file) is what the old loop did, and it is how a panic Go failed to persist could pass. A retry
  that reads both the text and the exit status cannot launder a killed process (exit 137) or a
  build failure (exit 2) as the flake; review round 1 found exactly that gap and it is cased.
- **Reproducing a shuffle failure.** `-shuffle=<seed>` seeds each test binary once and the
  `-count` loop reruns the same order, so the printed seed plus the same repetition settings
  reproduce it locally; the log prints the reproduce line.
- **Guard hardening.** Two stage-B review rounds found guard bypasses (`if:` / `continue-on-error:`
  on guarded jobs and steps, `defaults.run.shell` / step `shell:` overrides, Task `status:` /
  `ignore_error:`, operand-grammar gaps in the README reader). Each is reproduced as a case and
  fixed; see `openspec/changes/test-depth-signals/reviews/B-branch/`.
- **Spec and evidence:** `openspec/changes/test-depth-signals/` (proposal, `specs/test-depth/spec.md`
  TDS-1..3, tasks, per-group evidence).
- **Out of scope:** mutation testing (change 9), new fuzz targets, making `fuzz` required, running
  `task demo-up` in the guard.
