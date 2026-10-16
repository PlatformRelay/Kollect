# Spec Delta

## Purpose

Defines the pinned tool floors, where each version lives, how repeated sites are kept equal, and
that downloaded tool binaries are verified. Whether golangci-lint and govulncheck pins work with
the repository's Go is owned by change 4's CFC-2, not by this capability.

## ADDED Requirements

### Requirement: DTP-1 Task sites agree and meet the floor

Every `go-task/setup-task` `version:` input under `.github/workflows/` and `.github/actions/`, and
`mise.toml` `task`, SHALL be equal to each other and at least `3.52.0`.

#### Scenario: One site missed

- **WHEN** all sites but one say the floor and one says `3.51.1`
- **THEN** the guard SHALL fail and name the site

#### Scenario: Whole set moves to a newer version

- **WHEN** a bot PR moves every site to a newer release
- **THEN** the guard passes

#### Scenario: Whole set below the floor

- **WHEN** every site says `3.51.1`
- **THEN** the guard SHALL fail

### Requirement: DTP-2 govulncheck is pinned once

`Taskfile.yml` SHALL define `GOVULNCHECK_VERSION` with a Renovate annotation and `task vulncheck`
SHALL use it. No floor is set here: CFC-2 (change 4) checks that the pinned govulncheck builds and
runs under `go.mod`'s Go.

#### Scenario: Inline version returns

- **WHEN** `task vulncheck` contains a literal `govulncheck@v...`
- **THEN** the guard SHALL fail

#### Scenario: Annotation dropped

- **WHEN** `GOVULNCHECK_VERSION` has no `# renovate:` line above it
- **THEN** the guard SHALL fail

### Requirement: DTP-3 gitleaks and git-cliff meet their floors

gitleaks (`hack/install-gitleaks.sh`, the CI environment, `.pre-commit-config.yaml`) SHALL be equal
and at least `8.30.1`; git-cliff (`Taskfile.yml`) SHALL be at least `v2.13.1`. golangci-lint is not
checked here: CFC-2 checks `.custom-gcl.yml` against `Makefile` and smoke-runs the tool.

#### Scenario: Pre-commit rev drifts

- **WHEN** `.pre-commit-config.yaml` names a different gitleaks release than the others
- **THEN** the guard SHALL fail

#### Scenario: Empty scan

- **WHEN** the scan for any tool finds no site
- **THEN** the guard SHALL fail; an empty scan never passes

### Requirement: DTP-4 The update bot can find every pin

Every `customManagers` entry in `renovate.json` SHALL match at least one tracked file, where
"matches" is the definition settled by the task 3.1 probe (a file pattern alone is not enough when
the match strings find nothing in that file), and every pin site of DTP-1 to DTP-3, plus
golangci-lint in `Makefile` and `hack/tooling/.custom-gcl.yml` and shellcheck in the CI
environment, SHALL be covered by a manager of one group per tool.

#### Scenario: Dead manager

- **WHEN** a manager matches no file, as the gitleaks (`renovate.json:37-47`) and shellcheck
  (`renovate.json:48-58`) download-URL managers do today
- **THEN** the guard SHALL fail and name the manager

#### Scenario: Uncovered pin site

- **WHEN** a tool's pin (for example `Makefile` `GOLANGCI_LINT_VERSION`) is matched by no manager
- **THEN** the guard SHALL fail and name the file

### Requirement: DTP-5 mise mirrors CI, reads Go from go.mod and states no stale versions

`mise.toml` SHALL NOT list `go` under `[tools]` and SHALL enable `idiomatic_version_file_enable_tools
= ["go"]`, with `go.mod` carrying a `go` line; this half is already satisfied and checked by
DEV-MISE-01 and is carried over unchanged. `mise.toml` `node` and `python` SHALL equal the CI
`node-version` and `python-version` inputs, which SHALL agree with each other. Comments in
`mise.toml` and `Taskfile.yml` SHALL NOT restate a tool version other than the pin they sit on, nor
a count of pin sites.

#### Scenario: Go listed in mise

- **WHEN** `go` appears under `[tools]`
- **THEN** the guard SHALL fail

#### Scenario: Node drifts from CI

- **WHEN** `mise.toml` says `node = "20"` while CI says `22`
- **THEN** the guard SHALL fail and name both values

#### Scenario: Stale version comment

- **WHEN** a comment says `helm (v3.21.4)` while `Taskfile.yml` pins another version
- **THEN** the guard SHALL fail

#### Scenario: Restated site count

- **WHEN** a comment says "all 17 occurrences in .github/workflows/"
- **THEN** the guard SHALL fail

### Requirement: DTP-6 Downloaded tool binaries are checksum-verified

Every `hack/install-*.sh` that downloads a binary SHALL verify a published checksum before installing
it. git-cliff SHALL be verified against its per-asset `.sha512` with `sha512sum`; helm-docs against
its `checksums.txt`. The only exceptions list is `UNVERIFIED_BY_DESIGN` in
`hack/test/ci_fetch_lib_hardening_test.sh`; it SHALL only shrink, and this change removes both of
its entries.

#### Scenario: Installer without verification

- **WHEN** `install-git-cliff.sh` downloads a tarball and does not compare a checksum
- **THEN** the guard SHALL fail and name the script

#### Scenario: Tampered download

- **WHEN** the downloaded file's checksum does not match
- **THEN** the installer SHALL fail and leave nothing installed

#### Scenario: Checksum file missing upstream

- **WHEN** the checksum file cannot be fetched
- **THEN** the installer SHALL fail; it never installs unverified

#### Scenario: Exception added

- **WHEN** an installer name is added to `UNVERIFIED_BY_DESIGN`
- **THEN** the guard SHALL fail

### Requirement: DTP-7 The guards can fail

The checks of DTP-1 to DTP-6 SHALL live in `hack/test/consistency_toolchain_pins_test.sh`, which
SHALL carry throwaway-copy mutants for each requirement, every mutant of the folded
`hack/test/dev_mise_pin_drift_test.sh`, and a no-op copy that passes. It SHALL run in a required
job, plain and `--self-test` as separate steps, which CFC-6 enforces for every
`consistency_*_test.sh`.

#### Scenario: Mutant survives

- **WHEN** editing one site in a copy does not make the guard fail
- **THEN** the self-test SHALL fail

#### Scenario: Guard unwired

- **WHEN** either step of the new guard is removed from the required job
- **THEN** CFC-6 SHALL fail and name the guard
