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
| **Sink** | A `KollectSink` — **where to send** inventory: Git, GitLab, S3, GCS, Postgres, Kafka, or NATS endpoint configuration in the tenant namespace. |
| **Scope** | A `KollectScope` — optional **policy gate** limiting allowed GVKs, namespaces, and sinks for multitenant installs. Enforced at admission, not reconciled. |
| **Connection test** | A `KollectConnectionTest` or `spec.connectionTest` on `KollectSink` — probes sink reachability and sets `ConnectionVerified` ([ADR-0403](adr/0403-connection-test.md)). |
| **GVK** | Group, Version, Kind — identifies the Kubernetes API type a profile collects (for example `apps/v1` `Deployment`). |
| **Attribute** | Named extraction rule on a profile: maps a logical key (for example `image`) to a JSONPath or `cel:` expression. |
| **Snapshot** | The canonical in-memory inventory artifact per `KollectInventory` before any sink write. All exports are projections of this snapshot ([ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md)). |
| **Projection** | A sink-specific rendering of the snapshot — Parquet on S3, JSON in Git, upsert rows in Postgres, or events on NATS/Kafka. |
| **Export debounce** | Minimum interval between identical exports **per sink ref** (`exportMinInterval` precedence); reduces write amplification ([ADR-0413](adr/0413-export-interval-scheduling.md), [ADR-0603](adr/0603-performance-scalability.md)). |

## Sink roles

| Term | Definition |
| --- | --- |
| **Snapshot store** | Sink role that writes **whole current state** each cycle (Git, S3/GCS Parquet, HTTP). Deletes are implicit — absent resources are not in the latest file ([ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md)). |
| **SoR (system of record)** | Relational **state store** — typically Postgres — answering *what is deployed now?* with queryable rows. Requires **delete reconciliation** when resources disappear. |
| **Event emitter** | Sink role that publishes **change streams** (NATS JetStream, Kafka) for downstream consumers; tombstones and materialized views are consumer-owned. |
| **Delete reconciliation** | Postgres-specific logic to remove rows when UIDs leave the snapshot; snapshot stores get correct deletes for free. |

## Multi-cluster

| Term | Definition |
| --- | --- |
| **Hub** | Central cluster that **ingests** inventory events from spoke clusters, optionally merges, and exports to shared backends. Helm `| **Spoke** | Workload cluster running the operator in **single** or **tenant** mode; pushes or exports inventory toward hub-configured sinks or transport. |
| **Cluster profile / target / inventory** | Cluster-scoped counterparts (`KollectClusterProfile`, `KollectClusterTarget`, `KollectClusterInventory`) for cross-namespace rollup and platform-wide export. |
| **Remote cluster** | Hub registration object (`| **Transport** | Pluggable queue between spoke emitters and hub ingest: in-process (dev), Redis, NATS, or Kafka (ADR-0502). |

## Runtime and ops

| Term | Definition |
| --- | --- |
| **Reconciled CRD** | Kind with an active controller loop updating `status` (targets, inventories, connection tests). Contrast **static** profiles/scopes ([ADR-0202](adr/0202-static-vs-reconciled.md)). |
| **Watch mode** | `KollectTarget.spec.watchMode`: `All` (default, respect opt-out) or `OptIn` (only `kollect.dev/watch=enabled`). See [ADR-0205](adr/0205-watch-labels.md). |
| **Tenant mode** | Helm values restricting the operator informer cache and RBAC to listed namespaces ([ADR-0203](adr/0203-namespaced-multi-tenancy.md)). |
| **Leader election** | HA mode where only one manager pod reconciles; required for production multi-replica Deployments (ADR-0504). |

<!-- BEGIN AUTO-CRD -->

## Custom resources (from CRD schema)

Auto-generated from `config/crd/bases/` OpenAPI descriptions. Regenerate with
`python3 hack/gen-glossary.py`. Field-level detail: [CR reference](CR-REFERENCE.md).

### `KollectClusterInventory` (cluster)

KollectClusterInventory rolls up cluster targets for platform operators.

| Spec field | Description |
| --- | --- |
| `dedupe` | dedupe selects how overlapping target rows collapse on export (ADR-0305). |
| `exportMinInterval` | exportMinInterval is the minimum time between identical exports for this inventory. |
| `namespaceSelector` | namespaceSelector restricts rollup to namespaces matching the selector. |
| `profileRef` | profileRef names a KollectClusterProfile stub for shared extraction schema (optional). |
| `sinkNamespace` | sinkNamespace is the namespace where namespaced KollectSink objects are resolved. |
| `sinkRefs` | sinkRefs lists KollectSink names resolved in sinkNamespace. |
| `suspend` | suspend pauses reconciliation when set to true. |
| `targetRefs` | targetRefs lists KollectClusterTarget names to aggregate. |

Full reference: [KollectClusterInventory](crds/kollectclusterinventory.md).

### `KollectClusterProfile` (cluster)

KollectClusterProfile is the Schema for platform-wide shared extraction schemas.

| Spec field | Description |
| --- | --- |
| `attributes` | attributes lists the values to extract from each matching resource. |
| `metrics` | metrics lists kube-state-metrics-style Prometheus series emitted on operator /metrics. |
| `targetGVK` | targetGVK selects the Kubernetes resource kind this profile applies to. |

