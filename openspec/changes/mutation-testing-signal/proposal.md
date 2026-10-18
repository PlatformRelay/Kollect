# Proposal

## Why

Mutation testing is a process rule (`docs/development/spec-workflow.md:51`, the risk table:
deletion, tenancy, credentials need "a mutation or negative control") with no measurement: nobody
can say which packages' tests actually catch mutants. The reference project attune measures it
nightly, report only.

Measured on 2026-10-19 (gremlins v0.6.0, go1.27.2, default mutators, one run per package): about
6 s wall time for all three packages together, 21 mutants killed, 0 lived, 1 not covered. With the
default set the report would be near-constant, which is why decision "flat signal" (below) enables
the mutators gremlins ships switched off.

## What Changes

A nightly workflow of its own, `.github/workflows/mutation.yaml` (`name: Mutation testing`, cron
`30 1 * * *`, clear of `Test shuffle` at 02:30 and `E2E nightly` at 03:00), runs `task mutation`.
That task installs gremlins at the pinned version and runs `hack/ci/mutation.sh`, which:

- runs gremlins first on a fixture module (`hack/test/testdata/mutation/`, its own `go.mod`) and
  fails unless the fixture's control mutant is LIVED (MUT-3);
- runs gremlins once per package on `internal/aggregate`, `internal/redact` and
  `internal/pathvalidate`, each to its own JSON report, because the v0.6.0 JSON report names the
  Go module (`go_module`) and files but no package path;
- passes every one of v0.6.0's eleven mutator flags explicitly (so the six it ships switched off
  are enabled, and no environment value can switch one off) and `--config=hack/ci/gremlins.yaml`,
  a committed file whose content is `{}`, so gremlins reads no other config file;
- refuses to start when any `GREMLINS_` environment variable is set, whatever set it;
- writes the reports to a fresh temporary directory, so a stale report can never stand in for a
  missing one;
- hands the reports to `hack/ci/mutation-summary.sh`, which writes killed, lived, not covered,
  timed out and not viable counts per package to the job summary. It validates every report
  before it prints anything, and fails on a missing, empty or unreadable report without printing
  a partial table.

