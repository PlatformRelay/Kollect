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
  (`internal/controller/per_sink_export.go:59-99`). It returns true only when the sink's last
  recorded export has the same inventory generation, payload checksum and
  `kollect.dev/requestedAt` value, and the effective export interval (`exportMinInterval`,
  resolved per ref by `validation.ResolveSinkExportInterval`) has not elapsed, or the interval is
  `0s`. `record` (`:101-122`) runs only after a successful export
  (`internal/controller/kollectinventory_controller.go:446`, `kollectclusterinventory_controller.go:365`).
  The interval debounces identical payloads only and never holds back a changed one
  ([ADR-0413](../../../docs/adr/0413-export-interval-scheduling.md) §2), so there is no
  "held-back change" state to report.
- The tracker does not know which sink object it exported to: a sink spec change (which bumps the
  sink's generation, not the inventory's) or a deleted and recreated sink leaves the baseline in
  place, and a payload that returns to the last exported checksum after a failed export is skipped
  as unchanged. Today those cases read `Synced=False`, so they are hidden; once a debounce reads
  `Synced=True` they would claim an up-to-date sink that is not (SDC-4).
- A debounced sink gets a per-sink `Synced=False reason=Debounced` condition with the message
  "next export in 13s (interval 15s, checksum unchanged)"
  (`kollectinventory_controller.go:363-371`, preview `:498-506`;
  `kollectclusterinventory_controller.go:312-320`). The countdown changes the message on most
  reconciles.
- Every export pass rebuilds `status.sinkExports[]` from empty entries (`outcome.SinkExports` is
  made fresh at `kollectinventory_controller.go:300`, cluster path likewise) and carries over only
  `lastExportPaths` (`carryOverLastExportPaths`, `per_sink_export.go:272-298`). So a pass that
  debounces a sink, or fails it while another sink exports, erases the sink's `lastExportTime`
  and `lastChecksum` (documented as "last successful export to this sink",
  `docs/crds/kollectinventory.md:160`), and recreates its `Synced` condition with
  `lastTransitionTime` = now on every pass.
- The total-failure path does not write the per-sink entries at all: `applyInventoryExportOutcome`
  calls `setInventoryDegraded` (`kollectinventory_controller.go:251-270`, `:627-659`; cluster
  `kollectclusterinventory_controller.go:211-231`, `setDegraded` `:690`), so a failed sink keeps the `Synced=True`
  condition of its last success.
- `aggregateInventorySync` (`per_sink_export.go:336-358`) maps "any sink debounced, none failed"
  to `Synced=False reason=PartiallySynced`; both reconcilers call it
  (`kollectinventory_controller.go:558`, `kollectclusterinventory_controller.go:744`).
- The 15 s cadence is the inventory's own `exportMinInterval`: after an export the reconciler
  requeues after the interval (`validation.RequeueAfterForZeroInterval`,
  `internal/validation/export_interval.go:246-251`), the skip has expired by then, and the
  unchanged payload is exported again (`Synced=True reason=Exported`). Reconciles in between (the
  inventory's own status update re-enqueues it, because `For(&KollectInventory{})` has no
  predicate, `kollectinventory_controller.go:696-697`, plus store and sink watches) see the sink
  debounced and set `Synced=False` again. So the aggregate flips `True`/`False` once per interval
  and `lastTransitionTime` moves each time. Believed from the code; probe 0.1 confirms the two
  states in envtest.
- The read API (`internal/inventory/status.go:159-182`) looks up the per-sink entry by the bare
  sink name, but the reconcilers name entries `<family>/<name>` (`sinkExportKey`,
  `internal/controller/inventory_sink_helpers.go:81-83`). The per-sink override therefore never
  fires on a real inventory; the read API reports the aggregate status for every sink (today
  `degraded` during a debounce), and its test uses bare names (`internal/inventory/cov_s06_test.go`).

`Ready` is not affected by a debounce: with no failure it is `True/Exported`
(`kollectinventory_controller.go:563-577`). The troubleshooting row that says
`Ready=True, reason PartiallySynced` for a debounce (`docs/operator-manual/troubleshooting.md:222`)
does not match the code.

## What Changes

- SDC-1: a debounced sink reports its per-sink condition as `Synced=True reason=Debounced`, with a
  message that does not change between reconciles while the sink stays debounced.
- SDC-2: the aggregate `Synced` condition of both inventory kinds is `True reason=Exported` after
  an export pass in which no bound sink failed, whether the sinks exported or were debounced, with
  one message for both ("N/N sink(s) up to date"). `KollectClusterInventory`'s `ExportSucceeded`
  message uses the same count. Outcome of a model-aligned decision (2026-10-20, option A, pending
  operator validation).
- SDC-3: failures keep `Synced=False` with today's aggregate reasons; `PartiallySynced` then
  means only "some sinks failed". The total-failure path now writes the per-sink entries too, so a
  failed sink reads `Synced=False` even when no other sink exported.
- SDC-4: a sink is debounced only when it is known to be up to date: the baseline also records the
  family sink's UID and `metadata.generation`, a failed export clears the baseline, and an
  unresolvable sink is never counted as debounced (also in the preview path).
- SDC-5: no condition churn: a debounced reconcile after a debounced reconcile writes no status
  and emits no Event; the aggregate is identical across export and debounce passes; per-sink
  entries keep `lastExportTime`, `lastChecksum` and their condition's `lastTransitionTime` across
  passes while the condition's status holds.
