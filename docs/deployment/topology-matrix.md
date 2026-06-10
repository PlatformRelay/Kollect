# Deployment topology matrix

Kollect supports more than one operator deployment shape on a single cluster. This page compares
paths, RBAC expectations, sink resolution, and inventory CR kinds. It records maintainer decisions
from [ADR-0203](../adr/0203-namespaced-multi-tenancy.md) and the local tenancy ADR (0421) without
introducing anti-overlap guardrails — overlapping collection is allowed.

## Quick comparison

| Path | Priority | Operator installs | Watch scope | Tenancy policy | Cluster CRDs reconciled |
| --- | --- | --- | --- | --- | --- |
| **A. Platform golden path** | Default | One cluster-wide release | All namespaces (`watchNamespaces: []`) | `KollectScope` per tenant namespace | Yes — rollups + namespaced family sinks in `kollect-system` |
| **B. Team-owned operator** | Supported, lower doc priority | One release per team namespace | Explicit list (`watchNamespaces: [team-a, …]`) | `KollectScope` in team namespace | No — namespaced CRDs only |
| **C. Hybrid** | Supported | Platform + one or more team releases | Platform: all; teams: subset | Platform + per-team `KollectScope` | Platform reconciles cluster CRDs; teams namespaced only |
| **D. Multi-team overlap** | Operational scenario (not blocked) | Two or more independent installs | Overlapping namespace/GVK sets | Each install's `KollectScope` (if any) | Depends on which paths are combined |

Path **D** is not a separate install recipe. It describes what happens when paths **B** and/or **C**
leave two operators watching the same namespace and GVK — **intentionally allowed**. Kollect does not
reject overlapping watch scopes via webhook or admission.

## Path A — Platform golden path

**When:** A platform team operates one shared operator for the cluster.

| Aspect | Detail |
| --- | --- |
| Helm profile | Default chart values — `tenantMode: false`, `watchNamespaces: []` |
| Manager RBAC | `ClusterRole` / `ClusterRoleBinding` on the manager ServiceAccount |
| CRD bootstrap | Cluster admin applies CRDs once (standard for any operator) |
| Workload RBAC | ClusterRole (or equivalent) for dynamic informers on scraped GVKs |
| Validating webhooks | On by default (`ValidatingWebhookConfiguration` is cluster-scoped) |
| Leader election | One lease per operator deployment — dedupes work **within** this install only |

**Tenancy:** Per-tenant `KollectScope` in each namespace governs allowed GVKs, workload namespaces,
and sink refs. Violations hard-degrade targets and inventories (`ScopeGVKDenied`, `ScopeNamespaceDenied`,
`ScopeSinkDenied`).

**Inventory CR kinds:**

| Kind | Scope | Role on this path |
| --- | --- | --- |
| `KollectInventory` | Namespaced | Primary product unit — aggregates targets in the same namespace |
| `KollectClusterInventory` | Cluster | Platform rollup — composes namespace snapshots/shards (explicit federation) |
| `KollectTarget` / `KollectClusterTarget` | Namespace / cluster | Collection drivers |
| `KollectProfile` | Namespace | Extraction schemas — cluster targets reference by `name` + `namespace` ([ADR-0208](../adr/0208-cluster-static-refs-via-namespace.md)) |

**Sink resolution:**

| Sink kind | Scope | Resolved by |
| --- | --- | --- |
| `KollectSnapshotSink`, `KollectDatabaseSink`, `KollectEventSink` | Namespaced | Namespaced inventory in same namespace; `KollectClusterInventory` by `name` + `namespace`, defaulting to `spec.sinkNamespace` ([ADR-0208](../adr/0208-cluster-static-refs-via-namespace.md)) |

See [cluster rollup example](../examples/cluster-rollup.md) and [ADR-0501](../adr/0501-multi-cluster-fleet.md)
for fleet fan-in to shared sinks.

## Path B — Team-owned operator (minimal RBAC)

**When:** A team installs and operates Kollect in its own namespace with namespace-scoped reconciler
RBAC.

