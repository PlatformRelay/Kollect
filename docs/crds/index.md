# Custom resource reference

Detailed reference for each Kollect API kind. These pages document **purpose**, **spec fields**,
**status conditions**, **RBAC**, **sample usage**, and **failure modes** for operators and platform
teams.

!!! warning "Pre-beta API"
    All kinds are **`v1alpha1`**. Field names, status conditions, and webhook rules may change before
    beta. Treat [PLATFORM-DECISIONS.md](../PLATFORM-DECISIONS.md) as the locked decision summary; per-kind
    pages track current behavior.

## Architecture context

| Doc | Contents |
| --- | --- |
| [concepts/architecture.md](../concepts/architecture.md) | CRD model, reconciliation, deployment defaults |
| [concepts/export-pipeline.md](../concepts/export-pipeline.md) | Debouncing, collection pipeline, scope gates, connection tests |
| [PLATFORM-DECISIONS.md](../PLATFORM-DECISIONS.md) | Locked product decisions (2026-06-05 pivot) |
| [getting-started/first-inventory.md](../getting-started/first-inventory.md) | End-to-end Profile → Sink → Target → Inventory |
| [examples/helm-release-inventory.md](../examples/helm-release-inventory.md) | Argo / Helm release walkthrough |

## Pipeline overview

```mermaid
flowchart LR
  Profile[KollectProfile<br/>what to extract]
  Scope[KollectScope<br/>policy]
  Target[KollectTarget<br/>what to watch]
  Inv[KollectInventory<br/>aggregate + export]
  Snap[KollectSnapshotSink]
  Db[KollectDatabaseSink]
  Ev[KollectEventSink]
  ConnTest[KollectConnectionTest<br/>probe]
  CTarget[KollectClusterTarget]
  CInv[KollectClusterInventory]

  Profile --> Target
  Scope -.-> Target
  Scope -.-> Inv
  Target --> Inv
  Inv --> Snap
  Inv --> Db
  Inv --> Ev
  ConnTest -.-> Snap
  ConnTest -.-> Db
  ConnTest -.-> Ev
  Profile -.->|"profileRef (name + namespace)"| CTarget
  CTarget -.-> CInv
  CInv -.-> Snap
  CInv -.-> Db
  CInv -.-> Ev
```

**Typical team flow:** create Profile and family Sinks → bind Target to Profile → point Inventory at
snapshot/database/event sink refs.
Optional Scope constrains GVKs, namespaces, and sinks. Use ConnectionTest to verify sink reachability
before export.

!!! tip "Namespace rules"
    On **namespaced** kinds, `profileRef`, family sink refs (`snapshotSinkRefs`, `databaseSinkRefs`,
    `eventSinkRefs`), and connection-test `sinkRef` resolve CRs in the **same namespace** as the
    referring object. On **cluster** kinds, `profileRef` requires an explicit `name` + `namespace`;
    cluster-inventory sink refs resolve by `name` + `namespace`, defaulting to `spec.sinkNamespace`
    when a ref omits `namespace`. There are no cluster-scoped static config kinds (ADR-0208).

**Platform flow:** publish a namespaced `KollectProfile` (and family sinks) in `kollect-system`, then
`KollectClusterTarget` → `KollectClusterInventory` roll up and export cross-namespace. Both cluster
reconciled kinds reference the namespaced static config by `name` + `namespace`
([ADR-0208](../adr/0208-cluster-static-refs-via-namespace.md)).

### Snapshot export layout and spill

`KollectSnapshotSink.spec.pathTemplate` selects the Git/object-store object path (default
`inventory/{namespace}/{name}.json`; placeholders `{cluster}`, `{namespace}`, `{name}`,
`{generation}`, `{extension}`) — see [ADR-0407](../adr/0407-git-object-store-layout.md).

Payloads **≥ 1 MiB** warn; **> 1 MiB** require an `s3` or `gcs` snapshot sink in
`spec.snapshotSinkRefs` (Git receives smaller exports only). Hard cap ~**1.5 MiB** `maxExportBytes`
blocks export entirely. The ceiling is configurable per sink binding: each inventory family ref
accepts a `maxExportBytes` override that replaces the inventory-wide value (or, on cluster
inventories, the global cap) for that sink only — see
[KollectInventory](kollectinventory.md#spec-fields).

Per-sink export cadence is configured on inventory/cluster-inventory family refs (string or object),
optional sink defaults, and scope floors — [ADR-0413](../adr/0413-export-interval-scheduling.md).

## Kinds

| Kind | Scope | Reconciled | Reference |
| --- | --- | --- | --- |
| `KollectProfile` | Namespace | No | [crds/kollectprofile.md](kollectprofile.md) |
| `KollectSnapshotSink` | Namespace | Probe only | [crds/kollectsnapshotsink.md](kollectsnapshotsink.md) |
| `KollectDatabaseSink` | Namespace | Probe only | [crds/kollectdatabasesink.md](kollectdatabasesink.md) |
| `KollectEventSink` | Namespace | Probe only | [crds/kollecteventsink.md](kollecteventsink.md) |
| `KollectTarget` | Namespace | Yes | [crds/kollecttarget.md](kollecttarget.md) |
| `KollectInventory` | Namespace | Yes | [crds/kollectinventory.md](kollectinventory.md) |
| `KollectScope` | Namespace | No (enforced) | [crds/kollectscope.md](kollectscope.md) |
| `KollectConnectionTest` | Namespace | Yes | [crds/kollectconnectiontest.md](kollectconnectiontest.md) |
| `KollectClusterScope` | Cluster | No (enforced) | [crds/kollectclusterscope.md](kollectclusterscope.md) |
| `KollectClusterTarget` | Cluster | Yes | [crds/kollectclustertarget.md](kollectclustertarget.md) |
| `KollectClusterInventory` | Cluster | Yes | [crds/kollectclusterinventory.md](kollectclusterinventory.md) |

## Reserved kinds (stubs pending)

| Kind | Scope | Notes |
| --- | --- | --- |
| ~~`KollectSink`~~ | — | **Removed** — use family sinks ([ADR-0414](../adr/0414-sink-family-crds.md)) |
| ~~`KollectClusterProfile`~~ | — | **Removed** — reference a namespaced `KollectProfile` by `name` + `namespace` ([ADR-0208](../adr/0208-cluster-static-refs-via-namespace.md)) |
| ~~`KollectCluster*Sink`~~ | — | **Removed** — reference namespaced family sinks per ref namespace ([ADR-0208](../adr/0208-cluster-static-refs-via-namespace.md)) |
| ~~`KollectRemoteCluster`~~ | — | **Removed** — shared sink + `spec.cluster` ([ADR-0501](../adr/0501-multi-cluster-fleet.md)) |

## Short names

| Kind | Short name | `kubectl` example |
| --- | --- | --- |
| `KollectInventory` | `kinv` | `kubectl get kinv -A` |
| `KollectTarget` | `ktgt` | `kubectl get ktgt -n default` |
| `KollectClusterTarget` | `kctgt` | `kubectl get kctgt` |
| `KollectClusterInventory` | `kcinv` | `kubectl get kcinv` |
| `KollectConnectionTest` | `kconntest` | `kubectl get kconntest -A` |
| `KollectScope` | `kscope` | `kubectl get kscope -A` |

## Quick apply

```sh
kubectl apply -k config/samples/
kubectl get kprof,ksnap,kdb,kevt,ktgt,kinv,kscope,kconntest -A
```

See [getting-started/install.md](../getting-started/install.md) for kind cluster install prerequisites.
