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

`Taskfile.yml` SHALL define `GOVULNCHECK_VERSION` with a Renovate annotation on the Go module
datasource (`datasource=go depName=golang.org/x/vuln`) and `task vulncheck` SHALL use it. No floor is set here: CFC-2 (change 4) checks that the pinned govulncheck builds and
runs under `go.mod`'s Go.

#### Scenario: Inline version returns

- **WHEN** `task vulncheck` contains a literal `govulncheck@v...`
- **THEN** the guard SHALL fail

#### Scenario: Annotation dropped

- **WHEN** `GOVULNCHECK_VERSION` has no `# renovate:` line above it
- **THEN** the guard SHALL fail

#### Scenario: vulncheck reads another variable

- **WHEN** `task vulncheck` references a different variable holding the same version while the
  annotated `GOVULNCHECK_VERSION` stays defined
- **THEN** the guard SHALL fail and name the missing `GOVULNCHECK_VERSION` reference

#### Scenario: Variable definition removed

- **WHEN** `GOVULNCHECK_VERSION` is no longer defined
- **THEN** the guard SHALL fail with a missing-definition message

### Requirement: DTP-3 gitleaks, git-cliff and shellcheck sites agree

gitleaks (`ci.yaml:164` env, `hack/install-gitleaks.sh:14`, `.pre-commit-config.yaml:5` `rev`)
SHALL be equal and at least `8.30.1`. git-cliff (`Taskfile.yml:22` `GIT_CLIFF_VERSION`,
`.github/workflows/release.yaml:420` git-cliff-action `version:`, `hack/verify-changelog.sh:11`
fallback install) SHALL be equal and at least `v2.13.1`. shellcheck (`ci.yaml:299` env,
`hack/install-shellcheck.sh:15` default, `.pre-commit-config.yaml:56` shellcheck-py `rev`, compared
on its first three components) SHALL be equal, and `hack/install-shellcheck.sh`'s digest table SHALL
have an entry for the pinned version on every platform it supports. Versions compare with any
leading `v` stripped. golangci-lint is not checked here: CFC-2 checks `.custom-gcl.yml` against
`Makefile` and smoke-runs the tool.

#### Scenario: Pre-commit rev drifts

- **WHEN** `.pre-commit-config.yaml` names a different gitleaks release than the others
- **THEN** the guard SHALL fail

#### Scenario: git-cliff release pin drifts

- **WHEN** `release.yaml` or `hack/verify-changelog.sh` names a different git-cliff release than
  `Taskfile.yml`
- **THEN** the guard SHALL fail and name the file

#### Scenario: shellcheck bumped without a digest

- **WHEN** every shellcheck site says a release that `hack/install-shellcheck.sh`'s digest table
  has no entry for
- **THEN** the guard SHALL fail and name the missing table entry

#### Scenario: Empty scan

- **WHEN** the scan for any tool finds no site
- **THEN** the guard SHALL fail; an empty scan never passes

### Requirement: DTP-4 The update bot can find every pin

Every `customManagers` entry in `renovate.json` SHALL match at least one tracked file, where
"matches" is the definition settled by the task 3.1 probe (a file pattern alone is not enough when
the match strings find nothing in that file), and every pin site of DTP-1 to DTP-3 (including the
three git-cliff and three shellcheck sites), plus golangci-lint in `Makefile` and
`hack/tooling/.custom-gcl.yml`, SHALL be covered by a manager. All sites of one tool SHALL land in
one group, and shellcheck and golangci-lint SHALL each have a group of their own, so a shellcheck
bump that is red until its digests are added, or a golangci-lint bump red on new findings, holds
back no other tool. Stronger than the first sentence, one file pattern that matches no tracked file
fails its manager even when the manager's other patterns are alive (a rotted pattern is reported,
not averaged away). The check's mechanism (Renovate run in `lint` at a
pinned version, or a bash copy of the managers' regexes and `packageRules` order) is the one task
3.1 records.

#### Scenario: Dead manager

- **WHEN** a manager matches no file, as the gitleaks (`renovate.json:37-47`) and shellcheck
  (`renovate.json:48-58`) download-URL managers do today
- **THEN** the guard SHALL fail and name the manager

#### Scenario: Uncovered pin site

- **WHEN** a tool's pin (for example `Makefile` `GOLANGCI_LINT_VERSION`) is matched by no manager
- **THEN** the guard SHALL fail and name the file

#### Scenario: shellcheck shares a group

- **WHEN** a shellcheck site resolves to the same group as another tool
- **THEN** the guard SHALL fail and name the group

#### Scenario: golangci-lint shares a group

- **WHEN** the golangci-lint group rule is removed, so its sites fall back to "pinned build tools"
- **THEN** the guard SHALL fail and name the group

### Requirement: DTP-5 mise mirrors CI, reads Go from go.mod and states no stale versions

`mise.toml` SHALL NOT list `go` under `[tools]` and SHALL enable `idiomatic_version_file_enable_tools
= ["go"]`, with `go.mod` carrying a `go` line; this half is already satisfied and checked by
DEV-MISE-01 and is carried over unchanged. `mise.toml` `node` and `python` SHALL equal the CI
`node-version` and `python-version` inputs, which SHALL agree with each other.