The reports are uploaded as an artifact. It is a measurement, explicitly not a gate: a low score
never fails the run, a missing measurement always does. It is a separate workflow so its failures
get their own `ci-failure` issue (change 7's reporter lists it by name) and cannot mask the nightly
suite.

Model-aligned decisions applied (agent-context/decisions.md 2026-10-19, change 9, pending operator
validation):

- **Threshold sources (A):** MUT-2 also rejects thresholds set through a config file and through
  `GREMLINS_*THRESHOLD*` environment variables at workflow, job or step level, each with a mutant.
  Report-only must not be bypassable silently. The config-file ban covers `.gremlins` and every
  `.gremlins.*`, not only `.yaml`/`.yml`: gremlins names its config `.gremlins` and forces the YAML
  decoder, and viper tries every extension and the bare name (probe 2026-10-19: a YAML threshold
  in `.gremlins.json` and in a bare `.gremlins` both exit 10). The runner's `--config` and its
  `GREMLINS_` refusal enforce the same rule at run time, for sources a file scan cannot see (an
  `env:` built from a `${{ }}` expression, a Taskfile `dotenv:`).
- **Fixture control placement (A):** the MUT-3 fixture control runs in the nightly only, before
  the real packages. The required `lint` job runs only the static guard and never installs or runs
  gremlins. It does gain one module resolution: CFC-2, already in `lint`
  (`.github/workflows/ci.yaml:619-621`), resolves every pin through the module proxy with
  `go mod download`, the new one included; the `lint` delta is measured cold and warm.
- **Flat signal (A):** keep the three packages and enable gremlins' default-off mutators
  explicitly; task 1.1 re-measures counts and runtime with them; the run stays report-only.

## Capabilities

### New Capabilities

- `mutation-testing`: what the nightly mutation measurement must report and must not do.

### Modified Capabilities

None. Change 7's reporter list grows by one entry; that is a test change on its guard, not a
change to its requirements (CFR-1 already acts on any listed workflow).

## Impact

- New: `.github/workflows/mutation.yaml`, `hack/ci/mutation.sh` (runner),
  `hack/ci/mutation-summary.sh` (formatter), `hack/ci/gremlins.yaml` (the only config gremlins
  reads, content `{}`), `hack/test/mutation_signal_test.sh` (guard, plain and `--self-test`), `hack/test/testdata/mutation/` (fixture module and recorded reports).
- `Taskfile.yml`: `GREMLINS_VERSION: v0.6.0`, a two-space entry of the top-level `vars:` (the
  shape CFC-2's resolver reads, `hack/test/consistency_tool_pins_test.sh:129-132`), directly under
  `# renovate: datasource=go depName=github.com/go-gremlins/gremlins` (the shape the Taskfile
  custom manager matches, `renovate.json:27-36`); an install task running
  `go install github.com/go-gremlins/gremlins/cmd/gremlins@{{.GREMLINS_VERSION}}`; a `mutation`
  task. Both live in the root `Taskfile.yml`, because CFC-2 resolves a `{{.VAR}}` pin only there
  (`hack/test/consistency_tool_pins_test.sh:163`); an included Taskfile or a workflow `${{ }}`
  would read as an unresolved pin and fail CFC-2. DTP-5 comment rules apply to every comment near
  the new pin (`hack/test/consistency_toolchain_pins_test.sh:522`).
- `.github/workflows/ci-failure-report.yaml:22-25`: `Mutation testing` added to
  `on.workflow_run.workflows`, and `hack/test/ci_failure_report_test.sh:1549-1556` (the exact-list
  assertion, its comment and message) plus a self-test mutant that drops it, as for `Test shuffle`
  (`:426-427`). This is a test change on change 7's guard.
- `.github/workflows/ci.yaml` `lint` job (`:283`): two steps run the new guard plain and with
  `--self-test`; `hack/test/ci_workflow_security_test.sh:608-611` adds it to `cfc_guards`, with
  the CWS-6 header comment (`:20-33`) and the CFC-6 comment above the list (`:600-607`).
- The workflow reuses only actions already pinned in the repository (checkout, go-cache,
  setup-task, upload-artifact at their current SHAs), so dependency-review sees no new action. The
  zizmor gate passes.

## Dependencies

Landing order across the ten proposed changes: (1) ci-workflow-hardening, (2) task-check-entrypoint,
(3) golangci-lint-bump, (4) cross-file-consistency-gates, (5) developer-toolchain-pins,
(6) dependency-update-automation, (7) ci-failure-reporting, (8) test-depth-signals,
(9) mutation-testing-signal, (10) public-agent-contract.
Hard dependency: change 7 must have shipped (merged as #469; change 8 merged since, adding
`Test shuffle` to the same list).

## Non-goals

- A mutation-score threshold or any gate (needs a baseline from several nightly reports; a later
  change decides it). Also no baseline or suppression file for known lived mutants: a list that
  hides survivors is a gate by another name and belongs to that later change.
- Packages beyond the three; changing any test.
- Running the fixture control on pull requests. A non-required, path-filtered PR job would catch a
  broken pipeline before merge; it is a possible follow-up, not this change.
- Scanning gremlins configuration outside the repository (see *Risks*).

## Risks

- **Equivalent-mutant noise (flat-signal decision).** The default-off mutators, especially
  `invert-assignments` and `remove-self-assignments`, can produce mutants no test can kill. They
  would show as permanent LIVED noise and wear down trust in the report as a flat signal does. Task
  1.1 classifies every LIVED mutant (equivalent or a real test gap) in its evidence; the report
  stays report-only, and dropping a mutator that is mostly noise is a recorded decision, not a
  silent edit.
- **Late detection (fixture-placement decision).** A PR that breaks the pipeline (a gremlins bump,
  a Taskfile edit, a report format change) can merge and stay unnoticed until the next nightly,
  about a day. The reporter then opens an issue; if those issues are ignored the breakage lasts
  longer.
- **Guard scope (threshold-sources decision).** The static guard reads the repository's files
  only. Without `--config`, gremlins v0.6.0 would also read `/etc/gremlins`,
  `$XDG_CONFIG_HOME/gremlins/gremlins` and `~/.gremlins` (`internal/configuration/configuration.go`,
  `defaultConfigPaths`); passing `--config` closes those (probe 2026-10-19: a threshold in the
  module root's `.gremlins.yaml` exits 10 without it and 0 with it). A later gremlins that adds a
  config source `--config` does not override, changes its env prefix, or renames its threshold
  keys would pass the guard while missing the new source; the pinned version means that only
  happens on a reviewed bump, whose PR must re-check this list.

## Assumptions

- gremlins (`github.com/go-gremlins/gremlins`, Apache-2.0) v0.6.0 builds under the `go.mod` Go
  (its `go` directive is 1.25; `go.mod` here is 1.27.2) and was run on the three packages on
  2026-10-19. It is pinned explicitly: the module proxy's `@latest` answers v0.5.1, not v0.6.0.
- v0.6.0's default-off mutators are `invert-assignments`, `invert-bitwise`, `invert-bwassign`,
  `invert-logical`, `invert-loopctrl` and `remove-self-assignments`
  (`internal/configuration/mutantenabled.go`). `arithmetic-base` and `invert-negatives`, named as
  examples in the decision brief, are on by default; task 1.1 confirms the set from
  `gremlins unleash --help`.
- gremlins exits non-zero only on an error or a configured threshold (exit 10 for efficacy); with
  no threshold, any non-zero exit is a tool failure. It also exits 0 and writes no report when it
  finds nothing to report or is cancelled (`internal/report/report.go:255-260`,
  `cmd/unleash.go:135-136`), so a missing report is a failure in its own right.
- In v0.6.0 a threshold given on the command line or through the environment has no effect
  (probe 2026-10-19: `--threshold-efficacy 100` and `GREMLINS_UNLEASH_THRESHOLD_EFFICACY=100` both
  exit 0 on a 0 % efficacy run; viper hands the value back as a string and gremlins' typed lookup
  reads 0). Only a YAML threshold gates today. The ban on command-line and environment thresholds
  stays as policy, because a later version can fix that lookup.
- Any value of a `GREMLINS_MUTANTS_<TYPE>_ENABLED` variable switches that mutator off, `true`
  included (probe 2026-10-19, same lookup); an explicit flag wins over the environment. Hence the
  explicit flags for all eleven mutators and the runner's `GREMLINS_` refusal.
- A `go install`ed gremlins reports its version as `dev`, so the version is held by a
  version-named install path, not by asking the binary.
- Gremlins reports code that no test executes as NOT COVERED, never LIVED. The control therefore
  needs a function the test EXECUTES but does not assert on.
- A dry run (`--dry-run`) reports mutants as RUNNABLE, and v0.6.0 also has SKIPPED; neither is a
  measurement, so the formatter treats them, and any status it does not know, as a failure.
- The three packages: pure (no Docker, no network), small enough for a nightly budget, and
  correctness- or security-critical: `aggregate` (content hashing and dedupe, fuzzed by
  `FuzzContentHash`, `internal/aggregate/dedupe_test.go:92`), `redact` (secret scrubbing),
  `pathvalidate` (path safety).
- The reporter acts on scheduled runs only (`ci-failure-report.yaml:34`), so the live failure proof
  must be a scheduled run, not a dispatch. It is forced by an owner-set repository variable
  `MUTATION_FORCE_FAILURE` read by one step that can only fail the run (never skip or pass it),
  pinned by the guard. The runner reads it; the workflow passes it through `env:` only.
