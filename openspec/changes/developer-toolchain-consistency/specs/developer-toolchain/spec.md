# Spec Delta

## Purpose

Defines the tool baseline kollect pins, where each version is stated, how the places that repeat
it are kept equal, and which task names every contributor can rely on.

## ADDED Requirements

### Requirement: DTC-1 Go comes from go.mod and the image matches it

Every Go setup in `.github/` SHALL use `go-version-file: go.mod` and none SHALL state a Go version
literal. `go.mod` SHALL say `go 1.27.1`, equal to the `golang:` tag of every Dockerfile that builds
Go. Renovate SHALL move the `go` directive and the golang image tag in one group.

#### Scenario: Literal added

- **WHEN** a workflow adds `go-version: "1.26"`
- **THEN** the drift test SHALL fail and name the file

#### Scenario: mise

- **WHEN** `mise.toml` lists `go` under `[tools]`
- **THEN** the drift test SHALL fail (existing rule, kept)

#### Scenario: Image and go.mod disagree

- **WHEN** `go.mod` says `go 1.26.6` and a Dockerfile says `golang:1.27.1`
- **THEN** the tree fails the check (this change fixes the tree; `cross-file-consistency-gates` adds the permanent check)

#### Scenario: Group rule

- **WHEN** `renovate.json` lacks a rule grouping the `go` directive with the `golang` Docker image
- **THEN** the drift test SHALL fail

### Requirement: DTC-2 Task is at least 3.52.0 at every site, and all sites agree

Every `go-task/setup-task` `version:` input (18 sites at the time of writing) and `mise.toml`
`task` SHALL be equal to each other and at least `3.52.0`.

#### Scenario: One site missed

- **WHEN** 17 of 18 inputs say `3.52.0` and one says `3.51.1`
- **THEN** the drift test SHALL fail and name the site

#### Scenario: Whole set moves together to a newer version

- **WHEN** a bot PR moves all sites to `3.53.0`
- **THEN** the drift test passes

#### Scenario: Whole set below the floor

- **WHEN** all sites say `3.51.1`
- **THEN** the drift test SHALL fail

### Requirement: DTC-3 golangci-lint and govulncheck are pinned once and mirrored

golangci-lint SHALL be at least `v2.13.1` in `Makefile` `GOLANGCI_LINT_VERSION` and equal in
`hack/tooling/.custom-gcl.yml`. govulncheck SHALL be pinned through a `GOVULNCHECK_VERSION`
variable in `Taskfile.yml`, annotated for Renovate, at least `v1.6.0`, and `task vulncheck` SHALL
use that variable.

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

gitleaks SHALL be at least `8.30.1` in `hack/install-gitleaks.sh`, the CI environment and
`.pre-commit-config.yaml` (`v8.30.1`), all equal; git-cliff SHALL be at least `v2.13.1` in
`Taskfile.yml`.

#### Scenario: pre-commit rev drifts

- **WHEN** `.pre-commit-config.yaml` says `v8.31.0` and the others say `8.30.1`
- **THEN** the drift test SHALL fail

### Requirement: DTC-5 Standard entry points

`task check` SHALL run every required gate that does not need Docker or kind (verify, lint,
unit tests, scrub, shell and markdown lint, the `hack/test` meta-tests), and its description SHALL
list what it runs and name the Docker gates it leaves out (`test-integration`, `kind-smoke`,
`docker-build`). `task verify` SHALL remain the generated-artifact drift check. `task check` is an
addition, not a rename.

#### Scenario: Alias present

- **WHEN** a contributor runs `task --list-all`
- **THEN** both `check` and `verify` are listed with distinct descriptions

#### Scenario: A Docker-free CI gate has no local equivalent

- **WHEN** a required job from `verify-eligibility.sh` `required_checks` that needs no Docker or kind is not reachable from `task check` and has no listed exception with a reason
- **THEN** the test SHALL fail and name the job

#### Scenario: Rename attempted

- **WHEN** `verify` is removed or redefined to the broad matrix
- **THEN** the guard SHALL fail (that is open question 2, option B, and needs a spec change)

### Requirement: DTC-6 Pins are guarded and the update bot can find them

The drift test SHALL fail on an empty scan for any tool. Every `customManagers` entry in
`renovate.json` SHALL match at least one tracked file, and every pin site of one tool SHALL be
covered by a manager of one group.

#### Scenario: Empty scan

- **WHEN** the scan finds no `setup-task` pin
- **THEN** the test SHALL fail; an empty scan never passes

#### Scenario: Dead manager

- **WHEN** a `customManagers` entry matches no file (as the gitleaks download-URL manager does today)
- **THEN** the test SHALL fail and name the manager

#### Scenario: Uncovered pin site

- **WHEN** a tool's pin (for example `Makefile` `GOLANGCI_LINT_VERSION`) is matched by no manager
- **THEN** the test SHALL fail and name the file

#### Scenario: Defect controls

- **WHEN** each site of each tool is edited in a throwaway copy
- **THEN** the test fails for that edit, and passes for a no-op copy

### Requirement: DTC-7 Comments do not state versions that can go stale

`mise.toml` and `Taskfile.yml` comments SHALL NOT restate a tool version other than the pin they
sit on.

#### Scenario: Stale helm comment

- **WHEN** a comment says `helm (v3.21.4)` while `Taskfile.yml` pins `v3.22.0`
- **THEN** the test SHALL fail (the current `mise.toml` comment is fixed by this change)
