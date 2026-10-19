# agent-contract Specification

## Purpose
Defines the public instructions that AI coding assistants and new contributors get from a fresh
clone, and the properties that keep them safe to publish and true.

## Requirements

### Requirement: PAC-1 A committed AGENTS.md exists with the essentials

The repository SHALL contain a tracked `AGENTS.md` at the root with: the commands `task build`,
`task test`, `task lint`, `task verify`, `task check`, `task scrub`, `task lint:markdown` and
`task spec:validate`; every `task <name>` it names in a code span or code fence (flags and
leading `VAR=value` assignments skipped) SHALL be defined (as `task --list-all` reports it); a
list of the gates the full local gate runs, between the lines `<!-- gates:start -->` and
`<!-- gates:end -->`, that SHALL name (one code span per gate) exactly the gate names
`hack/check.sh` passes to `run_gate` (the first argument of each non-comment `run_gate` call, not
the `run_gate()` definition), so a gate added or removed later reds the test until `AGENTS.md`
follows;
the CI-enforced rules that exist on `main` when the change lands; the commit and merge
conventions (linking `CONTRIBUTING.md`); the spec workflow (linking
`docs/development/spec-workflow.md`); and the checklists of PAC-3. Every relative link in it SHALL
resolve (`hack/test/repo_root_links_test.sh` already checks tracked root Markdown). Its content
SHALL be written from tracked files only (code, `Taskfile.yml`, `hack/`, `.github/workflows/`,
`CONTRIBUTING.md`, `docs/`), never from a private agent file or from untracked notes.

#### Scenario: Fresh clone

- **WHEN** a clone has no ignored files
- **THEN** `AGENTS.md` is present and `git ls-files AGENTS.md` lists it

#### Scenario: Command that does not exist

- **WHEN** `AGENTS.md` tells the reader to run `task frobnicate`
- **THEN** the test SHALL fail and name `frobnicate`

#### Scenario: Gate list drifts

- **WHEN** `hack/check.sh` gains a `run_gate` that `AGENTS.md` does not name, or `AGENTS.md` names a gate `hack/check.sh` does not run
- **THEN** the test SHALL fail and name the gate

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

`AGENTS.md` SHALL include "when adding X, update these places" checklists under the headings
`### When adding a CRD field`, `### When adding a metric` and `### When adding a sink`, and every repository path and every code symbol named in them SHALL
exist in the tree. A path is a code-span token containing `/`; a `<placeholder>` segment is read
as `*` and a glob SHALL match at least one tracked file. A symbol is a code-span Go identifier
named on the same checklist line as one or more `.go` paths, and SHALL be declared in at least one
of them (a `func` including a method, a `type`, or a `var`/`const`, also inside a grouped
`var (`/`const (` block).

#### Scenario: Metric checklist

- **WHEN** a reader adds a metric
- **THEN** the checklist names `internal/metrics/metrics.go`, `Catalog` in `internal/metrics/metrics_catalog.go`, `registeredMetricNames` in `internal/metrics/metrics_catalog_test.go`, and `docs/operator-manual/metrics.md`

#### Scenario: Stale reference

- **WHEN** a checklist names `internal/sink/registry.go` and that file is moved
- **THEN** the test SHALL fail and name the missing path

#### Scenario: Checklist matches a dry-run

- **WHEN** the author applies each checklist to a throwaway change
- **THEN** no gate fires for a place the checklist did not name (recorded in task 2.1)

### Requirement: PAC-4 The public contract is scrub-safe and machine-independent

Committed agent files (`AGENTS.md`, `.github/copilot-instructions.md`), in both their
working-tree content and their index (staged) content, SHALL NOT match any pattern in
`hack/scrub-patterns.txt`, applied with the semantics of `hack/scrub.sh` (`rg -i -f`:
case-insensitive, regular-expression, substring), and an `rg` error or a missing `rg` SHALL fail
the test rather than read as "no match". They SHALL NOT contain an absolute home path, a
`~/`-relative path, or a reference to a gitignored harness path (`agent-context/`, `.claude/`,
`.agents/`, `.cursor/`, `AGENTS.local.md`, `worktrees/`). The test SHALL NOT contain a scrub
pattern as a literal: it reads them from `hack/scrub-patterns.txt` at run time.

#### Scenario: Scrub-pattern match

- **WHEN** `AGENTS.md` contains a string that any line of `hack/scrub-patterns.txt` matches, in any letter case
- **THEN** the test SHALL fail, whether or not the file is staged

#### Scenario: Regular-expression semantics

- **WHEN** `AGENTS.md` contains a string that a pattern matches only as a regular expression (its `.` replaced by another character)
- **THEN** the test SHALL fail

#### Scenario: Staged content differs from disk

- **WHEN** a forbidden string or a private header is staged for `AGENTS.md` while the working-tree file is clean
- **THEN** the test SHALL fail

#### Scenario: Pattern list unreadable

- **WHEN** `hack/scrub-patterns.txt` is missing or empty
- **THEN** the test SHALL fail

#### Scenario: Maintainer-only path

- **WHEN** `AGENTS.md` says "see /home/<user>/..." or "see agent-context/GUIDELINES.md"
- **THEN** the test SHALL fail

#### Scenario: Ignore rules

