# Glossary

Product vocabulary for Kollect — custom resources, sink roles, and multi-cluster terms. For
architecture context see [Understand the basics](UNDERSTAND-THE-BASICS.md) and
[Platform decisions](PLATFORM-DECISIONS.md).

## Core pipeline

| Term | Definition |
| --- | --- |
| **Profile** | A `KollectProfile` — static schema defining **what** to extract: target GVK plus JSONPath/CEL attribute rules. Not reconciled; admission-validated. See [ADR-0204](adr/0204-namespaced-profiles.md). |
| **Target** | A `KollectTarget` — **what to watch**: namespace selectors, label selectors, `profileRef`, and informer registration. The target controller reconciles collection state. |
| **Inventory** | A `KollectInventory` — **aggregate + export**: binds targets, holds the in-memory snapshot, debounces export, and writes to `sinkRefs`. |
| **Sink** | A family sink CR — **`KollectSnapshotSink`** (Git, GitLab, S3, GCS), **`KollectDatabaseSink`** (Postgres, MongoDB), or **`KollectEventSink`** (Kafka) — **where to send** inventory in the tenant namespace. |
| **Scope** | A `KollectScope` — optional **policy gate** limiting allowed GVKs, namespaces, and sinks for multitenant installs. Enforced at admission, not reconciled. |
| **Connection test** | A `KollectConnectionTest` — probes family sink reachability and sets `ConnectionVerified` ([ADR-0403](adr/0403-connection-test.md)). |
| **GVK** | Group, Version, Kind — identifies the Kubernetes API type a profile collects (for example `apps/v1` `Deployment`). |
| **Attribute** | Named extraction rule on a profile: maps a logical key (for example `image`) to a JSONPath or `cel:` expression. |
| **Snapshot** | The canonical in-memory inventory artifact per `KollectInventory` before any sink write. All exports are projections of this snapshot ([ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md)). |
| **Projection** | A sink-specific rendering of the snapshot — JSON in Git, objects on S3/GCS, upsert rows in Postgres/MongoDB, or events on Kafka. |
| **Export debounce** | Minimum interval between identical exports **per sink ref** (`exportMinInterval` precedence); reduces write amplification ([ADR-0413](adr/0413-export-interval-scheduling.md), [ADR-0603](adr/0603-performance-scalability.md)). |

## Sink roles

| Term | Definition |
| --- | --- |
| **Snapshot store** | Sink role that writes **whole current state** each cycle (Git, S3/GCS Parquet, HTTP). Deletes are implicit — absent resources are not in the latest file ([ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md)). |
| **SoR (system of record)** | Relational **state store** — typically Postgres — answering *what is deployed now?* with queryable rows. Requires **delete reconciliation** when resources disappear. |
| **Event emitter** | Sink role that publishes **change streams** (NATS JetStream, Kafka) for downstream consumers; tombstones and materialized views are consumer-owned. |
| **Delete reconciliation** | Postgres-specific logic to remove rows when UIDs leave the snapshot; snapshot stores get correct deletes for free. |

## Multi-cluster fleet

| Term | Definition |
| --- | --- |
| **Fleet** | Many clusters each running `mode: single`, exporting to a **shared sink** with distinct `spec.cluster` ([ADR-0501](adr/0501-multi-cluster-fleet.md)). |
| **Cluster label** | `spec.cluster` on database/event/snapshot sinks — merge dimension for Postgres PK, Git `pathTemplate`, or event subject. |
| **Cluster target / inventory** | Cluster-scoped reconciled kinds (`KollectClusterTarget`, `KollectClusterInventory`) for cross-namespace rollup and platform-wide export. They reference namespaced `KollectProfile` / family sinks by `name` + `namespace` ([ADR-0208](adr/0208-cluster-static-refs-via-namespace.md)). |

## Runtime and ops

| Term | Definition |
| --- | --- |
| **Reconciled CRD** | Kind with an active controller loop updating `status` (targets, inventories, connection tests). Contrast **static** profiles/scopes ([ADR-0202](adr/0202-static-vs-reconciled.md)). |
| **Watch mode** | `KollectTarget.spec.watchMode`: `All` (default, respect opt-out) or `OptIn` (only `kollect.dev/watch=enabled`). See [ADR-0205](adr/0205-watch-labels.md). |
| **Tenant mode** | Helm values restricting the operator informer cache and RBAC to listed namespaces ([ADR-0203](adr/0203-namespaced-multi-tenancy.md)). |
| **Leader election** | Optional HA mode where only one manager pod reconciles; enable for production multi-replica Deployments ([ADR-0706](adr/0706-testing-merge-gate-architecture.md)). |

<!-- BEGIN AUTO-CRD -->

## Custom resources (from CRD schema)

Auto-generated from `config/crd/bases/` OpenAPI descriptions. Regenerate with
`python3 hack/gen-glossary.py`. Field-level detail: [CR reference](CR-REFERENCE.md).

### `KollectClusterInventory` (cluster)