A version-bearing comment in `mise.toml` or `Taskfile.yml` is allowed only in the contiguous
comment block directly above its pin (no blank line between), or inline on the pin's line, and only
when it names that pin's tool (the pin key or its CI input name, such as `node-version` for `node`
or setup-task `version:` for `task`) at a value equal to the pin's, with any leading `v` stripped.
Any other tool-version restatement in a comment, and any count of pin sites (a number followed by
`occurrences`, `sites`, `places` or `workflows`), SHALL fail the guard. Stronger than that: any
dotted or quoted number in such a comment is read as a version, so a prose number away from a pin
fails closed too.

#### Scenario: Go listed in mise

- **WHEN** `go` appears under `[tools]`
- **THEN** the guard SHALL fail

#### Scenario: Node drifts from CI

- **WHEN** `mise.toml` says `node = "20"` while CI says `22`
- **THEN** the guard SHALL fail and name both values

#### Scenario: Stale version comment

- **WHEN** a comment says `helm (v3.21.4)` while `Taskfile.yml` pins another version
- **THEN** the guard SHALL fail

#### Scenario: Attached comment names another value

- **WHEN** the comment block directly above `task = "3.52.0"` says `version: 3.51.1`
- **THEN** the guard SHALL fail

#### Scenario: Attached comment names the pin

- **WHEN** the comment block directly above `node = "22"` says `node-version: "22"`, or the one
  above `python = "3.12"` says `python-version: "3.12"`
- **THEN** the guard passes

#### Scenario: Restated site count

- **WHEN** a comment says "all 17 occurrences in .github/workflows/"
- **THEN** the guard SHALL fail

### Requirement: DTP-6 Downloaded tool binaries are checksum-verified

Every `hack/install-*.sh` that downloads a binary SHALL verify a published checksum before installing
it. git-cliff SHALL be verified against its per-asset `.sha512` with `sha512sum`; helm-docs against
its `checksums.txt` with `sha256sum`. Where those are absent, as on stock macOS, `shasum -a 512` /
`shasum -a 256` SHALL stand in; with neither, the installer SHALL fail and install nothing. A
checksum line whose digest is not a full lower-case hex digest SHALL be refused before any
comparison. The only exceptions list is `UNVERIFIED_BY_DESIGN` in
`hack/test/ci_fetch_lib_hardening_test.sh`; it SHALL only shrink, this change removes both of its
entries, and once they are gone any entry SHALL fail the guard.

#### Scenario: Installer without verification

- **WHEN** `install-git-cliff.sh` downloads a tarball and does not compare a checksum
- **THEN** `hack/test/ci_fetch_lib_hardening_test.sh` SHALL fail and name the script

#### Scenario: Valid download

- **WHEN** the downloaded archive matches the published checksum
- **THEN** the installer SHALL succeed and the expected executable SHALL exist at the output path

#### Scenario: Tampered download

- **WHEN** the downloaded file's checksum does not match
- **THEN** the installer SHALL fail and leave nothing installed

#### Scenario: Checksum file missing upstream

- **WHEN** the checksum file cannot be fetched
- **THEN** the installer SHALL fail; it never installs unverified

#### Scenario: No hash tool

- **WHEN** neither the coreutils tool nor `shasum` is on `PATH`
- **THEN** the installer SHALL fail, say so, and leave nothing installed

#### Scenario: Exception added

- **WHEN** an installer name is added to `UNVERIFIED_BY_DESIGN`, including after the list is empty
- **THEN** the guard SHALL fail

### Requirement: DTP-7 The guards can fail

The checks of DTP-1 to DTP-6 SHALL live in `hack/test/consistency_toolchain_pins_test.sh`, with
one exception: the static installer-checksum scan of DTP-6 stays in
`hack/test/ci_fetch_lib_hardening_test.sh`, which owns it and which this change widens to accept
`sha512sum`. The new guard SHALL carry throwaway-copy mutants for each requirement, every mutant of
the folded `hack/test/dev_mise_pin_drift_test.sh`, the installer execution tests of DTP-6 (valid,
tampered and missing-checksum downloads) in its `--self-test`, and a no-op copy that passes. It SHALL
run in required jobs, plain and `--self-test` as separate steps, which CWS-6 and CFC-6 in
`hack/test/ci_workflow_security_test.sh` enforce; the `--self-test` runs in the
`tool-pins-selftest` job, out of `lint`, like the CFC-2 self-test. The self-test SHALL also bump
every pin Renovate extracts to the next release of its own value in a copy, as one group pull
request would, and pass both modes on it.

#### Scenario: Mutant survives

- **WHEN** editing one site in a copy does not make the guard fail
- **THEN** the self-test SHALL fail

#### Scenario: One step unwired

- **WHEN** one of the new guard's two steps is removed from the required job
- **THEN** `hack/test/ci_workflow_security_test.sh` SHALL fail, naming the guard and the mode that
  no required step runs (CWS-6 fires first)

#### Scenario: Correct bump

- **WHEN** every extracted pin moves to its next patch release at all its sites, with the
  shellcheck digests added
- **THEN** the guard and its `--self-test` SHALL pass on that copy

#### Scenario: Guard unwired

- **WHEN** both steps of the new guard are removed from the required job
- **THEN** CFC-6 SHALL fail and name the guard
