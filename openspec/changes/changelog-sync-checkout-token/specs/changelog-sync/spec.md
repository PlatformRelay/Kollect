# Spec Delta

## Purpose

Defines how the changelog self-healing workflow holds its privileged App token, so the token that
can bypass `protect-main` reaches only the one step that pushes.

## ADDED Requirements

### Requirement: CST-1 The privileged token reaches only the push step

In `.github/workflows/changelog-sync.yaml`, job `sync`, every `actions/checkout` step SHALL set
`persist-credentials: false` and SHALL NOT set a `token:` input. The output of the
`actions/create-github-app-token` step SHALL be referenced by no step other than the step that
pushes to `main`. (Moved from DUA-6 of `dependency-update-automation`.)

#### Scenario: Honest workflow

- **WHEN** the checkout has no `token:`, sets `persist-credentials: false`, and only the push step reads the App token
- **THEN** the guard passes

#### Scenario: Token handed to the checkout

- **WHEN** the checkout step sets `token:` to any value
- **THEN** the guard SHALL fail naming the checkout's `token` input

#### Scenario: Credentials persist

- **WHEN** the checkout sets `persist-credentials: true` or omits `persist-credentials`
- **THEN** the guard SHALL fail naming `persist-credentials`

#### Scenario: Token reaches another step

- **WHEN** a step other than the push step references the App token output
- **THEN** the guard SHALL fail naming that step

#### Scenario: Push still authenticates

- **WHEN** a push to `main` makes the regenerated `CHANGELOG.md` differ from the committed one
- **THEN** the sync job pushes with the App token through its own extraheader, and it SHALL NOT fail for lack of checkout credentials

### Requirement: CST-2 The credential checks are proven to fail

The guard's self-test SHALL run each CST-1 check against a mutated copy of the real workflow and
SHALL fail when a mutant passes or fails for a reason other than the one the mutant targets. An
honest variant SHALL pass.

#### Scenario: Mutant passes

- **WHEN** a mutant that re-adds the checkout token, flips or removes `persist-credentials`, or reads the token in another step passes the check
- **THEN** the guard SHALL fail and name the vacuous assertion

#### Scenario: No-op control

- **WHEN** the workflow is round-tripped unchanged, or `persist-credentials` is spelled as the string `"false"`
- **THEN** the check passes
