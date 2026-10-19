# Spec Delta

## Purpose

Defines the public instructions that AI coding assistants and new contributors get from a fresh
clone, and the properties that keep them safe to publish and true.

## ADDED Requirements

### Requirement: PAC-1 A committed AGENTS.md exists with the essentials

The repository SHALL contain a tracked `AGENTS.md` at the root with: the commands `task build`,
`task test`, `task lint`, `task verify`, `task check`, `task scrub`, `task lint:markdown` and
`task spec:validate`; every `task <name>` it names SHALL be defined (as `task --list-all` reports
it); the CI-enforced rules that exist on `main` when the change lands; the commit and merge
conventions (linking `CONTRIBUTING.md`); the spec workflow (linking
`docs/development/spec-workflow.md`); and the checklists of PAC-3. Every relative link in it SHALL
resolve (`hack/test/repo_root_links_test.sh` already checks tracked root Markdown). Its content
SHALL be written from tracked public sources only (`Taskfile.yml`, `hack/check.sh`,
`.github/workflows/`, `CONTRIBUTING.md`, `docs/`), never from a private agent file.

#### Scenario: Fresh clone

- **WHEN** a clone has no ignored files
- **THEN** `AGENTS.md` is present and `git ls-files AGENTS.md` lists it

#### Scenario: Command that does not exist

- **WHEN** `AGENTS.md` tells the reader to run `task frobnicate`
- **THEN** the test SHALL fail and name `frobnicate`

#### Scenario: Required command missing

- **WHEN** `AGENTS.md` no longer names `task check`
- **THEN** the test SHALL fail and name the missing command

### Requirement: PAC-2 Other assistants are pointed, not duplicated

`.github/copilot-instructions.md` SHALL be at most 3 lines and SHALL point to `AGENTS.md`. The
root `CLAUDE.md` SHALL NOT be tracked: it is the maintainer's ignored local loader, and git
replaces an ignored file with a tracked one on pull without warning.

#### Scenario: Pointer grows into a copy

- **WHEN** `.github/copilot-instructions.md` exceeds 3 lines
- **THEN** the test SHALL fail

#### Scenario: Pointer target missing

- **WHEN** `.github/copilot-instructions.md` does not name `AGENTS.md`, or names a path that does not exist
- **THEN** the test SHALL fail

#### Scenario: CLAUDE.md committed

- **WHEN** `git ls-files CLAUDE.md` lists the file
- **THEN** the test SHALL fail

### Requirement: PAC-3 Checklists say where else to change

`AGENTS.md` SHALL include "when adding X, update these places" checklists for at least: a CRD
field, a metric and a sink, and every repository path and every code symbol named in them SHALL
exist in the tree.

#### Scenario: Metric checklist

- **WHEN** a reader adds a metric
- **THEN** the checklist names `internal/metrics/metrics.go`, the `Catalog` and `registeredMetricNames` lists, and `docs/operator-manual/metrics.md`

#### Scenario: Stale reference

- **WHEN** a checklist names `internal/sink/registry.go` and that file is moved
- **THEN** the test SHALL fail and name the missing path

#### Scenario: Checklist matches a dry-run

- **WHEN** the author applies each checklist to a throwaway change
- **THEN** no gate fires for a place the checklist did not name (recorded in task 2.1)

### Requirement: PAC-4 The public contract is scrub-safe and machine-independent

Committed agent files (`AGENTS.md`, `.github/copilot-instructions.md`) SHALL NOT match any
pattern in `hack/scrub-patterns.txt`, applied with the semantics of `hack/scrub.sh`
(`rg -i -f`: case-insensitive, regular-expression, substring), and an `rg` error SHALL fail the
test rather than read as "no match". They SHALL NOT contain an absolute home path, a
`~/`-relative path, or a reference to a gitignored harness path (`agent-context/`, `.claude/`,
`.agents/`, `.cursor/`, `AGENTS.local.md`, `worktrees/`). The test SHALL NOT contain a scrub
pattern as a literal: it reads them from `hack/scrub-patterns.txt` at run time.

#### Scenario: Scrub-pattern match

- **WHEN** `AGENTS.md` contains a string that any line of `hack/scrub-patterns.txt` matches, in any letter case
- **THEN** the test SHALL fail, whether or not the file is staged

#### Scenario: Pattern list unreadable

- **WHEN** `hack/scrub-patterns.txt` is missing or empty
- **THEN** the test SHALL fail

#### Scenario: Maintainer-only path

- **WHEN** `AGENTS.md` says "see /home/<user>/..." or "see agent-context/GUIDELINES.md"
- **THEN** the test SHALL fail

#### Scenario: Ignore rules

- **WHEN** `.gitignore` ignores `AGENTS.md` again, or stops ignoring `CLAUDE.md`, `AGENTS.local.md` or a harness directory (as `git check-ignore` reports it)
- **THEN** the test SHALL fail

#### Scenario: Private local file would be un-ignored

- **WHEN** a file headed "LOCAL ONLY" or "never commit" is tracked or staged under `AGENTS.md` or `.github/copilot-instructions.md`
- **THEN** the test SHALL fail

#### Scenario: OpenSpec markers

- **WHEN** a committed agent file contains an OpenSpec managed-block marker (`<!-- OPENSPEC:START -->` or similar)
- **THEN** the test SHALL fail, because OpenSpec's legacy cleanup rewrites root `AGENTS.md` and `CLAUDE.md`; the contract is hand-written

#### Scenario: Agent files are secret-scanned

- **WHEN** `.github/gitleaks.toml` allowlists `AGENTS.md`, `CLAUDE.md` or `.github/copilot-instructions.md`
- **THEN** the test SHALL fail, because a committed agent file must not be exempt from secret scanning

### Requirement: PAC-5 The test can fail

`agent_contract_test.sh --self-test` SHALL run throwaway-copy mutants for each scenario of PAC-1
to PAC-4 and PAC-6, SHALL fail when any mutant leaves the plain check green, and SHALL run a
no-op copy that passes. Mutant strings for scrub patterns and hostnames SHALL be built at run time.

#### Scenario: Mutant survives

- **WHEN** a planted scrub string does not turn the test red
- **THEN** the self-test SHALL fail and name the mutant

#### Scenario: No-op copy

- **WHEN** the self-test runs an unmodified copy
- **THEN** the plain check on it SHALL pass

### Requirement: PAC-6 Hostnames and internal hosts are refused generically

Committed agent files SHALL NOT contain a `*.internal`, `*.local`, `*.lan` or `*.home.arpa`
host, a private (RFC 1918) or link-local IPv4 address, an SSH `Host` alias line, or a
`user@host:` address, as matched by a generic pattern set in the test in addition to
`hack/scrub-patterns.txt`; and SHALL NOT contain the short hostname of the machine running the
test (read at run time, ignored when shorter than 4 characters or `localhost`). No hostname is
written into a tracked file.

#### Scenario: Internal host string

- **WHEN** `AGENTS.md` names a host under `.internal` or a `10.x.y.z` address
- **THEN** the test SHALL fail

#### Scenario: Running host's name

- **WHEN** `AGENTS.md` contains the short hostname of the machine running the test
- **THEN** the test SHALL fail

#### Scenario: Pattern set can fail

- **WHEN** the generic pattern set is emptied in a copy
- **THEN** the self-test SHALL fail
