# Example: Deployment inventory

!!! tip "What this guide assumes"
    You have Kollect installed (see [QUICKSTART.md](../QUICKSTART.md) or
    [Kind local lab](kind-local-lab.md)) and can apply manifests from `config/samples/`. The default
    path uses a Postgres sink; swap to Git for audit-only workflows.

This walkthrough connects the core **namespaced** CRDs into a minimal pipeline: define **what**
to extract (`KollectProfile`), **where** to send it (family sinks — `KollectDatabaseSink`,
`KollectSnapshotSink`), **which** resources to watch (`KollectTarget`), and **when** to aggregate
and export (`KollectInventory`). Multi-cluster rollups use a **shared sink** and `spec.cluster` — see
[multi-cluster fleet](multi-cluster-fleet.md).

**Default sample path:** Postgres state store (`postgres-inventory-demo`). Swap to `git-inventory-demo`
for Git audit/CI. See [Postgres state store](postgres-state-store.md) and
[Connection test](connection-test.md).

Files live in `config/samples/` and can be applied with `kubectl apply -k config/samples/`.

## Overview

```mermaid
flowchart LR
  Profile[KollectProfile<br/>Deployment schema]
  Target[KollectTarget<br/>select Deployments]
  Inv[KollectInventory<br/>aggregate + export]
  Db[KollectDatabaseSink<br/>Postgres]
  Snap[KollectSnapshotSink<br/>Git]
  K8s[(Kubernetes API)]

  Profile --> Target
  Target --> K8s
  Target --> Inv
  Inv --> Db
  Inv --> Snap
  Db --> PG[(Postgres)]
  Snap --> Git[(Git repo)]
```

## Scale

!!! note "Large fleets"
    The collection path targets **100,000** collected rows **per cluster operator** (typical
    Deployment/Service profiles). **Export must be sharded:** one `KollectInventory` per workload
    namespace (or smaller groups) so each export stays **below ~2,000 rows** (~1.5 MiB). Tune
    namespace-scoped informers, `exportMinInterval`, Helm `resourcesProfile: large`, and reconcile
    parallelism per [PERFORMANCE.md](../PERFORMANCE.md) and [ADR-0603](../adr/0603-performance-scalability.md).

## Step 1 — KollectProfile

Defines the GVK and attribute extraction rules. This example collects container images and metadata
labels from `apps/v1` Deployments.

Sample: `config/samples/kollect_v1alpha1_kollectprofile.yaml`

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectProfile
metadata:
  name: deployment-images
  namespace: default
spec:
  targetGVK:
    group: apps
    version: v1
    kind: Deployment
  attributes:
    - name: image
      path: '$.spec.template.spec.containers[0].image'
      type: string
    - name: images
      path: '$.spec.template.spec.containers[*].image'
      type: list
    - name: containerCount
      path: "cel:size(object.spec.template.spec.containers)"
      type: int
    - name: labels
      path: '$.metadata.labels'
      type: map
      optional: true
```

**Behavior:** the target controller loads this profile when resolving `profileRef`. JSONPath and CEL
run against each cached Deployment object. Missing optional attributes do not fail the row; required
attributes surface extraction errors on the target status.

### All container images (`[*]` wildcard)

Multi-container Deployments need every image, not only the first. Use the kubectl JSONPath wildcard
**`[*]`** (not `[ ALL ]` or bare `[]`):

| Path | Export value (2 containers) |
| --- | --- |
| `$.spec.template.spec.containers[0].image` | `"nginx:1.25"` (scalar) |
| `$.spec.template.spec.containers[*].image` | `["nginx:1.25", "sidecar:0.1"]` (JSON array) |

Set `type: list` on multi-value attributes. CEL equivalent:
`cel:object.spec.template.spec.containers.map(c, c.image)`.

See [DATA-FLOWS.md](../DATA-FLOWS.md#3-attribute-extraction-jsonpath-arrays) and
[ADR-0302](../adr/0302-cel-jsonpath-extraction.md).

## Step 2 — Family sinks

!!! warning "Same-namespace sink refs"
    Family sink CRDs are **namespaced** — create sinks in the same namespace as
    `KollectInventory` family ref lists (`snapshotSinkRefs`, `databaseSinkRefs`, `eventSinkRefs`).
    Cross-namespace references fail admission with `SinkNotFound`
    ([ADR-0414](../adr/0414-sink-family-crds.md)).

### KollectDatabaseSink (default sample — Postgres)

`config/samples/kollect_v1alpha1_kollectdatabasesink.yaml`

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
  postgres:
    databaseRef:
      name: inventory-postgres-dsn
      namespace: kollect-system
    schema: public
    table: inventory_items
```

Create the DSN secret before export — see [Postgres state store](postgres-state-store.md).

### KollectSnapshotSink (Git audit / CI)

`config/samples/kollect_v1alpha1_kollectsnapshotsink.yaml`

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectSnapshotSink
metadata:
  name: git-inventory-demo
  namespace: default
spec:
  type: git
  endpoint: https://github.com/konih/kollect-inventory-demo.git
  connectionTest: true
  git:
    branch: main
    pushPolicy: Commit
    commitMessage: "chore(inventory): export {namespace}/{name}"
  # secretRef:
  #   name: git-push-credentials
  #   namespace: kollect-system
