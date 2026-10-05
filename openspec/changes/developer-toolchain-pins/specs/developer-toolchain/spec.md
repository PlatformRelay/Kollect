# Spec Delta

## Purpose

Defines the pinned tool floors, where each version lives, how repeated sites are kept equal, and
that downloaded tool binaries are verified.

## ADDED Requirements

### Requirement: DTP-1 Task sites agree and meet the floor

Every `go-task/setup-task` `version:` input and `mise.toml` `task` SHALL be equal to each other and
at least `3.52.0`.

#### Scenario: One site missed

- **WHEN** all sites but one say the floor and one says `3.51.1`
- **THEN** the drift test SHALL fail and name the site

#### Scenario: Whole set moves to a newer version

- **WHEN** a bot PR moves every site to a newer release
- **THEN** the drift test passes

#### Scenario: Whole set below the floor

- **WHEN** every site says `3.51.1`
- **THEN** the drift test SHALL fail

### Requirement: DTP-2 govulncheck is pinned once

`Taskfile.yml` SHALL define `GOVULNCHECK_VERSION` (Renovate-annotated, at least `v1.6.0`) and
`task vulncheck` SHALL use it.

#### Scenario: Inline version returns

- **WHEN** `task vulncheck` contains a literal `govulncheck@v...`
- **THEN** the drift test SHALL fail

### Requirement: DTP-3 Other pins agree and meet their floors

golangci-lint (`Makefile`, `hack/tooling/.custom-gcl.yml`) SHALL be equal and at least `v2.13.1`;
gitleaks (`hack/install-gitleaks.sh`, the CI environment, `.pre-commit-config.yaml`) SHALL be equal
and at least `8.30.1`; git-cliff (`Taskfile.yml`) SHALL be at least `v2.13.1`.

#### Scenario: Pre-commit rev drifts

- **WHEN** `.pre-commit-config.yaml` names a different gitleaks release than the others
- **THEN** the drift test SHALL fail

#### Scenario: Empty scan

- **WHEN** the scan for any tool finds no site
- **THEN** the drift test SHALL fail; an empty scan never passes

### Requirement: DTP-4 The update bot can find every pin

Every `customManagers` entry in `renovate.json` SHALL match at least one tracked file, and every pin
site of DTP-1 to DTP-3 SHALL be covered by a manager of one group per tool.

#### Scenario: Dead manager

- **WHEN** a manager matches no file (as the gitleaks download-URL manager does today)
- **THEN** the test SHALL fail and name the manager

#### Scenario: Uncovered pin site

- **WHEN** a tool's pin (for example `Makefile` `GOLANGCI_LINT_VERSION`) is matched by no manager
- **THEN** the test SHALL fail and name the file

### Requirement: DTP-5 mise reads Go from go.mod and states no stale versions

`mise.toml` SHALL NOT list `go` under `[tools]` and SHALL enable `idiomatic_version_file_enable_tools
= ["go"]`; its comments and those in `Taskfile.yml` SHALL NOT restate a tool version other than
the pin they sit on.

#### Scenario: Go listed in mise

- **WHEN** `go` appears under `[tools]`
- **THEN** the drift test SHALL fail

#### Scenario: Stale comment

- **WHEN** a comment says `helm (v3.21.4)` while `Taskfile.yml` pins another version
- **THEN** the test SHALL fail

### Requirement: DTP-6 Downloaded tool binaries are checksum-verified

Every `hack/install-*.sh` that downloads a binary SHALL verify a published checksum before
installing it, or be listed in an exceptions file with a reason.

#### Scenario: Installer without verification

- **WHEN** `install-git-cliff.sh` downloads a tarball and does not compare a checksum
- **THEN** the guard SHALL fail and name the script

#### Scenario: Tampered download

- **WHEN** the downloaded file's checksum does not match
- **THEN** the installer SHALL fail and leave nothing installed

#### Scenario: Checksum file missing upstream

- **WHEN** the checksum file cannot be fetched
- **THEN** the installer SHALL fail; it never installs unverified

### Requirement: DTP-7 The guards can fail

The drift tests SHALL carry throwaway-copy mutants for each of DTP-1 to DTP-6 and a no-op copy
that passes, and SHALL run in a required job, plain and `--self-test` as separate steps.

#### Scenario: Mutant survives

- **WHEN** editing one site in a copy does not make the test fail
- **THEN** the self-test SHALL fail
