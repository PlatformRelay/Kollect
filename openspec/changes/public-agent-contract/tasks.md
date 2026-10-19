# Tasks

Red first. The test task comes before the implementation it drives and records its red in
evidence, failing on the target assertion (not on a missing file, a parse error or a usage error).
`task check` runs every `hack/test/*_test.sh` bare and, when its code declares it, with
`--self-test` (`hack/check.sh:88-111`), so a red guard cannot be committed alone: group G2 (1.1
and 2.1-2.4) closes in one commit, the test first, and the tree is green at every commit. The
guard has both modes from its first commit; that commit wires both into the required `lint` job
(`.github/workflows/ci.yaml:283`) as separate steps, as CWS-6
(`hack/test/ci_workflow_security_test.sh:591-598`) demands for a guard CI runs, and adds it to
the CFC-6 `cfc_guards` list (`hack/test/ci_workflow_security_test.sh:608-611`), so deleting both
lint steps cannot drop it silently.

The guard collects every failure before it exits (the `bad` style of
`hack/test/ci_failure_report_test.sh`), so a mutant that breaks two clauses still reds with the
message of the clause it names. Each self-test mutant runs on a throwaway `mktemp -d` copy that
is its own `git init` repository with the tracked files committed, so `git ls-files` and
`git check-ignore` answer for the copy, never for this checkout.

No scrub pattern and no hostname is written into a tracked file, the test included: a literal
scrub string turns `task scrub` red on the commit that stages it (`hack/scrub.sh:14`), and a
hostname in a tracked file is the leak this change prevents. Mutant strings are built at run time
from `hack/scrub-patterns.txt` and from `hostname -s`. Because `rg -i -f` matches
case-insensitive regular-expression substrings and `.` matches any character, the author of
`AGENTS.md` and of this change's Markdown checks the staged text with `bash hack/scrub.sh` before
each commit; ordinary phrases can match.

Evidence is Markdown only (`hack/test/openspec_evidence_markdown_only_test.sh`): one
`evidence/G<N>.md` per group, no logs or fixtures under `evidence/`.

Line numbers are measured on `origin/main` @ `3c01608ff` (2026-10-19). Change 9 edits
`ci.yaml`'s `lint` job and the `cfc_guards` list, so re-measure them after 0.3.

Each case below reads: given / when / then · the failure it exposes.

## 0. Prerequisites (protect the private file first)

- [x] 0.1 (orchestrator, local) In the main checkout, rename the private `AGENTS.md` to
  `AGENTS.local.md` (stays ignored) — done 2026-10-19 on this host: `AGENTS.md` absent,
  `AGENTS.local.md` sha256 `01e72bfd…`, `git status` clean (verified 2026-10-19)
- [x] 0.1b (orchestrator, local) Create a local, ignored `CLAUDE.md` containing only
  `@AGENTS.local.md` (no `@AGENTS.md` line, so the public file is not loaded twice) — done
  2026-10-19; a fresh Claude Code session in the main checkout loaded the private notes (verified
  by the orchestrator 2026-10-19). Not `CLAUDE.local.md` (decision 2026-10-19, private loading A)
- [ ] 0.1c (maintainer, local, not committed) Repoint the local harness references from
  `AGENTS.md` to `AGENTS.local.md`: 24 `agent-context/` files (the `agent-context/decisions/`
  briefs are excluded: they discuss the public file) and 6 skill files under `.claude/`,
  `.agents/` and `.cursor/`; re-count with `rg -l --no-ignore 'AGENTS\.md'` at execution
- [ ] 0.2 HARD PREREQUISITE, merge gate: the rename is verified in every checkout the agent
  controls (each clone and worktree on this host that has an ignored root `AGENTS.md` or
  `AGENTS.local.md`): no untracked or ignored `AGENTS.md` carrying the "LOCAL ONLY" header, and
  `AGENTS.local.md` matching the 0.1 checksum where it exists; record each path and result in
  `evidence/G0.md`. The INBOX notice "rename before pulling" for the operator's other machines is
  posted (2026-10-19). Until 0.2 is recorded, no agent fast-forwards a shared `main` checkout past
  this change