Full reference: [KollectClusterProfile](crds/kollectclusterprofile.md).

### `KollectClusterTarget` (cluster)

KollectClusterTarget selects resources cluster-wide for platform operators.

| Spec field | Description |
| --- | --- |
| `namespaceSelector` | namespaceSelector restricts collection to namespaces matching the selector. |
| `profileRef` | profileRef names a KollectClusterProfile or a platform-namespace KollectProfile stub. |
| `suspend` | suspend pauses reconciliation when set to true (reserved for future controller). |

Full reference: [KollectClusterTarget](crds/kollectclustertarget.md).

### `KollectConnectionTest` (namespaced)

KollectConnectionTest triggers an audited one-shot sink connectivity probe.

| Spec field | Description |
| --- | --- |
| `ownerSink` | ownerSink sets an ownerReference to the sink when true (default). |
| `profileRef` | profileRef optionally names a KollectProfile for future composite probes. |
| `sinkRef` | sinkRef is the name of a KollectSink in the same namespace. |
| `ttlSecondsAfterFinished` | ttlSecondsAfterFinished deletes the CR after status.completed plus this TTL. |

Full reference: [KollectConnectionTest](crds/kollectconnectiontest.md).

### `KollectInventory` (namespaced)

KollectInventory is the Schema for the kollectinventories API

| Spec field | Description |
| --- | --- |
| `exportMinInterval` | exportMinInterval is the minimum time between identical exports for this inventory. |
| `httpEndpoint` | httpEndpoint exposes a read-only inventory summary over HTTP when enabled. |
| `maxExportBytes` | maxExportBytes caps the marshalled namespace payload for export and HTTP (optional). |
| `sinkRefs` | sinkRefs lists KollectSink names in the same namespace as this Inventory. |
| `suspend` | suspend pauses reconciliation of this inventory when set to true. |

Full reference: [KollectInventory](crds/kollectinventory.md).

### `KollectProfile` (namespaced)

KollectProfile is the Schema for the kollectprofiles API

| Spec field | Description |
| --- | --- |
| `attributes` | attributes lists the values to extract from each matching resource. |
| `metrics` | metrics lists kube-state-metrics-style Prometheus series emitted on operator /metrics. |
| `targetGVK` | targetGVK selects the Kubernetes resource kind this profile applies to. |

Full reference: [KollectProfile](crds/kollectprofile.md).

### `

| Spec field | Description |
| --- | --- |
| `apiServerURL` | apiServerURL is the spoke Kubernetes API server URL (optional for push-only spokes). |
| `clusterName` | clusterName is the stable DNS-1123 identifier for the spoke cluster. |
| `credentialsSecretRef` | credentialsSecretRef points to an Istio-style remote kubeconfig secret for optional hub pull. |
| `trustBundle` | trustBundle is a PEM-encoded CA bundle for spoke API TLS or future mTLS (optional). |

Full reference: [
### `KollectScope` (namespaced)

KollectScope is a namespaced governance boundary for targets, inventories, and sinks.

| Spec field | Description |
| --- | --- |
| `allowedGVKs` | allowedGVKs restricts which target resource kinds may be collected in this scope. |
| `allowedNamespaces` | allowedNamespaces restricts which workload namespaces may be collected. |
| `sinkRefs` | sinkRefs lists namespaced KollectSink names permitted for export from this scope. |

Full reference: [KollectScope](crds/kollectscope.md).

### `KollectSink` (namespaced)

KollectSink is the Schema for the kollectsinks API

| Spec field | Description |
| --- | --- |
| `cluster` | cluster labels exported inventory in multi-cluster installs. |
| `connectionTest` | connectionTest requests a connectivity check on create/update when true. |
| `endpoint` | endpoint is the backend-specific destination (URL, bucket, and so on). |
| `gitlab` | gitlab configures GitLab-specific settings when type is gitlab. |
| `kafka` | kafka configures a Kafka or Redpanda event sink. |
| `nats` | nats configures a NATS JetStream event sink. |
| `objectStore` | objectStore configures S3/GCS snapshot export format and layout. |
| `postgres` | postgres configures a PostgreSQL database sink. |

Full reference: [KollectSink](crds/kollectsink.md).

### `KollectTarget` (namespaced)

KollectTarget is the Schema for the kollecttargets API

| Spec field | Description |
| --- | --- |
| `labelSelector` | labelSelector restricts collection to resources matching the selector. |
| `names` | names optionally restricts collection to resources with these names. |
| `namespaceSelector` | namespaceSelector restricts collection to namespaces matching the selector. |
| `profileRef` | profileRef is the name of a KollectProfile in the same namespace as this Target. |
| `suspend` | suspend pauses reconciliation of this target when set to true. |
| `watchMode` | watchMode controls namespace/resource watch opt-in vs opt-out (ADR-0205). |

Full reference: [KollectTarget](crds/kollecttarget.md).

<!-- END AUTO-CRD -->

## See also

- [CR reference](CR-REFERENCE.md) — per-kind fields, conditions, failure modes
- [FAQ](FAQ.md) — symptom-oriented answers
- [Data flows](DATA-FLOWS.md) — collection and export diagrams