KollectClusterInventory rolls up cluster targets for platform operators.

| Spec field | Description |
| --- | --- |
| `databaseSinkRefs` | databaseSinkRefs lists namespaced KollectDatabaseSink refs (ADR-0414, ADR-0208). |
| `dedupe` | dedupe selects how overlapping target rows collapse on export (ADR-0305). |
| `eventSinkRefs` | eventSinkRefs lists namespaced KollectEventSink refs (ADR-0414, ADR-0208). |
| `exportMinInterval` | exportMinInterval is the minimum time between identical exports for this inventory. |
| `namespaceSelector` | namespaceSelector restricts rollup to namespaces matching the selector. |
| `namespaces` | namespaces explicitly lists namespace names that may contribute to the rollup. |
| `profileRef` | profileRef optionally overrides the rollup extraction schema with a namespaced |
| `sinkNamespace` | sinkNamespace is the default namespace for family sink refs that omit a namespace. |

Full reference: [KollectClusterInventory](crds/kollectclusterinventory.md).

### `KollectClusterScope` (cluster)

KollectClusterScope is a cluster governance boundary for cluster targets and inventories.

| Spec field | Description |
| --- | --- |
| `allowedGVKs` | allowedGVKs restricts which target resource kinds may be collected in this scope. |
| `allowedNamespaces` | allowedNamespaces restricts which workload namespaces may be collected. |
| `databaseSinkRefs` | databaseSinkRefs lists permitted KollectDatabaseSink names for this scope. |
| `deniedNamespaces` | deniedNamespaces is a platform blacklist; Target intent cannot override (D8). |
| `eventSinkRefs` | eventSinkRefs lists permitted KollectEventSink names for this scope. |
| `minExportInterval` | minExportInterval is a tenancy floor — inventory and sink intervals below this value are rejected. |
| `snapshotSinkRefs` | snapshotSinkRefs lists permitted KollectSnapshotSink names for this scope. |

Full reference: [KollectClusterScope](crds/kollectclusterscope.md).

### `KollectClusterTarget` (cluster)

KollectClusterTarget selects resources cluster-wide for platform operators.

| Spec field | Description |
| --- | --- |
| `excludedNamespaces` | excludedNamespaces is a static namespace denylist applied after include logic. |
| `includedNamespaces` | includedNamespaces is a static namespace allowlist. Empty means no extra restriction |
| `namespaceExcludeSelector` | namespaceExcludeSelector excludes namespaces whose labels match the selector. |
| `namespaceSelector` | namespaceSelector restricts collection to namespaces matching the selector. |
| `profileRef` | profileRef points at a namespaced KollectProfile by name and namespace (ADR-0208). |
| `resourceRules` | resourceRules declares GVK-scoped collection rules. When empty, collection falls back to |
| `suspend` | suspend pauses reconciliation when set to true (reserved for future controller). |

Full reference: [KollectClusterTarget](crds/kollectclustertarget.md).

### `KollectConnectionTest` (namespaced)

KollectConnectionTest triggers an audited one-shot sink connectivity probe.

| Spec field | Description |
| --- | --- |
| `ownerSink` | ownerSink sets an ownerReference to the sink when true (default). |
| `profileRef` | profileRef optionally names a KollectProfile for future composite probes. |
| `sinkRef` | sinkRef identifies exactly one family sink to probe (ADR-0414). |
| `ttlSecondsAfterFinished` | ttlSecondsAfterFinished deletes the CR after status.completed plus this TTL. |

Full reference: [KollectConnectionTest](crds/kollectconnectiontest.md).

### `KollectDatabaseSink` (namespaced)

KollectDatabaseSink is the Schema for relational export sinks.

| Spec field | Description |
| --- | --- |
| `bigquery` | bigquery configures BigQuery relational export (ADR-0420). |
| `cluster` | cluster labels exported inventory in multi-cluster installs. |
| `connectionTest` | connectionTest enables connectivity checks on create/update (default true). |
| `endpoint` | endpoint is the backend-specific destination (URL, bucket, and so on). |
| `exportMinInterval` | exportMinInterval is the default minimum time between identical exports when an inventory |
| `layout` | layout configures document shape and folder layout for snapshot Git/GitLab sinks (ADR-0419). |
| `mongodb` | mongodb configures MongoDB document upsert export (ADR-0417). |
| `options` | options carries non-secret, backend-specific pass-through settings (ADR-0416 §4, Option 2). |

Full reference: [KollectDatabaseSink](crds/kollectdatabasesink.md).

### `KollectEventSink` (namespaced)

KollectEventSink is the Schema for event export sinks.

| Spec field | Description |
| --- | --- |
| `cluster` | cluster labels exported inventory in multi-cluster installs. |
| `connectionTest` | connectionTest enables connectivity checks on create/update (default true). |
| `endpoint` | endpoint is the backend-specific destination (URL, bucket, and so on). |
| `exportMinInterval` | exportMinInterval is the default minimum time between identical exports when an inventory |
| `kafka` | kafka configures Kafka inventory change events. |
| `layout` | layout configures document shape and folder layout for snapshot Git/GitLab sinks (ADR-0419). |
| `nats` | nats configures NATS JetStream inventory change events. |
| `options` | options carries non-secret, backend-specific pass-through settings (ADR-0416 §4, Option 2). |

