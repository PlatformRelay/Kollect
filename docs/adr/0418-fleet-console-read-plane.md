# ADR-0418: Fleet console read plane

> A standalone, read-only fleet console server that materializes a fleet-wide inventory read model
> from the existing event stream (and optionally a database), and serves the existing Read API
> contract extended with a `cluster` dimension — without reviving a hub tier or letting clients
> touch kube-apiserver or the message bus.

**Theme:** 04 · Export & sinks (read side) · **Status:** Exploring (draft 2026-06-07)

## Context

The optional in-operator Read API (`internal/inventory`, `openapi/v1alpha1/inventory.yaml`) can
render **one** operator's in-memory inventory for **one** cluster. That is useful for debug and
local development, but the production deployment is a **fleet**: many clusters, each running an
independent single-mode operator, all exporting to a **shared sink** ([ADR-0501](0501-multi-cluster-fleet.md)).
A console that can only see one operator's volatile memory store cannot answer fleet questions —
"across all clusters, what is collected, how fresh is it, what is degraded?",
"show `Deployment` inventory for one team in every cluster", "which clusters stopped reporting?".

Two constraints shape the design:

1. **The event stream is the fleet transport.** The event sink ([ADR-0402](0402-sink-backends-database-kafka.md))
   already publishes a per-namespace envelope `{schemaVersion, timestamp, cluster, namespace, payload}`
   keyed `cluster/namespace` — every cluster already emits exactly the stream a fleet view needs. The
   fleet console standardizes on **one golden ingest path (Kafka)** plus an **optional database** for
   durability/history; it deliberately does **not** try to be as backend-agnostic as the operator.
2. **The console never writes to kube-apiserver** and clients hold **no** bus or database
   credentials. The console is a pure read mirror of the read model ([FR-READ-1](../REQUIREMENTS.md)),
   preserving the operator's single-responsibility guardrail ([ADR-0702](0702-doc-sync-templating.md)).

The producer side of a fleet feed already ships. The missing piece is the **consumer / read** side:
an `InventoryReader` interface and a Postgres/Parquet portal mode that were never built. This ADR
fills that gap **without** re-opening the hub tier that [ADR-0501](0501-multi-cluster-fleet.md)
removed. There is **no** shipped browser SPA; this design is for a future read service only.

## Decision

### 1. A standalone, read-only fleet console server

Introduce a new, standalone component (a dedicated `kollect-fleet-server` image, or a mode of the
optional `kollect-server` split) that:

- Joins a **consumer group** on the event topic, decodes the inventory envelope, and materializes a
  **fleet read model** keyed `(cluster, namespace, kind, name, uid)`.
- Serves the **existing Read API contract** plus a `cluster` dimension and a fleet roster endpoint.
- Is **not** a hub: no ingest endpoint from clusters, no reconciliation, **no kube credentials**, never
  writes to any cluster or to kube-apiserver. Operators are untouched — [ADR-0501](0501-multi-cluster-fleet.md)
  holds. Egress-only/DMZ clusters need only outbound bus access; nothing connects **into** clusters.

### 2. Implement `InventoryReader` with fleet adapters

The HTTP layer depends only on an `InventoryReader` interface, so memory-only and database-backed
deployments share one server and one OpenAPI contract:

| Adapter | Role | Trade-off |
| --- | --- | --- |
| `memoryFleet` | Live core — real-time, zero extra infra beyond the bus | Volatile; cold-start needs replay/rehydrate |
| `postgresFleet` | Durability + history/drift; reuses the Postgres sink identity `(cluster, namespace, name, uid)` and delete reconcile | Adds a DB dependency for the read plane |

A small `FleetSource` interface keeps the ingest pluggable (Kafka is the only **shipped/supported**
path; an event-emitter alternative or a database-poll fallback can be added later **without** touching
the read model or contract). This is "one golden path, not operator parity" made concrete.

### 3. Serving contract — extend the Read API additively

