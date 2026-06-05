# ADR-0413: Per-sink export interval scheduling

> Inventory owns per-ref export cadence; KollectSink may publish a default; KollectScope sets a
> tenancy floor. Debounce state is per (inventory, sink).

**Theme:** 04 · Export & sinks · **Status:** Current

## Context

`KollectInventory` and `KollectClusterInventory` previously exposed a single
`spec.exportMinInterval` (default **30s**). Every reconcile marshalled one snapshot and exported to
**all** `sinkRefs` on the same debounce tick. That matches FR-EXP-2 when all sinks share the same
freshness trade-off, but breaks down for multi-role fan-out (Postgres portal at 30s + Git audit at
1h + Kafka on material change only) — see [KOLLECTSINK-INTERVAL-PROPOSAL.md] (local).

## Decision

### 1. Structured `spec.sinkRefs[]`

`sinkRefs` accepts **plain strings** (backward compatible) or objects:

```yaml
spec:
  exportMinInterval: 30s
  sinkRefs:
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

- **Material checksum change** bypasses interval **per sink** (FR-EXP-2 spirit).
- **Spec generation bump** bypasses debounce for that sink (force refresh after spec edit).
- **`exportMinInterval: 0s`** — no periodic re-export of identical payload; controller requeues
  with a **30s watchdog** (`ZeroIntervalWatchdog`).

### 3. Optional `KollectSink.spec.exportMinInterval`

Shared platform sinks may publish a default when the inventory ref omits an override. Sink remains
static config — interval is read at export time, not reconciled on the Sink CR.

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

- [ADR-0401](0401-sink-taxonomy-state-vs-stream.md) · [ADR-0703](0703-platform-architecture-pivot.md) §10
- [DATA-FLOWS.md](../DATA-FLOWS.md#1-export-debouncing) · [kollectinventory.md](../crds/kollectinventory.md)
