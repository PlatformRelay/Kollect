# KollectDatabaseSink

**Scope:** Namespace · **Reconciled:** Yes (connection test) · **Short name:** `kdb`

Cluster-scoped variant: **`KollectClusterDatabaseSink`** (`kcdb`).

## What it is for

A `KollectDatabaseSink` configures **relational** export backends — PostgreSQL and BigQuery (stub)
([ADR-0402](../adr/0402-sink-backends-database-kafka.md)). Inventories reference database sinks via
`KollectInventory.spec.databaseSinkRefs`.

## Spec highlights

| Field | Purpose |
| --- | --- |
| `spec.type` | Backend: `postgres`, `bigquery` |
| `spec.postgres` / `spec.bigquery` | Type-specific connection and table settings |
| `spec.exportMinInterval` | Default per-ref debounce when inventory ref omits override |
| `spec.connectionTest` | Automatic probe on create/update (default `true`) |

## Status

`status.conditions` includes `ConnectionVerified` after the family sink reconciler runs an optional
connectivity probe ([ADR-0403](../adr/0403-connection-test.md)).

See [ADR-0414](../adr/0414-sink-family-crds.md) for the family CRD model.
