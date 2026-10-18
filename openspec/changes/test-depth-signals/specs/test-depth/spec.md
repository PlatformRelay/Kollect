# Spec Delta

## Purpose

Defines refusals that go beyond "the tests passed once, in source order": a fuzz runner that
cannot hide a crash or a no-op, shuffled repeated unit runs, and a README quick start that names
things that exist.

## ADDED Requirements

### Requirement: TDS-1 Fuzz runs once per target, retries only the known flake, and cannot be empty

`hack/ci/fuzz-retry.sh <target> <pkg>` SHALL run
`go test -run='^$' -fuzz=<target> -fuzztime=30s <pkg>`. It SHALL retry at most once, and only
when the failed attempt is exactly the known deadline flake: once `fuzz:` progress lines are
removed, its output is the flake's line sequence captured from the real runs in task 1.1, each
line exactly once and in this order (today: `--- FAIL: <target> (<duration>)` naming the matrix
target, an indented `context deadline exceeded`, `FAIL`, `exit status 1`,
`FAIL\t<import path>\t<duration>`); a missing, repeated or reordered line is not the flake. And
`git status --porcelain --untracked-files=all` shows no path under `testdata/fuzz/`. A `git status`
that fails SHALL fail the job (fail closed). The runner's own diagnostics are lines starting
`::error::fuzz-retry:`, distinct from the replayed `go test` output. The deny markers `panic:`,
`fatal error:`, `Failing input written` and `signal: killed` SHALL fail the job at once with the
diagnostic `::error::fuzz-retry: deny marker <marker>`. Every other failure SHALL fail the job at
once (fail closed) with the diagnostic `::error::fuzz-retry: unrecognised failure text`, so a
change in Go's wording is diagnosable from the log. A run that exits 0 with `no fuzz tests to fuzz`
in its output and no `fuzz: elapsed` line SHALL fail with `::error::fuzz-retry: no fuzz target ran`.
A run that exits 0 with no `fuzz: elapsed` line and without that warning SHALL also fail with the
`unrecognised failure text` diagnostic: a pass that fuzzed nothing is never green.

#### Scenario: Pure deadline artifact

- **WHEN** the first attempt's output is exactly the deadline flake for the matrix target, with no
  path under `testdata/fuzz/`, and the second attempt passes
- **THEN** the job passes and logs a warning naming the deadline flake

#### Scenario: Panic is never retried

- **WHEN** the first attempt prints `panic:`, even alongside `context deadline exceeded`
- **THEN** the job SHALL fail without a second attempt, naming `panic:`

#### Scenario: Killed process

- **WHEN** the output is the deadline flake plus `signal: killed`
- **THEN** the job SHALL fail without a second attempt, naming `signal: killed`

#### Scenario: Crash corpus written in a package with no tracked testdata

- **WHEN** the first attempt prints the deadline flake and no deny marker, and a new file appears
  under `<dir>/testdata/fuzz/`, where `<dir>` is the package directory (the matrix `pkg` without
  its trailing `/...`) and tracks no `testdata/` directory
- **THEN** the job SHALL fail without a second attempt

#### Scenario: git status fails

- **WHEN** the first attempt prints the deadline flake and `git status` exits non-zero
- **THEN** the job SHALL fail without a second attempt

#### Scenario: Deadline twice

- **WHEN** both attempts fail with the deadline artifact
- **THEN** the job SHALL fail after exactly two attempts

#### Scenario: Unknown failure text

- **WHEN** the output is empty, matches none of the known patterns, or is the deadline flake plus
  one line outside its line set
- **THEN** the job SHALL fail without a retry, and the message SHALL contain
  `unrecognised failure text`

#### Scenario: Deadline under another target

- **WHEN** the deadline lines follow `--- FAIL: FuzzOther` while the matrix target is another one
- **THEN** the job SHALL fail without a retry, and the message SHALL contain
  `unrecognised failure text`

#### Scenario: Zero fuzz targets

- **WHEN** `go test` exits 0, prints `no fuzz tests to fuzz` (a renamed target, a wrong package)
  and no `fuzz: elapsed` line
- **THEN** the job SHALL fail and say that no fuzz target ran; a matrix leg that fuzzes nothing
  never passes

### Requirement: TDS-2 Unit tests do not depend on order

`task test:shuffle` (`shuffle:` in `hack/task/Taskfile.test.yml`) SHALL choose a seed, print it
before any test runs, and run every unit package three times with `-shuffle=<seed>`. A package
whose `_test.go` files call `RunSpecs` SHALL run as three invocations of
`go test -shuffle=<seed> -count=1` and SHALL never get another `-count` (Ginkgo v2 refuses it);
every other unit package SHALL run with `go test -shuffle=<seed> -count=3`. Ginkgo packages are
found by searching the `_test.go` files, not by a hard-coded list. The unit packages are
`go list ./...` minus an explicit exclusion list in which every entry carries a reason; an entry is
allowed only for an e2e package. The list is empty on 2026-10-18 (`go list ./...` holds no e2e
package). The script SHALL provision envtest itself as `hack/coverage.sh:23-30` does: run
`make setup-envtest`, export `KUBEBUILDER_ASSETS` from `make -s echo-kubebuilder-assets`, and fail
when it is empty. A nightly workflow `test-shuffle.yaml` SHALL run it on `schedule:`, SHALL be a
workflow of its own, and SHALL be listed under `on.workflow_run.workflows` of
`ci-failure-report.yaml`, which reports scheduled runs only.

