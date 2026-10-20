# operatorhub-submission Specification

## Purpose
Defines how the OperatorHub submission script keeps its community-operators forks current with
upstream before it submits, and what it refuses to write.

## Requirements

### Requirement: OFS-1 A fork behind upstream is synced before the submission

Before it clones, branches or pushes, `hack/operatorhub-pr.sh` SHALL compare each fork's default
branch with upstream `main`. A fork that is behind SHALL be fast-forwarded with
`POST repos/<fork>/merge-upstream` and compared again; the submission SHALL continue only when the
fork is then identical to upstream. A fork that is already identical SHALL NOT be synced.

#### Scenario: Both forks behind

- **GIVEN** both forks are behind upstream `main`
- **WHEN** the script runs with `GH_TOKEN`
- **THEN** it calls `merge-upstream` for each fork before the first clone
- **AND** it pushes only the `kollect-v<version>` branch to each fork

#### Scenario: A sync that does not converge

- **GIVEN** a fork is still behind after `merge-upstream` returns
- **WHEN** the script compares it again
- **THEN** it exits non-zero naming the fork
- **AND** it SHALL NOT push to any fork

### Requirement: OFS-2 A fork with commits of its own fails closed

A fork whose default branch is ahead of or diverged from upstream `main`, a repository that is not
a fork of the expected upstream, or a compare that cannot be read SHALL stop the script with a
message naming the fork, its state and the manual remedy.

#### Scenario: Diverged fork

- **GIVEN** a fork is diverged from upstream
- **WHEN** the script runs
- **THEN** it exits non-zero with "is diverged relative to"
- **AND** it SHALL NOT call `merge-upstream`, force-push or push any branch

### Requirement: OFS-3 Every fork is checked before the first write

The script SHALL check all forks before it writes to any of them, so a fork that fails OFS-2 leaves
every fork untouched.

#### Scenario: First fork behind, second fork diverged

- **GIVEN** the first fork is behind and the second is diverged
- **WHEN** the script runs
- **THEN** it exits non-zero
- **AND** it SHALL NOT sync the first fork

### Requirement: OFS-4 DRY_RUN makes no remote writes

With `DRY_RUN=1` the script SHALL make no remote write. With `GH_TOKEN` set it SHALL run the OFS-1
to OFS-3 checks with read-only calls and print, per fork, the sync it would make (or that none is
needed), the branch it would create and push, and the PR it would open or update; a fork failing
OFS-2 SHALL fail the dry run. Without `GH_TOKEN` it SHALL make no API call and say that the fork
state was not checked.

#### Scenario: Dry run against a behind fork

- **GIVEN** one fork is behind and one identical
- **WHEN** the script runs with `DRY_RUN=1` and `GH_TOKEN`
- **THEN** it prints "would sync" with the behind count for the first and "would not sync" for the second
- **AND** it SHALL NOT call `merge-upstream`, `git` or any non-GET API

#### Scenario: Offline dry run

- **GIVEN** `GH_TOKEN` is unset
- **WHEN** the script runs with `DRY_RUN=1`
- **THEN** it exits 0 and says the fork state was not checked
- **AND** it SHALL NOT call `gh`, `git` or `curl`

### Requirement: OFS-5 The behaviour is guarded in a required job

`hack/test/operatorhub_fork_sync_test.sh` SHALL cover OFS-1 to OFS-4 against fake `gh`, `git`,
`curl` and `make`, and its `--self-test` SHALL show one mutant per clause turning its own case red,
plus a no-op control that stays green. Both modes SHALL run in the required `lint` job, enforced
by CFC-6 in `hack/test/ci_workflow_security_test.sh`.

#### Scenario: Self-test mode is unwired

- **GIVEN** the `--self-test` step is removed from `ci.yaml`
- **WHEN** `hack/test/ci_workflow_security_test.sh` runs
- **THEN** it fails naming `operatorhub_fork_sync_test.sh`
