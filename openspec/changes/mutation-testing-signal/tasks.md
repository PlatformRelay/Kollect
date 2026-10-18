# Tasks

Red first. Every test task comes before the implementation it drives and records its red in
evidence, failing on the target assertion (not on a missing file, a parse error or a usage error).
`task check` runs every `hack/test/*_test.sh` bare and, when its code declares it, with
`--self-test` (`hack/check.sh:71-111`), so a red guard cannot be committed alone: group G2 closes
in one commit, test tasks first (2.1-2.6, then 2.7 and 2.8), and the tree is green at every commit.
The new guard has both modes from its first commit, and that commit wires both into the required
`lint` job (`.github/workflows/ci.yaml:283`), as CWS-6 (`hack/test/ci_workflow_security_test.sh:432`)
demands for a guard CI runs, and adds it to the CFC-6 `cfc_guards` list
(`hack/test/ci_workflow_security_test.sh:608-611`), so deleting both of its lint steps cannot drop
it from CWS-6 discovery (the comment at `:600-607`). The group's evidence records
`ci_workflow_security_test.sh` green plain and with `--self-test` after that edit.

A wiring case whose file does not exist yet reds through the guard's own absence message (as
change 7's "workflow file deleted (absence)" mutant does), never through a shell error. The guard
collects every failure before it exits (the `bad` style of `ci_failure_report_test.sh`, not the
exit-on-first `fail` of `ci_workflow_security_test.sh:56-59`), so a mutant that breaks two clauses
still reds with the message of the clause it names. It reads YAML through `yq` with anchors
expanded (`explode(.)`), so block, flow and aliased forms are one form.

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
  (`GOBIN=<absolute tmp>/gremlins-v0.6.0 go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`;
  `go install` refuses a relative `GOBIN`, believed), and record:
  - `gremlins unleash --help`: all eleven mutator flags, and which default to false (expected
    `invert-assignments`, `invert-bitwise`, `invert-bwassign`, `invert-logical`, `invert-loopctrl`,
    `remove-self-assignments`); the exact flag list goes into the runner;
  - per package, once with the default set and once with the runner's exact argv (every mutator
    flag, absolute `--config=<an empty {} file>`, `--output=<dir>/<short-pkg>.json`): wall time and
    the killed, lived, not covered, timed out and not viable counts (the 2026-10-19 baseline with
    defaults: ~6 s total, 21 killed, 0 lived, 1 not covered); the workflow's `timeout-minutes`
    follows from the runner-argv time;
  - every LIVED mutant of the runner-argv run, classified as equivalent (with the reason) or a
    real test gap (file:line); none is fixed in this change (non-goal: no test changes);
  - the JSON report shape: `go_module`, `files[].file_name` (relative to the package directory in
    a 2026-10-19 scratch probe), `files[].mutations[]` with `type`, `status`, `line`, `column`;
    confirm it has no package path;
  - exit statuses: lived mutants (0); a package that does not compile (non-zero); a YAML threshold
    in the module root's `.gremlins.yaml`, `.gremlins.json` and bare `.gremlins` (10 each), and
    the same with `--config=<empty file>` (0); `--threshold-efficacy 100` and
    `GREMLINS_UNLEASH_THRESHOLD_EFFICACY=100` (0 in the scratch probe: inert in v0.6.0); nothing to
    report (0, no report written); an `--output` whose directory does not exist;
  - that `GREMLINS_MUTANTS_ARITHMETIC_BASE_ENABLED=true` switches the mutator off without the flag
    and not with it;
  - the statuses a `--dry-run` writes (RUNNABLE expected);
  - the fixture control (a draft of `hack/test/testdata/mutation/` with its own `go.mod`), under
    the runner's exact argv: the control mutant's file, line, column, mutator type and status
    (LIVED expected), and the status of a function no test calls (NOT COVERED expected).
  Save the recorded reports (three packages and the fixture under the runner's argv; a dry run)
  as fixtures for G2. If a measured fact contradicts proposal.md or spec.md, stop: it is a spec
  finding.

## 2. Measurement, guard and wiring (MUT-1 to MUT-4), group G2, one commit

- [ ] 2.1 Write `hack/test/mutation_signal_test.sh` (plain and `--self-test`). Formatter cases, red
  against a stub `hack/ci/mutation-summary.sh <pkg>=<report>...` that prints `ok` and exits 0. Each
  invalid-report case puts the bad report LAST of three, so a formatter that prints as it goes
  shows a partial table:
  - M1 given the three recorded reports, and a synthetic report holding each of KILLED, LIVED, NOT
    COVERED, TIMED OUT and NOT VIABLE at least once, when the formatter runs, then exit 0 and one
    row per package whose five counts equal those counted from that report's statuses · a wrong
    count, a dropped package, or timed out and not viable folded into another column (red today);
  - M2 given a report with LIVED mutants, then exit 0 and the lived count shown · a score gate in
    the formatter (red today: no count shown);
  - M3 given a missing file, an empty file, invalid JSON, and valid JSON without `files` (four
    cases), then non-zero, an `::error::mutation:` line naming the package, and no counts table
    · a missing measurement passing as clean, or a partial table (red today);
  - M4 given a report with zero mutations, then the same failure shape · a vacuous green (v0.6.0
    writes no report in that case, so this guards the format, not today's tool) (red today);
  - M5 given a RUNNABLE, a SKIPPED and an unknown status (three cases), then the same failure
    shape, naming the status · a dry run or a new gremlins status counted as clean (red today);
  - M6 given a report whose `file_name` does not exist in the package directory it is passed for,
    then the same failure shape · reports swapped between packages, which the JSON cannot reveal
    by itself (red today);
  - M8 given a package argument missing, given twice, or not one of the three (three cases), then
    the same failure shape · a run that measures fewer packages than it claims (red today).
- [ ] 2.2 Add measurement cases for the runner, red against a stub
  `hack/ci/mutation.sh <gremlins-bin>` that runs `<gremlins-bin> unleash ./...` once and exits 0.
  A fake gremlins (passed as the argument) records each call's argv and working directory, writes
  the recorded report named by its `--output`, and can fail or skip writing on a named call:
  - R1 given the fake, when the runner runs, then exactly four calls in order: the fixture (in its
    module directory), then `./internal/aggregate`, `./internal/redact`, `./internal/pathvalidate`,
    each argv exactly `unleash --config=<absolute repo root>/hack/ci/gremlins.yaml
    --output=<fresh dir>/<short-pkg>.json <the eleven 1.1 mutator flags> <pkg>` (`<short-pkg>`:
    `fixture`, `aggregate`, `redact`, `pathvalidate`), and the output directory did not exist
    before the run · a merged run with no package attribution, a missing mutator flag, a gate, a
    narrowed run, a config search left open, or an output path in a missing subdirectory (red
    today);
  - R2 given the fake exiting 1 on the second package, then the runner exits non-zero with an
    `::error::mutation:` line naming that package, makes no further gremlins call, and prints no
    counts table · `|| true` or a later package hiding the failure (red today: the stub exits 0);
  - R3 given the fake exiting 0 without writing a report, and given a stale valid report already
    at the path a previous run used, then non-zero with the runner's own
    `::error::mutation: no report for <pkg>` line · a missing or stale measurement (red today);
  - R4 given the fixture report with the control LIVED, then the run continues to the real
    packages; given it NOT COVERED, KILLED, twice, or absent while another mutant is LIVED (four
    cases), then non-zero before any real package call, naming the status · a pipeline blind to
    survivors (MUT-3, red today).
- [ ] 2.3 Add environment, night and hand-off cases for the runner, against the same stub and
  fake:
  - R5 given `MUTATION_FORCE_FAILURE=true`, then non-zero with `::error::mutation:` naming the
    forced failure and no gremlins call; given it unset, empty or `false`, then the run is
    unchanged · a knob that can do more than fail the run (red today);
  - R6 given, one at a time, `GREMLINS_UNLEASH_DIFF=HEAD`, `GREMLINS_UNLEASH_TAGS=x`,
    `GREMLINS_MUTANTS_ARITHMETIC_BASE_ENABLED=true`, `GREMLINS_UNLEASH_THRESHOLD_EFFICACY=100`, an
    unrelated `GREMLINS_KOLLECT_PROBE=1` and an exported empty `GREMLINS_UNLEASH_TAGS=` (six
    cases), then non-zero before any gremlins call, naming the variable · a narrowed or gated run
    through a source the static scan cannot see, or a refusal that is a fixed list of names (red
    today);
  - R7 (MUT-4) given three nights (nights 1 and 3 the recorded reports, night 2 the fake exiting 1
    with a build error on stderr), then nights 1 and 3 exit 0 with their own counts, and night 2
    exits non-zero with an `::error::mutation:` line naming the tool failure and no counts table
    · a tool failure that reads like a night with survivors (red today);
  - R8 given `GITHUB_OUTPUT` set to a temp file, then before the first gremlins call it holds
    `reports=<dir>`, and after a green run `<dir>` still exists and holds the four reports · an
    upload step pointed at nothing, or reports deleted on exit (red today).
- [ ] 2.4 Add workflow cases, red against the tree before 2.7. Each mutant edits a throwaway copy
  of only the files the guard reads:
  - W1 `mutation.yaml` has `name: Mutation testing`; `on:` exactly `schedule:` cron `30 1 * * *`
    and `workflow_dispatch:`; top-level `permissions: contents: read` and no job-level
    `permissions:`; no workflow or job `env:`; one job with a `timeout-minutes` (from 1.1) and
    steps exactly, in order: `actions/checkout` with `persist-credentials: false`,
    `./.github/actions/go-cache`, `go-task/setup-task`, a step with `id: mutation` whose `run:` is
    exactly `task mutation` and whose `env:` is exactly
    `MUTATION_FORCE_FAILURE: ${{ vars.MUTATION_FORCE_FAILURE }}`, and `actions/upload-artifact`
    with `if: always()`, `path: ${{ steps.mutation.outputs.reports }}` and
    `if-no-files-found: error`; each action at the SHA (and setup-task `version:`) already used
    elsewhere in `.github/workflows/`; no other `if:`, no `continue-on-error:`, no
    `defaults.run.shell` or step `shell:` · a nightly that cannot fail, leaks credentials, adds an
    action dependency-review never saw, uploads nothing, or runs without bound (red today).
    Mutants: `run: task mutation || true`; an extra `run:` step; `persist-credentials` dropped;
    `continue-on-error: true` on the job; `if:` on the run step; `upload-artifact` at another SHA;
    the upload `path:` changed; `if-no-files-found: warn`; the cron changed; `timeout-minutes`
    dropped; a job-level `permissions:`; the run step's `env:` given a second key.
- [ ] 2.5 Add install-chain and scope cases, red against the tree before 2.7:
  - W2 `Taskfile.yml` holds `GREMLINS_VERSION: v0.6.0` (full semver) as a two-space top-level
    `vars:` entry directly under `# renovate: datasource=go depName=github.com/go-gremlins/gremlins`;
    the install task's `cmds:` is exactly one string,
    `GOBIN={{.ROOT_DIR}}/bin/gremlins-{{.GREMLINS_VERSION}} go install github.com/go-gremlins/gremlins/cmd/gremlins@{{.GREMLINS_VERSION}}`;
    the `mutation` task's `deps:` is exactly the install task and its `cmds:` exactly one string,
    `bash hack/ci/mutation.sh {{.ROOT_DIR}}/bin/gremlins-{{.GREMLINS_VERSION}}/gremlins`; neither
    task has `status:`, `sources:`, `generates:`, `method:`, `preconditions:`, `ignore_error:`
    (task or command level), `env:` or `dotenv:`, and the Taskfile has no top-level `dotenv:` · a
    pin Renovate cannot bump, a pin CFC-2 cannot resolve, a relative `GOBIN`, a stale binary
    reused, the wrong binary run, or a skipped run (red today). Mutants: the annotation line
    dropped; `v0.6` as the version; a literal version on the install line; `sources:` +
    `generates:` on `mutation`; `status: [true]` on the install task; a command-level
    `ignore_error: true`; the runner argument pointed at `gremlins` on `PATH`;
  - W3 the scan set is the `Makefile`, `Taskfile.yml` and every Taskfile its `includes:` names
    (recursively), every file under `.github/`, and every file under `hack/` except `hack/test/`;
    in it, `go-gremlins` appears only on the W2 install line and the renovate annotation;
    `gremlins` as a command word only in `hack/ci/mutation.sh`; and the install task's name,
    `task mutation`, a `mutation` dependency and `hack/ci/mutation.sh` only at the W1 run step and
    the W2 `mutation` task · an install or run outside the pin, or gremlins reached from a
    required job (red today). Mutants: a `go install …/gremlins@v0.5.1` in the `Makefile`; a
    `hack/install-gremlins.sh` that downloads a release; a workflow step
    `run: gremlins unleash ./...`; a `run: task mutation` step in the `ci.yaml` `lint` job; a
    `deps: [mutation]` on another Taskfile task; an included Taskfile outside `hack/task/` that
    runs `hack/ci/mutation.sh`.
- [ ] 2.6 Add threshold-source, reporter and scan-error cases, red against the tree before 2.7:
  - T1 `--threshold-efficacy`, `--threshold_efficacy=100`, `--threshold-mcover` and
    `--threshold.mcover` added to the runner, the `mutation` task and a workflow step (one mutant
    per spelling and file), then the guard fails naming file and line · a gate on the command
    line;
  - T2 a tracked `.gremlins`, `.gremlins.yaml` and `.gremlins.json` at the root and a
    `.gremlins.yml` in the fixture module (four mutants, each added to the copy's git index), and
    `hack/ci/gremlins.yaml` given an `unleash:` key, then the guard fails naming the path · a gate
    through a config file the command-line scan never sees;
  - T3 `GREMLINS_UNLEASH_THRESHOLD_EFFICACY: "100"` at workflow `env:`, job `env:` and step `env:`,
    the same as a flow map (`env: {…}`), with a quoted key, and through an anchor and alias,
    `echo GREMLINS_UNLEASH_THRESHOLD_MUTANT_COVERAGE=100 >> "$GITHUB_ENV"` in a step, and the
    variable in `Taskfile.yml` `env:` (nine mutants), then the guard fails naming file and line
    · a gate through the environment, or a line scan blind to another YAML form;
  - C1 `ci-failure-report.yaml` lists `Mutation testing`, and that entry equals `mutation.yaml`'s
    `name:` · a nightly nobody hears from (red today);
  - C2 controls: the real tree passes; a no-op copy passes; a comment containing `threshold` in
    prose passes · an over-broad scan that reds on words, not settings;
  - C3 an unparsable copy of each YAML file the guard reads (`mutation.yaml`, `Taskfile.yml`,
    `hack/ci/gremlins.yaml`, `ci-failure-report.yaml`), an unreadable file in the W3 scan set, and
    `git ls-files` failing (a copy that is not a git repository), each red with the guard's own
    scan-error message · a scan error read as "nothing found".
- [ ] 2.7 Implement: the fixture module from 1.1 under `hack/test/testdata/mutation/` (own
  `go.mod`, a `go` line no newer than the root's); `hack/ci/gremlins.yaml` (`{}` and a comment);
  `hack/ci/mutation-summary.sh` and `hack/ci/mutation.sh` (`set -euo pipefail`, `GREMLINS_`
  prefix refusal and force-failure check first, a `mktemp -d` report directory written to
  `$GITHUB_OUTPUT` and kept, fixture first, one call per package with the 1.1 flag list and the
  absolute `--config`, validate all reports before printing, counts to `$GITHUB_STEP_SUMMARY`);
  in root `Taskfile.yml` the `GREMLINS_VERSION` pin, the install task and `mutation` as W2, with
  DTP-5-clean comments; `.github/workflows/mutation.yaml` as W1. Add `Mutation testing` to
  `ci-failure-report.yaml:22-25` and to `ci_failure_report_test.sh:1549-1556` (the list, its
  comment and its message), with a self-test mutant that drops it, like `:426-427`. Wire the guard
  into `lint` in both modes and into `cfc_guards`, updating the CWS-6 header (`:20-33`) and the
  CFC-6 comment (`:600-607`), with a CWS-7 "mutation signal guard unwired" mutant like
  `ci_workflow_security_test.sh:1136`. Grep `hack/test/` for step-index or step-count assertions on
  the workflows touched (`ci.yaml` lint, `ci-failure-report.yaml`) and update any found. Gates on
  the final tree: the new guard both modes; `ci_failure_report_test.sh` both modes;
  `ci_workflow_security_test.sh` both modes; `consistency_tool_pins_test.sh` (CFC-2 resolves
  gremlins v0.6.0, go 1.25 not newer than 1.27.2) and `consistency_toolchain_pins_test.sh` (DTP-5);
  `task lint:shell`; the zizmor gate at its CI command; the real chain once, `task mutation` with
  `GITHUB_OUTPUT` and `GITHUB_STEP_SUMMARY` pointed at temp files (green, summary and four reports
  recorded) and once with `MUTATION_FORCE_FAILURE=true` (red); wall time of the guard plain and
  `--self-test`, and of CFC-2 with a cold and a warm module cache, as the `lint` delta.
- [ ] 2.8 Defect controls in `--self-test`, one mutant each of a copy, judged by its case's message:
  formatter: count timed out as killed (M1), exit 1 on any LIVED (M2), print rows before
  validating the last report (M3), drop the zero-mutation check (M4), accept unknown statuses
  (M5), drop the package-file check (M6), accept a duplicate package (M8); runner: `|| true` on
  the gremlins call (R2, R7), drop the runner's report-exists check (R3), reuse a fixed output
  directory (R3), drop one mutator flag (R1), drop `--config` or make it relative (R1), drop the
  fixture step (R4), accept NOT COVERED as the control (R4), match any LIVED mutant as the control
  (R4), let `MUTATION_FORCE_FAILURE` skip gremlins and exit 0 (R5), drop the `GREMLINS_` refusal
  (R6), replace it with a fixed list of the three threshold and narrowing names (R6), ignore empty
  values (R6), drop the `$GITHUB_OUTPUT` line or remove the directory on exit (R8); static: the
  mutants 2.4-2.6 list. Each turns its case red; the no-op copy passes.

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
| MUT-1 | 1.1 probe: version, mutator flags, counts and runtime per package, report shape, exit statuses | recorded; matches the spec or stops as a finding | not-run | |
| MUT-1 | M1, M8, R1, R8, W1-W3, C1 and their 2.8 mutants | counts exact; four calls, exact argv; report hand-off; wiring and install chain pinned; each mutant red | not-run | |
| MUT-1 | 2.7 real chain: `task mutation` locally, plain and forced | green with summary and four reports; forced run red | not-run | |
| MUT-1 | first scheduled `Mutation testing` run and the `CI failure report` run it triggers | artifact present, summary has five counts per package; reporter run completes | post-merge | follow-up evidence PR |
| MUT-2 | M2-M6, R2, R3, R6, T1-T3, C2, C3 and their 2.8 mutants | survivors green; broken, stale or empty measurement red; every threshold source red; prose control green; scan errors red | not-run | |
| MUT-3 | R4 and its mutants; 1.1 fixture control on real gremlins | control LIVED continues; NOT COVERED, KILLED, duplicated, absent red | not-run | |
| MUT-3 | first scheduled run's fixture step | control LIVED | post-merge | follow-up evidence PR |
| MUT-4 | R7, R5 and their mutants | night 2 red and distinguishable; knob only fails | not-run | |
| MUT-4 | owner-forced scheduled failure (3.1), then recovery | red run, `CI failure: Mutation testing` issue opened, then closed on the green night | post-merge | follow-up evidence PR |
| all | `ci_workflow_security_test.sh`, `ci_failure_report_test.sh`, CFC-2 and DTP guards, zizmor, after the 2.7 edits | green | not-run | |
| all | guard wall time plain and `--self-test`; CFC-2 cold and warm; `lint` job delta | recorded in the G2 evidence | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; reviewed and archive revisions recorded | not-run | |
