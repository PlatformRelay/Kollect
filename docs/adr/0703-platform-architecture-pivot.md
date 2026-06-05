# ADR-0703: Platform architecture pivot — decision log

> A running log of the 2026-06-05 platform pivot. Authoritative current-state for each topic lives in
> the relevant theme ADR; this record preserves the reasoning and the batch of decisions together.

**Theme:** 07 · Project & meta · **Status:** Current (decision log)

## Context

Architecture review with the product owner clarified priorities before wider implementation:

- No external adopters on `v1alpha1` — **breaking API changes are acceptable** when tenancy or
  scalability require them.
- **Phases are build order**, not release milestones; nothing is “published” until the tree is
  beta-quality. Do not gate work on faux release trains.
- Implemented code (HTTP API, hub types, Postgres/Kafka sinks, etc.) **stays**; **narrative and
  priority** change.

## Decision

### 1. MVP focus (build order, not a release)

**MVP path:** namespaced Profile → Target → Inventory → **namespaced Sink** → export (Postgres or
Kafka preferred walkthrough; Git sample remains for determinism tests).

Extended work (scope hardening, all samples, hub merge, optional HTTP) continues in parallel but
does not block “MVP complete” for the owner.

### 2. Namespaced-by-default tenancy

| Kind | Scope | Cluster variant (later) |
| --- | --- | --- |
| `KollectProfile` | Namespace | `KollectClusterProfile` |
| `KollectSink` | **Namespace** (breaking — was cluster) | `KollectClusterSink` |
| `KollectTarget` | Namespace | **`KollectClusterTarget`** |
| `KollectInventory` | Namespace | `KollectClusterInventory` |
| `KollectScope` | Namespace | `KollectClusterScope` |

`profileRef` and `sinkRefs` on **namespaced** kinds resolve in the **same namespace** as the
referencing object.

**`KollectClusterTarget`** (cluster-scoped) — for platform teams that manage collection **across
namespaces** from one object (ESO `ClusterSecretStore` / cluster-wide operator pattern):

| Field | Rule |
| --- | --- |
| `spec.profileRef` | Names a **`KollectClusterProfile`** or a **`KollectProfile`** in a fixed platform namespace (webhook-validated) |
| `spec.namespaceSelector` | Required for cluster targets — which workload namespaces to scan |
| `spec.labelSelector` / `names` | Same as namespaced Target |
| Export rollup | Feeds **`KollectClusterInventory`** when implemented — pairs with cluster target; **no** namespaced `inventoryRef` hack |

Namespaced **`KollectTarget`** remains the **default** for `tenantMode` per-team installs.
**`KollectClusterTarget`** is for cluster-scoped platform operators — not the per-team MVP path.

**Default deployment:** per-team Helm with **`tenantMode: true`** and **`watchNamespaces`** set.
Cluster operator + **`KollectClusterTarget`** supported for platform-wide collection.

### 3. Sink narrative — Postgres/Kafka primary; Git audit

> **Reframed by [ADR-0401](0401-sink-taxonomy-state-vs-stream.md) (2026-06-05):** there is no single
> "primary" sink. Sinks are **role-based** — state stores (Git/object store, Postgres) vs event
> emitters (NATS default, Kafka opt-in); the in-memory snapshot is canonical and every sink is a
> projection. The historical framing below is retained for the decision log.

- **Primary integration** (as decided at pivot time) for portals and automation: **Postgres** and/or
  **Kafka** export.
- **Git** is an **audit/diff** sink and CI fixture — not the documented happy path for live query
  at hub scale across many clusters.
- Hub-scale read path: **merged store in Postgres/Kafka at hub**, not Git clones per spoke.

### 4. HTTP inventory — optional debug / small install

- Spoke HTTP inventory is **feature-gated** (`featureGates.inventoryHttp.enabled`, default **false**).
- **Not** a core product requirement; **debug and small single-cluster installs** only.
- **Scalable portal read** uses **sink export** (Postgres/Kafka). **Hub** may expose a read API
  backed by the merged store in a later build step — not spoke in-memory HTTP at fleet scale.
- Supersedes “HTTP core Phase 1” wording in [ADR-0103](0103-etcd-limit.md) intent (amend that ADR).

### 5. No `KollectHub` CRD

- Hub is **`mode: hub` on the same image** + Helm values (`transport`, shard count, ingest flags).
- **`internal/hub/`** merge library is the reusable core; no CRD spawns hub Deployments.
- **`KollectHub` API types removed** in v1alpha1 cleanup — hub/spoke is Helm `mode` only; not on the active roadmap.
- Supersedes hub CRD phasing in [ADR-0501](0501-multi-cluster-sync-rfc.md) (amend that RFC).

### 6. `KollectConnectionTest` CR — **accepted**

Supersedes [ADR-0403](0403-connection-test.md) rejection.

| Mechanism | Use |
| --- | --- |
| **`KollectConnectionTest`** (namespaced) | Audited one-shot or CI probes: `spec.sinkRef`, optional `spec.profileRef`, optional SAR check |
| **`spec.connectionTest` on Sink** | Samples/CI only; prod default `false` |
| **Annotation `kollect.dev/test-connection`** | Quick re-test on Sink (kept) |

Status on `KollectConnectionTest`: conditions with latency, last result, sanitized errors.
TTL or ownerRef garbage-collection policy is an implementation detail.

### 7. Shared informer per GVK — locked

One dynamic informer per distinct GVK across all Targets ([ADR-0301](0301-event-driven-informers.md)).
Per-Target informer caches **rejected** for scalability.

