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
message of the clause it names. Each self-test mutant runs on a throwaway `mktemp -d` copy of
every tracked file (`git ls-files`, so `Taskfile.yml` and its include `hack/task/Taskfile.test.yml`
are there for `task --list-all`) that is its own `git init` repository with those files
committed, so `git ls-files`, the index and `git check-ignore --no-index` answer for the copy,
never for this checkout; `git` in the copy runs with `-c core.excludesFile=/dev/null` and an
empty `.git/info/exclude`, so a maintainer's global ignore cannot change a result. A mutant
passes only when the plain run is red with the target diagnostic of its clause (PAC-5; other
diagnostics may appear too); a red with only a missing-tool or other-clause diagnostic fails the
self-test. Record the plain and `--self-test` wall times in `evidence/G2.md`. The guard needs `rg`: it fails closed with "rg not found" when it is absent, and 2.4
makes sure the `lint` runner has it.

No scrub pattern and no hostname is written into a tracked file, the test included: a literal
scrub string turns `task scrub` red on the commit that stages it (`hack/scrub.sh:14`), and a
hostname in a tracked file is the leak this change prevents. Mutant strings are built at run time
from `hack/scrub-patterns.txt` and from `hostname -s` (or `AGENT_CONTRACT_HOSTNAME`). Because `rg -i -f` matches
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
- [x] 0.1c (maintainer, local, not committed) Repoint the local harness references from
  `AGENTS.md` to `AGENTS.local.md`: 24 `agent-context/` files (the `agent-context/decisions/`
  briefs are excluded: they discuss the public file) and 6 skill files under `.claude/`,
  `.agents/` and `.cursor/`; re-count with `rg -l --no-ignore 'AGENTS\.md'` at execution — closed 2026-10-19, evidence: evidence/G1.md
- [x] 0.2 HARD PREREQUISITE, merge gate. Inventory: every kollect clone on this host (each
  directory under the workspace whose `origin` is the kollect GitHub repository) and every
  worktree `git worktree list` shows for it. For each, before it checks out, rebases onto, resets
  to or pulls any commit that tracks `AGENTS.md` (this branch included): no root `AGENTS.md`
  that is untracked or ignored, whatever its header; if it had one, it is renamed to
  `AGENTS.local.md` first, refusing to overwrite an existing `AGENTS.local.md`, with that
  checkout's own before/after sha256 compared. A local, untracked record outside the repository
  maps each opaque label (C1, C2, ...) to its checkout and keeps its baseline: whether it holds
  `AGENTS.local.md` and a loader `CLAUDE.md`, and their sha256. `evidence/G0.md` records only the
  count of checkouts, the labels and pass/fail, never paths, checksums of private files or
  machine names. No loader is created in a checkout that had none. The INBOX notice "rename before pulling" for the operator's other machines is posted
  (2026-10-19). Until 0.2 is recorded, no agent moves a listed checkout onto a commit that tracks
  `AGENTS.md` — closed 2026-10-19, evidence: evidence/G1.md (the dispatch put 0.2 in G1)
