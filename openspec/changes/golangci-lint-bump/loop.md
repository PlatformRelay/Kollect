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
- [x] R spec-set review — round 1 `reviews/R-spec-set/` on base: BLOCK, 9 CRITICALs verified (8 fixed in the spec set, 1 rejected as false) + 1 warning deferred; round 2 `reviews/R-spec-set-round2/` on aebb276b: 5/5 legs CONCERNS (unified BLOCK via the promote rule; no leg rated a CRITICAL itself), 8 findings — all mechanical, fixed below; two spec-set rounds spent, no further spec-set re-review per the round limit · CRITICAL: none surviving
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
so the round-2 re-review replaced the stop (logged 2026-10-06).

### Round 2 (5/5 legs, on aebb276b) — all findings WARNING-grade (promoted only by the 2+ rule)

- 1 plugin-pin deferral hollow (change 5 covers only the golangci pins — verified, zero
  plugin/logtools/latest mentions there): FIX IN THIS CHANGE — scope extension, logged as a
  decision: LTB-4 added, the pin lands in the version-only commit. Rationale: the file is
  already in scope, the pin directly serves LTB-3's reproducibility, and no other change owns
  it.
- 2 proposal still promised a mismatch probe "(task 1.1)": FIXED (assumption reworded).
- 3 "the review record" never named: FIXED (`evidence/probe.md`, `evidence/1.3.md`).
- 4 loop.md bookkeeping wrong (pre-announced round 2; count conflict): FIXED (triage row
  above rewritten; stage line now says round 2 done).
- 5 version string cannot distinguish custom from vanilla build: FIXED (task 1.3 states the
  structural proof: clean lint requires the custom build — vanilla fails config validation on
  logcheck; established as probe evidence).
- 6 spec promised change 5 a "runtime task lint failure" guard: FIXED (reworded: DTP-3 drift
  test red-flags the mismatch in CI; verified DTP-3 names both golangci sites).
- 7 make skips the stale `$(GOLANGCI_LINT)` file target on a version change: VERIFIED REAL
  (`make -n golangci-lint GOLANGCI_LINT_VERSION=v2.13.1` → "Nothing to be done"); FIXED in the
  probe procedure (`rm -f bin/golangci-lint*` first). The latent Makefile staleness itself is
  a pre-existing defect beyond this change's version-only scope — deferred, named in the
  archive record.
- 8 `task format:check` swallows stderr (`2>/dev/null` in Taskfile.yml:324): VERIFIED REAL;
  FIXED for this change's evidence (raw output + exit codes recorded; do not rely on the
  swallowed view). The Taskfile weakness is pre-existing — deferred, named in the archive
  record.
- 9 the v2.11.4 "before" count was never produced: FIXED (task 1.1 records it first).

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
