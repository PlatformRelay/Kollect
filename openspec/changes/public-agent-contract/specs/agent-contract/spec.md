# Spec Delta

## Purpose

Defines the public instructions that AI coding assistants and new contributors get from a fresh
clone, and the properties that keep them safe to publish and true.

## ADDED Requirements

### Requirement: PAC-1 A committed AGENTS.md exists with the essentials

The repository SHALL contain a tracked `AGENTS.md` at the root with: build, test and lint commands
that exist as tasks; the CI-enforced rules; the commit and merge conventions (linking
`CONTRIBUTING.md`); the spec workflow (linking `docs/development/spec-workflow.md`); and the
checklists of PAC-3.

#### Scenario: Fresh clone

- **WHEN** a clone has no ignored files
- **THEN** `AGENTS.md` is present and `git ls-files AGENTS.md` lists it

#### Scenario: Command that does not exist

- **WHEN** `AGENTS.md` tells the reader to run `task frobnicate`
- **THEN** the test SHALL fail, because every `task <name>` in it must be defined in `Taskfile.yml` or its includes

### Requirement: PAC-2 Other assistants are pointed, not duplicated

`CLAUDE.md` and `.github/copilot-instructions.md` SHALL each be at most 3 lines and SHALL point to
`AGENTS.md`.

#### Scenario: Pointer grows into a copy

- **WHEN** `CLAUDE.md` exceeds 3 lines
- **THEN** the test SHALL fail

#### Scenario: Pointer target missing

- **WHEN** `CLAUDE.md` names a file that does not exist
- **THEN** the test SHALL fail

### Requirement: PAC-3 Checklists say where else to change

`AGENTS.md` SHALL include "when adding X, update these places" checklists for at least: a CRD
field, a metric and a sink, and every file or symbol named in them SHALL exist in the tree.

#### Scenario: Metric checklist

- **WHEN** a reader adds a metric
- **THEN** the checklist names `internal/metrics/metrics.go`, the catalog and mirror list, and `docs/operator-manual/metrics.md`

#### Scenario: Stale reference

- **WHEN** a checklist names `internal/sink/registry.go` and that file is moved
- **THEN** the test SHALL fail and name the missing path

#### Scenario: Checklist matches a dry-run

- **WHEN** the author applies each checklist to a throwaway change
- **THEN** no gate fires for a place the checklist did not name (recorded in task 2.1)

### Requirement: PAC-4 The public contract is scrub-safe and machine-independent

Committed agent files SHALL NOT match any pattern in `hack/scrub-patterns.txt`, SHALL NOT contain
an absolute home path, `~/`-relative path, or a reference to a gitignored harness path
(`agent-context/`, `.claude/`, `.agents/`, `.cursor/`, `AGENTS.local.md`, `worktrees/`).

#### Scenario: Scrub-pattern match

- **WHEN** `AGENTS.md` contains any string from `hack/scrub-patterns.txt`
- **THEN** the test SHALL fail, whether or not the file is staged

#### Scenario: Maintainer-only path

- **WHEN** `AGENTS.md` says "see /home/<user>/..." or "see agent-context/GUIDELINES.md"
- **THEN** the test SHALL fail

#### Scenario: Ignore rules

- **WHEN** `.gitignore` ignores `AGENTS.md` or `CLAUDE.md` again
- **THEN** the test SHALL fail; `AGENTS.local.md` and the harness directories SHALL stay ignored

#### Scenario: Private local file would be un-ignored

- **WHEN** a file headed "LOCAL ONLY" or "never commit" is tracked or staged under `AGENTS.md`, `CLAUDE.md` or `.github/copilot-instructions.md`
- **THEN** the test SHALL fail

#### Scenario: OpenSpec markers

- **WHEN** a committed agent file contains an OpenSpec managed-block marker (`<!-- OPENSPEC:START -->` or similar)
- **THEN** the test SHALL fail, because OpenSpec's legacy cleanup rewrites root `AGENTS.md` and `CLAUDE.md`; the contract is hand-written

#### Scenario: Agent files are secret-scanned

- **WHEN** `.github/gitleaks.toml` allowlists `AGENTS.md` or `CLAUDE.md`
- **THEN** the test SHALL fail, because a committed agent file must not be exempt from secret scanning

### Requirement: PAC-6 Hostnames and internal hosts are refused generically

Committed agent files SHALL NOT contain a workstation hostname, a `*.internal`, `*.local` or
private-address host, or an SSH/host alias line, as matched by a generic pattern set in the test
in addition to `hack/scrub-patterns.txt`.

#### Scenario: Hostname string

- **WHEN** `AGENTS.md` names a machine hostname or an internal host
- **THEN** the test SHALL fail

#### Scenario: Pattern set can fail

- **WHEN** the generic pattern set is emptied in a copy
- **THEN** the self-test SHALL fail

### Requirement: PAC-5 The test can fail

`agent_contract_test.sh` SHALL carry throwaway-copy mutants for each of PAC-1 to PAC-4 and PAC-6 and a no-op
copy that passes.

#### Scenario: Mutant survives

- **WHEN** a planted scrub string does not turn the test red
- **THEN** the self-test SHALL fail