| Aspect | Detail |
| --- | --- |
| Helm profile | [`values-minimal-rbac.yaml`](../../charts/kollect/values-minimal-rbac.yaml) — `tenantMode: true`, non-empty `watchNamespaces` |
| Manager RBAC | `Role` / `RoleBinding` in install namespace only |
| CRD bootstrap | Still cluster-level (platform admin once per cluster) |
| Workload RBAC | Team grants `Role` per scraped namespace for target GVKs; SAR pre-check before informers |
| Validating webhooks | Usually off — team install cannot own cluster-scoped webhook config |
| Cluster CRDs | **Not reconciled** — no `KollectClusterInventory` or `KollectClusterTarget` |

**Inventory CR kinds (team path only):**

| Kind | Used |
| --- | --- |
| `KollectScope`, `KollectProfile`, family sinks, `KollectTarget`, `KollectInventory` | Yes |
| `KollectConnectionTest` | Yes (namespaced probe) |
| `KollectClusterInventory`, `KollectClusterTarget` | No — require platform golden-path RBAC |

**Sink resolution:** Namespaced family sinks in the team namespace. Inventory `*SinkRefs` resolve
`snapshotSinkRefs` / `databaseSinkRefs` / `eventSinkRefs` to CRs in the **same namespace**. Secrets
for sink credentials must live in the install namespace — the operator cannot read Secrets elsewhere.

Full install steps: [Team-owned operator (minimal RBAC)](team-operator.md).

## Path C — Hybrid (platform + team operators)

**When:** Platform runs cluster rollups and shared sinks while teams run namespaced-only operators
for delegated collection.

| Component | Path | Reconciles |
| --- | --- | --- |
| Platform release | A | `KollectClusterInventory`, namespaced family sinks in `kollect-system`, optional `KollectClusterTarget` |
| Team release(s) | B | Namespaced targets, inventories, sinks in team namespace |

**Main risks (honest):**

- **Double-processing:** Platform and team operators may both watch the same namespace/GVK when scopes
  overlap — **allowed**, not blocked.
- **Sink collisions:** Two inventories exporting the same logical rows to one Postgres table or Git repo
  without coordination produces duplicate rows unless ops policy or sink dedupe addresses it.
- **Metrics cardinality:** Multiple installs increase instance/namespace/profile/sink label dimensions
  — keep dashboards bounded ([ADR-0604](../adr/0604-target-scoped-prometheus-metrics.md)).

