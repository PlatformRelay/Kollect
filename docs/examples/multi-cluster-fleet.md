# Multi-cluster fleet (shared sink)

Run **one operator per cluster**, all exporting to the **same backend** with a distinct
`spec.cluster` value. No hub tier — Postgres, Git, or event sinks merge by cluster id.

## Prerequisites

- Two (or more) clusters with network reachability to the shared sink (Postgres, Git remote, or
  NATS/Kafka broker)
- Helm chart `mode: single` (default) on each cluster

## Postgres fleet

Each cluster installs the operator with the same DSN and a unique cluster label:

```yaml
# kollect-doc: ignore Helm values, not a kollect CR
# cluster-a — values fragment
mode: single
# …
```

Apply a database sink (see [`postgres-state-store.md`](postgres-state-store.md)) with:

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectDatabaseSink
metadata:
  name: fleet-postgres
  namespace: platform
spec:
  type: postgres
  cluster: cluster-a   # unique per installation
  postgres:
    databaseRef:                    # Secret holding the DSN; never inline credentials
      name: inventory-postgres-dsn
      namespace: kollect-system
    schema: public
    table: inventory_items
```

Repeat on cluster B with `spec.cluster: cluster-b`. Rows merge in one table; primary key includes
cluster ([ADR-0501](../adr/0501-multi-cluster-fleet.md)).

## Git fleet

Use `pathTemplate` on a snapshot sink:

```yaml
# kollect-doc: fragment KollectSnapshotSink
spec:
  type: git
  endpoint: https://github.com/org/inventory.git
  cluster: cluster-a
  pathTemplate: clusters/{cluster}/inventory.json
  git:
    branch: main
```

Each cluster commits under its own path; CI can aggregate `clusters/*` if a single commit is required.

## Inventory collection

Per cluster, use the same pattern as [`deployment-inventory.md`](../getting-started/first-inventory.md):

- `KollectProfile` for the GVK schema
- `KollectTarget` selecting workloads
- `KollectInventory` referencing the fleet sink

Samples: `config/samples/e2e/team-inventory.yaml` in the repository (single-cluster shape; add sink refs for your backend).

## Related

- [ADR-0501 — Multi-cluster fleet](../adr/0501-multi-cluster-fleet.md)
- [Best practices — fleet deployments](../operator-manual/production-checklist.md)
- [Postgres state store](postgres-state-store.md)