- [x] 0.3 Change 9 (mutation-testing-signal, PR #471) is merged; record its merge sha in
  `evidence/G0.md` and rebase this branch on it before 2.2 lists the gates — closed 2026-10-19, evidence: evidence/G1.md

## 1. Test first (PAC-1 to PAC-6)

- [x] 1.1 Write `hack/test/agent_contract_test.sh` (plain checks the real tree; `--self-test`
  runs one mutant copy per case and a no-op copy). Record its red on this tree (no `AGENTS.md`
  yet): plain mode fails on the PAC-1 "AGENTS.md is not tracked" assertion. Cases:
  - PAC-1 missing: given a copy without `AGENTS.md` / when plain runs / then red "AGENTS.md is
    not tracked" · a deleted or never-added contract.
  - PAC-1 untracked: given `AGENTS.md` on disk but `.gitignore` ignoring it and not added / then
    red · a contract that exists only locally.
  - PAC-1 unknown task: given `task frobnicate` in `AGENTS.md` / then red naming `frobnicate`
    (names checked against `task --list-all` in the copy) · a stale command.
  - PAC-1 gate list: given `AGENTS.md` without one `run_gate` name of `hack/check.sh`, given
    one naming a gate `hack/check.sh` does not run, and given the `<!-- gates:start -->` marker
    removed / then red naming the gate or the missing marker; a gate name mentioned in prose
    outside the markers changes nothing · a gate list that
    goes stale when a later change adds or drops a gate.
  - PAC-1 task-name parsing: given prose "the task that ...", `task --list-all` and
    `FOO=1 task build` in `AGENTS.md` / then green (only code spans and fences, flags and
    assignments skipped) · false reds on ordinary prose.
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
  - PAC-3 headings: given `AGENTS.md` without the `### When adding a sink` heading (one mutant per
    checklist heading) /
    then red · a missing checklist.
  - PAC-3 stale path: given `internal/sink/registry.go` moved in the copy / then red naming the
    path · a checklist pointing at nothing.
  - PAC-3 stale symbol: given `NewRegistry` renamed in the copy / then red naming the symbol; given
    a method (`Register` on `internal/sink/registry.go`) and a symbol declared in a grouped
    `var (` block / then green · a checklist naming a function that no longer exists, and false
    reds on real declarations.
  - PAC-3 metric entries: given the metric checklist without `registeredMetricNames`, without
    `Catalog`, and without `docs/operator-manual/metrics.md` (one mutant each) / then red · a
    checklist that drops a place CFC-4 or the catalog test needs.
  - PAC-3 placeholder: given `internal/sink/<name>` and `api/v1alpha1/*_types.go` / then green;
    given a glob matching nothing / then red · placeholders read as literal paths, or globs never
    checked.
  - PAC-4 scrub: for each line of `hack/scrub-patterns.txt`, read at run time, plant the line's
    text in the copy's `AGENTS.md` (one mutant per line), once more upper-cased, and, for each line
    containing `.`, a witness with every `.` replaced by `x` (checked with `rg -i` against the
    pattern before use; a line with no usable witness fails the self-test) / then red · a
    forbidden string, a case-sensitive check, and a fixed-string (`rg -F`) check.
  - PAC-4 fixed-string implementation: given the copy's guard with `-F` added to its scrub `rg` /
    then that copy's self-test fails with the regex-witness mutant's "survived" diagnostic (nested
    run under `AGENT_CONTRACT_NESTED=1`, which skips the mutants that start another nested
    self-test) · a guard that lost regex parity with `hack/scrub.sh`.
  - PAC-4 staged differs: given a planted scrub string, and separately a "LOCAL ONLY" first line,
    staged in the copy's index while the working-tree `AGENTS.md` is clean / then red · private
    content committed from the index while the disk looks clean (`hack/scrub.sh` reads disk).
  - PAC-4 pattern list: given `hack/scrub-patterns.txt` empty, and missing / then red · a guard
    that passes because it compared against nothing.
  - PAC-4 rg error: given a fake `rg` first on `PATH` that exits 2, and given no `rg` on `PATH` /
    then red ("rg failed" / "rg not found") · `rg` exit 2 read as "no match", as
    `hack/scrub.sh:14` does.
  - PAC-4 home and harness paths: given `/home/someone/x`, `/Users/someone/x`, `~/x`, and each of
    `agent-context/`, `.claude/`, `.agents/`, `.cursor/`, `AGENTS.local.md`, `worktrees/` / then
    red · machine-local or maintainer-only references.
  - PAC-4 ignore rules: given `.gitignore` ignoring the tracked `AGENTS.md` again, and given it no
    longer ignoring `CLAUDE.md`, `CLAUDE.local.md`, `AGENTS.local.md`, `agent-context/`,
    `.claude/`, `.agents/` or `.cursor/` (one mutant each; checked with
    `git -c core.excludesFile=/dev/null check-ignore --no-index`, directories through a child path
    such as `.claude/x`), also with `HOME` pointing at a directory whose global ignore lists
    `.claude/` / then
    red · an ignore rule on a tracked file that plain `git check-ignore` never reports, and an
    un-ignored private file one `git add -A` from a commit.
  - PAC-4 private header: given `AGENTS.md` whose first 10 lines carry "LOCAL ONLY" or "never
    commit" (any case), on disk and committed / then red · the private file committed under the
    public name.
  - PAC-4 OpenSpec marker: given `<!-- OPENSPEC:START -->`, and separately `<!-- openspec:end -->`,
    in `AGENTS.md` / then red · OpenSpec's
    legacy cleanup rewriting the hand-written contract.
  - PAC-4 gitleaks: given `.github/gitleaks.toml` with an allowlist path regex matching
    `AGENTS.md` (also a broad `.*\.md$`), one matching `CLAUDE.md`, and one matching
    `.github/copilot-instructions.md`, each also in a `[[allowlists]]` table; and given an
    invalid regex there / then red · an agent file exempt from secret scanning, or a regex the
    matcher could not read taken as "no match".
  - PAC-6 generic hosts: given a host under each of `.internal`, `.local`, `.lan` and
    `.home.arpa` in mixed case, an address at each edge of the five IPv4 ranges (for example
    `172.16.0.1` and `172.31.255.254`), an `fd00::1` address, a `Host` alias line and an
    scp-style `user@host:path` address, each built
    at run time / then red · an internal host leaked, or a range checked only at its first subnet.
  - PAC-6 allowed look-alikes: given `svc.cluster.local`, `localhost`, `mise.local.toml`,
    `git@github.com:`, `kollect@sha256:`, `postgres://user:pass@db.example:5432/x`, `172.32.0.1`
    and `192.0.2.1` / then green · false reds on content an
    operator contract needs.
  - PAC-6 running host: given `AGENT_CONTRACT_HOSTNAME` set to a run-time-built name and that name
    planted in any case / then red; given it set to `ab`, `localhost`, `build` (a task name),
    `operator` (a word in tracked Markdown) or empty / then a printed skip; the plain run reads
    `hostname -s`, and the evidence says whether the check was active on the maintainer's machine
    (without naming it) · the maintainer's workstation name
    leaked (it is not in the public scrub list by decision); false reds on contributors' machines.
  - PAC-6 empty pattern set: given the copy's guard with its generic pattern array emptied / then
    that copy's `--self-test` fails; the nested run is started with `AGENT_CONTRACT_NESTED=1`,
    which skips this mutant · a pattern set that can no longer fail, and unbounded recursion.
  - PAC-5 no-op: given an unmodified copy / then plain is green · a guard red on a correct tree.
  - PAC-5 surviving mutant: given a mutant whose plain run stays green / then `--self-test` fails
    naming the mutant · a self-test that reports success without its mutants failing.
  - PAC-5 wrong clause: given a mutant entry whose planted violation is absent and whose copy
    lacks `Taskfile.yml` (so the plain run reds only with the unloadable-Taskfile diagnostic) /
    then `--self-test` fails naming the mutant · a mutant counted as covered because something
    else broke. — closed 2026-10-19 red (known red until G2), evidence: evidence/G1.md

## 2. Contract (closes with 1.1 in one commit, group G2)

- [x] 2.1 Derive the three checklists by dry-run (change one CRD field, one metric, stub one sink
  on a scratch branch in a scratch worktree outside this one) and record in `evidence/G2.md`, as
  Markdown only, which gates fired for each and which files each needed; reconcile with the
  proposal's lists (no `INTEGRATION_PACKAGES` edit for a sink: `Taskfile.yml:20` covers
  `./internal/sink/...`) — closed 2026-10-19, evidence: evidence/G2.md
