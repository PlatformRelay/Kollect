# Loop — synced-debounce-condition (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/synced-debounce-condition` · Branch: `feat/synced-debounce-condition` · Base: `origin/main` @ `0686f8276` · Started: 2026-10-20 · Reviewers: two independent review legs (duo)
Next: L task loop — dispatch G1 (1.1 then 1.2) with the local task prompt
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
| L T0.1 | Preview test relies on the resolve-error fall-back SDC-4 removes; not in *Impact* | SPEC | reviewer-a agreed | fix | proposal *Impact*, tasks.md 3.1 (G3 *Test changes* row); decisions.md 2026-10-20 `preview-test-fixture` | no |
| R1 | SDC-5 self-contradiction; impossible mixed scenario; second precision; missed message tests; missed docs; matrix breadth; citations | WARNING/NOTE | both | fix | spec, tasks, proposal (`reviews/R-spec-set/verification.md`) | no |

## Tasks

| Task | Verdict | Review (legs, rounds, paid leg?) | Gaps / decision request |
|---|---|---|---|
| T0.1 · CLOSED (after spec fix) · probe: every 0.1 claim holds on `origin/main`; finding: `TestKollectInventoryReconciler_previewAllSinksDebounced_allDebounced` turns red under SDC-4's preview rule and is not in *Impact* | CLOSED | reviewer-a (diff), 1 round, no paid leg | finding 8 resolved by the orchestrator: option A applied to proposal *Impact* and tasks.md 3.1 (only option consistent with SDC-4; see *Triage* L) |
| G1 · CLOSED · 1.1 + 1.2: debounced sink `True/Debounced`, aggregate `True/Exported` "N/N sink(s) up to date", cluster `ExportSucceeded` count, read API `<family>/<name>` + reason-first + own time; M1-M3 killed, M0 survived | CLOSED | reviewer-a (diff), 1 round, APPROVE with one P2 (evidence, fixed); no paid leg | none; race `-count=3` unit reds are pre-existing on base (`evidence/G1.md` *Iterations* 5) |

## Known red

Tests a test task left red for a later task; excluded from "green" at close, nothing else is.

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

Tests from a test task changed by a later task (close-task `TEST` class); each one changes what green means.

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|
| `internal/controller/per_sink_export_test.go:TestAggregateInventorySync` | before this change | G1 | SDC-2: no failed sink → `True/Exported`, one message for export and debounce passes |
| `internal/controller/requested_at_annotation_test.go:257`, `:402` (message assertions) | before this change | G1 | SDC-2: message counts up-to-date sinks ("1/1 sink(s) up to date") |
| `internal/inventory/cov_s06_test.go:TestExportStatusFromInventory_perSinkConditionsOverrideInventoryStatus` | before this change | G1 | SDC-6: `<family>/<name>` lookup, reason-first mapping, entry's own time (empty when none) |

## Lessons

- T0.1: when a design removes a fall-back, grep the tests for fixtures that rely on it; a scratch mutant of just that rule over the whole package finds them in one run.
- G1: `go test -race -count=3 ./internal/controller/` is red on base about one run in three (process-wide circuit breaker for `default/postgres-demo` across repetitions); diff against a `git archive <base>` copy before calling it a regression. Ginkgo refuses `-count>1`: run the unit tests with `-skip TestControllers` and the suite once with `-race`.
- G1: envtest helpers that give each sink its own table let the registry selector route sinks to separate recording backends, so a spec can count exports per sink.
- T0.1: throwaway envtest specs run unchanged in a `git archive origin/main | tar -x` scratch tree with `KUBEBUILDER_ASSETS` set (the relative `bin/k8s` resolver path does not exist there).

## Where a human should look first

## Proposed harness changes

| Lesson | Seen in | Proposed change (AGENTS.md line / sensor / close-task.yml) |
|---|---|---|
| Race `-count=3` on `internal/controller` flakes on base: shared circuit breaker for `default/postgres-demo` | G1 | backlog RACE-COUNT-BREAKER-ISOLATION: reset or isolate the breaker per test in the four reconciler tests (`kollectinventory_export_test.go`, `kollectinventory_finalizer_test.go`) |

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
