# PR description — public-agent-contract

**Create at:** <https://github.com/platformrelay/kollect/compare/main...feat/public-agent-contract>

- **Source:** `feat/public-agent-contract`
- **Target:** `main`
- **Title:** `:page_facing_up: docs: publish the agent contract with a leak guard`
- **Opens as:** **ready** — every gate below ran before the PR opened; it merges on green
  required CI with a rebase merge.

Paste everything below the line.

---

## TL;DR

- **What:** commits a public root `AGENTS.md` (commands, CI gates, conventions, three "when
  adding X" checklists), a one-line Copilot pointer, and `hack/test/agent_contract_test.sh`,
  a guard that keeps the file true and free of private content.
- **Why:** `AGENTS.md` was gitignored, so a fresh clone or a contributor's coding assistant
  started with no commands and learned the gates from failed CI runs (#447).
- **Risk:** un-ignoring `AGENTS.md` means `git pull` silently overwrites any ignored private
  `AGENTS.md` still sitting in a checkout. Rename it to `AGENTS.local.md` **before** pulling.
- **Reviewer action:** read `AGENTS.md` as a stranger would (every sentence true on a fresh
  clone, nothing private), and check the guard's lint wiring.

`AGENTS.md`, `CLAUDE.md` and `AGENTS.local.md` were all gitignored. This PR publishes a public
contract written only from tracked files, keeps `CLAUDE.md` and `AGENTS.local.md` ignored, and
adds a guard with a mutation self-test that fails the required `lint` job if the contract goes
stale or leaks.

Closes #447

## Most important things

- **The public contract.** `AGENTS.md` lists the build and test commands, the full local gate
  (`task check`) with the gate list between `<!-- gates:start -->` markers, the required CI
  contexts and what each runs, links to `CONTRIBUTING.md` and
  `docs/development/spec-workflow.md` (no restated conventions), and checklists for adding a CRD
  field, a metric and a sink. The checklists come from dry-runs on a scratch branch, not from
  memory: each lists the files the gates actually asked for.
- **The leak guard.** `hack/test/agent_contract_test.sh` checks that every `task <name>` exists,
  that the gate list matches `hack/check.sh` exactly, and that checklist paths and symbols are
  real. It refuses any `hack/scrub-patterns.txt` match with the same `rg -i -f` semantics as
  `hack/scrub.sh`, on disk and in the index. It also refuses home and harness paths, internal
  hosts by generic pattern, the running machine's hostname, a "LOCAL ONLY" header, OpenSpec
  markers, a gitleaks allowlist entry for an agent file, and `.gitignore` drift. `--self-test`
  runs one mutant per clause on a throwaway `git init` copy, and each mutant must red with its
  own diagnostic. Both modes run as separate steps in the required `lint` job, and the guard is
  in the CFC-6 `cfc_guards` list.
- **Data-loss warning: rename before pulling.** Git replaces an ignored untracked file with a
  tracked one on pull, without a warning. Anyone with a private root `AGENTS.md` must rename it
  to `AGENTS.local.md` first. Every checkout on the maintainer's host was verified before the
  merge (task 0.2, `evidence/G1.md`), and `docs/development/setup.md` now carries the warning.
- **Copilot pointer.** `.github/copilot-instructions.md` is one line pointing to `AGENTS.md`.
  The guard caps it at three lines so it cannot grow into a second copy.
- **`CLAUDE.md` stays local.** No `CLAUDE.md` is committed. It stays ignored because it is the
  maintainer's local loader for `AGENTS.local.md`, and a tracked one would overwrite that loader
  on pull, the same failure as above. `CLAUDE.local.md` is now ignored too. The guard fails if
  `CLAUDE.md` is ever tracked.
- **The four decisions (2026-10-19).** *Rename ordering A:* rename the private file and verify
  every checkout before `.gitignore` stops ignoring `AGENTS.md`. *Private loading A:* an
  ignored local `CLAUDE.md` loads `AGENTS.local.md`, rather than a committed loader or
  `CLAUDE.local.md`. *Hostname A:* the workstation hostname is not added to the public scrub
  list (that would publish it); the guard checks the running host at run time instead.
  *Landing A:* this lands after change 9 (#471), so `AGENTS.md` names only gates that exist on
  `main`.

## Risk

- **What can break:** a checkout that still holds an ignored private `AGENTS.md` loses it on its
  first pull after merge. The guard can red `lint` on a later PR that adds a gate, renames a
  checklist symbol or moves a checklist path without updating `AGENTS.md`. That is intended.
- **Blast radius:** contributors' local files and the `lint` job. No product code, chart or
  manifest changes.
- **Rollback:** revert the PR. A revert re-ignores `AGENTS.md`, but it does not restore a
  private file a pull already overwrote. That is why the rename gate ran before the merge.
- **Residual risk accepted:** the generic host patterns are heuristics. The hostname check only
  runs where `hostname -s` is distinctive (it skips short or common names). The guard installs
  ripgrep from the runner's archive without a version pin, because a pinned Debian revision
  exists in one Ubuntu release only.

## Files to look at first

- `AGENTS.md` — read it cold: every command, gate and checklist line must be true on a fresh
  clone; `evidence/G2.md` cites the tracked file behind each line.
- `hack/test/agent_contract_test.sh` — the leak clauses (PAC-4, PAC-6) and the self-test
  harness that proves each one can fail.
- `.gitignore`, `.github/gitleaks.toml` — `AGENTS.md` un-ignored and no longer allowlisted;
  `CLAUDE.md`, `CLAUDE.local.md`, `AGENTS.local.md` ignored.
- `.github/workflows/ci.yaml` (`lint` job) — ripgrep install, then plain and `--self-test`
  steps.
- `docs/development/setup.md` — the rename-before-pull warning.

**Review this commit-by-commit, not as one flattened diff.** The spec set, the guard with the
contract, and each review fix (one mutant plus its fix per commit) are separate, so each fix
can be checked against the finding it closes.

## Testing protocol

- [x] `bash hack/test/agent_contract_test.sh` and `--self-test` green on the branch head.
- [x] `bash hack/test/ci_workflow_security_test.sh` and `--self-test`,
  `bash hack/test/repo_root_links_test.sh`, `task lint:shell`, `task lint:markdown`,
  `task spec:validate` green.
- [x] `bash hack/scrub.sh`, and gitleaks over `origin/main..HEAD` and with `--no-git`: no
  findings.
- [x] Merge gate (task 0.2): every kollect checkout on the maintainer's host has no ignored or
  untracked root `AGENTS.md` (10 checkouts, 10 pass).
- [ ] The first CI run prints `rg --version` in the `lint` job's "Ensure ripgrep is available"
  step. If it does not, the guard steps fail with "rg not found".
- [ ] Every check on the head SHA is green, `dependency-review` included.
- [ ] Archive commit (`openspec archive public-agent-contract`) is the PR's last commit, with
  `task spec:validate` green on it.
- [ ] After merge, in each checkout: pull, then confirm `AGENTS.local.md` and the loader
  `CLAUDE.md` are unchanged, `git check-ignore AGENTS.local.md CLAUDE.md` still matches, and the
  tracked `AGENTS.md` is the public one.

---

## Optional reading — the long version

*Nothing above depends on this. Read it if you want the full story, the road not taken, or the
deployment fine print — skip it if the top half already told you what you needed.*

**Spec and evidence.** `openspec/changes/public-agent-contract/`: `proposal.md`,
`specs/agent-contract/spec.md` (PAC-1 to PAC-6), `tasks.md`, `evidence/G1.md` (checkout gate,
rebase on change 9, guard red), `evidence/G2.md` (checklist dry-runs, the citation table for
every line of `AGENTS.md`, the clean-clone load check), and the review records under `reviews/`.

**Review.** The spec set had two review rounds and the branch had two (stage B). Every finding
was fixed or recorded. Round-1 fixes: empty gitleaks regex, SSH `Host` aliases, more home-path
forms, an unpinned ripgrep install and the setup warning. Round-2 fixes: negated `Host`
aliases, bare `/root`, and WSL and escaped Windows home paths. Each guard fix went red first and
lands with its mutant in one commit.

**Why a guard and not a review rule.** A private gotcha or a hostname that slips into a tracked
file is public as soon as it is pushed. The scrub list is public by design, so a guard has to
reuse it with identical semantics. It builds every mutant string at run time, from that list
and from `hostname -s`, so no forbidden string is ever committed, not even in the test.

**Out of scope.** Renovate automerge is not described; change 6 adds its own `AGENTS.md` entry
when it lands. Whether review records that name the reviewing legs belong in the public tree is
an open operator question, not decided here.
