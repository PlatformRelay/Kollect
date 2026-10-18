# Tasks

Red first. Every test task comes before the implementation it drives and records its red in
evidence, failing on the target assertion (not on a missing file, a parse error or a usage error).
`task check` runs every `hack/test/*_test.sh` bare and, when its code declares it, with
`--self-test` (`hack/check.sh:88-111`, since change 7), so a red guard cannot be committed alone:
each group closes in one commit, test tasks first (1.1-1.5; 2.2-2.5; 3.1 with 3.2), and the tree is
green at every commit. Each new guard has both modes from its first commit, and that commit wires
both into the required `lint` job (`ci.yaml:279`), as CWS-6
(`hack/test/ci_workflow_security_test.sh:429`) demands for a guard CI runs, and adds the guard to
the CFC-6 `cfc_guards` list (`hack/test/ci_workflow_security_test.sh:603-604`), so deleting both of
its lint steps cannot drop it from CWS-6 discovery (the comment at `:597-602`). The group's
evidence records `ci_workflow_security_test.sh` green plain and with `--self-test` after that edit.

Fixtures live under `hack/test/testdata/<guard>/`, never under `openspec/changes/**/evidence/`
(Markdown only, `hack/test/openspec_evidence_markdown_only_test.sh`). Each fixture records its
source (run id and job, or the local command and Go version) in a comment or a sibling `README`.

Each case below reads: given / when / then · the failure it exposes.

## 1. Fuzz runner (TDS-1)

- [x] 1.1 Capture fixtures first (log retention expires), then write `hack/test/fuzz_retry_test.sh`
  (plain and `--self-test`) with a fake `go` on `PATH` that replays one fixture per attempt and
  records each call's argv; the corpus cases run in a throwaway git repository.
  Fixtures in `hack/test/testdata/fuzz-retry/`:
  - deadline flake, trimmed to the failing step: run 37911142180, job
    `fuzz (FuzzValidateAttributePath, ./internal/collect/...)`, and run 37288739312, job
    `fuzz (FuzzContentHash, ./internal/aggregate/...)` (`gh run view <id> --log`; both show
    `--- FAIL: FuzzX`, an indented `context deadline exceeded`, `FAIL`, `exit status 1`, and a
    passing retry). Record the exact non-`fuzz:` line set of both first attempts in the evidence;
    it is TDS-1's allow-list. A line TDS-1 does not list stops the task as a spec finding;
  - a passing attempt from the same runs (with `fuzz: elapsed` lines and `ok`);
  - zero targets: local `go test -run='^$' -fuzz=FuzzNoSuch -fuzztime=1s ./internal/aggregate`
    (`testing: warning: no fuzz tests to fuzz`, `ok`, exit 0);
  - a crash: a throwaway failing fuzz target run outside the repository, its output with
    `Failing input written` and a `panic:`; marked as locally produced.

  Verify golang/go#75804 before it goes into the warning text; if it cannot be verified, the
  warning names the flake without the number (record which in the evidence).

  Retry-decision cases, red against a stub `hack/ci/fuzz-retry.sh <target> <pkg>` that holds
  today's inline loop (`ci.yaml:827-838`) verbatim, with `FUZZ_TARGET` and `FUZZ_PKG` set from its
  two arguments. "The diagnostic" below is the runner's own `::error::fuzz-retry:` line (TDS-1),
  matched on that line alone, never on the replayed `go test` output, which may hold the same text:
  - F1 given the deadline fixture then the pass fixture, when the runner runs, then exit 0, two
    `go` calls, each argv exactly `test -run=^$ -fuzz=<target> -fuzztime=30s <pkg>`, and a warning
    naming the deadline flake · a retry removed, a missing warning, or a wrong target or fuzztime;
  - F2 given `panic:` alongside `context deadline exceeded`, then non-zero after one call and the
    diagnostic is `deny marker panic:` · the blind retry (red today);
  - F3 given the deadline fixture plus `signal: killed`, then non-zero after one call and the
    diagnostic is `deny marker signal: killed` · a retried OOM kill (red today);
  - F4 given the deadline fixture plus `fatal error:`, then non-zero after one call and the
    diagnostic is `deny marker fatal error:` · a retried runtime crash (red today);
  - F5a given the deadline fixture with no deny marker, and the fake `go` writing
    `<dir>/testdata/fuzz/<target>/abc`, where `<dir>` is the matrix `pkg` without its trailing
    `/...` and tracks no `testdata/`, then non-zero after one call · a corpus check blind to a
    collapsed untracked directory (red today);
  - F5b given the deadline fixture plus `Failing input written` and no corpus file, then non-zero
    after one call and the diagnostic is `deny marker Failing input written` · a retried real
    finding Go reported but did not persist (red today);
  - F5c given the deadline fixture and a fake `git` whose `status` exits 128, then non-zero after
    one call · a corpus check that fails open (red today);
  - F6 given the deadline fixture twice and then the pass fixture, then non-zero after exactly two
    calls · an unbounded or swallowed retry. — closed 2026-10-18, evidence: evidence/G1.md
