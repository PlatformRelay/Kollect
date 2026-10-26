# Loop — dependency-update-automation (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/dependency-update-automation` · Branch: `spec/dependency-update-automation` · Base: `origin/main` @ `023313519` · Started: 2026-10-26 · Reviewers: two independent reviewers (A, B)
Next: L task loop, 1.1
Budget: paid review legs uncapped (operator goal 2026-10-16); estimate R 2, L 1 to 3 (CI-touching tasks take the extra leg), B 1 to 2 · delegated decisions logged in the maintainer's decision record, pending validation

Change 6 of 10 (issue #443), redesigned 2026-10-26 around the self-hosted Renovate (operator ruling). Spec set: proposal.md, design.md, specs/dependency-updates/spec.md, tasks.md.

## Stages

- [x] 0 orient — re-measured against `origin/main` @ `023313519` (2026-10-26): ruleset, secrets (names only), Renovate run history and log, renovate.json, Renovate 44.167.7 source (github-actions known actions), action `src/docker.ts` at v46.3.2 and v46.3.10. Auto-merge policy decided by a delegated three-leg vote: D (digest-only), pending operator validation
- [x] P spec set — rewritten for the self-hosted design (OpenSpec change; no Spec Kit plan step)
- [x] R spec-set review — round 1 (abaea4694): reviewer A BLOCK, reviewer B BLOCK; 3 CRITICAL, all mechanical and agreed, fixed (verification.md); round 2 (5e2f421c2, last): reviewer A BLOCK (1 CRITICAL: nested presets), reviewer B CONCERNS; all 9 findings verified and fixed unreviewed (round-2/verification.md); stage B re-checks them. No CRITICAL open
- [ ] L task loop
- [ ] B branch review
- [ ] Hand-off

## Fitness functions

| Characteristic | Command | Kind | Baseline | Per task / at B |
|---|---|---|---|---|
| Renovate policy guard | `bash hack/test/renovate_policy_test.sh` and `--self-test` (from 1.1) | triggered | red until section 2, then green | per task |
| CI wiring and zizmor gate (CWS-1..7) | `bash hack/test/ci_workflow_security_test.sh` and `--self-test`; `zizmor --offline --no-progress --min-severity=high --config .github/zizmor.yml .github/` | triggered | green | per task touching `.github/` or guards |
| Toolchain and tool pins (CFC-1, CFC-2, DTP) | `consistency_go_toolchain_test.sh`, `consistency_toolchain_pins_test.sh`, `consistency_tool_pins_test.sh`, each plain and `--self-test` | triggered | green | per task touching `renovate.json` or workflows |
| Workflow permissions | `bash hack/test/sonar_ko_08_workflow_permissions_test.sh` | triggered | green | 2.1 |
| Shell hygiene | `task lint:shell` | triggered | no warnings | per task |
| Markdown | `task lint:markdown` (tracked and intent-to-add files) | triggered | 0 issues | per task and every state edit |
| OpenSpec strict | `task spec:validate` | triggered | green | per spec-set edit |
| Evidence is Markdown | `bash hack/test/openspec_evidence_markdown_only_test.sh` and `--self-test` | triggered | green | per task |
| Full local gate | `task check` | holistic | green on base | at B |

Holistic run at B: not run yet.

## Triage

| Stage | Finding (one line) | Sev | Raised by | Disposition | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| 0 | auto-merge policy: A none / B digest + aged Go patch / C native / D digest only | decision | leg 1 A, leg 2 D, tiebreak D | fix (D) | spec DUA-7, design | pending operator validation |
| R1 | 17 findings (`reviews/R-spec-set/verification.md`): T11 key set, automerge outside packageRules / presets / global config / env, dispatch `RENOVATE_FORCE` | 3 CRITICAL | A+B | fix (B-12 reject) | spec set, round 2 review | no |
| R2 | 9 findings (`reviews/R-spec-set/round-2/verification.md`): nested presets, workflow-level env, `updateNotScheduled`, T13 baseline, mixed-branch fixture, hourly cron | 1 CRITICAL | A+B | fix | spec set, unreviewed (last round) | no |
| 0 | public-text rule: docs task 3.1 runs in a normal task subagent | process | orchestrator | deviate | the section's content is fixed by DUA-10 and T15; the reviewer and the scrub gate check for private content | no |
| L G0 | DUA-9 scenario names branch `renovate/helm`; a major Helm release lands in `renovate/major-helm` (same group, `separateMajorMinor`) | WARNING | A | defer | spec wording precision for stage B; 2.4 asserts both branch names | no |
| L G0 | proposal assumptions "one automerge per run" (job restarts once) and "from v46.3.10 on" (present from v46.3.4) are imprecise, not false | NIT | author, A agrees | record | `evidence/0.4.md` *Assumption check* | no |

## Tasks

| Task | Verdict | Review (legs, rounds, paid leg?) | Gaps / decision request |
|---|---|---|---|
| G0 (0.4, 0.5) | CLOSED | reviewer A, 1 round, REQUEST_CHANGES (wording/evidence only, fixed); paid leg no | none; decision for B: DUA-9 scenario names `renovate/helm`, a major Helm release lands in `renovate/major-helm` (`evidence/0.4.md` *Observations*). Pin for 2.1: `ghcr.io/renovatebot/renovate:44.179.1@sha256:f30ad41a…` (`evidence/0.5.md`) |

## Known red

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|

## PRs

| PR | Purpose | State |
|---|---|---|

## Lessons

<!-- master list: one line per reusable lesson with the task ids that hit it -->

Carried over from earlier changes:

- The Renovate model in `consistency_toolchain_pins_test.sh` reads tracked files only: stage a new workflow before judging DTP-4 (change 9)
- A mutant judged by a message another rule also prints proves nothing about its own rule: judge it by its rule's own message (change 9)
- YAML folds plain and quoted scalars as well as `>` blocks: a line scan of a workflow needs the parsed strings too (change 9)
- Under `set -e`, a `$(grep … | head | cut)` with no match aborts the whole guard: add `|| true` to every probe substitution (change 8)
- A workflow pin must also refuse the keys that skip or soften a job or step (`if:`, `continue-on-error:`), not only its triggers and pins (change 8)
- In yq, `A | B or C` is `(A | B) or C`: parenthesise a boolean after a pipe (change 8)
- Keep `mise.toml` out of a scratch tree that runs a guard: mise refuses the untrusted config (change 5, 9)
- In zsh an unmatched glob aborts the `&&` chain; run loops and globs under `bash` (change 7, 8)
- Renovate grouping resolves only in `lookup` mode; `extract` shows dependencies, not branches (change 5)
- `--platform=local` forces `lookup` whatever `RENOVATE_DRY_RUN` says, so schedule, status and merge decisions never run locally; the resolved branch configs (`automerge`, `schedule`, `rebaseWhen`) are only in the trace-level `branches` record, and `isScheduledNow` from the same package judges a window (G0)
- A single-update group resolves `groupName` to null at branch level; read the branch name or the trace `generateBranchConfig` input for the group (G0)
- The mise `jq` shim refuses inside a scratch tree with `mise.toml` even when no guard runs: call `/usr/bin/jq` there; and zsh does not word-split `$var`, so loops over flag sets go in a bash script (G0)
- A trace-level Renovate log is too large for one Node string and holds non-JSON lines: pre-filter with `jq -R 'fromjson? // empty'` (G0)
