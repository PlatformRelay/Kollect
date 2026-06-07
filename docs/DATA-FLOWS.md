# Kollect data flows

Visual walkthroughs of how data moves through the operator. For CRD roles see
[ARCHITECTURE.md](ARCHITECTURE.md); for locked decisions see
[PLATFORM-DECISIONS.md](PLATFORM-DECISIONS.md).

!!! note "Sink roles on export paths"
    Export fans out to whatever `sinkRefs` name — each sink has a **role** (snapshot store, relational
    SoR, or event emitter), not a fixed Postgres+Kafka pair
    ([ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md)). Diagrams below may show Postgres and
    Kafka as examples; hub and inventory export use the same role-based contract for all seven shipped
    types.

---

## 1. Export debouncing

**Problem:** Event-driven informers can fire hundreds of updates per minute. Without coalescing,
every watch event would trigger a Postgres upsert or Git commit.

**Design:** The in-memory collect store updates **immediately** on every target reconcile. Only the
**sink export** step is debounced **per sink ref** on `KollectInventory` ([ADR-0413](adr/0413-export-interval-scheduling.md)).
One payload is marshalled per reconcile; each ref exports when its effective interval elapses or the
checksum/generation bypass rules fire for that sink.

### Per-inventory state machine

```mermaid
flowchart TD
  Start([Inventory reconcile]) --> Scope{In KollectScope?}
  Scope -->|no / not enforced| SinkOK{Sink reachable?}
  Scope -->|violation| Degrade[Degraded — no export]
  SinkOK -->|no| Degrade
  SinkOK -->|yes| Marshal[Marshal namespace payload]
  Marshal --> Size{Within maxExportBytes?}
  Size -->|no| Degrade
  Size -->|yes| Hash[Compute payload checksum]
  Hash --> Gen{generation changed?}
  Gen -->|yes| Export[Export to sinks now]
  Gen -->|no| Chk{checksum changed?}
  Chk -->|yes| Export
  Chk -->|no| Interval{elapsed ≥ exportMinInterval?}
  Interval -->|yes| Export
  Interval -->|no| Wait[RequeueAfter remainder]
  Export --> Status[Update status: lastExportTime, conditions]
  Wait --> Start
```

### Timing example (default `exportMinInterval: 30s`)

```mermaid
sequenceDiagram
  participant API as Kubernetes API
  participant Store as Collect store
  participant Inv as KollectInventory
  participant PG as Postgres sink

  API->>Store: Deployment image patch (t=0s)
  Note over Store: row updated immediately
  Store->>Inv: trigger reconcile
  Inv->>PG: export (checksum changed)

  API->>Store: unrelated resync (t=5s)
  Store->>Inv: trigger reconcile
  Note over Inv: same checksum — debounced
  Inv-->>Inv: requeue ~25s

  API->>Store: second patch (t=12s)
  Store->>Inv: trigger reconcile
  Inv->>PG: export (checksum changed — bypass interval)

  Note over Inv,PG: identical payload at t=40s → export allowed (30s elapsed)
```

### Configuration

Effective interval per sink ref ([ADR-0413](adr/0413-export-interval-scheduling.md)):

```text
effectiveInterval(ref) =
  max(
    ref.exportMinInterval ?? sink.exportMinInterval ?? inventory.exportMinInterval ?? 30s,
    scope.minExportInterval ?? 0s
  )
```

| Field | Default | Effect |
| --- | --- | --- |
| `spec.sinkRefs[].exportMinInterval` | — | Per-sink override (string refs inherit inventory default) |
| `KollectSink.spec.exportMinInterval` | — | Sink default when ref and inventory omit override |
| `KollectInventory.spec.exportMinInterval` | **30s** (CRD default) | Inventory-wide default for plain string refs |
| `KollectScope.spec.minExportInterval` | — | Tenancy floor — webhook rejects intervals below this |
| `metadata.generation` bump | — | Immediate export to **all** sinks (spec edit) |
| Payload checksum change | — | Immediate export to **that** sink (material change) |
| `exportMinInterval: 0s` | — | Material-change only; controller requeues with 30s watchdog |

