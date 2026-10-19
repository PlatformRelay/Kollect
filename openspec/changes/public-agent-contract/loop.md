# Loop — public-agent-contract (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/public-agent-contract` · Branch: `feat/public-agent-contract` · Base: `origin/main` @ `3c01608ff` · Started: 2026-10-19 · Reviewers: astra + claude (duo)
Next: G0 (0.1c, 0.2, 0.3 — 0.3 waits for change 9's merge), then G2 (1.1 + 2.1-2.4) in one commit
Budget: Claude review legs uncapped (operator goal 2026-10-16) · run under GOAL-2026-10-16-announcement-ready (decisions by model alignment, logged in the maintainer's local decision log)

Change 10 of 10 (issue #447). Spec set: proposal.md, specs/agent-contract/spec.md, tasks.md.
Lands after change 9 (PR #471) merges; the branch is rebased on it at task 0.3.

## Stages

- [x] 0 orient — re-measured against `origin/main` @ `3c01608ff` (2026-10-19); four decisions applied (rename ordering A, private loading A, hostname A, landing A)
- [x] P plan + tasks — present; spec set updated cb93d3750 (re-measure, decisions, red-first order with given/when/then cases)
- [x] R spec-set review — round 1 (1c880b52f): astra BLOCK, opus CONCERNS; A-R2/A-R4/A-R5 CRITICAL (migration gate, index vs disk, `check-ignore` on tracked files) verified and fixed in 4685ebd33 with the rest (A-R1 partly rejected, O-9 partly rejected). Round 2 (last, 4685ebd33): astra CONCERNS, opus CONCERNS, no CRITICAL; all findings fixed, unreviewed. No CRITICAL remains open
- [ ] L task loop — see *Tasks*
- [ ] B branch review — `reviews/B-branch/round-N/` · rounds: <n> · verdict: <…>
- [ ] Hand-off — <date> · `pr-description.md` · PR: none

## Fitness functions

| Characteristic | Command | Kind | Baseline | Per task / at B |
|---|---|---|---|---|
| Agent contract guard | `bash hack/test/agent_contract_test.sh` and `--self-test` (once it exists) | triggered | green | per task |
| CI wiring and zizmor gate (CWS-1..7, CFC-6) | `hack/test/ci_workflow_security_test.sh` and `--self-test`; `zizmor --offline --no-progress --min-severity=high --config .github/zizmor.yml .github/` | triggered | green; zizmor no findings | per task touching `.github/` or guards |
| Repo-root links | `bash hack/test/repo_root_links_test.sh` | triggered | green | per task adding root Markdown |
| Scrub and secrets | `bash hack/scrub.sh` (files staged); `gitleaks protect --staged --no-banner`; `gitleaks detect --source . --config .github/gitleaks.toml --redact --no-git` | triggered | `scrub: ok`; no leaks | per commit |
| Shell hygiene | `task lint:shell` | triggered | no warnings | per task |
| Markdown | `task lint:markdown` (tracked files only) | triggered | 0 issues | per task + every orchestrator state edit |
| OpenSpec strict | `task spec:validate` | triggered | green | per spec-set edit |
| Evidence is Markdown | `hack/test/openspec_evidence_markdown_only_test.sh` and `--self-test` | triggered | green | per task |
| Full local gate | `task check` | holistic | green on base | at B |

Holistic run at B: not run yet.

## Triage

| Stage | Finding (one line) | Sev | Models | Disposition (fix / reject / defer) | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| 0 | rename ordering A, private loading A, hostname A, landing A | decision | astra+opus | fix | spec set cb93d3750 | pending operator validation |
| 0 | private loading A (ignored local `CLAUDE.md` loader) contradicts the old committed `CLAUDE.md` pointer: git replaces an ignored file on pull | spec | orchestrator | fix | PAC-2: `CLAUDE.md` stays untracked and ignored; Copilot pointer only | no |
| 0 | the old spec's literal scrub-string mutants would red `task scrub` on the commit that stages them | spec | orchestrator | fix | PAC-4/PAC-5: mutants built at run time | no |
| R1 | A-R1..A-R10, O-1..O-13 (`reviews/R-spec-set/verification.md`) | 3 CRITICAL | astra+opus | fix (A-R1, A-R9, O-9 in part) | spec set 4685ebd33 | no |
| R2 | A2-1..A2-5, O2-1..O2-10 (`reviews/R-spec-set/round-2/verification.md`) | none CRITICAL | astra+opus | fix | spec set, last round, unreviewed: stage B checks the PAC-6 skip rules and self-test cost | no |

## Tasks

| Task | Verdict | Review (legs, rounds, Claude?) | Gaps / decision request |
|---|---|---|---|
| 0.1 | CLOSED (orchestrator, 2026-10-19) | none (local, checksum recorded) | — |
| 0.1b | CLOSED (orchestrator, 2026-10-19) | none (local, fresh-session load verified) | — |

## Known red

Tests a test task left red for a later task; excluded from "green" at close, nothing else is.

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

Tests from a test task changed by a later task (close-task `TEST` class); each one changes what green means.

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|

## Lessons

- R: plain `git check-ignore` never reports a tracked file, and it reads the global excludes file; use `git -c core.excludesFile=/dev/null check-ignore --no-index` in guards (R)
- R: a committed task prompt copied from another change can carry literal scrub patterns or machine names; re-scrub it on copy (R)

<!-- master list: one line per reusable lesson with the task ids that hit it; two or more ids → a proposed harness change at hand-off -->

## Where a human should look first

<!-- filled at hand-off -->

## Proposed harness changes

| Lesson | Seen in | Proposed change (AGENTS.md line / sensor / close-task.yml) |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
| 0.2 | verify the rename in every checkout the agent controls; the operator's other machines follow the INBOX notice |
| 4.2 | merge after 0.2, 0.3, 3.1, the 4.1 archive re-check and green CI; post-merge checksum check in each listed checkout |
