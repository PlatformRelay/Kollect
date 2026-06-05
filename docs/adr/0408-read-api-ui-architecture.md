# ADR-0408: Read API and UI architecture

> A stable, versioned Read API as the UI's only contract, with a pluggable backing store
> (memory → Postgres → Parquet), so one read-only SPA serves both a zero-infra console and a
> multi-cluster portal without a rewrite.

**Theme:** 04 · Export & sinks (read side) · **Status:** Exploring (planned v0.2/v0.3)

## Context

An inventory product lives or dies on whether people can *see* their inventory. ArgoCD's adoption is
driven far more by its UI — a live, filterable overview of status, topology, and drift — than by its
reconciler. For kollect, a read-only UI (searchable resource catalog, export/freshness health,
multi-cluster rollup, attribute drift over time) is a higher-leverage adoption investment than more
sink backends or advanced collection features.

But kollect has a tension ArgoCD does not. Our thesis ([ARCHITECTURE.md](../ARCHITECTURE.md),
[REQUIREMENTS.md](../REQUIREMENTS.md)) is that consumers must **not** read the live kube-apiserver — they
read the durable export (the read model). A UI that hammers the apiserver, or couples portal
availability to the operator process, would violate that. ArgoCD's UI is intentionally coupled to its
controller; kollect's must remain a **consumer of the read model**.

## Decision

### 1. The Read API is the contract — not the store

The UI depends on a **stable, versioned Read API** and nothing else (never kube-apiserver, never an
internal store). It extends the existing inventory HTTP surface ([ADR-0103](0103-etcd-limit.md),
[ADR-0404](0404-inventory-api-auth.md)):

- Versioned (`/v1alpha1/…`), OpenAPI-described, returning the export data contract
  ([ADR-0405](0405-export-data-contract.md)) with its envelope `schemaVersion`.
- List/filter/search over inventories, targets, and items (by cluster, namespace, GVK, name,
  attribute); per-inventory/sink/`KollectConnectionTest` status and freshness; multi-cluster rollup
  views ([ADR-0305](0305-aggregation-dedupe.md)).
- Auth is the K8s-native path already built — TokenReview + SAR, optional oauth2-proxy/OIDC sidecar
  ([ADR-0404](0404-inventory-api-auth.md)).

### 2. Pluggable backing-store adapter

Behind the Read API sits a **backing-store adapter**, registered like the sink registry
([ADR-0406](0406-sink-registry.md)) and returning the same data contract regardless of source:

| Adapter | Use | Trade-off |
| --- | --- | --- |
| **`memory`** (default for first UI) | Operator's in-memory `Store` — live, single-cluster, **zero extra infra** | No history; bounded by operator memory/availability |
| **`postgres`** | Scale + **history/drift** | Needs a DB |
| **`parquet`** (DuckDB/object store) | Scale, queryable, **no DB server** | Snapshot granularity; compaction |

The same indirection that protects the thesis (UI reads the read model, not live state) lets the same
SPA serve a live console *or* a scale portal by swapping the adapter — no UI change.

### 3. SPA is static and read-only

A React single-page app that talks only to the Read API. **Read-only in v1** — kollect observes; it does
not mutate cluster state from the UI. Shipped first **embedded in / served by the operator**
(all-in-one, feature-gated, like ArgoCD's default), later optionally by a dedicated **`kollect-server`**
Deployment for scale/HA — keeping a busy web server out of the controller process
([ADR-0504](0504-operator-runtime-modes-ha-leader-election.md)).

### 4. Phasing

| Milestone | Scope |
| --- | --- |
| **v0.1.0** | No UI. Harden the Read API (filters, `schemaVersion`, OpenAPI) and **freeze it as the UI contract**; finish the queryable sinks (Postgres deletes → NATS → Parquet). |
| **v0.2.0** | Minimal read-only SPA on the **`memory` adapter**, operator-served, feature-gated. Catalog + filter/search + freshness/health. The zero-infra demo. |
| **v0.3.0+** | Portal mode on **`postgres`/`parquet`** adapter; **drift-over-time** views; optional split into `kollect-server`. |

## Consequences

- The UI is decoupled from storage and from the cluster API — the read-model thesis holds at every tier.
- The biggest UI investment is the **Read API + adapter interface**, not the SPA; doing it before the UI
  avoids churn.
- **Drift-over-time requires a historical store**, so the headline differentiator only lands on the
  `postgres`/`parquet` adapters — reinforcing that the scale UI reads a sink.
- Operator-embedded serving adds a feature-gated HTTP surface to the controller; acceptable at small
  scale, motivates the `kollect-server` split later.
- Read-only scope avoids turning kollect into a control plane (out of scope; it's an observer).

## Open questions

- **OPEN:** Embed the SPA in the operator binary (single image) vs ship a separate `kollect-ui` image?
- **OPEN:** Adapter interface shape — does it reuse `Backend` ([ADR-0406](0406-sink-registry.md)) inverted
  (read side), or a distinct `Reader` interface?
- **OPEN:** Real-time updates — SSE/websocket from the `memory` adapter (operator already has a `Store`
  watch) vs poll-only for the sink-backed adapters?
- **OPEN:** Cross-cluster auth for portal mode reading a shared sink — same K8s SAR model, or sink-native?
- **OPEN:** Minimal viable feature set for the v0.2 "wow" demo — catalog + freshness only, or include a
  single-cluster diff?
