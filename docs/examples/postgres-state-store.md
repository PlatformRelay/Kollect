# Example: Postgres state store

This walkthrough sets up a **relational state-of-record** sink for namespace inventory
exports. Postgres is the default MVP path in samples (`postgres-inventory-demo`) and the
recommended backend for portals and SQL analytics
([ADR-0402](../adr/0402-sink-backends-database-kafka.md),
[ADR-0703](../adr/0703-platform-architecture-pivot.md)).

Pair with [Deployment inventory](deployment-inventory.md) for the full Profile → Target →
Inventory pipeline, or apply only the sink + secret pieces here.

## Overview

```mermaid
flowchart LR
  Inv[KollectInventory]
  Sink[KollectSink<br/>type: postgres]
  Secret[(Secret<br/>dsn / url)]
  PG[(Postgres<br/>inventory_items)]

  Inv --> Sink
  Sink --> Secret
  Sink -->|upsert + delete recon| PG
```

## Step 1 — DSN secret

`KollectSink.spec.postgres.databaseRef` points at a Secret containing the connection string.
The operator reads **`dsn`**, **`url`**, **`connectionString`**, or **`DATABASE_URL`** — never
inline credentials on the sink CR.

Create the Secret in the namespace referenced by `databaseRef` (sample uses `kollect-system`):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: inventory-postgres-dsn
  namespace: kollect-system
type: Opaque
stringData:
  dsn: postgres://kollect:example@postgres.kollect-system.svc:5432/inventory?sslmode=disable
```

```sh
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Secret
metadata:
  name: inventory-postgres-dsn
  namespace: kollect-system
type: Opaque
stringData:
  dsn: postgres://kollect:example@postgres.kollect-system.svc:5432/inventory?sslmode=disable
EOF
```

For local **kind** dev, provision Postgres alongside the operator or point at an external
instance. Never commit real credentials to Git.

## Step 2 — KollectSink

Sample: `config/samples/kollect_v1alpha1_kollectsink_postgres.yaml`

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectSink
metadata:
  name: postgres-inventory-demo
  namespace: default
spec:
  type: postgres
  cluster: kind-kollect-dev
  connectionTest: true
  postgres:
    databaseRef:
      name: inventory-postgres-dsn
      namespace: kollect-system
    schema: public
    table: inventory_items
```

| Field | Role |
| --- | --- |
| `spec.cluster` | Labels rows for multi-cluster fan-in |
| `spec.postgres.schema` / `table` | Target table; DDL created on first export |
| `spec.connectionTest` | Samples/CI only — sets `ConnectionVerified` |

## Step 3 — Wire inventory

Reference the sink from `KollectInventory` in the **same namespace**:

```yaml
spec:
  sinkRefs:
    - postgres-inventory-demo
```

```sh
kubectl apply -k config/samples/
kubectl wait --for=condition=ConnectionVerified kollectsink/postgres-inventory-demo \
  -n default --timeout=60s
```

## Delete reconciliation

Postgres is a **relational SoR** — stale rows must disappear when objects leave the snapshot.
After each export the backend upserts current rows and deletes rows for that inventory +
cluster that are **not** in the snapshot
([ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md),
[ADR-0402](../adr/0402-sink-backends-database-kafka.md)).

Integration coverage: `internal/sink/postgres/export_integration_test.go`.

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| `ConnectionVerified=False` | Missing Secret or wrong `databaseRef` namespace |
| `SecretResolveFailed` | Secret lacks `dsn` / `url` key |

## Related

- [Deployment inventory](deployment-inventory.md) · [Connection test](connection-test.md)
- [KollectSink](../crds/kollectsink.md) · [ADR-0402](../adr/0402-sink-backends-database-kafka.md)
