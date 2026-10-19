# Loop — sink-credential-rotation (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/sink-credential-rotation` · Branch: `feat/sink-credential-rotation` · Base: `origin/main` @ `ae98ef106` · Started: 2026-10-19 · Reviewers: two independent review legs (duo)
Next: L task loop, dispatch T0.1 (probe)
Task prompt: local, `agent-context/loops/sink-credential-rotation/task-prompt.md` (harness, not committed) · Budget: paid review legs uncapped (operator goal 2026-10-16) · decisions by model alignment, logged in agent-context/decisions.md

Backlog SECRET-ROTATION-01 (release inventory 2026-10-19 items 12-13). Release note: v0.22.0. Spec set: proposal.md, design.md, specs/sink-secret-rotation/spec.md, tasks.md.

## Stages

- [x] 0 orient — new change, created with `task spec:cli -- new change sink-credential-rotation`; file:line references measured against `origin/main` @ `ae98ef106` (2026-10-19)
- [x] P plan + tasks — written; spec set committed: `84f61bba3`
- [x] R spec-set review — round 1 on `84f61bba3`: reviewer-a BLOCK, reviewer-b CONCERNS; CRITICAL A1 (cached reads) accepted via model-aligned decision → C, CRITICAL A5 (SCR-4 scope) accepted and narrowed; all other findings fixed (`reviews/R-spec-set/verification.md`). Round 2 (last) on `a0ac1aefe`: reviewer-a BLOCK (WARNINGs only), reviewer-b CONCERNS, no CRITICAL; all verified findings fixed (`reviews/R-spec-set/round-2/verification.md`). No CRITICAL remains
- [ ] L task loop — see *Tasks*
- [ ] B branch review — `reviews/B-branch/round-N/`
- [ ] Hand-off — `pr-description.md` · PR: none

## Fitness functions

| Characteristic | Command | Kind | Baseline (number, list, allow-list) | Per task / at B |
|---|---|---|---|---|
| Unit tests, changed packages | `go test -v ./internal/sink/... ./internal/pipeline/...` | triggered | green | per task |
| Envtest, controller path | `go test -v -run TestControllers ./internal/controller/` (Ginkgo focus on the new specs) | triggered | green | G1, G2 |
| Integration-tagged build | `go vet -tags integration ./internal/sink/... ./internal/pipeline/...` | triggered | clean | per task |
| Integration rotation test | `go test -tags integration -v -run TestPostgresExport_passwordRotation <pkg>` (Docker) | triggered | runs, not skipped where Docker exists | G1 |
| Race | `go test -race -count=3` on changed packages (`-count=10 ./internal/sink/` in G3) | triggered | green | per task |
| Architecture | `task lint` (golangci-lint v2 and go-arch-lint) | triggered | no new findings | per task |
| Markdown | `task lint:markdown` (tracked files only; stage new files first) | triggered | 0 issues | per task + every orchestrator state edit |
| OpenSpec strict | `task spec:validate` | triggered | green | per spec-set edit |
| Full local gate | `task check` | holistic | green on base | at B |

Holistic run at B: not run yet.

## Triage

| Stage | Finding (one line) | Sev | Models | Disposition (fix / reject / defer) | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| R1 | Cached Secret reads give only eventual next-export semantics | CRITICAL | reviewer-a | fix | decision C (uncached metadata GET, full GET on rebuild); SCR-1, design *Readers* | no (models agreed) |
| R1 | SCR-4 promised redaction of third-party constructor errors | CRITICAL | reviewer-a | fix | SCR-4 narrowed; non-goal | no |
| R1 | Cross-namespace Secret under restricted cache fails | WARNING | reviewer-a | fix | by decision C; envtest 1.1 | no |
| R1 | Key/values from different reads; slow builder replaces newer entry | WARNING | both | fix | design steps 3-4; 1.1, 3.1 tests | no |
| R1 | Stream-sink cleanup blocked by a deleted Secret | WARNING | reviewer-b | fix | retain before acquire; SCR-2; 2.1 | no |
| R1 | Tombstone wording, namespace param, reds per base, matrix gaps, Close assumption, legacy kind, citations, retire-on-any-error, partial S3 Secret, multiline rg | WARNING/NOTE | both | fix | verification.md rows A4, A6-A12, B2, B4, B6 | no |
| R1 | Metric for pool Secret reads | NOTE | decision counterpoint | defer | follow-up (proposal non-goals) | no |
| R2 | Store fence fails after own retire / absent-as-permission | WARNING | both | fix | generation counter, share-then-fence (design step 4) | no |
| R2 | Cleanup paths not wired to the uncached reader | WARNING | both | fix | `cleanupSinkExports`, `pruneRelationalExport`, reconciler default | no |
| R2 | Postgres assertion on the upsert path; reds green on G1; annotations in metadata; SCR-2/SCR-3 retire contradiction; terminal Git auth doc | WARNING | both | fix | round-2/verification.md A3-A5, B2, B3 | no |
| R2 | TTL wording, stream set, namespace attribution, empty refs, CLI message, CA compat row | NOTE | reviewer-b | fix | round-2/verification.md B4-B10 | no |

## Tasks

| Task | Verdict | Review (legs, rounds, paid leg?) | Gaps / decision request |
|---|---|---|---|
| T0.1 | BLOCKED (SPECIFICATION) | reviewer-a diff, 1 round, REQUEST_CHANGES on evidence class and wording, fixed; no paid leg (evidence only, no BLOCK) | `exportMinInterval: 0s` bindings never re-export unchanged inventories and an inventory-level `0s` does not requeue a transient total failure; contradicts proposal *Non-goals*/*Assumptions* and SCR-2 recovery for that case. Options A/B/C in `evidence/T0.1.md` *Decision request*. Rows 4, 5 by reading (gap). 0.1 left unticked until the decision |

## Known red

Tests a test task left red for a later task; excluded from "green" at close, nothing else is.

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

Tests from a test task changed by a later task (close-task `TEST` class); each one changes what green means.

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|

## Lessons

<!-- master list: one line per reusable lesson with the task ids that hit it; two or more ids → a proposed harness change at hand-off -->

- T0.1: the metadata GET of a missing Secret returns an API NotFound that `ClassOf` classes terminal; `secretMeta` must rewrite it to the plain "not found" text (as `ResolveSecret` does) or a deleted Secret is never retried.
- T0.1: the fake client keeps the UID a test sets and starts `resourceVersion` at 999, +1 per write; model delete-and-recreate by setting a new UID.
- T0.1: a probe that only prints is agent-review, not machine; assert the reported outcome in the probe.
- T0.1: check "bounded by the export interval" claims against `exportMinInterval: 0s` (material-change only, `per_sink_export.go:85-87`).

## Where a human should look first

<!-- filled at hand-off -->

## Proposed harness changes

| Lesson | Seen in | Proposed change (AGENTS.md line / sensor / close-task.yml) |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