`-shuffle` reorders top-level `Test*` functions only and does not control ordering inside
`RunSpecs`. Ginkgo keeps its default randomization of top-level containers with its own seed; this
change neither enables full spec randomization nor ties that seed to the Go shuffle seed, so
reproducing a Ginkgo failure may need the Ginkgo seed the suite reports. A pass of this workflow is
not evidence that the Ginkgo suites are order-independent (see proposal *Non-goals*).

The script SHALL take the seed from `SHUFFLE_SEED` when it is set (a decimal integer, else it
fails) and choose one otherwise; it SHALL discover packages only through `go list ./...`, so a test
reaches a fixture by putting a fake `go` first on `PATH`. The workflow SHALL NOT set
`SHUFFLE_SEED`.

#### Scenario: Order-dependent test

- **WHEN** a unit test passes only after another test has run
- **THEN** the run SHALL fail on some seed and the output names the seed

#### Scenario: Seed reproducible

- **WHEN** a shuffled run fails with seed N at `-count=3`
- **THEN** `go test -shuffle=N -count=3` on the same package reproduces the failure (each test
  binary shuffles once with seed N and repeats that order; reproduce with the same seed and
  repetition settings; for a `RunSpecs` package, `-count=1`, and its Ginkgo seed, see below)

#### Scenario: Ginkgo package

- **WHEN** a unit package's tests call `RunSpecs`
- **THEN** the script SHALL run it three times with `-count=1` and never pass it another `-count`

#### Scenario: Every discovered package runs

- **WHEN** the script runs
- **THEN** the set of packages it tests SHALL equal `go list ./...` minus the exclusion list

#### Scenario: Envtest assets missing

- **WHEN** `make -s echo-kubebuilder-assets` prints nothing
- **THEN** the script SHALL fail before any test runs

#### Scenario: Exclusion without a reason

- **WHEN** a package is added to the exclusion list with no reason
- **THEN** the guard test SHALL fail

#### Scenario: Exclusion beyond e2e

- **WHEN** a non-e2e package (for example an envtest-backed one) is added to the exclusion list
- **THEN** the guard test SHALL fail

#### Scenario: Shuffle failure does not hide the nightly

- **WHEN** the scheduled shuffle run and the scheduled E2E nightly both fail
- **THEN** two separate issues exist (held by the reporter's per-workflow marker filter,
  `hack/ci/ci-failure-report.sh:133-140`, change 7's case "both open, nightly success" at `hack/test/ci_failure_report_test.sh:1226-1233`,
  and its job concurrency per `workflow_id`, `ci-failure-report.yaml:40-42`)

### Requirement: TDS-3 The README quick start names things that exist

`hack/test/readme_quickstart_test.sh` SHALL take the README section whose `##` heading starts with
`Quick start` (today `## Quick start (MVP)`) and extract the commands of its fenced `sh` blocks,
including commands in `#` comments (`# or: task dev-up` offers `task dev-up` to users, so it
counts); prose, other sections and column-0 backtick fences of another language are not
extracted. Inside the section every fence SHALL open with backticks at column 0: an indented or
`~~~` fence of any language SHALL fail, and so SHALL a fence of any language that is never closed
(it reaches the next `##` heading or the end of the file), because either would hide `sh` commands
from the reader. It SHALL fail when the extraction is empty, a `task <name>` is not a task of
`Taskfile.yml` or of a Taskfile it includes (an included task is `<namespace>:<name>`; a task or an
include marked `internal: true` does not count, as `task` refuses to run it), a
`kubectl apply -k <path>` path does not exist, a flag comes before the `kubectl` verb (its value
would be read as the verb), or a resource name in `kubectl get` is not a `shortNames`, `singular`
or `plural` entry of a CRD in `config/crd/bases` (fail closed: built-in resources are not in the
quick start). The resource names of `kubectl get` are each comma-separated name of its first
operand; once any operand is `TYPE/NAME`, they are the type part of every operand, and a plain
operand next to a `TYPE/NAME` one fails (kubectl rejects the mix). It SHALL NOT duplicate the
delegation checks of `demo_task_aliases_test.sh` (held by review).

#### Scenario: Task renamed

- **WHEN** `demo-up` is renamed and the README is not updated
- **THEN** the test SHALL fail and name the task

#### Scenario: Commented alternative renamed

- **WHEN** `dev-up` is renamed and the README comment `# or: task dev-up` is not updated
- **THEN** the test SHALL fail and name `dev-up`

#### Scenario: Unknown namespaced task

- **WHEN** the section says `task test:nosuch`
- **THEN** the test SHALL fail and name `test:nosuch`

#### Scenario: Unknown short name

- **WHEN** the README says `kubectl get kinv,foo`
- **THEN** the test SHALL fail and name `foo`

#### Scenario: Unknown type in a TYPE/NAME operand

- **WHEN** the README says `kubectl get kinv/a foo/b`
- **THEN** the test SHALL fail and name `foo`

#### Scenario: Internal task

- **WHEN** a task the section names is marked `internal: true`
- **THEN** the test SHALL fail and name the task

#### Scenario: Missing apply path

- **WHEN** the README says `kubectl apply -k config/samples/nosuch/`
- **THEN** the test SHALL fail and name the path

#### Scenario: Section missing

- **WHEN** the README has no `##` heading starting with "Quick start"
- **THEN** the test SHALL fail; an empty extraction never passes

#### Scenario: Section without commands

- **WHEN** the section exists but holds no `sh` fence
- **THEN** the test SHALL fail

#### Scenario: Fence the reader cannot see

- **WHEN** the section holds an indented or `~~~` fence, of any language, or a fence that is never
  closed
- **THEN** the test SHALL fail

#### Scenario: Commands outside the section

- **WHEN** invalid commands appear only in prose, in another section or in a closed, column-0
  backtick fence of another language
- **THEN** the test SHALL pass
