# ADR-0414: Sink family CRDs

> Separate destination roles into typed, namespaced CRDs with a shared internal adapter.

**Theme:** 04 · Sink architecture · **Status:** Current

## Context

Snapshot stores, databases, and event streams have different configuration and delivery semantics.
A single public sink union made invalid field combinations easy and obscured those roles.

## Decision

Kollect exposes three namespaced sink CRDs:

| CRD | Role | Admitted `spec.type` values |
| --- | --- | --- |
| `KollectSnapshotSink` | Whole-state snapshots | `git`, `gitlab`, `s3`, `gcs` |
| `KollectDatabaseSink` | Queryable state | `postgres`, `mongodb`, `bigquery` |
| `KollectEventSink` | Change events | `kafka`, `nats` |

There are nine shipped backends. `azureblob` and `http` are reserved constants but admission rejects
them until a real backend and its tests ship. Parquet is an S3/GCS serialization format, not another
sink type.

All public sink resources are namespaced. A `KollectClusterInventory` references platform-owned
profiles and sinks by name plus namespace, as specified by [ADR-0208](0208-cluster-static-refs-via-namespace.md).

Family specs normalize to the Go-only `KollectSinkSpec` adapter used by the registry, connection
probes, and export code. That internal name is not a Kubernetes kind.

## Validation and reconciliation

- Admission allowlists types per family and rejects incompatible type-specific blocks and formats.
- Each family resource has a lightweight reconciler for optional connection tests and preview
  status; export remains inventory-driven.
- `KollectScope` has separate allowlists for snapshot, database, and event references.

## Consequences

- Manifests communicate destination semantics and receive family-specific validation.
- Adding a backend requires implementation, registry wiring, admission, schema, probes, and tests;
  declaring a constant alone never makes a backend usable.
- Cluster inventories retain a single namespaced source of truth for shared configuration instead
  of duplicating cluster-scoped sink APIs.
