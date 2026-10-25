# Loop — synced-debounce-condition (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/synced-debounce-condition` · Branch: `feat/synced-debounce-condition` · Base: `origin/main` @ `0686f8276` · Started: 2026-10-20 · Reviewers: two independent review legs (duo)
Next: 5.1 integrator (pull request, merge, then archive in a follow-up pull request)
Task prompt: local, `agent-context/loops/synced-debounce-condition/task-prompt.md` (harness, not committed) · Budget: paid review legs uncapped (operator goal 2026-10-16) · decisions by model alignment, logged in agent-context/decisions.md

Backlog SYNCED-DEBOUNCE-STEADY-STATE (kind quick start 2026-10-20; operator: fix before v0.22.0).
Release note: v0.22.0. Spec set: proposal.md, design.md, specs/inventory-sync-status/spec.md,
tasks.md.

## Stages

- [x] 0 orient — new change; file:line references measured against `origin/main` @ `0686f8276` (2026-10-20)
- [x] P plan + tasks — written; decision 2026-10-20 `synced-debounce-reason` (both legs A, pending operator validation); spec set committed: `b7871afcd`
- [x] R spec-set review — round 1 on `a268bfc96`: reviewer-a BLOCK, reviewer-b BLOCK; four CRITICALs accepted (sink identity, preview resolve error, total-failure per-sink write, read API key) and fixed with the WARNINGs (`reviews/R-spec-set/verification.md`). Round 2 (last) on `138dada5c`: reviewer-a BLOCK, reviewer-b CONCERNS; CRITICAL inventory UID fixed, CRITICAL Secret-repointed destination rejected as scope (decision 2026-10-19 `rotation-zero-interval`), WARNINGs fixed (`reviews/R-spec-set/round-2/verification.md`). No CRITICAL remains
- [x] L task loop — see *Tasks* (T0.1, G1-G4 closed; 5.1 is the integrator's)
- [x] B branch review — `reviews/B-branch/round-N/`. Round 1 on `df2c9e501` (rebased on `origin/main` @ `cf448c4a6`): reviewer-a (adversarial) REQUEST_CHANGES, reviewer-b (spec) CONCERNS; two WARNINGs fixed (B1 status-write loss, breaking footer), spec and FR-EXP-9 wording fixed, three NOTEs rejected or deferred (`reviews/B-branch/round-1/verification.md`). Round 2 (last) on `5213abae0`: reviewer-a APPROVE, reviewer-b CLEAN, NOTEs only; two wording fixes after round 2 (design.md, upgrade guide: re-export per failed status write), not reviewed again; per-commit gate green on all 11 commits (`reviews/B-branch/round-2/verification.md`)
- [x] Hand-off — `pr-description.md` · PR opened by the integrator (5.1)

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

Holistic run at B: `task check` on `df2c9e501` exit 1, four gates red, none from this branch (vulncheck: Go standard library advisory, red on `main`; gitleaks-detect: git-ignored mkdocs cache, clean without it; two dist guards: no `operator-sdk`, green after `make operator-sdk`); `go vet -tags=integration ./...` exit 0; per-commit gate: 10 commits build and pass the changed packages (`reviews/B-branch/round-1/verification.md`).

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
| B1 | Status write lost after an export: next pass debounces against an unpersisted baseline | WARNING | reviewer-a | fix | B1.1/B1.2, SDC-5 | no |
| B1 | `BREAKING CHANGE:` footer omits the read API and per-sink message changes | WARNING | reviewer-a | fix | G1 commit footer reworded | no |
| B1 | SDC-3 lacks the pre-export carve-out; FR-EXP-9 doc hit; SDC-7 section name | WARNING/NOTE | reviewer-b | fix | spec, `docs/REQUIREMENTS.md` | no |
| B1 | Aggregate denominator; entry without condition | NOTE | reviewer-b | reject | unreachable today | no |
| B1 | Read API rebuilds the sink key by hand | NOTE | reviewer-b | defer | SINK-KEY-SHARED-HELPER (pull request) | no |
| R1 | SDC-5 self-contradiction; impossible mixed scenario; second precision; missed message tests; missed docs; matrix breadth; citations | WARNING/NOTE | both | fix | spec, tasks, proposal (`reviews/R-spec-set/verification.md`) | no |

## Tasks

| Task | Verdict | Review (legs, rounds, paid leg?) | Gaps / decision request |
|---|---|---|---|
| T0.1 · CLOSED (after spec fix) · probe: every 0.1 claim holds on `origin/main`; finding: `TestKollectInventoryReconciler_previewAllSinksDebounced_allDebounced` turns red under SDC-4's preview rule and is not in *Impact* | CLOSED | reviewer-a (diff), 1 round, no paid leg | finding 8 resolved by the orchestrator: option A applied to proposal *Impact* and tasks.md 3.1 (only option consistent with SDC-4; see *Triage* L) |
| G1 · CLOSED · 1.1 + 1.2: debounced sink `True/Debounced`, aggregate `True/Exported` "N/N sink(s) up to date", cluster `ExportSucceeded` count, read API `<family>/<name>` + reason-first + own time; M1-M3 killed, M0 survived | CLOSED | reviewer-a (diff), 1 round, APPROVE with one P2 (evidence, fixed); no paid leg | none; race `-count=3` unit reds are pre-existing on base (`evidence/G1.md` *Iterations* 5) |
| G2 · CLOSED · 2.1 + 2.2: `carryOverSinkExport` seeds time, checksum, paths and conditions (deep copy) at the three call sites; both total-failure branches store the per-sink entries; 10 envtest specs + unit; M1, M2, M2t, M3a, M3b killed, M0 survived | CLOSED | reviewer-a (diff), 1 round, REQUEST_CHANGES with one P2 (test assertion, fixed); no paid leg | none; race `-count=3` unit red is the pre-existing breaker flake (`evidence/G2.md` *Iterations* 5) |
| G3 · CLOSED · 3.1 + 3.2: debounce identity (inventory UID, sink UID, sink generation) in `shouldSkip`/`record`, `ResolvedSink.Generation`, `forget` via `failSinkExport` on all six failure branches, preview never debounces an unresolvable sink; 6 unit + 8 envtest specs (2 controls); M1-M5 killed, M0 survived | CLOSED | reviewer-a (diff), 1 round, APPROVE, no findings; no paid leg | none; race `-count=3` unit red is the pre-existing breaker flake (base archive red 4 of 6, `evidence/G3.md` *Iterations* 4) |
| B1 · CLOSED · status write lost after an export: `forgetUnpersistedExports` on a failed `updateStatus` write, `markExported` at both record sites; 2 unit tests with controls; M1-M5 killed, M0 survived | CLOSED | stage B round 2 (both legs), no findings above NOTE | none |
| G4 · CLOSED · 4.1: conditions reference, troubleshooting (condition, per-sink, debounce and partial-failure rows, read API line), metrics note, both CRD pages, ADR-0413 §6 amendment note, upgrade-guide entry with old/new messages and what alerts and scripts must change; per-line `rg` check lists only the three allowed lines; markdown `0 issues`; 14 docs guards green | CLOSED | reviewer-a (diff), 1 round, REQUEST_CHANGES with two P2 wording findings (fixed, no round 2); no paid leg | none; `task docs:verify` stops on the local kubectl mise shim (environmental), remaining steps incl. `mkdocs build --strict` run by hand, green (`evidence/G4.md` row 6) |

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
| `internal/controller/per_sink_export_test.go:TestCarryOverLastExportPaths` → `TestCarryOverSinkExport` | before this change | G2 | SDC-5: the seed carries time, checksum, paths and conditions (deep copy); the "fresh paths win" case is moot because the seed runs on an empty entry first |
| Mechanical identity argument in 35 `shouldSkip`/`record` call lines of six test files (list in `evidence/G3.md` *Test changes*) | before this change | G3 | SDC-4 and design *When a sink counts as up to date*: the identity is one struct argument; `sinkExportIdentity{}` on both sides keeps every assertion |
| `internal/controller/kollectinventory_debounce_test.go:TestKollectInventoryReconciler_previewAllSinksDebounced_allDebounced` | before this change | G3 | SDC-4 (no unresolvable sink is debounced, also on the preview); T0.1 finding 8: fixture gains the two `KollectDatabaseSink` objects and records against their identity, assertion unchanged |

## Lessons

- T0.1: when a design removes a fall-back, grep the tests for fixtures that rely on it; a scratch mutant of just that rule over the whole package finds them in one run.
- G1: `go test -race -count=3 ./internal/controller/` is red on base about one run in three (process-wide circuit breaker for `default/postgres-demo` across repetitions); diff against a `git archive <base>` copy before calling it a regression. Ginkgo refuses `-count>1`: run the unit tests with `-skip TestControllers` and the suite once with `-race`.
- G1: envtest helpers that give each sink its own table let the registry selector route sinks to separate recording backends, so a spec can count exports per sink.
- T0.1: throwaway envtest specs run unchanged in a `git archive origin/main | tar -x` scratch tree with `KUBEBUILDER_ASSETS` set (the relative `bin/k8s` resolver path does not exist there).
- G2: assert a multi-pass transition rule as a map of named booleans (`HaveEach(BeTrue())`): one red line then shows every pass that broke, not only the first.
- G2: a shallow-copy mutant survives envtest when the previous entries come from a fresh `Get`; only a unit test that mutates the seeded entry and re-reads the source catches it.
- G3: a missing sink stops the namespaced reconcile at the reachability check, before the preview, so no envtest reaches the preview's resolve-error branch; its unit test (empty fake client) is the only sensor.
- G3: for a new argument the stub ignores, a zero value on both sides of every existing call is the mechanical edit; one regex substitution over the test files plus a green suite proves it.
- G3: the race `-count=3` breaker flake hit the base archive 4 of 6 runs (G1: 1 of 3, G2: 0 of 3); sample at least six base runs before calling a single red run noise.
- G4: the per-line debounce check also catches a cell that says a debounce never causes `PartiallySynced`; keep debounce wording out of any line or table row that names `Synced=False` or `PartiallySynced`, and explain it in a separate paragraph.
- B: a status write that fails after a successful export leaves the in-memory debounce baseline ahead of the persisted entry; drop the baselines on a failed write (round 1, reviewer-a).
- B: per-commit gate concurrently with `task check` gave one load-induced red; rerun a red commit alone before treating it as real.
- G4: `task docs:verify` needs kubectl through mise (`MISE_KUBECTL_VERSION=1.36.1`) and `mkdocs build --strict` needs the `requirements-docs.txt` plugins (scratch venv outside the repo); without them run the remaining steps by hand.

## Where a human should look first

- *Test changes* table above (six rows; they change what green means)
- `internal/controller/per_sink_export.go` — `shouldSkip` identity, `failSinkExport`, `forgetUnpersistedExports`
- `internal/controller/kollectinventory_controller.go`, `kollectclusterinventory_controller.go` — every failure branch and `updateStatus`
- `internal/inventory/status.go` — read API key and reason mapping
- `docs/operator-manual/upgrading.md` — breaking-change note

## Proposed harness changes

| Lesson | Seen in | Proposed change (AGENTS.md line / sensor / close-task.yml) |
|---|---|---|
| Race `-count=3` on `internal/controller` flakes on base: shared circuit breaker for `default/postgres-demo` | G1 | backlog RACE-COUNT-BREAKER-ISOLATION: reset or isolate the breaker per test in the four reconciler tests (`kollectinventory_export_test.go`, `kollectinventory_finalizer_test.go`) |

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
