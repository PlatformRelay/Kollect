# Tasks

Red first. Every test task comes before the implementation it drives and records its red in
evidence, failing on the target assertion (not on a missing file, a parse error or a usage error).
`task check` runs every `hack/test/*_test.sh` bare and, when its code declares it, with
`--self-test` (`hack/check.sh:71-111`), so a red guard cannot be committed alone: group G2 closes
in one commit, test tasks first (2.1-2.3, then 2.4 and 2.5), and the tree is green at every commit.
The new guard has both modes from its first commit, and that commit wires both into the required
`lint` job (`.github/workflows/ci.yaml:283`), as CWS-6 (`hack/test/ci_workflow_security_test.sh:432`)
demands for a guard CI runs, and adds it to the CFC-6 `cfc_guards` list
(`hack/test/ci_workflow_security_test.sh:608-611`), so deleting both of its lint steps cannot drop
it from CWS-6 discovery (the comment at `:600-607`). The group's evidence records
`ci_workflow_security_test.sh` green plain and with `--self-test` after that edit.

Fixtures and recorded reports live under `hack/test/testdata/mutation/`, never under
`openspec/changes/**/evidence/` (Markdown only, `hack/test/openspec_evidence_markdown_only_test.sh`).
Each recorded report names its source (local command, gremlins and Go version, date) in a sibling
`README.md`.

Each case below reads: given / when / then · the failure it exposes.

## 0. Prerequisite

