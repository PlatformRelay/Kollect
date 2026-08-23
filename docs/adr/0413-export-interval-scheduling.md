# ADR-0413: Per-sink export interval scheduling

> Inventory owns per-ref export cadence; a family sink may publish a default; KollectScope sets a
> tenancy floor. Debounce state is per (inventory, sink).

**Theme:** 04 · Export & sinks · **Status:** Current

## Context

`KollectInventory` and `KollectClusterInventory` previously exposed a single
`spec.exportMinInterval` (default **30s**). Every reconcile marshalled one snapshot and exported to
**all** `sinkRefs` on the same debounce tick (that single list was later split per family by
[ADR-0414](0414-sink-family-crds.md)). That matches FR-EXP-2 when all sinks share the same
freshness trade-off, but breaks down for multi-role fan-out (Postgres portal at 30s + Git audit at
1h + Kafka on material change only) — see [KOLLECTSINK-INTERVAL-PROPOSAL.md] (local).

## Decision

### 1. Structured `spec.sinkRefs[]` (superseded by [ADR-0414](0414-sink-family-crds.md))

`sinkRefs` accepts **plain strings** (backward compatible) or objects — superseded per-family by
[ADR-0414](0414-sink-family-crds.md), which kept this entry shape:

```yaml
# kollect-doc: superseded the single sinkRefs list became snapshotSinkRefs / databaseSinkRefs / eventSinkRefs in ADR-0414
# kollect-doc: superseded the single sinkRefs list became snapshotSinkRefs /
# databaseSinkRefs / eventSinkRefs in ADR-0414; the CRDs accept only the object form.
spec:
  exportMinInterval: 30s
  sinkRefs:   # superseded by ADR-0414 — now snapshotSinkRefs / databaseSinkRefs / eventSinkRefs
    - team-postgres
    - name: audit-git
      exportMinInterval: 1h
    - name: events-kafka
      exportMinInterval: 0s   # material-change only
```

List capped at **20** entries; `status.sinkExports[]` mirrors per-sink observation.

### 2. Precedence (effective interval per ref)

```text
effectiveInterval(ref) =
  max(
    ref.exportMinInterval ?? sink.exportMinInterval ?? inventory.exportMinInterval ?? 30s,
    scope.minExportInterval ?? 0s
  )
```

- **Material checksum change** bypasses interval **per sink** (FR-EXP-2 spirit). The interval is a
  debounce for **identical payloads only** — it never delays or rate-limits a changed payload.
- **Spec generation bump** bypasses debounce for that sink (force refresh after spec edit).
- **`exportMinInterval: 0s`** — no periodic re-export of identical payload; controller requeues
  with a **30s watchdog** (`ZeroIntervalWatchdog`). The watchdog refreshes status only, it does not
  re-export.
- **Sub-second intervals** pass validation (any non-negative duration ≤ 24h) but requeue wake-ups
  floor at **1s** (`nextDue`); since material changes bypass the interval, values below `1s` are
  equivalent to `0s` in practice.

### 3. Optional family sink `spec.exportMinInterval`

Shared platform **family sinks** ([ADR-0414](0414-sink-family-crds.md)) may publish a default when
the inventory ref omits an override. The sink remains static config — interval is read at export
time, not reconciled on the sink CR. The unified `KollectSink` kind was removed; interval defaults
live on the family sink specs instead.

### 4. `KollectScope.spec.minExportInterval` floor

Webhook rejects inventory/sink intervals **below** the scope floor at admission. Reconciler clamps
as a backstop via `ResolveSinkExportInterval`.

### 5. Global cap

All duration fields validated ≤ **24h** without cron (`MaxExportInterval`). Cron scheduling deferred
(Phase 3).

### 6. Status and aggregate Synced

- `status.sinkExports[]` — per-sink `lastExportTime`, `lastChecksum`, `Synced` condition.
- `status.lastExportTime` — **max** of per-sink times (backward compatible).
- Aggregate `Synced=False`, reason **`PartiallySynced`** when some sinks debounced, none failed.

## Consequences

- Controller debounce maps keyed by `(inventoryKey, sinkName)`; marshal-once fan-out per reconcile.
- Hub env `KOLLECT_HUB_SINK_REFS` unchanged in this ADR — structured hub intervals deferred.
- Read API `/status` export list prefers per-sink `sinkExports` when present.

## See also

- [ADR-0401](0401-sink-taxonomy-state-vs-stream.md) · [ADR-0201](0201-crd-model.md) §10
- [concepts/export-pipeline.md](../concepts/export-pipeline.md#debounce-and-interval-precedence) · [kollectinventory.md](../crds/kollectinventory.md)