Keep the `internal/inventory` JSON shapes; add a `cluster` query param and response field, plus
fleet-level endpoints — an **additive** change so single-cluster clients keep working
(`cluster` defaults to the only one present):

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/v1alpha1/clusters` | Fleet roster: per-cluster `lastSeen`, item count, degraded count, lag |
| `GET` | `/v1alpha1/inventory?cluster=&namespace=&kind=…` | Fleet inventory; `cluster` is filter **and** column; existing filters preserved |
| `GET` | `/v1alpha1/inventory/watch?cluster=` | SSE off the consumer (optional `cluster` scope) |
| `GET` | `/v1alpha1/status/{targets,inventories}?cluster=` | Status rollup per cluster |

One OpenAPI document (`openapi/v1alpha1/inventory.yaml`) extended with `cluster`; Go contract tests
bind to it.

### 4. Stream semantics

The envelope payload is the **authoritative per-`(cluster, namespace)` snapshot** at `timestamp`:

- **Granularity:** replace that partition of the read model wholesale on each message.
- **Ordering:** per-key ordering is guaranteed by the bus (key = `cluster/namespace`); drop messages
  older than the stored snapshot.
- **Deletes:** an empty snapshot clears that namespace's rows; a cluster that stops emitting is marked
  **stale via `lastSeen`, never auto-deleted** (avoid false "gone").
- **Cold-start:** compacted topic replay and/or database rehydrate; show a "rebuilding" condition until
  the first full pass. Honor `schemaVersion` — park unknown majors with a visible condition rather than
  mis-parsing ([ADR-0405](0405-export-data-contract.md)).

### 5. Auth, security, exposure

- **GET-only** server; no kube client wired. Off ingress by default.
- AuthN/Z via Kubernetes TokenReview + SAR when a bearer token is present, or **oauth2-proxy at ingress**
  for browsers ([ADR-0404](0404-inventory-api-auth.md)). Optional SAR/redaction applies per `cluster` and
  per `namespace`; a forbidden cluster/namespace returns `403`, not an empty list.
- Clients never hold bus or database credentials; those stay server-side.

## Consequences

### Positive

- The fleet console answers fleet-wide questions from the stream the operators already emit — no new
  transport to run, no per-cluster fan-out, no inbound cluster access.
- Implements the long-planned `InventoryReader` and the Postgres portal adapter behind one contract;
  memory-only small installs and database-backed scale installs share one OpenAPI document.
- Honors [ADR-0501](0501-multi-cluster-fleet.md) — no hub resurrection — and the read-model thesis
  ([FR-READ-1](../REQUIREMENTS.md)); the read plane stays GET-only and does not need kube-apiserver access.

### Negative

- Adds a new long-running component (one image + optional database) to deploy and operate.
- Memory-only mode is empty after restart unless the topic is compacted or a database rehydrates it.
- The console standardizes on the event-stream golden path; database-direct and other-bus sources are
  fallbacks/later work, not first-class parity with the operator's sink matrix.

## Phasing

Aligned with [ROADMAP.md](../ROADMAP.md) § Read API (design-only):

| Milestone | Deliverable |
| --- | --- |
| Design | `InventoryReader` + `memoryFleet` adapter; fleet console live core (consumer → memory) serving Read API + `/v1alpha1/clusters` + `cluster` filter + SSE; OpenAPI `cluster` extension |
| Later | `postgresFleet` adapter + consume-to-database upsert; cold-start rehydrate / compacted-topic replay; drift-over-time views; `kollect-fleet-server` chart + oauth2-proxy overlay |

## Explicitly rejected / deferred

- **Operator hub tier** — rejected; [ADR-0501](0501-multi-cluster-fleet.md) stands. The fleet console is
  a read consumer, not a hub.
- **Browser-side bus access** — rejected (credentials in the browser, no redaction, no pagination).
- **Per-cluster Read API fan-out from the browser** — rejected as primary (N×auth, unreachable DMZ
  clusters, volatile per-operator memory); acceptable only as a single-cluster bootstrap helper.
- **Read-plane write paths to kube-apiserver** — permanently out of scope; useful "actions" belong to a
  separate publisher component, not cluster writes.
- **Shipped browser SPA** — out of product scope; sink exports remain the adoption path.

## See also

- `openapi/v1alpha1/inventory.yaml` / `internal/inventory` — current single-cluster Read API contract
- [ADR-0402: Postgres and Kafka sink backends](0402-sink-backends-database-kafka.md) — the producer the console consumes
- [ADR-0501: Multi-cluster fleet](0501-multi-cluster-fleet.md) — shared sink, no hub (the constraint this ADR honors)
- [ROADMAP.md](../ROADMAP.md) § Read API (design-only)
