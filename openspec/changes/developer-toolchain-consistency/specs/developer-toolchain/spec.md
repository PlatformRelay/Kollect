# Spec Delta

## Purpose

Defines the tool baseline kollect pins, where each version is stated, how the places that repeat
it are kept equal, and which task names every contributor can rely on.

## ADDED Requirements

### Requirement: DTC-1 Go comes from go.mod

Every Go setup in `.github/` SHALL use `go-version-file: go.mod` and none SHALL state a Go version
literal.

#### Scenario: Literal added

- **WHEN** a workflow adds `go-version: "1.26"`
- **THEN** the drift test SHALL fail and name the file

#### Scenario: mise

- **WHEN** `mise.toml` lists `go` under `[tools]`
- **THEN** the drift test SHALL fail (existing rule, kept)

### Requirement: DTC-2 Task is 3.52.0 everywhere

Every `go-task/setup-task` `version:` input and `mise.toml` `task` SHALL be `3.52.0`.

#### Scenario: One site missed

- **WHEN** 16 of 17 inputs say `3.52.0` and one says `3.51.1`
- **THEN** the drift test SHALL fail and name the site

#### Scenario: Whole set moves together to another version

- **WHEN** all sites say `3.53.0`
- **THEN** the baseline assertion SHALL fail until the baseline is changed deliberately

### Requirement: DTC-3 golangci-lint and govulncheck are pinned once and mirrored

golangci-lint SHALL be `v2.13.1` in `Makefile` `GOLANGCI_LINT_VERSION` and equal in
`hack/tooling/.custom-gcl.yml`. govulncheck SHALL be pinned through a `GOVULNCHECK_VERSION`
variable in `Taskfile.yml`, annotated for Renovate, at a release of at least `v1.6.0` (exact pin
chosen at implementation), and `task vulncheck` SHALL use that variable.

#### Scenario: custom build file lags

- **WHEN** `Makefile` says `v2.13.1` and `.custom-gcl.yml` says `v2.11.4`
- **THEN** the drift test SHALL fail

#### Scenario: Inline govulncheck version returns

- **WHEN** `task vulncheck` contains a literal `govulncheck@v...`
- **THEN** the drift test SHALL fail

#### Scenario: Bump does not loosen the linter

- **WHEN** the bump is merged
- **THEN** `.golangci.yaml` has no newly disabled linter and no new blanket exclusion; any new `//nolint` carries a reason

### Requirement: DTC-4 gitleaks and git-cliff stay at the baseline

gitleaks SHALL be `8.30.1` in `hack/install-gitleaks.sh`, the CI environment and
`.pre-commit-config.yaml` (`v8.30.1`); git-cliff SHALL be `v2.13.1` in `Taskfile.yml`.

#### Scenario: pre-commit rev drifts

- **WHEN** `.pre-commit-config.yaml` says `v8.31.0` and the others say `8.30.1`
- **THEN** the drift test SHALL fail

### Requirement: DTC-5 Standard entry points

`task check` SHALL run the full local gate matrix that CI runs as required checks, and `task verify`
SHALL remain the generated-artifact drift check. `task check` is an addition, not a rename.

#### Scenario: Alias present

- **WHEN** a contributor runs `task --list-all`
- **THEN** both `check` and `verify` are listed with distinct descriptions

#### Scenario: A CI gate has no local equivalent

- **WHEN** a job in the required set (`verify-eligibility.sh` `required_checks`) has no task reachable from `task check` and no listed exception
- **THEN** the test SHALL fail and name the job

#### Scenario: Rename attempted

- **WHEN** `verify` is removed or redefined to the broad matrix
- **THEN** the guard SHALL fail (that is open question 2, option B, and needs a spec change)

### Requirement: DTC-6 Pins are guarded and keep moving together

The drift test SHALL fail on an empty scan for any tool, and `renovate.json` regex managers SHALL
cover every pin site of one tool in one group.

#### Scenario: Empty scan

- **WHEN** the scan finds no `setup-task` pin
- **THEN** the test SHALL fail; an empty scan never passes

#### Scenario: Defect controls

- **WHEN** each site of each tool is edited in a throwaway copy
- **THEN** the test fails for that edit, and passes for a no-op copy