- **WHEN** `.gitignore` ignores `AGENTS.md` again, or stops ignoring any of `CLAUDE.md`, `CLAUDE.local.md`, `AGENTS.local.md`, `agent-context/`, `.claude/`, `.agents/` or `.cursor/` (evaluated against the repository's `.gitignore` only, with `git -c core.excludesFile=/dev/null check-ignore --no-index` and an empty `.git/info/exclude` in the copy, directories probed through a child path; plain `git check-ignore` does not report tracked files, and a global excludes file would make the result machine-dependent)
- **THEN** the test SHALL fail

#### Scenario: Private local file would be un-ignored

- **WHEN** the first 10 lines of `AGENTS.md` or `.github/copilot-instructions.md`, in the working tree or the index, contain "local only" or "never commit" (case-insensitive)
- **THEN** the test SHALL fail

#### Scenario: OpenSpec markers

- **WHEN** a committed agent file contains `<!-- OPENSPEC:START -->` or `<!-- OPENSPEC:END -->` (case-insensitive)
- **THEN** the test SHALL fail, because OpenSpec's legacy cleanup rewrites root `AGENTS.md` and `CLAUDE.md`; the contract is hand-written

#### Scenario: Agent files are secret-scanned

- **WHEN** any allowlist path regex in `.github/gitleaks.toml`, in any table form gitleaks reads (`[allowlist]`, `[[allowlists]]`, `[[rules.allowlists]]`), matches `AGENTS.md`, `CLAUDE.md` or `.github/copilot-instructions.md` (each regex evaluated against each path with a PCRE-compatible matcher such as `grep -P`, which fails the test on a regex error, not a text search)
- **THEN** the test SHALL fail, because a committed agent file must not be exempt from secret scanning

### Requirement: PAC-5 The test can fail

`agent_contract_test.sh --self-test` SHALL run throwaway-copy mutants for every scenario of
PAC-1, PAC-2, PAC-4 and PAC-6 and for the PAC-3 stale-reference and checklist-entry cases (the
fresh-clone scenario is covered by the missing-file mutant; the dry-run scenario is evidence,
not a mutant). Each mutant SHALL red the plain check with the diagnostic of the clause it
targets (other diagnostics may appear too); the self-test SHALL fail when a mutant stays green or
reds without its target diagnostic. A no-op copy SHALL pass. Mutant strings for scrub patterns and
hostnames SHALL be built at run time. Mutants that change the guard itself (the PAC-6
empty-pattern set, the PAC-4 fixed-string matcher) expect the copy's own `--self-test` to fail
for the reason they name, and SHALL NOT recurse: a nested self-test runs with
`AGENT_CONTRACT_NESTED=1` and skips every mutant that would start another nested self-test.

#### Scenario: Mutant survives

- **WHEN** a planted scrub string does not turn the test red
- **THEN** the self-test SHALL fail and name the mutant

#### Scenario: Wrong clause

- **WHEN** a mutant reds the plain check only with an unrelated diagnostic (a missing tool, an unloadable Taskfile, another clause) and not with its target diagnostic
- **THEN** the self-test SHALL fail and name the mutant

#### Scenario: No-op copy

- **WHEN** the self-test runs an unmodified copy
- **THEN** the plain check on it SHALL pass

### Requirement: PAC-6 Hostnames and internal hosts are refused generically

Committed agent files SHALL NOT contain, matched case-insensitively by a generic pattern set in
the test in addition to `hack/scrub-patterns.txt`: a host whose last DNS label is `internal`,
`local`, `lan` or `home.arpa` (a label ends at a character outside `[a-z0-9-]`; `cluster.local`
service names, `localhost` and file names such as `mise.local.toml` are allowed); an IPv4 address
in `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `169.254.0.0/16` or `100.64.0.0/10` (valid
octets only), or an IPv6 unique-local address (`fc00::/7`); a line starting with an SSH `Host`
directive; or an scp-style `user@host:path` address (no `://` before the `@`, the host not
followed by a port number), other than `git@github.com:` (image digests such as
`kollect@sha256:` and URLs with credentials are not this form). They SHALL NOT contain the short hostname of the machine running the test, read
at run time (`AGENT_CONTRACT_HOSTNAME` overrides it for tests), matched case-insensitively as a
whole word; the check is skipped with a printed reason when the name cannot be read, is shorter
than 4 characters, is `localhost`, is a task name `task --list-all` reports, or occurs as a
whole word in a tracked Markdown file other than the agent files (an ordinary word, such as
`operator`). A contributor may set `AGENT_CONTRACT_HOSTNAME=` (empty) to skip it. This check is
supplementary: it sees only the machine it runs on, and the independent review covers other
machines' names. No hostname is written into a tracked file.

#### Scenario: Internal host string

- **WHEN** `AGENTS.md` names a host under `.internal` or a `10.x.y.z` address
- **THEN** the test SHALL fail

#### Scenario: Running host's name

- **WHEN** `AGENTS.md` contains the short hostname of the machine running the test
- **THEN** the test SHALL fail

#### Scenario: Allowed look-alikes

- **WHEN** `AGENTS.md` contains `svc.cluster.local`, `localhost`, `mise.local.toml`, `git@github.com:`, `kollect@sha256:`, `postgres://user:pass@db.example:5432/x` and a public example address
- **THEN** the test SHALL pass

#### Scenario: Pattern set can fail

- **WHEN** the generic pattern set is emptied in a copy
- **THEN** the self-test SHALL fail
