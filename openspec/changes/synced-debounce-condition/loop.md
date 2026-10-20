# Loop — synced-debounce-condition (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/synced-debounce-condition` · Branch: `feat/synced-debounce-condition` · Base: `origin/main` @ `0686f8276` · Started: 2026-10-20 · Reviewers: two independent review legs (duo)
Next: L task loop — dispatch T0.1 (probe) with the local task prompt
Task prompt: local, `agent-context/loops/synced-debounce-condition/task-prompt.md` (harness, not committed) · Budget: paid review legs uncapped (operator goal 2026-10-16) · decisions by model alignment, logged in agent-context/decisions.md

Backlog SYNCED-DEBOUNCE-STEADY-STATE (kind quick start 2026-10-20; operator: fix before v0.22.0).
Release note: v0.22.0. Spec set: proposal.md, design.md, specs/inventory-sync-status/spec.md,
tasks.md.

## Stages

- [x] 0 orient — new change; file:line references measured against `origin/main` @ `0686f8276` (2026-10-20)
- [x] P plan + tasks — written; decision 2026-10-20 `synced-debounce-reason` (both legs A, pending operator validation); spec set committed: `b7871afcd`
- [x] R spec-set review — round 1 on `a268bfc96`: reviewer-a BLOCK, reviewer-b BLOCK; four CRITICALs accepted (sink identity, preview resolve error, total-failure per-sink write, read API key) and fixed with the WARNINGs (`reviews/R-spec-set/verification.md`). Round 2 (last) on `138dada5c`: reviewer-a BLOCK, reviewer-b CONCERNS; CRITICAL inventory UID fixed, CRITICAL Secret-repointed destination rejected as scope (decision 2026-10-19 `rotation-zero-interval`), WARNINGs fixed (`reviews/R-spec-set/round-2/verification.md`). No CRITICAL remains
- [ ] L task loop — see *Tasks*
- [ ] B branch review — `reviews/B-branch/round-N/`
- [ ] Hand-off — `pr-description.md` · PR: none

## Fitness functions

| Characteristic | Command | Kind | Baseline (number, list, allow-list) | Per task / at B |
|---|---|---|---|---|
| Unit tests, changed packages | `go test -v ./internal/controller/ ./internal/inventory/` | triggered | green | per task |
| Envtest, controller path | `go test -v -run TestControllers ./internal/controller/` (Ginkgo focus on the new specs) | triggered | green | G1, G2, G3 |
| Race | `go test -race -count=3 ./internal/controller/ ./internal/inventory/` | triggered | green | per task |
| Architecture | `task lint` (golangci-lint v2 and go-arch-lint) | triggered | no new findings | per task |
| Markdown | `task lint:markdown` (tracked files only; stage new files first) | triggered | 0 issues | per task + every orchestrator state edit |
| OpenSpec strict | `task spec:validate` | triggered | green | per spec-set edit |
| Full local gate | `task check` | holistic | green on base | at B |

Holistic run at B: not run yet.

## Triage

| Stage | Finding (one line) | Sev | Reviewers | Disposition (fix / reject / defer) | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| R1 | Baseline ignores sink UID and generation | CRITICAL | reviewer-a | fix | SDC-4, design, G3 | no |
| R1 | Preview debounces an unresolvable sink; deleted sink bypasses `forget` | CRITICAL | reviewer-a, reviewer-b | fix | SDC-4 (UID check, preview), G3 | no |
| R1 | Total failure leaves per-sink `True/Exported` | CRITICAL | reviewer-a, reviewer-b | fix | SDC-3, design, G1 | no |
| R1 | Read API looks up bare names, never matches | CRITICAL | reviewer-a, reviewer-b | fix | SDC-6, G1 | no |
| R1 | Shard warning Event every pass at >= 1800 rows | WARNING | reviewer-a | defer | non-goal; SHARD-WARN-EVENT-CHURN at 5.1 | no |
| R2 | Recreated inventory (same name) debounced against an emptied backend | CRITICAL | reviewer-a | fix | SDC-4 inventory UID, G3 | no |
| R2 | Secret repointing the sink not detected by the debounce | CRITICAL→scope | reviewer-a, reviewer-b | reject (scope) | non-goal; decision 2026-10-19 `rotation-zero-interval` | no |
| R2 | Read API time fallback; untested failure branches and snapshot path; doc regex; archive re-review; G1 T1/C1 regression; test-change rows | WARNING/NOTE | both | fix | `reviews/R-spec-set/round-2/verification.md` | no |
| R1 | SDC-5 self-contradiction; impossible mixed scenario; second precision; missed message tests; missed docs; matrix breadth; citations | WARNING/NOTE | both | fix | spec, tasks, proposal (`reviews/R-spec-set/verification.md`) | no |

## Tasks

| Task | Verdict | Review (legs, rounds, paid leg?) | Gaps / decision request |
|---|---|---|---|

## Known red

Tests a test task left red for a later task; excluded from "green" at close, nothing else is.

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

Tests from a test task changed by a later task (close-task `TEST` class); each one changes what green means.

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|

## Lessons

## Where a human should look first

## Proposed harness changes

| Lesson | Seen in | Proposed change (AGENTS.md line / sensor / close-task.yml) |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
