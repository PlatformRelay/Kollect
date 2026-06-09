# KollectDatabaseSink

**Scope:** Namespace · **Reconciled:** Yes (connection test) · **Short name:** `kdb`

Cluster-scoped variant: **`KollectClusterDatabaseSink`** (`kcdb`).

## What it is for

A `KollectDatabaseSink` configures **relational / document** export backends — PostgreSQL and MongoDB
([ADR-0417](../adr/0417-mongodb-database-sink.md)). Inventories reference database sinks via
`KollectInventory.spec.databaseSinkRefs`. Postgres uses a bulk upsert path for high-row exports.

## Spec highlights

| Field | Purpose |
| --- | --- |
| `spec.type` | Backend: `postgres`, `mongodb` |
| `spec.postgres` / `spec.mongodb` | Type-specific connection and table/collection settings |
| `spec.provisioning.mode` | `ensure` (create table, default) or `existing` (never issue DDL) ([ADR-0416](../adr/0416-sink-config-layering.md)) |
| `spec.options` | Non-secret pass-through tuning (secret-like keys rejected by the webhook) |
| `spec.exportMinInterval` | Default per-ref debounce when inventory ref omits override |
| `spec.connectionTest` | Automatic probe on create/update (default `true`) |

MongoDB upserts one document per inventory item into `spec.mongodb.database`/`spec.mongodb.collection`,
keyed on `(inventory_namespace, inventory_name, target_name, source_uid)`
([ADR-0417](../adr/0417-mongodb-database-sink.md)).

## Example

A Postgres sink that upserts into an existing table and never issues DDL
([`config/samples/kollect_v1alpha1_kollectdatabasesink.yaml`](https://github.com/konih/kollect/blob/main/config/samples/kollect_v1alpha1_kollectdatabasesink.yaml)):

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectDatabaseSink
metadata:
  name: postgres-inventory-demo
  namespace: default
spec:
  type: postgres
  cluster: kind-kollect-dev
  connectionTest: true
  provisioning:
    mode: existing          # 'ensure' (default) creates the table; 'existing' never touches DDL
  options:
    statement_timeout: "30000"
  postgres:
    databaseRef:            # DSN lives in a Secret — never inline credentials
      name: inventory-postgres-dsn
      namespace: kollect-system
    schema: public
    table: inventory_items
```

MongoDB variant:
[`config/samples/kollect_v1alpha1_kollectdatabasesink_mongodb.yaml`](https://github.com/konih/kollect/blob/main/config/samples/kollect_v1alpha1_kollectdatabasesink_mongodb.yaml).

## Status

`status.conditions` includes `ConnectionVerified` after the family sink reconciler runs an optional
connectivity probe ([ADR-0403](../adr/0403-connection-test.md)).

### Preview (`status.preview`)

Annotate a sink with `kollect.dev/preview: "true"` to have the reconciler render a side-effect-free
preview of its export implications under `status.preview`
([ADR-0416](../adr/0416-sink-config-layering.md) §8): the effective provisioning mode and
serialization format, plus the expected Postgres DDL or MongoDB index keys for the configured
backend, and any warnings. Removing the annotation clears `status.preview`.

See [ADR-0414](../adr/0414-sink-family-crds.md) for the family CRD model and
[ADR-0416](../adr/0416-sink-config-layering.md) for provisioning and option layering.
