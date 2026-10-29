# Loop — dependency-update-automation (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/dependency-update-automation` · Branch: `spec/dependency-update-automation` · Base: `origin/main` @ `023313519` · Started: 2026-10-26 · Reviewers: two independent reviewers (A, B)
Next: none — archived
Budget: paid review legs uncapped (operator goal 2026-10-16); estimate R 2, L 1 to 3 (CI-touching tasks take the extra leg), B 1 to 2 · delegated decisions logged in the maintainer's decision record, pending validation

Change 6 of 10 (issue #443), redesigned 2026-10-26 around the self-hosted Renovate (operator ruling). Spec set: proposal.md, design.md, specs/dependency-updates/spec.md, tasks.md.

## Stages

- [x] 0 orient — re-measured against `origin/main` @ `023313519` (2026-10-26): ruleset, secrets (names only), Renovate run history and log, renovate.json, Renovate 44.167.7 source (github-actions known actions), action `src/docker.ts` at v46.3.2 and v46.3.10. Auto-merge policy decided by a delegated three-leg vote: D (digest-only), validated by the operator 2026-10-28
- [x] P spec set — rewritten for the self-hosted design (OpenSpec change; no Spec Kit plan step)
- [x] R spec-set review — round 1 (abaea4694): reviewer A BLOCK, reviewer B BLOCK; 3 CRITICAL, all mechanical and agreed, fixed (verification.md); round 2 (5e2f421c2, last): reviewer A BLOCK (1 CRITICAL: nested presets), reviewer B CONCERNS; all 9 findings verified and fixed unreviewed (round-2/verification.md); stage B re-checks them. No CRITICAL open
- [x] L task loop — G0 to G4 closed (see *Tasks*)
- [x] B branch review — rebased on `e95e8005d`; holistic `task check`: 4 environment reds, all green after fixing the environment (verification.md); round 1 (adccaeae2): reviewer A CONCERNS, reviewer B spec CONCERNS, fitness CONCERNS; no CRITICAL; fixes 5.1 to 5.4 closed in G5 (reviewed, both legs, 2 rounds); no round 2 of B (no accepted CRITICAL). `AGENTS.md` gate list gained the `renovate-config` line (PAC-1 sync, accepted)
- [x] Hand-off — #502 merged (`2cf23a200`, 2026-10-28); post-merge fix #506 (`c0e8629ab`): the `dependencies` label cancelled the E2E extended run and Renovate read the cancelled checks as pending (T18 added); post-merge rows pass (`evidence/post-merge.md`): scheduled runs, pinned image, dashboard #505, bot auto-merges #503 and #504; archived in the follow-up evidence PR. Open, non-gating: task 4.2 (operator removes the hosted App)

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

Holistic run at B: `task check` on adccaeae2, 2026-10-28: red only on gitleaks-detect (ignored docs-build cache), demo-04 kustomize (mise kubectl shim), two dist OLM guards (operator-sdk absent); each green after fixing the environment.

## Triage

| Stage | Finding (one line) | Sev | Raised by | Disposition | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| 0 | auto-merge policy: A none / B digest + aged Go patch / C native / D digest only | decision | leg 1 A, leg 2 D, tiebreak D | fix (D) | spec DUA-7, design | validated by the operator 2026-10-28 |
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
| L G3 | the validator step has not run in CI: it follows the guard's plain step, red on T17 until 3.1 | NOTE | B | record | Verification row DUA-5 "`lint` on the PR head" stays not-run until a head with 3.1 | no |
| L G4 | `docs/security/sca-remediation-policy.md` said `vulncheck` and `lint` are not required checks | P2 | A | fix | `4dca03ca8` | no |
| L G4 | `CONTRIBUTING.md:139` allows squash merges; the ruleset allows rebase only | NOTE | author | defer | stage B | no |
| L G3 | a mixed group branch can inherit `rebaseWhen: behind-base-branch` from a digest upgrade that sorts first (`generate.js:217-229`): rebased when behind, never merged | NOTE | B | record | `evidence/2.4.md` *Observations*; cost only, no requirement; for stage B | no |
| L G5 | design.md *Hourly* and *Risks* say "such a digest" / "an image digest" where only Dockerfile and devcontainer digests carry the setting | NOTE | B | defer | hand-off (wording; after the last round) | no |
| L G5 | `AGENTS.md`'s `lint` bullet omits the validator step | NOTE | B | defer | hand-off (pre-existing, unguarded) | no |
| L G5 | `docker info` in `hack/check.sh` has no timeout | NOTE | B | defer | hand-off (slow skip, never a wrong result) | no |
| L G0 | proposal assumptions "one automerge per run" (job restarts once) and "from v46.3.10 on" (present from v46.3.4) are imprecise, not false | NIT | author, A agrees | record | `evidence/0.4.md` *Assumption check* | no |

## Tasks

| Task | Verdict | Review (legs, rounds, paid leg?) | Gaps / decision request |
|---|---|---|---|
| G5 (5.1, 5.2, 5.3, 5.4) | CLOSED | reviewer A + reviewer B, 2 rounds; round 1 A APPROVE, B BLOCK (PAC-1 red: the conditional `renovate-config` gate missing from the `AGENTS.md` gate list: fixed; four wording/matrix notes fixed); round 2 A APPROVE, B CLEAN (three NOTEs deferred to the hand-off); paid leg yes | none; `AGENTS.md` gate list edited for PAC-1 (alternative: an exclusion line, revert `fb3e9269f` with `88895b723`) |
| G4 (3.1) | CLOSED-WITH-GAPS | reviewer A, 1 round, REQUEST_CHANGES (SCA policy said `vulncheck`/`lint` not required: fixed; T17 lexical: row qualified); paid leg no | T17 lexical (from 1.2); `CONTRIBUTING.md:139` squash-merge line left for stage B |
| G3 (2.1, 2.2, 2.3, 2.4) | CLOSED | reviewer A + reviewer B, 1 round; A APPROVE (one matrix dispute: tool-pins self-test pending, run in FINAL), B CLEAN (three NOTEs, recorded); paid leg yes | none in the group; the `lint` validator step's CI proof waits for 3.1 (Verification row DUA-5) |
| G2 (1.2) | CLOSED-WITH-GAPS | reviewer A + reviewer B, 2 rounds; round 1 A REQUEST_CHANGES (T16 took the image from any argv position: fixed), B APPROVE (P2s fixed or deferred); round 2 A APPROVE, B APPROVE; paid leg yes | T14 orders the image rule only against `github actions`; T15 sees a literal `matchDepNames` only; T17 is lexical; T16 runs on the fixture workflow and lists value-taking docker options (`evidence/1.2.md` *Harness gaps*); round-2 message wording unreviewed |
| G1 (1.1) | CLOSED-WITH-GAPS | reviewer A + reviewer B, 2 rounds; round 1 A REQUEST_CHANGES, B CONCERNS (T2 judged an unspecified shell with pipefail: fixed); round 2 A BLOCK (no defect: its sandbox could not execute; its P2 fixed), B CONCERNS (fixed or recorded); paid leg yes | round-2 fixes unreviewed (stage B); T9 does not see a third-party merge action or a line-continued merge command (`evidence/1.1.md` *Harness gaps*) |
| G0 (0.4, 0.5) | CLOSED | reviewer A, 1 round, REQUEST_CHANGES (wording/evidence only, fixed); paid leg no | none; decision for B: DUA-9 scenario names `renovate/helm`, a major Helm release lands in `renovate/major-helm` (`evidence/0.4.md` *Observations*). Pin for 2.1: `ghcr.io/renovatebot/renovate:44.179.1@sha256:f30ad41a…` (`evidence/0.5.md`) |

## Known red

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|
| ~~`hack/test/renovate_policy_test.sh` T1, T2, T3, T4, T5, T6, T7 (plain mode; `lint` red)~~ cleared | DUA-1, DUA-2, DUA-3, DUA-7, DUA-8 | 1.1 | 2.1 (`7ddb47c08`) |
| ~~`hack/test/renovate_policy_test.sh` T8 (plain mode; `lint` red)~~ cleared | DUA-5 | 1.1 | 2.3 (`779ec7182`) |
| ~~`hack/test/renovate_policy_test.sh` T10, T11, T12, T14, T15 (plain mode; `lint` red)~~ cleared | DUA-3, DUA-4, DUA-7, DUA-8, DUA-9 | 1.2 | 2.2 (`c7a649aec`) |
| ~~`hack/test/renovate_policy_test.sh` T16 (plain mode, red on the stub; `lint` red)~~ cleared | DUA-5 | 1.2 | 2.3 (`779ec7182`) |
| ~~`hack/test/renovate_policy_test.sh` T17 (plain mode; `lint` red)~~ cleared | DUA-10 | 1.2 | 3.1 (`2a107fecc`) |

## Test changes

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|
| `hack/test/renovate_policy_test.sh` T7 (refuses `NODE_OPTIONS`, `GITHUB_COM_TOKEN`, `*_proxy` keys), mutants `m_t7_node_options`, `m_t7_github_com_token`, `m_t7_step_proxy`, control `c_env_neighbours` | 1.1 | 5.2 | incomplete, not wrong: stage B finding B5; DUA-3/DUA-7 allow no environment override, and the action forwards these next to the token (`evidence/5.1.md` *Test changes*) |
| `hack/test/renovate_policy_test.sh` self-test runner (`self_test`, `run_case`) | 1.1 | 1.2 | not wrong, too slow: 148 cases took 136 s sequentially; now parallel copies (16 s), each judged as before, under `set -e`, an early end failing by name (`evidence/1.2.md` *Test changes*); no 1.1 clause changed |

## PRs

| PR | Purpose | State |
|---|---|---|
| #502 | change 6 | merged 2026-10-28, `2cf23a200`; `main` CI green by headSha |
| #506 | post-merge fix: label-cancelled E2E extended run (T18) | merged 2026-10-28, `c0e8629ab`; `main` CI green by headSha |
| follow-up | post-merge evidence and archive (with ci-workflow-hardening) | this PR |

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
- Renovate's branch config is its first sorted upgrade's config spread over the group (`generate.js:225-229`); only `automerge` and a few lists are recomputed over all upgrades, so a rule-level setting such as `rebaseWhen` can leak into a mixed branch (2.4)
- A lint step placed after a guard step that is red by design never runs in CI: its CI row stays not-run until the last known red clears (2.3)
- `/tmp` is a quota'd tmpfs: three trace-level Renovate logs (about 0.8 GB each) made a later Go link fail with `disk quota exceeded`, which the Makefile's `|| true` custom golangci-lint build turned into a misleading `plugin "logcheck" not found`; delete trace logs once summarised, and check `df /tmp` before blaming a red (G3)
- Two guards parse `hack/check.sh` (`task_check_test.sh` and `agent_contract_test.sh` PAC-1); a new `run_gate`, even inside an `if`, must be in the `AGENTS.md` gate list (5.3)
- An evidence file that quotes the scrub pattern makes the scrub over `git diff origin/main` match itself: name the scrub, do not quote it (5.1)
- `task docs:verify` in a worktree needs `MISE_KUBECTL_VERSION` set and a venv of `docs/requirements-docs.txt` on `PATH`; without them it reds on the base too (3.1)