- [ ] 0.3 Change 9 (mutation-testing-signal, PR #471) is merged; record its merge sha in
  `evidence/G0.md` and rebase this branch on it before 2.2 lists the gates

## 1. Test first (PAC-1 to PAC-6)

- [ ] 1.1 Write `hack/test/agent_contract_test.sh` (plain checks the real tree; `--self-test`
  runs one mutant copy per case and a no-op copy). Record its red on this tree (no `AGENTS.md`
  yet): plain mode fails on the PAC-1 "AGENTS.md is not tracked" assertion. Cases:
  - PAC-1 missing: given a copy without `AGENTS.md` / when plain runs / then red "AGENTS.md is
    not tracked" · a deleted or never-added contract.
  - PAC-1 untracked: given `AGENTS.md` on disk but `.gitignore` ignoring it and not added / then
    red · a contract that exists only locally.
  - PAC-1 unknown task: given `task frobnicate` in `AGENTS.md` / then red naming `frobnicate`
    (names checked against `task --list-all` in the copy) · a stale command.
  - PAC-1 required command: given `AGENTS.md` without `task check` (one mutant per required
    command) / then red naming it · a contract that drops the full local gate.
  - PAC-1 links: given no link to `CONTRIBUTING.md` or to `docs/development/spec-workflow.md` /
    then red · conventions restated instead of linked.
  - PAC-2 long pointer: given a 4-line `.github/copilot-instructions.md` / then red · a pointer
    grown into a second copy.
  - PAC-2 pointer target: given a pointer that does not name `AGENTS.md`, and one naming a missing
    path / then red · a dangling pointer.
  - PAC-2 CLAUDE.md tracked: given `CLAUDE.md` committed in the copy / then red · a tracked file
    that would replace the maintainer's local loader on pull.
  - PAC-3 headings: given `AGENTS.md` without the sink checklist (one mutant per checklist) /
    then red · a missing checklist.
  - PAC-3 stale path: given `internal/sink/registry.go` moved in the copy / then red naming the
    path · a checklist pointing at nothing.
  - PAC-3 stale symbol: given `NewRegistry` renamed in the copy / then red naming the symbol · a
    checklist naming a function that no longer exists.
  - PAC-4 scrub: for each line of `hack/scrub-patterns.txt`, read at run time, plant the line's
    text in the copy's `AGENTS.md` (one mutant per line), and once more upper-cased / then red ·
    a forbidden string, and a case-sensitive check that misses it.
  - PAC-4 pattern list: given `hack/scrub-patterns.txt` empty, and missing / then red · a guard
    that passes because it compared against nothing.
  - PAC-4 rg error: given an unreadable agent file (`chmod 000`, skipped with a printed reason
    when running as root) / then red · `rg` exit 2 read as "no match", as `hack/scrub.sh:14` does.
  - PAC-4 home and harness paths: given `/home/someone/x`, `/Users/someone/x`, `~/x`, and each of
    `agent-context/`, `.claude/`, `.agents/`, `.cursor/`, `AGENTS.local.md`, `worktrees/` / then
    red · machine-local or maintainer-only references.
  - PAC-4 ignore rules: given `.gitignore` ignoring `AGENTS.md` again, and given it no longer
    ignoring `CLAUDE.md`, `AGENTS.local.md`, `agent-context/` or `.claude/` (checked with
    `git check-ignore`, not by text) / then red · an un-ignored private file one `git add -A` from
    a commit.
  - PAC-4 private header: given `AGENTS.md` whose first lines carry "LOCAL ONLY" or "never
    commit", tracked, and separately only staged / then red · the private file committed under
    the public name.
  - PAC-4 OpenSpec marker: given `<!-- OPENSPEC:START -->` in `AGENTS.md` / then red · OpenSpec's
    legacy cleanup rewriting the hand-written contract.
  - PAC-4 gitleaks: given `.github/gitleaks.toml` with an allowlist path matching `AGENTS.md`, and
    one matching `.github/copilot-instructions.md` / then red · an agent file exempt from secret
    scanning.
  - PAC-6 generic hosts: given a `*.internal`, `*.local`, `*.lan` and `*.home.arpa` host, a
    `10.`/`172.16.`/`192.168.`/`169.254.` address, a `Host` alias line and a `user@host:` address,
    each built at run time / then red · an internal host leaked.
  - PAC-6 running host: given the output of `hostname -s` (read at run time) planted / then red;
    when it is shorter than 4 characters or `localhost`, the case is skipped with a printed reason
    · the maintainer's workstation name leaked (it is not in the public scrub list by decision).
  - PAC-6 empty pattern set: given the copy's guard with its generic pattern array emptied / then
    that copy's `--self-test` fails · a pattern set that can no longer fail.
  - PAC-5 no-op: given an unmodified copy / then plain is green · a guard red on a correct tree.
  - PAC-5 surviving mutant: given a mutant whose plain run stays green / then `--self-test` fails
    naming the mutant · a self-test that reports success without its mutants failing.

## 2. Contract (closes with 1.1 in one commit, group G2)

- [ ] 2.1 Derive the three checklists by dry-run (change one CRD field, one metric, stub one sink
  on a scratch branch in a scratch worktree outside this one) and record in `evidence/G2.md`, as
  Markdown only, which gates fired for each and which files each needed; reconcile with the
  proposal's lists (no `INTEGRATION_PACKAGES` edit for a sink: `Taskfile.yml:20` covers
  `./internal/sink/...`)
