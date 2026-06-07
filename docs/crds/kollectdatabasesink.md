# KollectDatabaseSink

**Scope:** Namespace · **Reconciled:** Yes (connection test) · **Short name:** `kdb`

Cluster-scoped variant: **`KollectClusterDatabaseSink`** (`kcdb`).

## What it is for

A `KollectDatabaseSink` configures **database** export backends — PostgreSQL, MongoDB
([ADR-0417](../adr/0417-mongodb-database-sink.md)), and BigQuery (stub)
([ADR-0402](../adr/0402-sink-backends-database-kafka.md)). Inventories reference database sinks via
`KollectInventory.spec.databaseSinkRefs`.

## Spec highlights

| Field | Purpose |
| --- | --- |
| `spec.type` | Backend: `postgres`, `mongodb`, `bigquery` |
| `spec.postgres` / `spec.mongodb` / `spec.bigquery` | Type-specific connection and table/collection settings |
| `spec.exportMinInterval` | Default per-ref debounce when inventory ref omits override |
| `spec.connectionTest` | Automatic probe on create/update (default `true`) |

MongoDB upserts one document per inventory item into `spec.mongodb.database`/`spec.mongodb.collection`,
keyed on `(inventory_namespace, inventory_name, target_name, source_uid)`
([ADR-0417](../adr/0417-mongodb-database-sink.md)).

## Status

`status.conditions` includes `ConnectionVerified` after the family sink reconciler runs an optional
connectivity probe ([ADR-0403](../adr/0403-connection-test.md)).

### Preview (`status.preview`)

Annotate a sink with `kollect.dev/preview: "true"` to have the reconciler render a side-effect-free
preview of its export implications under `status.preview`
([ADR-0416](../adr/0416-sink-config-layering.md) §8): the effective provisioning mode and
serialization format, plus the expected Postgres DDL or MongoDB index keys for the configured
backend, and any warnings. Removing the annotation clears `status.preview`.

See [ADR-0414](../adr/0414-sink-family-crds.md) for the family CRD model.