- [x] 2.2 A fresh subagent started in this worktree (no private agent file in it or loaded: `ls
  AGENTS.local.md CLAUDE.md` fails here; dispatched with `task-prompt.md` only, no inherited
  conversation) writes `AGENTS.md` from the tracked files PAC-1 lists, citing in
  `evidence/G2.md` the tracked file behind each command, gate and checklist line, with the gate
  list between the PAC-1 markers and the checklists under the PAC-3 headings, in these
  sections: commands (the PAC-1 set plus `task lint:shell`,
  `task helm-test`, `task test-integration`, as defined); CI-enforced rules (the required
  contexts and what each runs, read from `.github/workflows/ci.yaml` and `hack/check.sh` on
  `main` after 0.3, so it names only gates that exist there); conventions (link
  `CONTRIBUTING.md` for commit format and rebase merge, link `docs/development/spec-workflow.md`);
  the three checklists from 2.1; and gotchas only where a tracked file states them (cite the
  file). Do not restate CONTRIBUTING.md. Every relative link resolves
  (`bash hack/test/repo_root_links_test.sh` green). Change 6 adds its own entry when it lands — closed 2026-10-19, evidence: evidence/G2.md
- [x] 2.3 Add `.github/copilot-instructions.md` (at most 3 lines; one is enough) pointing to
  `AGENTS.md`; in
  `.gitignore` (the edit formerly numbered 0.1d) delete only the `AGENTS.md` line, keep
  `CLAUDE.md` and `AGENTS.local.md`, and add `CLAUDE.local.md`; remove `.github/gitleaks.toml:10`
  and run `gitleaks detect --source . --config .github/gitleaks.toml --redact --no-git` as CI
  does (`ci.yaml:170`); add the pointer line to `CONTRIBUTING.md`; correct
  `docs/development/setup.md:382`. Confirm `git check-ignore -v AGENTS.local.md CLAUDE.md`
  still matches in the main checkout. In a fresh Claude Code session on a clean clone of the
  branch, confirm the root `AGENTS.md` loads without a `CLAUDE.md` (record in `evidence/G2.md`) — closed 2026-10-19, evidence: evidence/G2.md
