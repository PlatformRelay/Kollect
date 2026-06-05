# ADR-0032: Platform architecture pivot (2026-06-05)

## Status

Accepted (2026-06-05)

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

- **Primary integration** for portals and automation: **Postgres** and/or **Kafka** export.
- **Git** is an **audit/diff** sink and CI fixture — not the documented happy path for live query
  at 60+ clusters.
- Hub-scale read path: **merged store in Postgres/Kafka at hub**, not Git clones per spoke.

### 4. HTTP inventory — optional debug / small install

- Spoke HTTP inventory is **feature-gated** (`featureGates.inventoryHttp.enabled`, default **false**).
- **Not** a core product requirement; **debug and small single-cluster installs** only.
- **Scalable portal read** uses **sink export** (Postgres/Kafka). **Hub** may expose a read API
  backed by the merged store in a later build step — not spoke in-memory HTTP at fleet scale.
- Supersedes “HTTP core Phase 1” wording in [ADR-0006](0006-etcd-limit.md) intent (amend that ADR).

### 5. No `KollectHub` CRD

- Hub is **`mode: hub` on the same image** + Helm values (`transport`, shard count, ingest flags).
- **`internal/hub/`** merge library is the reusable core; no CRD spawns hub Deployments.
- Existing `KollectHub` API types may remain as **deprecated stubs** in the tree or be removed in a
  breaking cleanup — they are **not** the product surface and must not appear on the active roadmap.
- Supersedes hub CRD phasing in [ADR-0022](0022-multi-cluster-sync-rfc.md) (amend that RFC).

### 6. `KollectConnectionTest` CR — **accepted**

Supersedes [ADR-0030](0030-connection-test.md) rejection.

| Mechanism | Use |
| --- | --- |
| **`KollectConnectionTest`** (namespaced) | Audited one-shot or CI probes: `spec.sinkRef`, optional `spec.profileRef`, optional SAR check |
| **`spec.connectionTest` on Sink** | Samples/CI only; prod default `false` |
| **Annotation `kollect.dev/test-connection`** | Quick re-test on Sink (kept) |

Status on `KollectConnectionTest`: conditions with latency, last result, sanitized errors.
TTL or ownerRef garbage-collection policy is an implementation detail.

### 7. Shared informer per GVK — locked

One dynamic informer per distinct GVK across all Targets ([ADR-0014](0014-event-driven-informers.md)).
Per-Target informer caches **rejected** for scalability.

### 8. Watch opt-in / opt-out — both modes

Platform manages collection centrally; teams may **exclude** resources via
`kollect.dev/watch: disabled` ([ADR-0029](0029-watch-labels.md)). `KollectTarget.spec.watchMode`:
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

- Coalesce exports **per `KollectInventory`** via **`spec.exportMinInterval`** (duration; **default
  `30s`** when unset). **Not** a global manager flag — remove or ignore `--export-debounce` once
  per-Inventory field is wired.
- **Immediate export** when inventory payload materially changes (generation/checksum bump) even
  inside the min interval.
- Per-target collection updates in-memory store immediately; export is **debounced**.

### 11. Inventory location (design direction)

| Layer | Holds | Durability |
| --- | --- | --- |
| **Informer cache + collect store** | Live extracted rows per Target | Lost on pod restart; rebuilt via resync |
| **`KollectInventory.status`** | Counts, conditions, export refs only | etcd — never full payload ([ADR-0006](0006-etcd-limit.md)) |
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

**`KollectClusterInventory`** pairs with **`KollectClusterTarget`** (and `KollectClusterProfile` /
`KollectClusterSink`). No interim `inventoryRef` hack to a namespaced `KollectInventory`.

### 14. Hub shard routing (no `KollectHub` CRD)

Shard assignment uses **`hash(clusterName) % shardCount`** configured via **Helm values or
environment** on hub Deployments (`mode: hub`). **No `KollectHub` CRD** — dynamic shard
registration is a Phase 2+ spike. See [ADR-0022](0022-multi-cluster-sync-rfc.md).

## Open questions

- **Deferred:** Hub federated mTLS behind external LB — [ADR-0028](0028-hub-cluster-auth-istio-pattern.md).

## Supersedes / amends

- [ADR-0030](0030-connection-test.md) — connection-test CR **now accepted** (0030 sink-only path remains supplementary).
- [ADR-0022](0022-multi-cluster-sync-rfc.md) — no `KollectHub` CRD.
- [ADR-0006](0006-etcd-limit.md) — HTTP not core.
- [ADR-0004](0004-crd-model.md) — namespaced `KollectSink`.
- [ADR-0027](0027-helm-release-inventory.md) — Argo `Application` primary over Flux.