Full reference: [KollectEventSink](crds/kollecteventsink.md).

### `KollectInventory` (namespaced)

KollectInventory is the Schema for the kollectinventories API

| Spec field | Description |
| --- | --- |
| `databaseSinkRefs` | databaseSinkRefs lists KollectDatabaseSink names in this namespace (ADR-0414). |
| `eventSinkRefs` | eventSinkRefs lists KollectEventSink names in this namespace (ADR-0414). |
| `exportMinInterval` | exportMinInterval is the minimum time between identical exports for this inventory. |
| `httpEndpoint` | httpEndpoint exposes a read-only inventory summary over HTTP when enabled. |
| `maxExportBytes` | maxExportBytes caps the marshalled namespace payload for export and HTTP (optional). |
| `snapshotSinkRefs` | snapshotSinkRefs lists KollectSnapshotSink names in this namespace (ADR-0414). |
| `suspend` | suspend pauses reconciliation of this inventory when set to true. |

Full reference: [KollectInventory](crds/kollectinventory.md).

### `KollectProfile` (namespaced)

KollectProfile is the Schema for the kollectprofiles API

| Spec field | Description |
| --- | --- |
| `attributes` | attributes lists the values to extract from each matching resource. |
| `export` | export optionally enables full-resource export with path pruning (ADR-0306). |
| `metrics` | metrics lists kube-state-metrics-style Prometheus series emitted on operator /metrics. |
| `targetGVK` | targetGVK selects the Kubernetes resource kind this profile applies to. |

Full reference: [KollectProfile](crds/kollectprofile.md).

### `KollectScope` (namespaced)

KollectScope is a namespaced governance boundary for targets, inventories, and sinks.

| Spec field | Description |
| --- | --- |
| `allowedGVKs` | allowedGVKs restricts which target resource kinds may be collected in this scope. |
| `allowedNamespaces` | allowedNamespaces restricts which workload namespaces may be collected. |
| `databaseSinkRefs` | databaseSinkRefs lists permitted KollectDatabaseSink names for this scope. |
| `deniedNamespaces` | deniedNamespaces is a platform blacklist; Target intent cannot override (D8). |
| `eventSinkRefs` | eventSinkRefs lists permitted KollectEventSink names for this scope. |
| `minExportInterval` | minExportInterval is a tenancy floor — inventory and sink intervals below this value are rejected. |
| `snapshotSinkRefs` | snapshotSinkRefs lists permitted KollectSnapshotSink names for this scope. |

Full reference: [KollectScope](crds/kollectscope.md).

### `KollectSnapshotSink` (namespaced)

KollectSnapshotSink is the Schema for snapshot export sinks.

| Spec field | Description |
| --- | --- |
| `cluster` | cluster labels exported inventory in multi-cluster installs. |
| `connectionTest` | connectionTest enables connectivity checks on create/update (default true). |
| `endpoint` | endpoint is the backend-specific destination (URL, bucket, and so on). |
| `exportMinInterval` | exportMinInterval is the default minimum time between identical exports when an inventory |
| `git` | git configures git sink settings when type is git. |
| `gitlab` | gitlab configures GitLab-specific settings when type is gitlab. |
| `http` | http configures webhook snapshot export when type is http. |
| `layout` | layout configures document shape and folder layout for snapshot Git/GitLab sinks (ADR-0419). |

Full reference: [KollectSnapshotSink](crds/kollectsnapshotsink.md).

### `KollectTarget` (namespaced)

KollectTarget is the Schema for the kollecttargets API

| Spec field | Description |
| --- | --- |
| `excludedNamespaces` | excludedNamespaces is a static namespace denylist applied after include logic. |
| `includedNamespaces` | includedNamespaces is a static namespace allowlist. Empty means no extra restriction |
| `labelSelector` | labelSelector restricts collection to resources matching the selector. |
| `names` | names optionally restricts collection to resources with these names. |
| `namespaceExcludeSelector` | namespaceExcludeSelector excludes namespaces whose labels match the selector. |
| `namespaceSelector` | namespaceSelector restricts collection to namespaces matching the selector. |
| `profileRef` | profileRef is the name of a KollectProfile in the same namespace as this Target. |
| `resourceRules` | resourceRules declares GVK-scoped collection rules. When empty, collection falls back to |

Full reference: [KollectTarget](crds/kollecttarget.md).

<!-- END AUTO-CRD -->

## See also

- [CR reference](CR-REFERENCE.md) — per-kind fields, conditions, failure modes
- [FAQ](FAQ.md) — symptom-oriented answers
- [Data flows](DATA-FLOWS.md) — collection and export diagrams