- [x] 2.4 Wire the guard into the required `lint` job as two steps (plain, then `--self-test`;
  CWS-6/CFC-6) and add it to `cfc_guards` (`hack/test/ci_workflow_security_test.sh:608-611`; a
  test change on change 4's guard, listed under *Test changes*); run
  `bash hack/test/ci_workflow_security_test.sh` and `--self-test`, the new guard in both modes,
  `task lint:shell`, `task lint:markdown`, and `bash hack/scrub.sh` plus
  `gitleaks protect --staged --no-banner` with the files staged. Check whether the `lint` runner
  image has `rg` (a CI run of the guard printing `rg --version`); if not, install it in the
  `lint` job the way that job installs its other tools, before the guard steps — closed 2026-10-19, evidence: evidence/G2.md

## 3. Review

- [x] 3.1 HARD GATE: an independent reviewer (not the author) reads the committed text as a
  stranger: every sentence true on a fresh clone; no private gotcha, hostname or internal host
  leaked; the review record names the revision — closed 2026-10-19 (stage B, two legs and the
  stage-B orchestrator, none of them an author): reviewed revision `73e92c8c4`; `AGENTS.md`,
  `.github/copilot-instructions.md`, `.gitignore` and `.github/gitleaks.toml` are unchanged
  from it to `96d4ec716`. Every statement true, nothing private. Records:
  `reviews/B-branch/round-1/verification.md`, `reviews/B-branch/round-1/holistic.md`,
  `reviews/B-branch/round-2/verification.md`

## 4. Land

- [x] 4.1 After 3.1 approves and CI is green, archive the change (`openspec archive`) as the last
  commit of the PR (`docs/development/spec-workflow.md` step 6); the reviewer re-checks that
  commit, and the review record names the reviewed and the archive revision; `git diff` of the
  archive commit touches no root `AGENTS.md` or `CLAUDE.md`, and the sha256 of the ignored local
  files in the checkout where `openspec archive` ran is unchanged around it (pass/fail only) — closed 2026-10-19: the archive commit is the last
  commit of PR #472 after green CI on `75ea54eee`; no local `AGENTS.local.md` or `CLAUDE.md` in
  the archiving checkout (absent before and after, pass); the re-check of the archive commit is posted on PR #472
- [ ] 4.2 (integrator) Merge on green required CI on the archive revision, only after 0.2 and 0.3
  are recorded; after merge, in each listed checkout: pull, then confirm the 0.2 baseline holds
  (`AGENTS.local.md` and the loader `CLAUDE.md` unchanged where they existed, absent where they
  were absent); `evidence` gets pass/fail per label only

## 5. Stage-B fixes

From the stage-B branch review (`reviews/B-branch/`). Each guard fix went red first (its new
mutant survived on the unfixed guard, run but not committed) and lands with the mutant in one
commit, so the guard is green at every commit.

- [x] 5.1 Refuse an empty gitleaks path regex (round-1 A3), mutant `PAC-4 gitleaks empty regex` —
  `5312acd86`
- [x] 5.2 Refuse an SSH `Host` line with any number of aliases (round-1 A4), mutant
  `PAC-6 Host line with four aliases`; the `PAC-6 prose look-alikes` control has "host" mid-line
  (a test change) — `8f59b5779`
- [x] 5.3 Refuse `/var/home/<user>`, `/root/` and `C:\Users\<user>` as home paths (round-1 O3),
  one mutant each — `3148ff5d6`
- [x] 5.4 Install ripgrep in the `lint` job without the noble-only version pin (round-1 A5/O2) —
  `221eb661e`
- [x] 5.5 Warn in `docs/development/setup.md` to rename a private `AGENTS.md` before pulling
  (round-1 A2) — `48dae840f`
- [x] 5.6 Replace the absolute worktree and harness paths in `task-prompt.md` and `loop.md` with
  placeholders (round-1 A1/O1, in part) — `4d5209acf`
- [x] 5.7 Refuse a `Host` line with a negated alias (round-2 R2-1), mutant
  `PAC-6 Host line with a negated alias`; an `expect_red` control pins the accepted false red on a
  prose line that starts with "host" (R2-4) — `ca5854194`
- [x] 5.8 Refuse bare `/root`, `/mnt/c/Users/<user>`, `/c/Users/<user>` and an escaped
  `C:\\Users\\<user>`, one mutant each, plus a `c:/users` pin (round-2 R2-2, R2-3) — `96d4ec716`

Not fixed, for the operator: whether loop and review records that name reviewers and models
belong in the public tree, and whether to reword the two spec commits (`ec288c216`, `3cb6d45df`)
whose messages name reviewer models (a history rewrite of reviewed commits).

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| PAC-1 | guard plain on the tree; missing, untracked, unknown-task, task-name-parsing, gate-list, required-command and link mutants; `git ls-files AGENTS.md` in a fresh clone; `repo_root_links_test.sh` | green; each mutant red | pass | `evidence/G2.md` rows 3-5; CI run 39217141284 on `75ea54eee` (lint printed `ripgrep 14.1.0`; guard plain and `--self-test` green) |
| PAC-2 | pointer length, target and tracked-`CLAUDE.md` mutants | red | pass | `evidence/G2.md` rows 4, 6; CI run 39217141284 on `75ea54eee` (lint printed `ripgrep 14.1.0`; guard plain and `--self-test` green) |
| PAC-3 | heading, stale-path, stale-symbol, metric-entry and placeholder mutants; dry-run record from 2.1 | red; no unnamed gate fired | pass | `evidence/G2.md` rows 1, 4 (dry-runs A/B/C); CI run 39217141284 on `75ea54eee` (lint printed `ripgrep 14.1.0`; guard plain and `--self-test` green) |
| PAC-4 | per-pattern, upper-case and regex-witness scrub, fixed-string, staged-differs, empty/missing list, rg-error and rg-missing, home/harness-path, ignore-rule (7 paths), private-header, OpenSpec-marker and gitleaks (3 paths) mutants; `hack/scrub.sh` with files staged; `gitleaks detect --no-git` | each mutant red; scrub ok; no leaks | pass | `evidence/G2.md` rows 4, 6, 10; `reviews/B-branch/` fixes 5.1, 5.3, 5.8; CI run 39217141284 on `75ea54eee` (lint printed `ripgrep 14.1.0`; guard plain and `--self-test` green) |
| PAC-5 | `--self-test` exit status; no-op copy; surviving-mutant and wrong-clause cases | mutants red, no-op green | pass | `evidence/G1.md` rows 8, 10; `evidence/G2.md` row 4; CI run 39217141284 on `75ea54eee` (lint printed `ripgrep 14.1.0`; guard plain and `--self-test` green) |
| PAC-6 | generic-host (range edges, mixed case, ULA, scp form), allowed look-alikes (digest, DSN), running-host and empty-pattern-set mutants | red; look-alikes green; no recursion | pass | `evidence/G2.md` row 4; fixes 5.2, 5.7; CI run 39217141284 on `75ea54eee` (lint printed `ripgrep 14.1.0`; guard plain and `--self-test` green) |
| all | 0.2 checkout record; CI on the PR head; independent review | recorded; green; APPROVE; both revisions recorded | pass | `evidence/G1.md` 0.2 (10 checkouts, 10 pass; pre-merge re-run, 10 pass); PR #472 head `75ea54eee`: 39 checks pass, 3 skipped, `dependency-review` green; 3.1 review revisions `73e92c8c4` and `96d4ec716` |