- [ ] 2.2 In this worktree (no private agent file in it or loaded: `ls AGENTS.local.md CLAUDE.md`
  fails here, and the session was not started in the main checkout), write `AGENTS.md` from the
  public sources PAC-1 lists, in these sections: commands (the PAC-1 set plus `task lint:shell`,
  `task helm-test`, `task test-integration`, as defined); CI-enforced rules (the required
  contexts and what each runs, read from `.github/workflows/ci.yaml` and `hack/check.sh` on
  `main` after 0.3, so it names only gates that exist there); conventions (link
  `CONTRIBUTING.md` for commit format and rebase merge, link `docs/development/spec-workflow.md`);
  the three checklists from 2.1; and gotchas only where a tracked file states them (cite the
  file). Do not restate CONTRIBUTING.md. Every relative link resolves
  (`bash hack/test/repo_root_links_test.sh` green). Change 6 adds its own entry when it lands
- [ ] 2.3 Add `.github/copilot-instructions.md` (one line pointing to `AGENTS.md`); in
  `.gitignore` (the edit formerly numbered 0.1d) delete only the `AGENTS.md` line, keep
  `CLAUDE.md` and `AGENTS.local.md`, and add `CLAUDE.local.md`; remove `.github/gitleaks.toml:10`
  and run `gitleaks detect --source . --config .github/gitleaks.toml --redact --no-git` as CI
  does (`ci.yaml:170`); add the pointer line to `CONTRIBUTING.md`; correct
  `docs/development/setup.md:382`. Confirm `git check-ignore -v AGENTS.local.md CLAUDE.md`
  still matches in the main checkout. In a fresh Claude Code session on a clean clone of the
  branch, confirm the root `AGENTS.md` loads without a `CLAUDE.md` (record in `evidence/G2.md`)
- [ ] 2.4 Wire the guard into the required `lint` job as two steps (plain, then `--self-test`;
  CWS-6/CFC-6) and add it to `cfc_guards` (`hack/test/ci_workflow_security_test.sh:608-611`; a
  test change on change 4's guard, listed under *Test changes*); run
  `bash hack/test/ci_workflow_security_test.sh` and `--self-test`, the new guard in both modes,
  `task lint:shell`, `task lint:markdown`, and `bash hack/scrub.sh` plus
  `gitleaks protect --staged --no-banner` with the files staged

## 3. Review

- [ ] 3.1 HARD GATE: an independent reviewer (not the author) reads the committed text as a
  stranger: every sentence true on a fresh clone; no private gotcha, hostname or internal host
  leaked; the review record names the revision

## 4. Land

- [ ] 4.1 (integrator) Merge only after 0.2 and 0.3 are recorded, 3.1 approves and CI is green;
  after merge, in the main checkout: pull, then confirm `AGENTS.local.md` still matches the 0.1
  checksum and the local `CLAUDE.md` still contains `@AGENTS.local.md`
- [ ] 4.2 Archive the change (`openspec archive`) as the last commit of the PR, after review and
  green CI; the review record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| PAC-1 | guard plain on the tree; missing, untracked, unknown-task, required-command and link mutants; `git ls-files AGENTS.md` in a fresh clone; `repo_root_links_test.sh` | green; each mutant red | not-run | |
| PAC-2 | pointer length, target and tracked-`CLAUDE.md` mutants | red | not-run | |
| PAC-3 | heading, stale-path and stale-symbol mutants; dry-run record from 2.1 | red; no unnamed gate fired | not-run | |
| PAC-4 | per-pattern and upper-case scrub, empty/missing list, rg-error, home/harness-path, ignore-rule, private-header, OpenSpec-marker and gitleaks mutants; `hack/scrub.sh` with files staged; `gitleaks detect --no-git` | each mutant red; scrub ok; no leaks | not-run | |
| PAC-5 | `--self-test` exit status; no-op copy; surviving-mutant case | mutants red, no-op green | not-run | |
| PAC-6 | generic-host, running-host and empty-pattern-set mutants | red | not-run | |
| all | 0.2 checkout record; CI on the PR head; independent review | recorded; green; APPROVE; both revisions recorded | not-run | |
