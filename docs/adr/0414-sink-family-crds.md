# ADR-0414: Sink family CRDs (clean break, pre-GA)

**Status:** Current · **Supersedes:** [RFC: sink family CRDs](../rfc/sink-family-crds.md)

## Decision

Replace the unified **`KollectSink`** CRD with **three sink-family CRDs** aligned with
[ADR-0401](0401-sink-taxonomy-state-vs-stream.md):

| Family CRD (namespaced) | Cluster variant | Role | `spec.type` values |
| --- | --- | --- | --- |
| `KollectSnapshotSink` | `KollectClusterSnapshotSink` | Snapshot store | `git`, `gitlab`, `s3`, `gcs`, `azureblob`, `http` |
| `KollectDatabaseSink` | `KollectClusterDatabaseSink` | Relational SoR | `postgres`, `mongodb`, `bigquery` ([ADR-0417](0417-mongodb-database-sink.md)) |
| `KollectEventSink` | `KollectClusterEventSink` | Event emitter | `nats`, `kafka` |

**Clean break (pre-GA):** `KollectSink` is **removed** — no deprecation period, no conversion webhook,
no dual-write. Inventory references sinks only via typed family lists:

```yaml
spec:
  snapshotSinkRefs: [git-backup]
  databaseSinkRefs: [{name: warehouse, exportMinInterval: 5m}]
  eventSinkRefs: [audit-stream]
```

`KollectScope` allowlists mirror the same shape (`snapshotSinkRefs`, `databaseSinkRefs`,
`eventSinkRefs`). `KollectConnectionTest.spec.sinkRef` is an object with exactly one of
`snapshotSinkRef`, `databaseSinkRef`, or `eventSinkRef`.

Internally, family specs normalize to **`KollectSinkSpec`** (Go-only) for the compile-time
[sink registry](0406-sink-registry.md). Stub backends (`azureblob`, `http`, `bigquery`) register in
the registry and fail at export/probe with a clear *not implemented* error until shipped.

## Context

The unified `KollectSink` enum + optional sibling blocks scaled poorly as backends grew: weak OpenAPI
cross-field rules, duplicated secret/TLS fields, and inventory refs that could not express family
semantics (e.g. Postgres delete reconciliation vs Git snapshot-only).

Pre-GA we can adopt family CRDs without migration machinery. Post-GA adopters would require a
conversion story; that is explicitly out of scope for v1alpha1.

## Consequences

**Positive**

- Family-scoped RBAC (`kollectsnapshotsinks` vs `kollectdatabasesinks` vs `kollecteventsinks`).
- Webhook validation is per-family (`ValidateSnapshotSinkSpec`, …) with forbidden sibling blocks.
- Inventory export resolves sinks through `ResolveSink(family, name, clusterScoped)`.
- Cluster-scoped sinks (`KollectCluster*Sink`) support platform-wide destinations.

**Negative / trade-offs**

- Three CRDs + cluster variants increase manifest surface vs one `KollectSink`.
- Operators upgrading from early v1alpha1 builds that used `spec.sinkRefs` must rewrite manifests.
- `KollectSinkSpec` remains as an internal normalization type (not a user-facing CRD).

## Implementation notes

- **Controllers:** `Family*SinkReconciler` per family (namespaced + cluster) run connection tests.
- **Inventory:** `CollectInventorySinkBindings` fans out family ref lists; per-sink status keys use
  `family/name`.
- **Helm:** CRD bundle drops `kollect.dev_kollectsinks.yaml`; adds six family CRD YAMLs.
- **Stubs:** `azureblob`, `http`, `bigquery` — valid CRD apply + webhook pass; registry returns error
  until backend ships.

Promoted from [RFC: sink family CRDs](../rfc/sink-family-crds.md). Pre-GA clean break — no dual-write window.

## References

- [ADR-0401](0401-sink-taxonomy-state-vs-stream.md) — sink taxonomy
- [ADR-0406](0406-sink-registry.md) — registry / `Backend` interface
- [ADR-0413](0413-export-interval-scheduling.md) — per-ref export intervals
- [RFC (superseded)](../rfc/sink-family-crds.md)