### 8. Watch opt-in / opt-out — both modes

Platform manages collection centrally; teams may **exclude** resources via
`kollect.dev/watch: disabled` ([ADR-0205](0205-watch-labels.md)). `KollectTarget.spec.watchMode`:
`All` (default) or `OptIn` — **offer both**, document trade-offs.

### 9. Helm release sample — Argo CD primary

Primary demo GVK for chart/version inventory: **`argoproj.io/v1alpha1` / `Application`**
(status sync history, chart revision fields) — not Flux `HelmRelease`.

Flux sample may remain as secondary. Plain `helm.sh/v1` Secret still deferred until `helm:` decode.

**Contract test** (required, **first**): `internal/collect/argo_application_contract_test.go` +
golden `Application` fixture — locks chart/version JSONPath set and `status.history` newest-first
ordering. Samples: `config/samples/kollect_v1alpha1_kollectprofile_argo-application-summary.yaml`,
`config/samples/kollect_v1alpha1_kollecttarget_argo-applications.yaml`.

### 10. Export debouncing

Event-driven collection must not cause **export storms** to Git/Postgres/Kafka.

- Coalesce exports **per `KollectInventory`** via **`spec.exportMinInterval`** (duration; CRD default
  **`30s`** when unset).
- **Immediate export** when inventory payload materially changes (generation/checksum bump) even
  inside the min interval.
- Per-target collection updates in-memory store immediately; export is **debounced**.

### 11. Inventory location (design direction)

| Layer | Holds | Durability |
| --- | --- | --- |
| **Informer cache + collect store** | Live extracted rows per Target | Lost on pod restart; rebuilt via resync |
| **`KollectInventory.status`** | Counts, conditions, export refs only | etcd — never full payload ([ADR-0103](0103-etcd-limit.md)) |
| **Sink (Postgres/Kafka)** | **System of record** for portals/automation | Durable |
| **Optional spoke HTTP** | Snapshot of in-memory aggregate | Ephemeral debug only |

Hub merge reads spoke exports / push reports into **hub Postgres/Kafka** — not hub RAM mirror of
all clusters.

## Consequences

### Positive

- Coherent namespaced tenancy (Profile, Sink, Target, Inventory aligned).
- Clear scalability story: query sinks and hub store, not kube-apiserver or spoke HTTP.
- Breaking changes batched freely before beta.
- `KollectConnectionTest` improves audit and composite probes.

### Negative

- Multiple breaking API scope changes (Profile, Sink) require codegen + sample sweep.
- Argo-primary Helm sample excludes Flux-only shops until secondary sample exists.
- Debouncing adds latency to export freshness — tunable.

### 12. `KollectConnectionTest` garbage collection

Use **`spec.ttlSecondsAfterFinished`** (Job-style). **Default `300`** when unset. Controller deletes
the CR after TTL once `status.completed` is true. Manual delete remains valid for CI.

### 13. `KollectClusterTarget` rollup

**One `KollectClusterInventory`** aggregates **all** `KollectClusterTarget` objects (platform rollup
in a single CR). Optional `spec.targetRefs` narrows the set; empty/absent means **all cluster
targets**. If a deployment truly needs 1:1 target↔inventory, use `targetRefs: [that-target]` on the
same kind — do not introduce a separate per-target inventory CRD. Pairs with `KollectClusterProfile`
/ `KollectClusterSink`. No interim `inventoryRef` hack to namespaced `KollectInventory`.

### 14. Hub first milestone

Hub `mode: hub` ingest supports **Postgres and Kafka in parallel** from day one of multi-cluster
export — same merge lib, two sink adapters; spokes may push to either or both per `sinkRefs`.

### 14. Hub shard routing (no `KollectHub` CRD)

Shard assignment uses **`hash(clusterName) % shardCount`** configured via **Helm values or
environment** on hub Deployments (`mode: hub`). **No `KollectHub` CRD** — dynamic shard
registration is a Phase 2+ spike. See [ADR-0501](0501-multi-cluster-sync-rfc.md).

### 15. Build-order locks (session 4)

- **Secondary watches:** `KollectProfile` → enqueue Targets with `profileRef`; `KollectSink` →
  enqueue Inventories listing sink in `sinkRefs`.
- **Generic CRD demo:** `cert-manager.io/Certificate` + contract test.
- **`KollectClusterTarget`:** controller **deferred**; `profileRef` → `KollectProfile` in platform
  namespace (`platformNamespace` Helm value); **`namespaceSelector` required** (webhook).
- **`helm-release-values-redacted`:** deferred until operator **`scrubKeys[]`** export scrub.
- **GitLab sink:** **Phase 2** — `tls.caSecretRef` for enterprise internal CA; Git default for small
  single-cluster installs without Postgres/Kafka.
- **Hub ingest SAR:** **`create`** on `kollectremoteclusters` — [ADR-0503](0503-hub-cluster-auth-istio-pattern.md).

## Open questions

- **Deferred:** Hub federated mTLS behind external LB — [ADR-0503](0503-hub-cluster-auth-istio-pattern.md).

## Supersedes / amends

- [ADR-0403](0403-connection-test.md) — connection-test CR **now accepted** (0030 sink-only path remains supplementary).
- [ADR-0501](0501-multi-cluster-sync-rfc.md) — no `KollectHub` CRD.
- [ADR-0103](0103-etcd-limit.md) — HTTP not core.
- [ADR-0201](0201-crd-model.md) — namespaced `KollectSink`.
- [ADR-0303](0303-helm-release-inventory.md) — Argo `Application` primary over Flux.
