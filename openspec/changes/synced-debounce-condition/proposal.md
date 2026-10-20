# Proposal

## Why

A healthy `KollectInventory` reports `Synced=False` most of the time (backlog
SYNCED-DEBOUNCE-STEADY-STATE, kind quick start 2026-10-20, on 0.21.0 and on `main`). With one Git
sink and `exportMinInterval: 15s` (`config/samples/demo/git-only/inventory.yaml:12`,
`hack/demo/hero/kollect-demo.yaml:57`) the inventory showed
`Synced=False reason=PartiallySynced "1 sink(s) debounced"` for more than a minute, re-asserted
every 15 s with `checksum unchanged`. `docs/reference/conditions.md:10` defines `Synced` as "the
latest observed inventory was exported successfully", which was true the whole time. A first-time
user, or an alert on `Synced=False`, reads it as a failure.

Where it comes from (read 2026-10-20 on `origin/main` `0686f8276`):

- "Debounced" is the per-sink coalesce skip, `perSinkCoalesceTracker.shouldSkip`
  (`internal/controller/per_sink_export.go:59-97`). It returns true only when the sink's last
  recorded export has the same generation, payload checksum and `kollect.dev/requestedAt` value,
  and the effective export interval (`exportMinInterval`, resolved per ref by
  `validation.ResolveSinkExportInterval`) has not elapsed, or the interval is `0s`. `record`
  (`:99-120`) runs only after a successful export (`internal/controller/kollectinventory_controller.go:446`, `kollectclusterinventory_controller.go:365`).
  So a debounced sink has, by construction, no pending change: the interval debounces identical
  payloads only and never holds back a changed one ([ADR-0413](../../../docs/adr/0413-export-interval-scheduling.md) §2).
  There is no "held-back change" state to report today.
- Every pass rebuilds `status.sinkExports[]` from empty entries (`outcome.SinkExports` is made
  fresh at `kollectinventory_controller.go:300`, cluster path likewise) and carries over only
  `lastExportPaths` (`carryOverLastExportPaths`, `per_sink_export.go:272-298`). So a debounced
  or failed pass erases the sink's `lastExportTime` and `lastChecksum` (documented as "last
  successful export to this sink", `docs/crds/kollectinventory.md:160`), and its per-sink
  `Synced` condition is created anew with `lastTransitionTime` = now on every pass. Believed from
  the code; probe 0.1 confirms it.
- A debounced sink gets a per-sink `Synced=False reason=Debounced` condition with the message
  "next export in 13s (interval 15s, checksum unchanged)"
  (`kollectinventory_controller.go:363-371`, preview `:498-506`;
  `kollectclusterinventory_controller.go:312-320`). The countdown changes the message on most
  reconciles, so status is rewritten even when nothing changed.
- `aggregateInventorySync` (`internal/controller/per_sink_export.go:336-358`) maps "any sink
  debounced, none failed" to `Synced=False reason=PartiallySynced`; both reconcilers call it
  (`kollectinventory_controller.go:558`, `kollectclusterinventory_controller.go:744`).
- The 15 s cadence is the inventory's own `exportMinInterval`: after an export the reconciler
  requeues after the interval (`validation.RequeueAfterForZeroInterval`,
  `internal/validation/export_interval.go:246-251`), the coalesce skip has expired by then, and
  the unchanged payload is exported again (`Synced=True reason=Exported`). Reconciles in between
  (the inventory's own status update re-enqueues it, because `For(&KollectInventory{})` has no
  predicate, `kollectinventory_controller.go:696-697`, plus store and sink watches) see the sink
  debounced and set `Synced=False` again. So the aggregate flips `True`/`False` once per interval
  and `lastTransitionTime` moves each time. Believed from the code; probe 0.1 confirms it in
  envtest.

`Ready` is not affected: with no failure it is `True/Exported`
(`kollectinventory_controller.go:563-577`). The troubleshooting row that says
`Ready=True, reason PartiallySynced` for a debounce (`docs/operator-manual/troubleshooting.md:222`)
does not match the code: `Ready` gets `PartiallySynced` only when some sinks exported and some
failed.

## What Changes

- SDC-1: a debounced sink (payload unchanged since its last successful export, interval not due)
  reports its per-sink condition as `Synced=True reason=Debounced`, with a message that does not
  change between reconciles while the sink stays debounced (no countdown).
- SDC-2: the aggregate `Synced` condition of `KollectInventory` and `KollectClusterInventory` is
  `True reason=Exported` whenever no bound sink failed, whether the sinks exported in this pass or
  were debounced, with one message for both ("N/N sink(s) up to date"). `KollectClusterInventory`'s
  `ExportSucceeded` message uses the same up-to-date count instead of "to 0 sink(s)" on a pass
  where every sink was debounced. Outcome of a model-aligned decision (2026-10-20, option A).
- SDC-3: failures keep `Synced=False`. Any failed sink keeps today's aggregate reasons
  (`PartiallySynced` when another sink exported, `Progressing` or the terminal or spill reason
  otherwise) and the failed sink's per-sink `Synced=False`. `PartiallySynced` then means only
  "some sinks failed".