!!! tip "Dual-cadence fan-out"
    Portal Postgres at **30s** plus Git audit at **1h** is the canonical multi-role pattern — see
    `config/samples/kollect_v1alpha1_kollectinventory.yaml`
    and [deployment-inventory example](examples/deployment-inventory.md#step-4-kollectinventory).

When some sinks export and others are debounced, aggregate `Synced=False` with reason
**`PartiallySynced`**; per-sink detail lives in `status.sinkExports[]`.

---

## 2. Collection pipeline

How a watched object becomes an inventory row.

![Left-to-right operator pipeline from Kubernetes API through shared per-GVK informers and an in-memory collect store, KollectInventory debounce, to fan-out sink projections for Git, object store, and Postgres.](assets/illustrations/how-it-works-informer-sink-dark.webp){ .kollect-illus .kollect-illus--wide width="800" }

```mermaid
flowchart LR
  subgraph api [Kubernetes API]
    Obj[Deployment / CRD / …]
  end

  subgraph operator [kollect operator]
    Inf[Shared informer<br/>one per GVK]
    Tgt[KollectTarget<br/>reconciler]
    Prof[KollectProfile<br/>schema]
    Ext[Extractor<br/>JSONPath / CEL]
    Store[(Collect store<br/>per namespace)]
    Inv[KollectInventory]
  end

  Obj -->|watch| Inf
  Inf -->|enqueue| Tgt
  Prof -.->|profileRef| Tgt
  Tgt -->|label/NS filter| Ext
  Obj -.->|cached object| Ext
  Ext -->|attribute map| Store
  Store --> Inv
```

**Key properties:**

- **One informer per GVK** across all targets ([ADR-0301](adr/0301-event-driven-informers.md)).
- Targets only differ by **namespace/label selectors** and **profileRef**.
- Extraction runs on the **cached unstructured object** — no per-target API list calls.

### Collection filter layers

Before a watched object reaches the collect store, it passes through stacked policy layers — Helm
watch boundary, Scope denials, Target include/exclude intent, `resourceRules`, CEL `matchPolicy`, and
watch labels ([ADR-0207](adr/0207-target-collection-filtering.md)).

![Stacked filter layers showing how Kubernetes resources pass through operator watch scope, Scope denials, Target include and exclude rules, resource rules, CEL match policy, and watch labels before becoming inventory rows.](assets/illustrations/collection-filter-layers-dark.webp){ .kollect-illus .kollect-illus--portrait width="360" }

---

## 3. Attribute extraction (JSONPath arrays)

`KollectProfile` attributes are evaluated per object. Single-index paths return a scalar; wildcard
paths return a **JSON array** in the export row.

```mermaid
flowchart TD
  Obj[Unstructured object] --> Path{Path type?}
  Path -->|CEL expression| CEL[CEL evaluator]
  Path -->|JSONPath wildcard| JP[kubectl JSONPath]
  CEL --> Val[Go value]
  JP --> Matches{match count}
  Matches -->|1| Scalar[scalar in row]
  Matches -->|many matches| List[array in row]
  Matches -->|0| Opt{optional?}
  Opt -->|yes| Skip[omit attribute]
  Opt -->|no| Null[null in row]
```

**Deployment containers example:**

| Path | Result for 2-container pod |
| --- | --- |
| `$.spec.template.spec.containers[0].image` | `"app:1.0"` (string) |
| `$.spec.template.spec.containers[*].image` | `["app:1.0", "sidecar:2.0"]` (list) |

See [ADR-0302](adr/0302-cel-jsonpath-extraction.md) for syntax rules.

---

## 4. `KollectScope` enforcement gate

Static scope object; enforced at **target** and **inventory** reconcile time (hard degrade).

```mermaid
flowchart TD
  Tgt[KollectTarget reconcile] --> Load[Load KollectScope in namespace]
  Load --> Enforced{scope exists?}
  Enforced -->|no| Collect[Register watch + collect]
  Enforced -->|yes| GVK{profile GVK in allowedGVKs?}
  GVK -->|no| DenyT[Degraded ScopeGVKDenied]
  GVK -->|yes| NS{workload NS in allowedNamespaces?}
  NS -->|no| DenyT
  NS -->|yes| Collect

  Inv[KollectInventory reconcile] --> Load2[Load KollectScope]
  Load2 --> Sinks{sinkRefs ⊆ scope.sinkRefs?}
  Sinks -->|no| DenyI[Degraded ScopeSinkDenied]
  Sinks -->|yes| Export[Continue export path]
```

Example: [ADR-0203](adr/0203-namespaced-multi-tenancy.md#enforcement-example-gvk-denied).

---

## 5. `KollectConnectionTest` lifecycle

One-shot probe CR for audited connectivity checks.

```mermaid
stateDiagram-v2
  [*] --> Pending: CR created
  Pending --> Probing: reconciler starts
  Probing --> Succeeded: sink OK
  Probing --> Failed: sink error
  Succeeded --> TTL: status.completed
  Failed --> TTL
  TTL --> Deleted: after ttlSecondsAfterFinished
  Pending --> Probing: spec change (generation bump)
  Succeeded --> Probing: spec change re-probe
  Failed --> Probing: spec change re-probe
```

Default TTL: **300s**. Patch `spec.sinkRef` to force a fresh probe.

---

## 6. Hub merge and multi-cluster ingest

Spokes run the **same operator image** with `in-memory store keyed by **`(cluster, namespace, name, uid)`** ([ADR-0501](adr/0501-multi-cluster-fleet.md),
ADR-0503).

### Spoke → hub transport

```mermaid
flowchart LR
  subgraph spoke [Spoke cluster]
    Tgt[KollectTarget<br/>reconciler]
    Store[(Collect store)]
    Pub[spoke.TryPublishReport]
    Tgt --> Store
    Store --> Pub
  end

  subgraph hub [Hub cluster]
    HTTP[HTTP ingest<br/>POST /hub/v1alpha1/reports]
    Queue[Queue consumer<br/>Redis / NATS]
    Recv[ReceiveReport]
    Merge[Merger.Apply]
    HubStore[(Hub collect store)]
    HTTP --> Recv
    Queue --> Recv
    Recv --> Merge
    Merge --> HubStore
  Export[Parallel sink export<br/>per sinkRefs roles]
  HubStore --> Export
  end

  Pub -->|Bearer + X-Kollect-Cluster-Id| HTTP
  Pub -->|cluster_id metadata| Queue
```

| Channel | Identity | Registration gate |
| --- | --- | --- |
| **HTTP push** (default) | `TokenReview` + SAR **`create`** on `kollectremoteclusters` | `KOLLECT_REMOTE_CLUSTERS` allowlist |
| **Queue** (Phase 2) | `cluster_id` wire field / header + TLS | Same allowlist; broker ACL is operator-owned |

### HTTP ingest sequence

```mermaid
sequenceDiagram
  participant Spoke as Spoke operator
  participant Hub as Hub ingest HTTP
  participant Auth as TokenReview + SAR
  participant Recv as ReceiveReport
  participant Merge as Merger
  participant Store as Hub collect store

  Spoke->>Hub: POST /hub/v1alpha1/reports<br/>Authorization: Bearer token<br/>X-Kollect-Cluster-Id: spoke-a
  Hub->>Auth: validate token + create on kollectremoteclusters
  Auth-->>Hub: allowed
  Hub->>Recv: body + header cluster id
  Recv->>Recv: cluster id match + ACL check
  Recv->>Merge: SpokeReport
  Merge->>Store: upsert items / remove UIDs
  Store-->>Hub: ok
  Hub->>Hub: export to hub sinks (parallel)
  Hub-->>Spoke: 202 Accepted
```

### Merge semantics

```mermaid
flowchart TD
  Report[SpokeReport JSON] --> Wire{X-Kollect-Cluster-Id<br/>matches body.cluster?}
  Wire -->|no| Reject[400 Bad Request]
  Wire -->|yes| ACL{cluster in<br/>KOLLECT_REMOTE_CLUSTERS?}
  ACL -->|no when enforced| Reject
  ACL -->|yes / dev open| Apply[Merger.Apply]
  Apply --> Upsert[Upsert items<br/>key: cluster + target + uid]
  Apply --> Remove[Remove removedUIDs]
  Upsert --> HubExport[Parallel sink export]
  Remove --> HubExport
  HubExport --> SoR[Relational SoR<br/>e.g. Postgres]
  HubExport --> Emit[Event emitter<br/>e.g. NATS / Kafka]
  HubExport --> Snap[Snapshot store<br/>e.g. Git / S3]
```

**Post-merge export:** hub consumer resolves namespaced `KollectSink` objects from
`KOLLECT_HUB_EXPORT_NAMESPACE` + `KOLLECT_HUB_SINK_REFS`, marshals the merged target inventory
(`cluster` + `inventoryRef.name`), and fans out to **each configured sink in parallel** — typically
a relational SoR (Postgres) plus optional snapshot or event sinks — using the same payload contract
as namespaced `KollectInventory` export ([ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md)).

**Idempotency:** duplicate reports with the same `(cluster, namespace, name, uid)` overwrite the
stored row; `removedUIDs` tombstones delete stale rows. At-least-once delivery is safe.

**Status:** successful ingest marks `in the hub platform namespace.

### Spoke queue publish (Phase 2)

When `transport: queue` is configured on the spoke, `spoke.TryPublishReport` enqueues a
`SpokeReport` JSON message instead of (or in addition to) HTTP push. The hub consumer drains the
broker and calls the same `ReceiveReport` → `Merger.Apply` path as HTTP ingest.

```mermaid
sequenceDiagram
  participant Tgt as KollectTarget reconciler
  participant Store as Collect store
  participant Pub as spoke.TryPublishReport
  participant Broker as Redis / NATS
  participant Cons as Hub queue consumer
  participant Recv as ReceiveReport
  participant Merge as Merger
  participant HubStore as Hub collect store

  Tgt->>Store: object add/update/delete
  Store->>Pub: namespace delta ready
  Pub->>Pub: marshal SpokeReport + cluster_id
  Pub->>Broker: publish (TLS + ACL)
  Broker->>Cons: deliver (at-least-once)
  Cons->>Recv: body + cluster_id metadata
  Recv->>Recv: ACL + idempotency check
  Recv->>Merge: SpokeReport
  Merge->>HubStore: upsert / tombstone UIDs
```

| Setting | Role |
| --- | --- |
| `KOLLECT_SPOKE_CLUSTER` | Wire `cluster_id` and report body `cluster` field |
| `KOLLECT_TRANSPORT` | `http` (default) or `queue` on spoke |
| Broker URL / credentials | Operator-owned secret; not in CR spec |
| `KOLLECT_REMOTE_CLUSTERS` | Hub allowlist — same gate as HTTP ingest |

Queue delivery is **at-least-once**; merge keys `(cluster, namespace, name, uid)` make replays safe.

### Post-merge hub export

After merge, hub `KollectInventory` objects use the **same per-sink debounced export path** as
single-cluster mode ([§1](#1-export-debouncing)): marshal once, then fan out with per-ref intervals.
Hub env `KOLLECT_HUB_SINK_REFS` remains comma-separated names — structured hub intervals deferred
([ADR-0413](adr/0413-export-interval-scheduling.md)).

```mermaid
flowchart TD
  HubStore[(Hub collect store<br/>key: cluster + target + uid)]
  HubInv[Hub KollectInventory<br/>reconciler]
  Scope{KollectScope<br/>in export NS?}
  Marshal[Marshal multi-cluster payload]
  Debounce{checksum / interval}
  Export[Export to sinkRefs]
  PG[(Postgres / Git / …)]

  HubStore --> HubInv
  HubInv --> Scope
  Scope -->|denied| Degrade[Degraded — no export]
  Scope -->|ok| Marshal
  Marshal --> Debounce
  Debounce -->|material change or interval| Export
  Debounce -->|unchanged| Wait[RequeueAfter]
  Export --> Sinks[(Configured sinks)]
  Export --> Status[status.lastExportTime + conditions]
```

Multi-cluster rows include a **`cluster`** dimension in each item so downstream consumers can filter
or partition by spoke. Hub export does **not** persist full payloads in CR `status` — only summaries
and last-export metadata ([ADR-0103](adr/0103-etcd-limit.md)).

### Configuration

| Env / setting | Role |
| --- | --- |
| `KOLLECT_SPOKE_CLUSTER` | Spoke identity; enables publish |
| `KOLLECT_HUB_INGEST_AUTH_MODE` | `kubernetes` (default) or `disabled` (dev/CI) |
| `KOLLECT_PLATFORM_NAMESPACE` | Namespace for ingest SAR on `kollectremoteclusters` |
| `KOLLECT_REMOTE_CLUSTERS` | Hub registration allowlist (comma-separated `spec.clusterName` values); **set (even empty) = fail-closed** |
| `KOLLECT_HUB_EXPORT_NAMESPACE` | Namespace for hub `KollectSink` resolution (Helm `hub.exportNamespace`) |
| `KOLLECT_HUB_SINK_REFS` | Comma-separated hub sink names for parallel export (Helm `hub.sinkRefs`) |

See ADR-0503 for RBAC grants and Istio-style remote
secret registration.

---

## See also

- [ARCHITECTURE.md](ARCHITECTURE.md) — CRD model and deployment defaults
- [PERFORMANCE.md](PERFORMANCE.md) — metrics and tuning
- [examples/deployment-inventory.md](examples/deployment-inventory.md) — end-to-end walkthrough