**Sink resolution on hybrid:** Platform cluster inventories resolve namespaced family sinks by `name` +
`namespace` (default `sinkNamespace`). Team inventories resolve namespaced family sinks locally. **No
automatic cross-install dedupe** — see [Multi-operator sink dedupe](#multi-operator-sink-dedupe-runbook) below.

## Path D — Multi-team overlap (no guardrails)

Two team-owned operators (or a team operator plus the platform operator) **may** watch the same
namespace and collect the same GVK. Kollect **does not**:

- Enforce exclusive namespace ownership
- Reject overlapping `watchNamespaces` at admission
- Coordinate exports across independent deployments

Leader election prevents duplicate reconcile loops **inside one Helm release** only. Independent
installs each run their own leader election lease.

**Operational choices when overlap is undesirable:**

1. **Prevent overlap** — ops policy: one operator per namespace, or disjoint `watchNamespaces` lists
   (voluntary, not product-enforced).
2. **Accept duplicates** — `spec.dedupe: keepAll` (default) preserves per-target attribution; downstream
   consumers dedupe if needed.
3. **Sink dedupe backstop** — `spec.dedupe: byResourceUID` on `KollectClusterInventory` or downstream
   merge policy (see runbook below).

## RBAC requirements summary

| Requirement | Golden path (A) | Team path (B) | Hybrid (C) |
| --- | --- | --- | --- |
| CRD install | Cluster admin | Cluster admin (reuse platform CRDs) | Cluster admin |
| Manager reconciler | `ClusterRole` | `Role` in install namespace | Both profiles |
| Workload informers | Cluster-wide or scoped `ClusterRole` | `Role` per scraped namespace | Per component |
| Secrets for sinks | Any namespace (cluster list) | Install namespace only | Per component |
| `KollectCluster*` reconcile | Yes | No | Platform only |
| Validating webhook | Platform default | Off or shared platform webhook | Platform optional |

## Sink resolution summary

| Inventory kind | Sink CR kinds | Resolution rule |
| --- | --- | --- |
| `KollectInventory` | Namespaced family sinks | Same namespace as inventory; refs in `snapshotSinkRefs`, `databaseSinkRefs`, `eventSinkRefs` |
| `KollectClusterInventory` | namespaced family sinks | `name` + `namespace` per ref (default `spec.sinkNamespace`); optional `spec.dedupe` for cross-target merge ([ADR-0305](../adr/0305-aggregation-dedupe.md)) |

`KollectScope` allowlists use the same ref shapes — denied sinks hard-degrade the inventory.

## Multi-operator sink dedupe runbook

Sink dedupe is **optional defensive safety** when overlapping collection is intentional or accidental.
It is **not** a primary conflict-control mechanism and does not replace clear ownership policy.

### When duplicates appear

Duplicates occur when:

- Two operator installs watch the same namespace and register informers for the same GVK
- Two `KollectTarget`s in one inventory select the same object (intra-inventory — controlled by
  `spec.dedupe` on cluster inventory only)
- Platform and team paths both export the same namespace rows to one shared sink (cross-install —
  **not** automatically deduped by Kollect)

Each independent operator computes its own export fingerprint and writes independently. There is no
cross-deployment export coordinator.

### What Kollect dedupe covers

| Layer | Mechanism | Scope |
| --- | --- | --- |
| Intra-inventory (cluster rollup) | `KollectClusterInventory.spec.dedupe` — `keepAll` (default) or `byResourceUID` | Collapses rows from multiple **targets in one cluster inventory** ([ADR-0305](../adr/0305-aggregation-dedupe.md)) |
| Namespaced inventory | No cross-target merge | Per-namespace snapshot; same object from two targets → two rows unless targets do not overlap |
| Cross-operator | **None built-in** | Ops policy or downstream sink dedupe |

`byResourceUID` collapses to one row per `(namespace, uid)` across targets in **one** cluster inventory
export — last writer wins. It does **not** dedupe exports from two separate operator deployments to
the same Postgres table or Git branch.

### Runbook steps

1. **Confirm overlap** — list operator releases and `watchNamespaces` / `tenantMode` per install;
   compare with `KollectTarget` GVKs and namespace selectors in overlapping namespaces.
2. **Decide policy** — choose one:
   - *Exclusive collection* — adjust voluntary watch scope so only one install scrapes a namespace
     (recommended when duplicates are unacceptable).
   - *Shared collection, separate sinks* — each install exports to distinct sink CRs (simplest backstop).
   - *Shared sink, accept duplicates* — document for downstream consumers; use `keepAll` semantics.
   - *Shared sink, collapse in rollup* — platform `KollectClusterInventory` with `spec.dedupe: byResourceUID`
     if **only** the platform path feeds that inventory (team rows still duplicate if team also exports).
3. **Downstream dedupe** — for Postgres/BigQuery/Git consumers, dedupe on `(cluster, namespace, uid)` or
   envelope checksum ([ADR-0405](../adr/0405-export-data-contract.md)) when two operators feed one store.
4. **Fleet fan-in** — multi-cluster shared sinks use `(cluster, uid)` identity for delete reconciliation
   ([ADR-0501](../adr/0501-multi-cluster-fleet.md)); per-cluster duplicate operators still multiply rows
   for the same cluster id unless ops policy prevents overlap.

### What not to expect

- No webhook rejects a second operator watching `team-a`
- No leader election across Helm releases
- No automatic merge of team and platform exports to one sink
- `byResourceUID` does not union attributes from different targets — last writer wins ([ADR-0305](../adr/0305-aggregation-dedupe.md))

## See also

- [ADR-0203: Namespaced multi-tenancy](../adr/0203-namespaced-multi-tenancy.md)
- [Team-owned operator (minimal RBAC)](team-operator.md)
- [ADR-0305: Aggregation and dedupe](../adr/0305-aggregation-dedupe.md)
- [ADR-0414: Sink family CRDs](../adr/0414-sink-family-crds.md)
- [Multi-tenant watch scope example](../examples/multi-tenant-watch-namespaces.md)
- [Helm values — per-team install](../operator-manual/helm-values.md#per-team-install-recommended-default)
