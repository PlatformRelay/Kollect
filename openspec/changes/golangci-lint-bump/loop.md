# Loop — golangci-lint-bump (spec-loop run state)

Repo: `/Users/kheimel/.treehouse/kollect-79dca7/6/kollect` (treehouse pool worktree slot 6;
primary checkout untouched) · Branch: `fm/kollect-dev-continue` · Base: `3ee21266`
(origin/main after PR #450) · Started: 2026-10-06 · Reviewers: fanout (free only)
Implementer model: GLM-5.3 (task processes). Review pairing: task diff legs
`DeepSeek-V4.1-Flash:diff,Qwen3.8-Flash-Next:diff` (both families other than the implementer's);
gate legs GLM-5.3 / DeepSeek-V4.1-Flash / Qwen3.8-Flash-Next plus strong free leg
`Qwen3.8-2.4T-A95B-NVFP4` where spec-loop would use `claude/opus`. NO Claude leg, ever (brief).
Budget: claude review legs 0/0 · active hours 0/8 (default) · source: default (no claude part)

This is an OpenSpec repo (no `.specify/`); the spec set lives in `openspec/changes/golangci-lint-bump/`
(proposal.md, specs/lint-toolchain/spec.md, tasks.md). The orchestrator and every reviewer leg are
free internal models only, per the firstmate brief.

## Stages
- [x] 0 orient — feature dir, what existed (proposal / spec delta / tasks, all present); branch and base recorded
- [ ] P plan + tasks — skipped: present (authored when the change was proposed; re-verified not implemented: tasks.md all unchecked, both version pins still v2.11.4)
- [ ] R spec-set review — `reviews/R-spec-set/` on base · round 1: BLOCK 9 CRITICAL + 1 WARNING (7/7 legs) · all mechanical, fixed in the spec set; round 2 pending · CRITICAL: none surviving
- [ ] L task loop — see *Tasks*
- [ ] B branch review — `reviews/B-branch/round-N/` · rounds: <n> · verdict: <…>
- [ ] Hand-off — <date> · `pr-description.md` · PR: <none | link>

## Fitness functions (inventory at orient)

| Characteristic | Command | Kind | Baseline | Per task / at B |
|---|---|---|---|---|
| Go layering | `go-arch-lint check` (via `task lint`) | triggered | allow-list | per task |
| Go coverage floor | `task coverage` with `COVERAGE_MIN=90` (internal/) | holistic, per change | ≥90% | at B |
| Shell hygiene | `task lint:shell` (shellcheck --severity=warning over hack/**) | triggered | no warnings | per task |
| Workflow supply chain | zizmor `--offline --min-severity=high` over `.github/` (required lint job) | triggered | 0 findings at high | per task |
| CI guard meta-tests | `hack/test/*_test.sh` suite in the required lint job | triggered | green suite | per task |
| Static Go analysis | golangci-lint (`.golangci.yaml`, v2) + Sonar + vulncheck | holistic | green on main | at B (this branch MOVES the linter) |
| Markdown lint | `task lint:markdown` (required) | triggered | clean | per task |

This branch is version-only + lint-fix commits; the linter move itself is the fitness event:
findings count before (v2.11.4) vs after (v2.13.1) is the recorded measure (LTB-3).

## Triage
| Stage | Finding (one line) | Sev | Models | Disposition | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| R | 1 LTB-1 pin-mismatch scenario delivered by no task (`\|\| true` custom-build downgrade; guard deferred to change 5) | CRITICAL | 4/7 | fix | scenario re-scoped to this change's pin-equality check; runtime guard stays change 5's | no |
| R | 2 LTB-2 nolint/exclusion reason rule absent from task 1.3 and the LTB-2 check row | CRITICAL | 4/6 | fix | task 1.3 + verification row now cover reasonless nolint/exclusions | no |
| R | 3 LTB-3 after-count/fixed/justified numbers have no producing task | CRITICAL | 5/7 | fix | task 1.3 records count after, fixed/justified numbers, binary version | no |
| R | 4 `task format:check` runs the bumped binary; formatter drift uncovered | CRITICAL | 4/7 | fix | probe 1.1 runs format:check; 1.3 gate requires it clean; LTB-1 row covers it | no |
| R | 5 spec "each finding its own commit" vs task "one commit per group" | CRITICAL | 4/7 | fix | spec reconciled: commits separate from the version-only commit, grouped when one rule produces many findings | no |
| R | 6 "the Go bump is in no landing list" (proposal premise unverifiable) | CRITICAL | 4/7 | reject | FALSE after verification: cross-file-consistency-gates (change 4) explicitly bumps go.mod to go 1.27.1 in its proposal; DeepSeek-adv's conf-55 doubt of the ordering mechanism also rejected — the ordering constraint holds regardless | no |
| R | 7 no task verifies the executed binary is the pinned custom build | CRITICAL | 2/2 | fix | task 1.3 records `bin/golangci-lint version` (must report the pin); a vanilla binary would fail config validation on logcheck, so a silent downgrade to vanilla is loud at task lint | no |
| R | 8 logcheck plugin pinned `version: latest` floats | CRITICAL/WARN | 2-3 | defer | change 5 (developer-toolchain-pins): out of scope per this proposal's non-goals; GLM legs' NOTE agrees; destination named in the archive record | no |
| R | 9 LTB-3 "runnable" undefined once findings appear | CRITICAL | 3/7 | fix | spec defines runnable = binary starts and reports findings (exit may be non-zero until 1.3) | no |
| R | 10 probe 1.1 not executable as written (vanilla binary fails config validation on the custom plugin) | WARNING | 3/7 | fix | task 1.1 names the procedure: temp bump of BOTH pins, make golangci-lint, record, revert | no |

Decision logged per the skill's CRITICAL-with-mechanical-fix rule: no CRITICAL survived
verification as needing a stop — every fix is a spec-set edit the spec set already implies,
so the round-2 re-review replaces the stop (logged 2026-10-06).

## Tasks
| Task | Verdict | Review (legs, rounds) | Gaps / decision request |
|---|---|---|---|

## Known red
(none)

## Test changes
(none)

## Lessons
<!-- master list; __LESSONS__ is rendered from it -->

## Where a human should look first
<!-- filled at hand-off -->

## Proposed harness changes
| Lesson | Seen in | Proposed change |
|---|---|---|

## Owner tasks (skipped by the loop)
| Task | Command sheet |
|---|---|
| 2.1 archive lands as PR's last commit | merged by the merge authority; PR must be green at that point |
