# Tasks

## 0. Protect the private file first (PAC-4)

- [ ] 0.1 (maintainer, local) In the main checkout, rename the local `AGENTS.md` to `AGENTS.local.md` (stays ignored); confirm `git status` there shows nothing new
- [ ] 0.1b (maintainer, local) Create `CLAUDE.local.md` containing `@AGENTS.local.md`, because Claude Code loads `CLAUDE.md` (soon a one-line public pointer) and the private notes would otherwise stop loading
- [ ] 0.1d (committed) Add `CLAUDE.local.md` to `.gitignore` (a committed edit, in this PR, in the same commit that stops ignoring `AGENTS.md` and `CLAUDE.md`)
- [ ] 0.1c (maintainer, local, not committed) Repoint the roughly 20 local `agent-context/` files that reference the repo `AGENTS.md` to `AGENTS.local.md`
- [ ] 0.2 Only after 0.1: remove `.github/gitleaks.toml:10` (`AGENTS\.md` allowlist); run the gitleaks scan on the tree

## 1. Test first (PAC-1 to PAC-5)

- [ ] 1.1 Write `hack/test/agent_contract_test.sh` with mutants: missing file, unknown task, long pointer, missing pointer target, stale checklist path, scrub string, hostname-like string, internal-host string, home path, harness path, ignored again, "LOCAL ONLY" header, gitleaks allowlist, OpenSpec marker; watch it fail on the missing-file assertion

## 2. Contract

- [ ] 2.1 Derive the three checklists by dry-run (change one CRD field, one metric, stub one sink on a scratch branch) and record which gates fired; reconcile with the proposal's lists
- [ ] 2.2 In a worktree with no local `AGENTS.md`/harness file in scope, write `AGENTS.md` (commands that exist, CI-enforced rules, conventions, checklists); do not restate CONTRIBUTING.md
- [ ] 2.3 (after 0.1) Add the two pointers; adjust `.gitignore`; add the pointer line to `CONTRIBUTING.md`
- [ ] 2.4 Wire the test into `lint`; run `bash hack/scrub.sh` with the files staged as well

## 3. Review

- [ ] 3.1 HARD GATE: an independent reviewer (not the author) reads the committed text as a stranger: every sentence true on a fresh clone; no private gotcha, hostname or internal host leaked; the review record names the revision

## 4. Land

- [ ] 4.1 Archive the change (`openspec archive`) as the last commit of the PR, after review and green CI; the review record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| PAC-1 | test file-present and task-exists checks; `git ls-files AGENTS.md` in a fresh clone | green; mutant red | not-run | |
| PAC-2 | pointer length and target mutants | red | not-run | |
| PAC-3 | stale-path mutant; dry-run record from 2.1 | red; no unnamed gate fired | not-run | |
| PAC-4 | scrub, home-path, harness-path, gitignore, LOCAL-ONLY-header and gitleaks-allowlist mutants; `task scrub` with files staged | each mutant red; scrub ok | not-run | |
| PAC-5 | self-test plus no-op copy, exit status | mutants red, no-op green | not-run | |
| all | CI on the PR head; independent review | green; APPROVE; both revisions recorded | not-run | |