```

**Behavior:** the inventory controller resolves family sinks via the registry. With
`connectionTest: true`, family sink reconcilers probe on create/update and set `ConnectionVerified`.
Export commits deterministic JSON snapshots to Git or upserts rows to Postgres; status stores
summary refs (commit SHA), not the full payload ([ADR-0103](../adr/0103-etcd-limit.md)).

Production installs should set `connectionTest: false` (chart default) and re-probe on demand —
see [Connection test](connection-test.md).

## Step 3 — KollectTarget

Namespaced resource that binds a profile to selectors. Deployed in `default` in the sample.

`config/samples/kollect_v1alpha1_kollecttarget.yaml`

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectTarget
metadata:
  name: nginx-deployments
  namespace: default
spec:
  profileRef: deployment-images
  labelSelector:
    matchLabels:
      app.kubernetes.io/name: nginx
  suspend: false
```

`profileRef` resolves a `KollectProfile` in the **same namespace** as the target
([ADR-0204](../adr/0204-namespaced-profiles.md)).

**Watch labels (optional):** set `spec.watchMode: OptIn` to collect only namespaces/resources
labeled `kollect.dev/watch: enabled`, or annotate a namespace with
`kollect.dev/namespace-watch: disabled` to skip all workloads in that namespace while keeping
`watchMode: All` (default). See [ADR-0205](../adr/0205-watch-labels.md).

```yaml
# Opt out an entire namespace (All mode)
apiVersion: v1
kind: Namespace
metadata:
  name: kube-system
  annotations:
    kollect.dev/namespace-watch: disabled
```

**Behavior:**

1. Controller registers a dynamic informer for `apps/v1` Deployments (from the profile GVK).
2. Only Deployments matching `labelSelector` in the target namespace are collected.
3. Extracted rows feed the namespace inventory aggregator.

Create a matching workload to exercise selection:

```sh
kubectl create deployment nginx --image=nginx:1.27
kubectl label deployment nginx app.kubernetes.io/name=nginx --overwrite
```

## Step 4 — KollectInventory

Namespaced aggregator (same namespace as targets) referencing family sinks.

`config/samples/kollect_v1alpha1_kollectinventory.yaml`

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectInventory
metadata:
  name: team-inventory
  namespace: default
spec:
  exportMinInterval: 30s
  databaseSinkRefs:
    - postgres-inventory-demo
  snapshotSinkRefs:
    - name: git-inventory-demo
      exportMinInterval: 1h
  suspend: false
```

!!! info "Dual-cadence export (ADR-0413)"
    The sample fans out to **Postgres every 30s** (portal freshness) and **Git every 1h** (audit trail).
    Plain string refs inherit `spec.exportMinInterval`; object refs override per sink. Precedence:
    ref → sink default → inventory default → scope floor — see
    [ADR-0413](../adr/0413-export-interval-scheduling.md).

For Git-only audit, use a single snapshot ref: `snapshotSinkRefs: [git-inventory-demo]`.

**Behavior:**

- `status.itemCount` reflects aggregated rows from all active targets in the namespace.
- `status.lastExportTime` is the **max** of per-sink export times.
- `status.sinkExports[]` holds per-sink `lastExportTime`, `lastChecksum`, and `Synced` conditions.
- Conditions: `Ready`, `Synced` (or `PartiallySynced` when cadences differ), `SinkReachable`, `Degraded` per
  [error taxonomy](../adr/0602-error-taxonomy.md).

## Apply everything

```sh
kubectl apply -k config/samples/
kubectl get kinv,ktgt,ksnap,kdb -A
```

Verify sink connectivity before relying on export:

```sh
kubectl wait --for=condition=ConnectionVerified kollectdatabasesink/postgres-inventory-demo \
  -n default --timeout=60s
kubectl describe kollectinventory team-inventory -n default
```

## Troubleshooting

!!! tip "First checks"
    Run `kubectl describe` on the sink and inventory when export stalls — `ConnectionVerified`,
    `SinkReachable`, and `Synced` conditions usually pinpoint credential, namespace, or selector
    issues before diving into controller logs.

| Symptom | Likely cause |
| --- | --- |
| Target not found | `KollectTarget` is namespaced — ensure namespace matches |
| Profile not found | `profileRef` must name a `KollectProfile` in the **same namespace** as the Target |
| Sink not found | Family ref must name a sink in the **same namespace** as the Inventory (`snapshotSinkRefs`, `databaseSinkRefs`, or `eventSinkRefs`) |
| No export | Missing DSN/`secretRef`, `ConnectionVerified=False`, or `SinkReachable=False` with reason `SinkNotFound` / `SinkUnreachable` — see `kubectl describe kollectdatabasesink` / `kollectsnapshotsink` and inventory `status.conditions` |
| Empty item count | No Deployments match selector, or target suspended / scope denied |
| Namespace skipped | `kollect.dev/namespace-watch: disabled` or `watchMode: OptIn` without `enabled` label |

See [Kind local lab](kind-local-lab.md), [QUICKSTART.md](../QUICKSTART.md), and
[DEVELOPMENT.md](../DEVELOPMENT.md) for cluster setup and log inspection.

## Related

- [Spoke cluster inventory](multi-cluster-fleet.md) — Helm `mode: single` install narrative
- [Postgres state store](postgres-state-store.md) — DSN secret and delete reconciliation
- [Connection test](connection-test.md) — `ConnectionVerified` and `KollectConnectionTest`
- [KollectProfile](../crds/kollectprofile.md) · [KollectSnapshotSink](../crds/kollectsnapshotsink.md) ·
  [KollectDatabaseSink](../crds/kollectdatabasesink.md) · [KollectEventSink](../crds/kollecteventsink.md) ·
  [KollectTarget](../crds/kollecttarget.md) · [KollectInventory](../crds/kollectinventory.md)
- [CR reference](../CR-REFERENCE.md)
- [ADR-0201: Platform architecture pivot](../adr/0201-crd-model.md)
- [ADR-0603: Performance and scalability](../adr/0603-performance-scalability.md)
