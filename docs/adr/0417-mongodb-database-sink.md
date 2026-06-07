# ADR-0417: MongoDB database sink

> The first additional `KollectDatabaseSink` backend after Postgres: a document-store sink that
> upserts per-item inventory documents into a MongoDB collection with the same identity and
> delete-reconcile semantics as the Postgres sink, proving the database family is genuinely pluggable.

**Theme:** 04 · Export & sinks · **Status:** Current (accepted 2026-06-07)

## Context

The database sink family ([ADR-0414](0414-sink-family-crds.md)) shipped with one real backend
(Postgres) and a BigQuery stub. To validate that the family CRD is a real abstraction — not a
Postgres-shaped hole — and to serve teams whose system of record is a document store, Kollect needs a
second, genuinely different relational/document backend. MongoDB is the natural choice: widely
deployed, document-native (so the JSON inventory item maps directly), and integration-testable with a
container ([NFR-EXT-3](../REQUIREMENTS.md)).

The sink must honor the locked database-family contracts:

- **Identity** `(inventory_namespace, inventory_name, target_name, source_uid)` and **delete
  reconciliation** so deletions in the cluster are reflected ([FR-EXP-5](../REQUIREMENTS.md),
  [ADR-0401](0401-sink-taxonomy-state-vs-stream.md), [ADR-0402](0402-sink-backends-database-kafka.md)).
- **Credentials only via `secretRef`/`databaseRef`** — never in spec/status/logs
  ([NFR-SEC-1](../REQUIREMENTS.md)).
- **Cross-cutting `provisioning`** ([ADR-0416](0416-sink-config-layering.md)): `ensure` (default)
  creates the collection + unique identity index if missing; `existing` never creates and preflights
  that the collection exists.

## Decision

### 1. `type: mongodb` on the database family

Add `mongodb` to `KollectDatabaseSink.spec.type` (and the cluster variant) with a typed `spec.mongodb`
block:

```yaml
spec:
  type: mongodb
  provisioning: { mode: ensure }   # or existing
  mongodb:
    databaseRef: { name: inventory-mongodb-uri, namespace: kollect-system }
    database: inventory
    collection: inventory_items
```

`databaseRef` resolves a Secret whose `uri` / `url` / `connectionString` / `MONGODB_URI` key carries
the full MongoDB connection string (credentials included). The webhook requires the `mongodb` block for
`type: mongodb` and forbids the `postgres`/`bigquery` blocks, mirroring the existing family validation.

### 2. Document shape and upsert semantics

Each inventory item becomes one document keyed by the canonical identity:

```json
{
  "inventory_namespace": "...", "inventory_name": "...",
  "target_name": "...", "source_uid": "...",
  "cluster": "...", "resource_namespace": "...",
  "payload": { /* the full extracted item */ },
  "exported_at": "<UTC timestamp>"
}
```

Export `ReplaceOne(filter=identity, upsert=true)` per item, then **delete-reconcile**: documents for
this `(inventory, cluster)` whose `(target_name, source_uid)` is not in the current snapshot are
removed (`$nor`); an empty snapshot clears the partition. This matches the Postgres sink's stale-row
reconcile, so MongoDB and Postgres are interchangeable behind the family CRD.

### 3. Provisioning, capabilities, and connectivity

- **`ensure`** creates the collection if absent and a **unique index** on the identity tuple.
  **`existing`** verifies the collection exists and never issues create operations.
- Capabilities are the relational-store profile (upsert + delete reconcile, no object-store spill,
  JSON serialization only) — the same matrix the validation webhook gates `serialization.format` on.
- `connectionTest` runs a `Ping` against the deployment, surfaced through the standard sink condition
  and `KollectConnectionTest` flow ([ADR-0403](0403-connection-test.md)).

## Consequences

### Positive

- Proves the database sink family is pluggable; the registry/probe/validation/adapter seams added for
  Postgres absorbed MongoDB without changing the family CRD or the inventory reference shape.
- Document-store teams get a first-class system-of-record sink with delete reconciliation.
- Reuses the cross-cutting `provisioning`/`options`/`serialization` layering from
  [ADR-0416](0416-sink-config-layering.md) with no backend-specific config drift.

### Negative

- Adds the `go.mongodb.org/mongo-driver` dependency to the operator image.
- The driver's v1 line is in maintenance (v2 exists); a future bump to v2 is tracked as a dependency
  chore and is isolated to `internal/sink/mongodb`.
- MongoDB joins Postgres under "no merge without integration/e2e proof" ([ADR-0414](0414-sink-family-crds.md));
  a container-backed integration test gates production readiness.

## See also

- [ADR-0401: Sink taxonomy — state stores vs event emitters](0401-sink-taxonomy-state-vs-stream.md)
- [ADR-0402: Postgres and Kafka sink backends](0402-sink-backends-database-kafka.md)
- [ADR-0414: Sink family CRDs](0414-sink-family-crds.md)
- [ADR-0416: Sink configuration layering](0416-sink-config-layering.md)
- [ADR-0403: Connection test](0403-connection-test.md)