- [x] 1.2 Add the remaining cases to `fuzz_retry_test.sh`, red against the same stub:
  - F7 given empty output and given unknown failure text, then non-zero after one call and the
    diagnostic is `unrecognised failure text` · fail open, and an undiagnosable Go wording
    change (red today);
  - F8 given the zero-target fixture (exit 0), then non-zero and the diagnostic is `no fuzz target
    ran` · a leg that fuzzes nothing reports green (red today);
  - F9 given exit 0 with no `fuzz: elapsed` line and no warning, then non-zero with the
    `unrecognised failure text` diagnostic · a pass that fuzzed nothing for a reason the runner
    does not know (red today);
  - F10 control: given the pass fixture, then exit 0 after one call whose argv is exactly
    `test -run=^$ -fuzz=<target> -fuzztime=30s <pkg>` · an over-strict runner, or a wrong argv;
  - F11 wiring: the `fuzz` job's fuzz step runs `hack/ci/fuzz-retry.sh "${FUZZ_TARGET}"
    "${FUZZ_PKG}"` with both set from the matrix in `env:`, and has no inline `for attempt` loop ·
    the job bypassing the runner (red today);
  - F12 given the deadline fixture plus one line outside its line set
    (`fuzzing process hung or terminated unexpectedly`), then non-zero after one call and the
    `unrecognised failure text` diagnostic · a substring match that retries unknown diagnostics
    (red today);
  - F13 given the deadline lines under `--- FAIL: FuzzOther` while the target is `FuzzX`, then
    non-zero after one call and the `unrecognised failure text` diagnostic · a retry that does not
    pin the matrix target (red today);
  - F14 given the deadline fixture truncated before `exit status 1` (its last two lines removed),
    then non-zero after one call and the `unrecognised failure text` diagnostic · a runner that
    retries a partial capture because every line it saw is allowed (red today);
  - F15 given the deadline fixture with one line repeated, and given it with `FAIL` and
    `context deadline exceeded` swapped, then non-zero after one call each and the
    `unrecognised failure text` diagnostic · a membership check standing in for the ordered
    sequence (red today). — closed 2026-10-18, evidence: evidence/G1.md
- [x] 1.3 Implement `hack/ci/fuzz-retry.sh` (`-fuzztime=30s`, allow-list, deny markers, corpus
  check with `--untracked-files=all` failing closed, messages as TDS-1); point the `fuzz` job at
  it; wire `fuzz_retry_test.sh` into `lint` in both modes and into `cfc_guards`; zizmor gate passes.
  No existing guard asserts on the `fuzz` job's steps (`grep -n fuzz hack/test/*.sh` outside the
  new guard matched nothing on 2026-10-18), so none needs updating. — closed 2026-10-18, evidence: evidence/G1.md
- [x] 1.4 Retry-decision defect controls in `--self-test`, one mutant each of a copy of the runner:
  drop each deny marker (four mutants, killed by F2-F4 and F5b through the diagnostic, since the
  allow-list still rejects the output), drop the corpus check (F5a), drop
  `--untracked-files=all` (F5a), ignore a failing `git status` (F5c), raise the retry cap (F6);
  each turns its case red; a no-op copy passes. — closed 2026-10-18, evidence: evidence/G1.md
- [x] 1.5 Output-shape defect controls in `--self-test`, one mutant each: drop the zero-target
  check (F8), drop the `fuzz: elapsed` check (F9), drop the `unrecognised failure text` diagnostic
  (F7), use `-fuzztime=1s` (F1, F10), replace the allow-list with a substring match (F12), drop the
  target pin (F13), stop requiring every line of the sequence (F14), check membership instead of
  the ordered sequence (F15); each turns its case red; a no-op copy passes. — closed 2026-10-18, evidence: evidence/G1.md
- [x] 1.6 Add an uncaptured `go mod download` step before the fuzz runner in the fuzz job
  (model-aligned decision 2026-10-18: cold-cache `go: downloading` lines must not reach the strict
  matcher); wiring assertion + mutant in fuzz_retry_test.sh --self-test. — closed 2026-10-18,
  evidence: evidence/T1.6.md

## 2. Shuffle (TDS-2)

- [x] 2.1 Probe, evidence only:
  - how `-shuffle=<seed>` prints the seed (`-test.shuffle N`), and that two packages in one
    `go test` call each get the same explicit seed;
  - that a binary shuffles once and `-count=3` repeats that order; whether a fixture failure at
    `-count=3` also reproduces with the same seed at `-count=1`;
  - that `go list ./...` skips a package under `hack/test/testdata/` while an explicit path still
    builds it (else the fixture goes behind a build tag);
  - the unit package list (`go list ./...` minus `/e2e`, as `Makefile:70` filters), confirming it
    holds no e2e package so the exclusion list starts empty;
  - the Ginkgo packages found by searching `_test.go` files for `RunSpecs` (today
    `internal/controller`, `internal/webhook/v1alpha1`); one smoke run of both suites under the
    split (three `-shuffle=<seed> -count=1` invocations each, green), and one at `-count=3`
    showing Ginkgo's "Only -count=1 is allowed";
  - the wall time of the whole split run with envtest provisioned (`make setup-envtest`), to set
    the workflow's `timeout-minutes`;
  - a seed that runs the 2.2 fixture's dependent test first. — closed 2026-10-18, evidence:
    evidence/G2.md
- [x] 2.2 Write `hack/test/test_shuffle_test.sh` (plain and `--self-test`) and an order-dependent
  fixture package under `hack/test/testdata/shuffle/` (or behind a build tag, per 2.1), so
  `go list ./...` ignores it. Script cases, red against a stub `hack/ci/test-shuffle.sh` that runs
  `go test -count=3` without `-shuffle` on `go list ./...` and ignores `SHUFFLE_SEED`. Every case
  puts a fake `make` first on `PATH` that records its calls and whose `echo-kubebuilder-assets`
  prints a temp assets directory (unless the case says otherwise). S1, S3 and S4 also put a fake
  `go` first on `PATH` whose `list` prints the fixture's import path and which forwards `test` to
  the real `go` (the TDS-2 seam; no package override exists):
  - S1 given the fixture and `SHUFFLE_SEED` set to the 2.1 seed, when the script runs, then
    non-zero and the output names the seed · the script does not pass `-shuffle` (red
    today);
  - S2 control: given the fixture run directly with `go test -shuffle=off -count=3`, then it
    passes · a fixture red for a reason
    other than order;
  - S3 given a failing run's printed seed, when rerun with that seed and `-count=3`, then the same
    test fails · `-shuffle=on` with an uncontrolled seed, or a printed seed that is not the one
    used;
  - S4 given any run, then the seed line precedes the first test output · a seed lost when the job
    times out;
  - S5 given an exclusion entry with no reason, then the guard fails · silent exclusions;
  - S6 given an exclusion entry for a non-e2e package, then the guard fails and names it · envtest
    packages quietly dropped against decision A;
  - S7 given an empty package list, then the script fails · a vacuous green;
  - S8 given a fake `go list` with one package whose `_test.go` calls `RunSpecs` and one plain
    package, then the Ginkgo package is invoked three times, never with `-count` other than 1,
    and the plain package once with `-count=3` · a nightly red every night on Ginkgo's
    `-count` refusal (red today);
  - S9a given the fake `make` printing an assets directory, then `make setup-envtest` is called
    before `make -s echo-kubebuilder-assets`, and every `go test` call sees `KUBEBUILDER_ASSETS`
    equal to that directory · provisioning or the export dropped, so envtest tests skip on a
    cache miss (red today);
  - S9b given a fake `make` whose `echo-kubebuilder-assets` prints nothing, then non-zero before
    any `go test` call, naming the missing assets · an empty assets path accepted (red today);
  - S10 given a fake `go list` with several packages, when the script runs, then the set of
    packages passed to `go test` equals that list minus the exclusion list · a package silently
    dropped outside the exclusion list. — closed 2026-10-18, evidence:
    evidence/G2.md
- [x] 2.3 Add the wiring cases to `test_shuffle_test.sh`, red against the tree before 2.4:
  - S11 `test-shuffle.yaml` has `name: Test shuffle`, `schedule:` cron `30 2 * * *`,
    `permissions: contents: read`, a SHA-pinned `actions/checkout` with
    `persist-credentials: false`, `./.github/actions/go-cache` with `envtest: true`, a SHA-pinned
    `go-task/setup-task`, `timeout-minutes` as measured in 2.1, runs `task test:shuffle`, and sets
    no `SHUFFLE_SEED` anywhere · a nightly that cannot run, leaks credentials, runs without bound
    or uses the same seed every night;
  - S12 `shuffle:` in `hack/task/Taskfile.test.yml` runs `hack/ci/test-shuffle.sh`, and
    `ci-failure-report.yaml` lists `Test shuffle` · a nightly nobody hears from. — closed 2026-10-18, evidence:
    evidence/G2.md
- [x] 2.4 Implement `hack/ci/test-shuffle.sh` (seed from `SHUFFLE_SEED` or chosen, echoed first;
  Ginkgo packages three times at `-count=1` and the rest at `-count=3`, all with
  `-shuffle=<seed>`; exclusion list
  with reasons, e2e only, empty today; envtest provisioned as `hack/coverage.sh:23-30`),
  `shuffle:` in `hack/task/Taskfile.test.yml` (so `task test:shuffle`), and
  `.github/workflows/test-shuffle.yaml` (as S11; a `workflow_dispatch` run is never reported,
  CFR-1 acts on scheduled runs only). Add `Test shuffle` to `on.workflow_run.workflows` in
  `.github/workflows/ci-failure-report.yaml:22-24` and to the hard-coded list, its comment and its
  message in `hack/test/ci_failure_report_test.sh:1547-1553`, with a self-test mutant like W1
  (`:424`) that drops it. Wire `test_shuffle_test.sh` into `lint` in both modes and into
  `cfc_guards`. The zizmor gate passes. Fix any order dependence found, each in its own commit
  before this one. — closed 2026-10-18, evidence:
  evidence/G2.md
- [x] 2.5 Defect controls in `--self-test`, one mutant each of a copy: drop `-shuffle` (S1), drop
  `-count=3` (S8), echo the seed after the run (S4), a reasonless exclusion (S5), a non-e2e
  exclusion (S6), an empty package list (S7), `-count=3` on the Ginkgo package (S8), drop
  `make setup-envtest` (S9a), drop the `KUBEBUILDER_ASSETS` export (S9a), drop the empty-assets
  check (S9b), drop one discovered package (S10), drop `persist-credentials: false` (S11), set
  `SHUFFLE_SEED` in the workflow (S11), drop `Test shuffle` from the reporter list (S12); each turns
  its case red; a no-op copy passes. — closed 2026-10-18, evidence:
  evidence/G2.md

## 3. README truth (TDS-3)

- [x] 3.1 Write `hack/test/readme_quickstart_test.sh` (plain and `--self-test`). Cases, each a
  mutant of a copy of the tree:
  - R1 given `demo-up` renamed in `Taskfile.yml`, then the test fails and names `demo-up` · a
    README naming a task that is gone;
  - R2 given `dev-up` renamed, then the test fails and names `dev-up` · commented alternatives
    skipped;
  - R3 given `kubectl get kinv,foo`, then the test fails and names `foo` · short names unchecked;
  - R3b given `kubectl get kollectprofiles,pods`, then the test fails and names `pods` only · CRD
    plural names rejected, or a non-Kollect resource accepted;
  - R4 given `kubectl apply -k config/samples/nosuch/`, then the test fails and names the path ·
    sample paths unchecked;
  - R5 given the heading renamed so no `##` heading starts with "Quick start", then the test fails
    · an empty extraction passing;
  - R6 given an included task (`task test:run`) in the section, then it passes · namespaced tasks
    reported as missing;
  - R7 control: the real tree passes;
  - R8 given the section kept but its `sh` fences removed, then the test fails · a parser that
    checks the heading and accepts an empty extraction;
  - R9 given `task test:nosuch` in the section, then the test fails and names `test:nosuch` · every
    namespaced task accepted;
  - R10 control: given invalid commands (`task nosuch`, `kubectl get foo`) only in the section's
    prose, in another section and in a `text` fence, then the test passes · an extractor that
    reads outside the section's `sh` fences. — closed 2026-10-18, evidence: evidence/G3.md
- [x] 3.2 Implement; wire into `lint` in both modes and into `cfc_guards`. — closed 2026-10-18,
  evidence: evidence/G3.md

## 5. Stage-B fixes (round 1)

- [x] 5.1 A1: readme_quickstart_test.sh checks the type of EVERY `kubectl get` TYPE/NAME operand (`kubectl get kinv/a foo/b` must fail); case + mutant — closed 2026-10-18, evidence: evidence/G5.md
- [x] 5.2 A2 + deferred F16: fuzz_retry_test.sh (F11, F16) and test_shuffle_test.sh (S11) refuse `if:` and `continue-on-error:` on the guarded job and its steps (precedent dist_operatorhub_pr_test.sh:154); cases + mutants — closed 2026-10-18, evidence: evidence/G5.md
- [x] 5.3 A3: fuzz-retry.sh retries only when the attempt exited 1 (exact flake text on rc 2/137 is a real failure); cases + mutant — closed 2026-10-18, evidence: evidence/G5.md
- [x] 5.4 A4: the fake go records a per-call exit status; a case where Ginkgo run 1 of 3 fails; mutant `|| rc=1` → `|| true` (test-shuffle.sh:78) — closed 2026-10-18, evidence: evidence/G5.md
- [x] 5.5 Deferred: `make setup-envtest` failure emits an `::error::` line; F0 empty-package case + mutant; a case replaying the real crash fixture (no retry); `internal: true` tasks do not count as existing in the README guard — closed 2026-10-18, evidence: evidence/G5.md
- [ ] 5.6 O1 (spec text): TDS-3 states that a `~~~` or indented fence of any language in the Quick start section fails (stronger than before); fold opus notes O2/O3

## 4. Land

- [ ] 4.1 Merge with the `post-merge` row open; archive in a follow-up evidence PR once it passes

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| TDS-1 | `fuzz_retry_test.sh` F1-F15, both modes; 1.4 and 1.5 mutants by exit status | only the pure deadline case retries; argv exact; zero targets and unknown text red | not-run | |
| TDS-2 | `test_shuffle_test.sh` S1-S12 with the fixed seed; 2.5 mutants | fixture red; seed printed; Ginkgo split; envtest provisioned; control green | not-run | |
| TDS-2 | 2.1 smoke run of both Ginkgo suites under the split | green; `-count=3` refused | not-run | |
| TDS-2 | first scheduled `Test shuffle` run, and the `CI failure report` run (`workflow_run`) it triggers | both complete | post-merge | follow-up evidence PR |
| TDS-3 | `readme_quickstart_test.sh` R1-R10 and R3b | each mutant red, controls and tree green | not-run | |
| all | `ci_workflow_security_test.sh` plain and `--self-test` after each `cfc_guards` edit | green | not-run | |
| all | wall time of each new guard, plain and `--self-test`, and the `lint` job delta | recorded in the group evidence | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; reviewed and archive revisions recorded | not-run | |
