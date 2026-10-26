# Loop — dependency-update-automation (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/dependency-update-automation` · Branch: `spec/dependency-update-automation` · Base: `origin/main` @ `023313519` · Started: 2026-10-26 · Reviewers: two independent reviewers (A, B)
Next: L task loop, 2.1
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
| L G0 | DUA-9 scenario names branch `renovate/helm`; a major Helm release lands in `renovate/major-helm` (same group, `separateMajorMinor`) | WARNING | A | fix | DUA-9 scenario and its verification row name both branches; 2.4 asserts both | no |
| L G1 | T2 judged a body with no `shell:` under pipefail; the runner uses `bash -e` | WARNING | A+B | fix | guard `run_body`, mutant `m_t2_pipe_masks` | no |
| L G1 | T6 accepted any `configurationFile`, bypassing the global config T13 will check | WARNING | B | fix (after the last round) | T6, mutant `m_t6_other_config`; stage B to re-check | no |
| L G1 | reviewer A round-2 BLOCK rests on its inability to execute, names no defect | process | A | record | `evidence/1.1.md` *Independent review*; clause-removal proof recorded instead | no |
| L G2 | T16 took the expected image from any argv position (another image with it as an argument passed) | WARNING | A | fix | T16 image position parsed; decoy `m_t16_image_as_arg` | no |
| L G2 | self-test cases ran with `set -e` off (subshell left of `&&`); T17 secondary patterns without mutants | NOTE | B | fix | runner marks success inside the case; five T17 mutants | no |
| L G2 | T16 never runs against the tree's own `renovate.yaml`; an unlisted value option can hide the image; T17 lexical | NOTE | B | defer | `evidence/1.2.md` *Harness gaps*; the real validator run in `lint` (2.3) and 3.1's review | no |
| L G0 | proposal assumptions "one automerge per run" (job restarts once) and "from v46.3.10 on" (present from v46.3.4) are imprecise, not false | NIT | author, A agrees | record | `evidence/0.4.md` *Assumption check* | no |

## Tasks

| Task | Verdict | Review (legs, rounds, paid leg?) | Gaps / decision request |
|---|---|---|---|
| G2 (1.2) | CLOSED-WITH-GAPS | reviewer A + reviewer B, 2 rounds; round 1 A REQUEST_CHANGES (T16 took the image from any argv position: fixed), B APPROVE (P2s fixed or deferred); round 2 A APPROVE, B APPROVE; paid leg yes | T14 orders the image rule only against `github actions`; T15 sees a literal `matchDepNames` only; T17 is lexical; T16 runs on the fixture workflow and lists value-taking docker options (`evidence/1.2.md` *Harness gaps*); round-2 message wording unreviewed |
| G1 (1.1) | CLOSED-WITH-GAPS | reviewer A + reviewer B, 2 rounds; round 1 A REQUEST_CHANGES, B CONCERNS (T2 judged an unspecified shell with pipefail: fixed); round 2 A BLOCK (no defect: its sandbox could not execute; its P2 fixed), B CONCERNS (fixed or recorded); paid leg yes | round-2 fixes unreviewed (stage B); T9 does not see a third-party merge action or a line-continued merge command (`evidence/1.1.md` *Harness gaps*) |
| G0 (0.4, 0.5) | CLOSED | reviewer A, 1 round, REQUEST_CHANGES (wording/evidence only, fixed); paid leg no | none; decision for B: DUA-9 scenario names `renovate/helm`, a major Helm release lands in `renovate/major-helm` (`evidence/0.4.md` *Observations*). Pin for 2.1: `ghcr.io/renovatebot/renovate:44.179.1@sha256:f30ad41a…` (`evidence/0.5.md`) |

## Known red

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|
| `hack/test/renovate_policy_test.sh` T1, T2, T3, T4, T5, T6, T7 (plain mode; `lint` red) | DUA-1, DUA-2, DUA-3, DUA-7, DUA-8 | 1.1 | 2.1 |
| `hack/test/renovate_policy_test.sh` T8 (plain mode; `lint` red) | DUA-5 | 1.1 | 2.3 |
| `hack/test/renovate_policy_test.sh` T10, T11, T12, T14, T15 (plain mode; `lint` red) | DUA-3, DUA-4, DUA-7, DUA-8, DUA-9 | 1.2 | 2.2 |
| `hack/test/renovate_policy_test.sh` T16 (plain mode, red on the stub; `lint` red) | DUA-5 | 1.2 | 2.3 |
| `hack/test/renovate_policy_test.sh` T17 (plain mode; `lint` red) | DUA-10 | 1.2 | 3.1 |

## Test changes

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|
| `hack/test/renovate_policy_test.sh` self-test runner (`self_test`, `run_case`) | 1.1 | 1.2 | not wrong, too slow: 148 cases took 136 s sequentially; now parallel copies (16 s), each judged as before, under `set -e`, an early end failing by name (`evidence/1.2.md` *Test changes*); no 1.1 clause changed |

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
- In yq, a union after a pipe needs parentheses: `."on" | ((A), (B), C)`; unparenthesised it printed nothing, and only a list-form mutant caught it (1.1)
- The runner's bash differs by `shell:`: none is `bash -e {0}` (no pipefail), `shell: bash` is `bash --noprofile --norc -eo pipefail {0}`; a guard that executes a `run:` body must emulate the effective one (1.1)
- An error-swallowing read helper is safe only for single values judged as violations when empty; a read whose rows feed a loop must red on a yq error, or no rows means a pass (1.1)
- Reviewer A's read-only sandbox also refuses `/tmp`, so it cannot mutate a scratch copy: record the clause-removal proof in the evidence before the brief (1.1)
- A self-test for a guard whose clauses are red on today's tree runs on a known-good fixture of the guarded file, or every mutant reds on an already-red clause (1.1)
- A guard that finds an expected value "anywhere in the argv" passes when it sits in another slot (the image as an argument of another image): judge argv by position, with a decoy that puts the right value in the wrong slot (1.2)
- A subshell on the left of `&&` runs with `set -e` off inside it: a parallel case runner marks success from inside the subshell, never `(…) && touch ok &` (1.2)
- A yq TSV row with an empty field collapses under `IFS=$'\t'` (tab is IFS whitespace): prefix each field with a marker such as `=` and strip it after the read (1.2)
- A guard over a JSON config gets a fixture checked by the pinned validator in both modes, so the known-good tree is valid for the real tool, not only for the guard (1.2)
- A trace-level Renovate log is too large for one Node string and holds non-JSON lines: pre-filter with `jq -R 'fromjson? // empty'` (G0)
