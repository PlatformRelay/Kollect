# ADR-0305: Aggregation, row identity, and dedupe semantics

> How collected items are keyed, deduplicated across targets, and fingerprinted for debounced export.

**Theme:** 03 · Collection & extraction · **Status:** Current

## Context

Multiple `KollectTarget`s can select overlapping objects, and `KollectClusterInventory` rolls many
targets (and, via the hub, many clusters) into one export. That requires explicit answers to: what is a
row's identity? when do two rows collapse into one? and when can an export be skipped? These rules live
in `internal/aggregate` and were prototyped as a spike inside [ADR-0304](0304-custom-resource-aggregation-rfc.md);
they are now stable enough to record on their own.

## Decision

### Row identity

```
RowIdentity = (targetNamespace, targetName, namespace, name, uid)
```

The stable key for one collected row (`aggregate.RowIdentity`). Hub merge extends it with cluster id
(`internal/hub/merge.go`, [ADR-0501](0501-multi-cluster-sync-rfc.md)). `uid` is the authority — names
recur after recreation, uids do not.

### Dedupe modes

When several targets observe the **same** object, `MergeRows` collapses by mode:

- **`DedupeKeepAll`** (default) — one row per `(target, uid)`; the same object selected by two targets
  appears twice, attributed to each. Preserves "who collected this".
- **`DedupeByResourceUID`** — collapse to one row per `(namespace, uid)` across targets; **last writer
  wins**, order stable. For "unique inventory of objects" regardless of selecting target.

`KollectClusterInventory` uses `MergeRows` on the export path, selected by **`spec.dedupe`**
(`keepAll` default | `byResourceUID`); namespaced `KollectInventory` marshals per-namespace snapshots
directly (no cross-target merge).

### Export fingerprint and coalescing

`ExportCoalesce { LastGeneration, LastHash, LastExport }` debounces exports
(`exportMinInterval` — [ADR-0201](0201-crd-model.md)). `ShouldSkip` returns true **only** when all hold:

1. spec `generation` unchanged (a spec edit always exports),
2. payload SHA-256 (`ContentHash`) unchanged (real data change always exports), and
3. within `minInterval` of the last successful export.

So config changes and content changes bypass the timer; pure churn within the window is coalesced.

## Consequences

- Diffs and golden tests are deterministic (stable merge order — [ADR-0405](0405-export-data-contract.md)).
- `DedupeByResourceUID`'s last-writer-wins can drop per-target attribute differences — acceptable for an
  inventory, surprising if two targets extract different attributes from one object.
- The fingerprint is the same mechanism the inventory reconciler uses, so behavior is consistent across
  namespaced and cluster-scoped paths.

## Open questions

- **DECIDED (2026-06-05):** Expose **`spec.dedupe`** (`keepAll` | `byResourceUID`) on
  `KollectClusterInventory`, **default `keepAll`** — explicit, not inferred.
- **DECIDED (2026-06-05):** Collision resolution stays **last-writer-wins** (no attribute union).
- **DECIDED (2026-06-05):** Add **hub-side delete reconciliation** when a spoke stops reporting an
  object, tracked with Postgres delete recon ([ADR-0401](0401-sink-taxonomy-state-vs-stream.md),
  [ADR-0402](0402-sink-backends-database-kafka.md)).