- SDC-6: the read API matches entries by `<family>/<name>` and reports a debounced sink as
  `debounced`, with that sink's own `lastExportTime`.
- SDC-7: docs match (`docs/reference/conditions.md`, `docs/operator-manual/troubleshooting.md`,
  `docs/operator-manual/metrics.md`, `docs/crds/kollectinventory.md`,
  `docs/crds/kollectclusterinventory.md`, the `ReasonPartiallySynced` comment in
  `api/v1alpha1/export_interval_types.go:16`), an amendment note on ADR-0413 §6, and an
  upgrade-guide entry under *Smaller behaviour changes*, because users may alert on
  `Synced=False` or `PartiallySynced`.

## Capabilities

### New Capabilities

- `inventory-sync-status`: what the per-sink and aggregate `Synced` conditions of an inventory
  report for exported, debounced and failed sinks, when a sink counts as up to date, and how the
  read API reports them.

### Modified Capabilities

None. ADR-0413 §2 (the interval debounces identical payloads only) is unchanged and is what makes
SDC-1 sound.

## Impact

- Entry points: the `KollectInventory` and `KollectClusterInventory` reconcilers
  (`internal/controller/kollectinventory_controller.go`,
  `internal/controller/kollectclusterinventory_controller.go`) and the read API status endpoint
  (`internal/inventory/status.go`).
- Code: `internal/controller/per_sink_export.go` (aggregate, per-sink helper, carry-over, tracker
  identity and reset), the debounce and failure branches of both reconcilers,
  `internal/sink/resolver.go` (`ResolvedSink` gains the sink's `metadata.generation`),
  `internal/inventory/status.go`. No CRD schema, RBAC, metric or dependency change.
  `kollect_export_debounced_total` keeps counting skips. No printer column reads `Synced`
  (`api/v1alpha1/*inventory_types.go` have none). No e2e script asserts a `Synced` reason
  (`rg Synced hack/kind` is empty).
- Behaviour change, release-noted for v0.22.0 (SDC-7): `Synced=False` and `PartiallySynced` no
  longer fire for a debounce; per-sink `Debounced` becomes `True`; the aggregate messages change;
  a sink spec change or replacement re-exports an unchanged payload once; the read API reports
  per-sink status.
- Tests that encode today's behaviour and change (each a *Test changes* row):
  `TestAggregateInventorySync` (`internal/controller/per_sink_export_test.go:86-123`); the two
  message assertions `"exported to 1 sink(s)"` in `internal/controller/requested_at_annotation_test.go:257`
  and `:402`; `TestExportStatusFromInventory_perSinkConditionsOverrideInventoryStatus`
  (`internal/inventory/cov_s06_test.go`, entry names become `<family>/<name>`). Tests that assert
  only the `Debounced` reason stay green: `kollectinventory_dual_sink_test.go:121`,
  `kollectinventory_per_sink_envtest_test.go:198`, `kollectclusterinventory_export_envtest_test.go:257`.

## Non-goals

- `Ready` and `Degraded` semantics. A mix of debounced and failed sinks stays `Ready=False` (the
  fix "clear cluster Ready when every export fails", `docs/operator-manual/upgrading.md`
  *Smaller behaviour changes*; namespaced inventory in #387).
- Per-sink entries on paths that stop before an export pass (scope denial, unreachable or missing
  sink, spill gate, registry missing): they keep what the last export pass wrote, and the
  aggregate `Synced=False` with the blocking reason reports the problem. Only export passes write
  per-sink entries.
- Rate-limiting or delaying changed payloads. A later change that holds a changed payload back
  must report it with its own reason (design *Future pending state*); this change adds none,
  because no such state exists.
- The `ExportSucceeded` Normal Event on a periodic re-export of an unchanged payload. That pass
  does export (the Git sink then skips an empty commit), so the Event stays; the recorder
  aggregates repeats.
- The `ExportShardWarn` Warning, which the namespaced full export path records on every pass for
  inventories at or above `validation.ExportShardWarnRows` (`internal/controller/export_spill.go:87`,
  caller `kollectinventory_controller.go:225-229`). SDC-5's no-Event clause is scoped below that
  threshold; follow-up backlog item SHARD-WARN-EVENT-CHURN.
- The per-sink reason alternating `Exported`/`Debounced` once per interval. The status stays
  `True`, so `lastTransitionTime` does not move.
- A new condition type or aggregate reason (decision options B and D, rejected).
- The preview path comparing snapshot sinks' partition checksum with the raw fingerprint
  (`previewAllSinksDebounced`, `kollectinventory_controller.go:463-519`), which only costs a
  fall-through to the full path.

## Assumptions

- `apimeta.SetStatusCondition` keeps `lastTransitionTime` when the status does not change and
  replaces reason and message in place (k8s.io/apimachinery `pkg/api/meta/conditions.go`). Probe
  0.1 cites the pinned version.
- The API server does not persist a status update whose object is unchanged, so
  `resourceVersion` stays and no watch event fires. Kubernetes API behaviour; probe 0.1 checks it
  in envtest.
- Family sinks have the status subresource (`+kubebuilder:subresource:status` on all three kinds,
  `api/v1alpha1/kollect*sink_types.go`), so a spec change bumps `metadata.generation` and a status
  write does not.
- The steady-state flip described above. Probe 0.1 reproduces both states in envtest on
  `origin/main`.