- [ ] 0.1 Confirm change 7 is merged (#469) and `ci-failure-report.yaml` `on.workflow_run.workflows`
  (`:22-25`) is the exact list `ci_failure_report_test.sh:1549-1556` asserts; record both in the
  1.1 evidence

## 1. Probe (evidence only)

- [ ] 1.1 Probe gremlins v0.6.0 under the `go.mod` Go (1.27.2), installed exactly as the task will
  (`GOBIN=<tmp> go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`), and record:
  - `gremlins unleash --help`: the mutator flags whose default is false (expected
    `invert-assignments`, `invert-bitwise`, `invert-bwassign`, `invert-logical`, `invert-loopctrl`,
    `remove-self-assignments`); the exact flag list goes into the runner;
  - per package, once with the default set and once with every default-off mutator enabled: wall
    time and the killed, lived, not covered, timed out and not viable counts (the 2026-10-19
    baseline with defaults: ~6 s total, 21 killed, 0 lived, 1 not covered); the timeout-minutes
    for the workflow follows from the enabled-set time;
  - every LIVED mutant of the enabled set, classified as equivalent (with the reason) or a real
    test gap (file:line); none is fixed in this change (non-goal: no test changes);
  - the JSON report shape: `go_module`, `files[].file_name` (relative or absolute, which decides
    the formatter's package check), `files[].mutations[].status`; confirm it has no package path;
  - the exit status with lived mutants (0), with a package that does not compile (non-zero), and
    with `--threshold-efficacy 100` (non-zero), so the runner's "any non-zero is a tool failure"
    holds;
  - the statuses a `--dry-run` writes (RUNNABLE expected);
  - the fixture control (a draft of `hack/test/testdata/mutation/` with its own `go.mod`): the
    control mutant's file, line, mutator type and status (LIVED expected), and the status of a
    function no test calls (NOT COVERED expected).
  Save the recorded reports (three packages, the enabled set; the fixture; a dry run) as fixtures
  for G2. If a measured fact contradicts proposal.md or spec.md, stop: it is a spec finding.

## 2. Measurement, guard and wiring (MUT-1 to MUT-4), group G2, one commit

- [ ] 2.1 Write `hack/test/mutation_signal_test.sh` (plain and `--self-test`). Formatter cases, red
  against a stub `hack/ci/mutation-summary.sh <pkg>=<report>...` that prints nothing and exits 0:
  - M1 given the three recorded reports, when the formatter runs, then exit 0 and the summary has
    one row per package whose five counts equal those counted from that report's statuses · a
    wrong count, a dropped package, or timed out and not viable folded into another column;
  - M2 given a report with LIVED mutants, then exit 0 and the lived count shown · a score gate in
    the formatter;
  - M3 given, for one package, a missing file, an empty file, and invalid JSON (three cases), then
    non-zero, an `::error::` line naming the package, and no counts table · a missing measurement
    passing as clean (red today);
  - M4 given a report with zero mutations, then non-zero naming the package · a vacuous green from
    a wrong path or a narrowing flag (red today);
  - M5 given a report with a RUNNABLE, a SKIPPED and an unknown status (three cases), then non-zero
    naming the status · a dry run or a new gremlins status counted as clean (red today);
  - M6 given a report whose files lie outside the package it is passed for, then non-zero · reports
    swapped between packages, which the JSON cannot reveal by itself (red today);
  - M7 (MUT-4) given three nights replayed through the runner (night 1 and 3 the recorded reports,
    night 2 a fake gremlins exiting 1 with a build error on stderr), then nights 1 and 3 exit 0
    with their own counts and night 2 exits non-zero with an `::error::` line naming the tool
    failure and no counts table · a tool failure that reads like a night with survivors (red
    today).
- [ ] 2.2 Add runner cases to `mutation_signal_test.sh`, red against a stub `hack/ci/mutation.sh`
  that runs one `gremlins unleash ./...` and exits 0. A fake `gremlins` first on `PATH` records each
  call's argv and working directory, writes the recorded report named by its `--output`, and fails
  on demand:
  - R1 given the fake, when the runner runs, then exactly four calls in order: the fixture (in its
    module directory) and then `./internal/aggregate`, `./internal/redact`,
    `./internal/pathvalidate`, each argv exactly `unleash --output=<own report> <the 1.1 flag list>
    <pkg>` with no `--threshold-`, `--diff`, `--dry-run` or `--config` · a merged run with no
    package attribution, a missing mutator flag, a gate or a narrowed run (red today);
  - R2 given the fake exiting 1 on the second package, then the runner exits non-zero, names that
    package, and makes no further gremlins call · `|| true` or a later package hiding the failure
    (red today);
  - R3 given the fake exiting 0 without writing a report, then non-zero · a missing measurement
    (red today);
  - R4 given the fixture report with the control LIVED, then the run continues to the real
    packages; given it NOT COVERED, KILLED or absent (three cases), then non-zero before any real
    package call, naming the status · a pipeline blind to survivors (MUT-3, red today);
  - R5 given `MUTATION_FORCE_FAILURE=true`, then non-zero with `::error::` naming the forced
    failure and no gremlins call; given it unset, empty or `false`, then the run is unchanged ·
    a knob that can do more than fail the run (red today).
- [ ] 2.3 Add static cases to `mutation_signal_test.sh`, red against the tree before 2.4. Each
  mutant edits a throwaway copy of the files the guard reads (not the whole tree):
  - W1 `mutation.yaml` has `name: Mutation testing`, `schedule:` cron `30 1 * * *`,
    `workflow_dispatch:`, top-level `permissions: contents: read` and no job-level
    `permissions:`, a `timeout-minutes` (from 1.1), SHA-pinned `actions/checkout` with
    `persist-credentials: false`, `./.github/actions/go-cache`, `go-task/setup-task` and
    `actions/upload-artifact`, each at the SHA already used elsewhere in `.github/workflows/`, runs
    `task mutation`, passes `MUTATION_FORCE_FAILURE` only through `env:` from `vars.`, and has no
    `if:` or `continue-on-error:` on the job or any step except `if: always()` on the upload step,
    and no `defaults.run.shell` or step `shell:` · a nightly that cannot fail, leaks credentials,
    adds an action dependency-review never saw, or runs without bound (red today);
  - W2 `Taskfile.yml` holds `GREMLINS_VERSION: v0.6.0` (full semver) directly under
    `# renovate: datasource=go depName=github.com/go-gremlins/gremlins`; its install task runs
    `go install github.com/go-gremlins/gremlins/cmd/gremlins@{{.GREMLINS_VERSION}}` into a
    version-named path, and the `mutation` task runs `hack/ci/mutation.sh` with no `status:` or
    `ignore_error:`; no `gremlins@` appears under `.github/`, `hack/ci/` or `hack/task/` · a pin
    Renovate cannot bump, a pin CFC-2 cannot resolve, a stale binary reused, or a skipped run (red
    today);
  - T1 `--threshold-efficacy` added to the runner, to the `mutation` task and to a workflow step
    (three mutants), then the guard fails naming file and line · a gate on the command line;
  - T2 a `.gremlins.yaml` at the root and a `.gremlins.yml` in the fixture module (two mutants,
    each added to the copy's git index), then the guard fails naming the path · a gate through a
    config file the command-line scan never sees;
  - T3 `GREMLINS_UNLEASH_THRESHOLD_EFFICACY: "100"` at workflow `env:`, job `env:` and step `env:`,
    `echo GREMLINS_UNLEASH_THRESHOLD_MCOVER=100 >> "$GITHUB_ENV"` in a step, and the variable in
    `Taskfile.yml` `env:` (five mutants), then the guard fails naming file and line · a gate
    through the environment;
  - C1 `ci-failure-report.yaml` lists `Mutation testing`, and that entry equals `mutation.yaml`'s
    `name:` · a nightly nobody hears from (red today);
  - C2 control: the real tree passes; a no-op copy passes; a comment containing
    `threshold` in prose passes · an over-broad scan that reds on words, not settings.
- [ ] 2.4 Implement: the fixture module from 1.1 under `hack/test/testdata/mutation/` (own
  `go.mod`, a `go` line no newer than the root's); `hack/ci/mutation-summary.sh` and
  `hack/ci/mutation.sh` (`set -euo pipefail`, fixture first, one call per package, the 1.1 flag
  list, `::error::` lines, counts to `$GITHUB_STEP_SUMMARY`); in root `Taskfile.yml` the
  `GREMLINS_VERSION` pin, the install task and `mutation`, with DTP-5-clean comments;
  `.github/workflows/mutation.yaml` as W1. Add `Mutation testing` to
  `ci-failure-report.yaml:22-25` and to `ci_failure_report_test.sh:1549-1556` (the list, its
  comment and its message), with a self-test mutant that drops it, like `:426-427`. Wire the guard
  into `lint` in both modes and into `cfc_guards`, updating the CWS-6 header (`:20-33`) and the
  CFC-6 comment (`:600-607`). Grep `hack/test/` for step-index or step-count assertions on the
  workflows touched (`ci.yaml` lint, `ci-failure-report.yaml`) and update any found. Gates on the
  final tree: the new guard both modes; `ci_failure_report_test.sh` both modes;
  `ci_workflow_security_test.sh` both modes; `consistency_tool_pins_test.sh` (CFC-2 resolves
  gremlins v0.6.0, go 1.25 not newer than 1.27.2) and `consistency_toolchain_pins_test.sh` (DTP-5);
  `task lint:shell`; the zizmor gate at its CI command; the guard's wall time, plain and
  `--self-test`, and the lint delta.
- [ ] 2.5 Defect controls in `--self-test`, one mutant each of a copy, judged by its case's message:
  formatter: count timed out as killed (M1), exit 1 on any LIVED (M2), drop the missing-report
  check (M3), drop the zero-mutation check (M4), accept unknown statuses (M5), drop the package
  check (M6); runner: `|| true` on the gremlins call (R2, M7), drop the report-exists check (R3),
  drop one default-off flag (R1), drop the fixture step (R4), accept NOT COVERED as the control
  (R4), let `MUTATION_FORCE_FAILURE` skip gremlins and exit 0 (R5); static: each W1, W2, T1-T3 and
  C1 clause (the case lists name the mutants). Each turns its case red; the no-op copy passes.

## 3. Land

- [ ] 3.1 Merge with the post-merge rows open; archive in a follow-up evidence PR once they pass.
  Owner: after the first green scheduled run, set the repository variable
  `MUTATION_FORCE_FAILURE=true` for one night (the reporter ignores `workflow_dispatch`,
  `ci-failure-report.yaml:34`, so only a scheduled run proves it), confirm the red run and its
  `CI failure: Mutation testing` issue, unset it, and confirm the next scheduled run is green and
  the issue closes.

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| MUT-1 | 1.1 probe: version, default-off flags, counts and runtime per package, report shape | recorded; matches the spec or stops as a finding | not-run | |
| MUT-1 | M1, R1, W1, W2, C1 and their 2.5 mutants | counts exact; four calls, exact argv; wiring pinned; each mutant red | not-run | |
| MUT-1 | first scheduled `Mutation testing` run and the `CI failure report` run it triggers | artifact present, summary has five counts per package; reporter run completes | post-merge | follow-up evidence PR |
| MUT-2 | M2-M5, R2, R3, T1-T3, C2 and their 2.5 mutants | survivors green; broken or empty measurement red; every threshold source red; prose control green | not-run | |
| MUT-3 | R4 and its mutants; 1.1 fixture control on real gremlins | control LIVED continues; NOT COVERED, KILLED, absent red | not-run | |
| MUT-3 | first scheduled run's fixture step | control LIVED | post-merge | follow-up evidence PR |
| MUT-4 | M7, R5 and their mutants | night 2 red and distinguishable; knob only fails | not-run | |
| MUT-4 | owner-forced scheduled failure (3.1), then recovery | red run, `CI failure: Mutation testing` issue opened, then closed on the green night | post-merge | follow-up evidence PR |
| all | `ci_workflow_security_test.sh`, `ci_failure_report_test.sh`, CFC-2 and DTP guards, zizmor, after the 2.4 edits | green | not-run | |
| all | guard wall time plain and `--self-test`; `lint` job delta | recorded in the G2 evidence | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; reviewed and archive revisions recorded | not-run | |