- SDC-4: a sink with a pending change is never reported as debounced: a changed checksum,
  generation or `requestedAt` value exports in the same reconcile. A failed export clears that
  sink's debounce baseline, so a payload that returns to the last exported checksum after a
  failed export is exported again instead of being reported up to date.
- SDC-5: no condition churn: while nothing changes, a reconcile in which every sink is debounced
  writes no status (the object's `resourceVersion` stays) and emits no Event; the aggregate
  `Synced` condition (status, reason, message, `lastTransitionTime`) is identical before and
  after a periodic re-export of an unchanged payload. A sink's `status.sinkExports[]` entry keeps
  `lastExportTime`, `lastChecksum` and its condition's `lastTransitionTime` across debounced and
  failed passes; only a successful export to it replaces them.
- SDC-6: the read API keeps reporting a debounced sink as `debounced`
  (`internal/inventory/status.go:167-180` maps the reason, now with status `True`).
- SDC-7: docs match: `docs/reference/conditions.md`, `docs/operator-manual/troubleshooting.md`
  (condition table, per-sink table, debounce rows), `docs/crds/kollectinventory.md`,
  `docs/crds/kollectclusterinventory.md`, an amendment note on ADR-0413 §6, and an upgrade-guide
  entry under *Smaller behaviour changes*, because users may alert on `Synced=False` or on the
  `PartiallySynced` reason.

## Capabilities

### New Capabilities

- `inventory-sync-status`: what the per-sink and aggregate `Synced` conditions of an inventory
  report for exported, debounced and failed sinks, and when they change.

### Modified Capabilities

None. ADR-0413 §2 (interval debounces identical payloads only) is unchanged and is what makes
SDC-1 sound.

## Impact

- Entry points: the `KollectInventory` and `KollectClusterInventory` reconcilers
  (`internal/controller/kollectinventory_controller.go`,
  `internal/controller/kollectclusterinventory_controller.go`) through
  `aggregateInventorySync` and the per-sink condition writes; the read API status endpoint
  (`internal/inventory/status.go`).
- Code: `internal/controller/per_sink_export.go` (aggregate, per-sink helper, tracker baseline
  reset), the debounce branches of both reconcilers, `internal/inventory/status.go`. No CRD
  schema, RBAC, metric or dependency change. `kollect_export_debounced_total` keeps counting
  skips. No printer column reads `Synced` (`api/v1alpha1/*inventory_types.go` have none). No
  e2e script asserts a `Synced` reason (`rg Synced hack/kind` is empty).
- Behaviour change, release-noted for v0.22.0 (SDC-7): `Synced=False` and `PartiallySynced` no
  longer fire for a debounce; the per-sink `Debounced` condition becomes `True`.
- Tests that encode today's behaviour and change: `TestAggregateInventorySync`
  (`internal/controller/per_sink_export_test.go:86-123`, rows "all debounced" and "partial
  debounce"); `TestExportStatusFromInventory_perSinkConditionsOverrideInventoryStatus`
  (`internal/inventory/cov_s06_test.go`, gains a `True`+`Debounced` row). Tests that assert only
  the `Debounced` reason stay green: `kollectinventory_dual_sink_test.go:121`,
  `kollectinventory_per_sink_envtest_test.go:198`, `kollectclusterinventory_export_envtest_test.go:257`.

## Non-goals

- `Ready` and `Degraded` semantics. A mix of debounced and failed sinks stays `Ready=False`
  (the deliberate fix "clear cluster Ready when every export fails", `docs/operator-manual/upgrading.md`
  *Smaller behaviour changes*; namespaced inventory in #387).
- Rate-limiting or delaying changed payloads. Should a later change hold back a changed payload,
  it must report that state with its own reason (design *Future pending state*); this change adds
  none, because no such state exists.
- The `ExportSucceeded` Normal Event on a periodic re-export of an unchanged payload. That pass
  does export (the Git sink then skips an empty commit), so the Event stays; the recorder
  aggregates repeats.
- The per-sink reason alternating `Exported`/`Debounced` once per interval. The status stays
  `True`, so `lastTransitionTime` does not move; the reason tells a reader which pass ran.
- A new condition type or aggregate reason (decision options B and D, rejected).
- The preview path comparing snapshot sinks' partition checksum with the raw fingerprint
  (`previewAllSinksDebounced`, `kollectinventory_controller.go:463-517`), which only costs a
  fall-through to the full path.

## Assumptions

- `apimeta.SetStatusCondition` keeps `lastTransitionTime` when the status does not change and
  replaces reason and message in place (k8s.io/apimachinery `pkg/api/meta/conditions.go`). Probe
  0.1 cites the pinned version.
- The API server does not persist a status update whose object is unchanged, so
  `resourceVersion` stays and no watch event fires. Kubernetes API behaviour; probe 0.1 checks it
  in envtest.
- The steady-state flip described above (export pass `True`, debounced pass `False`). Probe 0.1
  reproduces it in envtest on `origin/main`.
