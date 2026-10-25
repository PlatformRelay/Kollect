# PR description — synced-debounce-condition

- **Source:** `feat/synced-debounce-condition`
- **Target:** `main`
- **Title:** `:bug: fix(controller)!: report debounced sinks as synced`
- **Opens as:** **ready**. The operator authorised a ready PR and a rebase merge once every check
  on the head is green.

Paste everything below the line.

---

## TL;DR

- **What:** a debounced sink now counts as up to date. Its per-sink condition is
  `Synced=True`, reason `Debounced`, and the aggregate `Synced` of both inventory kinds is
  `True/Exported` "N/N sink(s) up to date" whenever no sink failed.
- **Why:** backlog SYNCED-DEBOUNCE-STEADY-STATE. A healthy inventory read
  `Synced=False reason=PartiallySynced "1 sink(s) debounced"` most of the time, so alerts on
  `Synced=False` fired on healthy inventories.
- **Risk:** breaking for alerts and scripts that read these conditions or match their messages
  (upgrade note in `docs/operator-manual/upgrading.md`). A debounce is now trusted only while
  the export it rests on is still current, so some cases that used to wait out the interval now
  re-export at once.
- **Reviewer action:** read `internal/controller/per_sink_export.go` and the failure branches of
  both reconcilers. Approving accepts the v0.22.0 upgrade note.

## What changes

- **Conditions (SDC-1, SDC-2, SDC-3).** Debounced sink: `True/Debounced`, and the message names
  the interval with no countdown. Aggregate: `True/Exported` "N/N sink(s) up to date" when no
  sink failed. `PartiallySynced` now means only "some sinks exported, some failed". When every
  sink fails, the per-sink entries are written too (`False/ExportFailed` or `SpillRequired`). The
  cluster `ExportSucceeded` message counts debounced sinks.
- **When a debounce is trusted (SDC-4).** The debounce baseline now carries the inventory UID,
  the sink UID and the sink generation. Each of these re-exports on the next reconcile: a sink
  spec edit, a deleted and recreated sink or inventory, a failed export (the baseline is dropped
  on every failure branch), and a status write that fails after a successful export. The preview
  path never debounces a sink it cannot resolve.
- **No churn (SDC-5).** A sink's entry keeps `lastExportTime`, `lastChecksum`, paths and
  conditions across debounced and failed passes. Repeated debounced reconciles write no status
  and emit no Event.
- **Read API (SDC-6).** Entries are matched by `<family>/<name>`. The `Debounced` reason maps to
  `debounced`, and each sink reports its own `lastExportTime`, which is empty when it never
  exported.
- **Docs (SDC-7).** Conditions reference, troubleshooting, metrics, both CRD pages, an amendment
  note on ADR-0413 §6, and the upgrade-guide entry with the old and new messages.

## Risk

- Alerts on `Synced=False` or `PartiallySynced` stop firing for debounces. Scripts that match
  `sink(s) debounced`, `next export in`, `exported to N sink(s)` or
  `exported N item(s) to N sink(s)` must change. The commit `fix(controller)!: report debounced
  sinks as synced` carries the `BREAKING CHANGE:` footer.
- After a failed status write, the next reconcile exports the same payload again, once per
  failed write. Event sinks (Kafka, NATS) receive it again. This is documented in the upgrade
  guide.

## Files to check first

1. `internal/controller/per_sink_export.go` — `shouldSkip` identity, `failSinkExport`,
   `carryOverSinkExport`, `forgetUnpersistedExports`
2. `internal/controller/kollectinventory_controller.go`,
   `internal/controller/kollectclusterinventory_controller.go` — every failure branch and
   `updateStatus`
3. `internal/inventory/status.go` — read API key and reason mapping
4. The *Test changes* table in `openspec/changes/synced-debounce-condition/loop.md` (six rows,
   each citing the SDC requirement that changes what green means)

Review commit by commit. Each commit builds, and each passes the changed packages' tests on its
own (per-commit gate in `reviews/B-branch/round-2/verification.md`).

## Testing

- `task check` on the reviewed tree. Every gate is green except four, none caused by this
  branch: `vulncheck` (Go advisory, fixed on `main` by the Go 1.27.3 bump, now green locally);
  `gitleaks-detect` (a git-ignored mkdocs cache; clean without it); two dist guards (no
  `operator-sdk` on the host; green after `make operator-sdk`).
- `go vet -tags=integration ./...` clean. Docs verify green (run by hand with a fresh mkdocs
  venv).
- Each behaviour fix had a recorded red test first and was mutation-proved. G1 M1-M3, G2 M1-M3b,
  G3 M1-M5 and B1 M1-M5 were all killed, and every no-op control survived.
- `go test -race -count=3` on the changed packages: no data race. The only failures are the four
  reconciler tests of the known shared-breaker flake, which is also red on the base (follow-up
  below).

## Reviews

- Spec set: two rounds, two independent legs each. The CRITICALs were fixed, and one was
  rejected as out of scope (Secret repointing).
- Per task: one leg each (T0.1, G1-G4). Findings were fixed.
- Branch: round 1 returned REQUEST_CHANGES and CONCERNS. The two WARNINGs were fixed: the status
  write lost after an export, and the breaking footer. Round 2 returned APPROVE and CLEAN, with
  notes only. The wording fixes made after round 2 are in `design.md` and the upgrade guide.

## Follow-ups (backlog)

- **SHARD-WARN-EVENT-CHURN.** An inventory at or above the export-shard warning threshold
  (`validation.ExportShardWarnRows`, `internal/controller/export_spill.go`) emits the shard
  warning Event on every pass, debounced or not, so SDC-5's no-Event rule holds only below the
  threshold. Emit it on a change of the shard count or once per interval.
- **RACE-COUNT-BREAKER-ISOLATION.** `go test -race -count=3 ./internal/controller/` is red on the
  base in some runs. Four reconciler tests (`kollectinventory_export_test.go`,
  `kollectinventory_finalizer_test.go`) share the process-wide circuit breaker for
  `default/postgres-demo` across repetitions. Reset or isolate the breaker per test.
- **SINK-KEY-SHARED-HELPER.** The read API (`internal/inventory/status.go`) builds the
  `<family>/<name>` entry key by hand, and the reconcilers use `sinkExportKey`
  (`internal/controller/inventory_sink_helpers.go`). If one side changes alone, every binding
  falls back to the inventory-level status without a sound. Move one key builder next to
  `InventorySinkBinding` in `api/v1alpha1` and use it on both sides.

Closes no GitHub issue; the work is tracked in the backlog item.
