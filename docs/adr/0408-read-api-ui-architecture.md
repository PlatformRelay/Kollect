# ADR-0408: Read API and UI architecture

> A stable, versioned Read API as the UI's contract for inventory rows, with a pluggable backing store
> (memory → Postgres → Parquet) and a separate read-only SPA — so one console serves both a
> zero-infra cluster view and a multi-cluster portal without violating the read-model thesis.

**Theme:** 04 · Export & sinks (read side) · **Status:** Current (accepted 2026-06-05; was Exploring)

## Context

An inventory product lives or dies on whether people can *see* their inventory. Argo CD's adoption is
driven far more by its UI — a live, filterable overview of status, topology, and drift — than by its
reconciler. For Kollect, a read-only UI (searchable resource catalog, export/freshness health,
multi-cluster rollup, attribute drift over time) is a higher-leverage adoption investment than more
sink backends or advanced collection features.

But Kollect has a tension Argo CD does not. Our thesis ([ARCHITECTURE.md](../ARCHITECTURE.md),
[REQUIREMENTS.md](../REQUIREMENTS.md)) is that consumers must **not** read the live kube-apiserver for
inventory rows at scale — they read the durable export (the read model). A UI that hammers the apiserver
for catalog data, or couples portal availability to the controller process, would violate that. Argo CD's
UI is intentionally coupled to its controller; Kollect's inventory SPA must remain a **consumer of the
read model**.

Maintainer decisions (2026-06-05, rev 2 engineering spec + UX approach) resolve the former open questions
(OQ-1–12). Deployment, engineering gates, and Read API extensions are split into companion ADRs so this
record stays the **architecture spine**:

- [ADR-0409](0409-kollect-ui-deployment.md) — separate `kollect-ui` image and Helm subchart
- [ADR-0410](0410-ui-engineering-and-quality-gates.md) — monorepo `ui/`, stack, CI pyramid
- [ADR-0411](0411-read-api-extensions-for-ui.md) — pagination, filters, envelope, CRD status hybrid

## Decision

### 1. The Read API is the contract for inventory rows — not the store

The UI depends on a **stable, versioned Read API** for inventory catalog data. It extends the existing
inventory HTTP surface ([ADR-0103](0103-etcd-limit.md), [ADR-0404](0404-inventory-api-auth.md)):

- Versioned (`/v1alpha1/…`), OpenAPI-described (`openapi/v1alpha1/inventory.yaml`), returning the export
  data contract ([ADR-0405](0405-export-data-contract.md)) with its envelope `schemaVersion`.
- List/filter/search over inventories and items (by cluster, namespace, GVK, target, name, attribute);
  per-inventory/sink export status and freshness; multi-cluster rollup views
  ([ADR-0305](0305-aggregation-dedupe.md)).
- Auth on the Read API: Kubernetes **TokenReview + SAR** when a bearer token is present
  ([ADR-0404](0404-inventory-api-auth.md)). The **MVP SPA has no login shell** — auth offload via
  **oauth2-proxy at ingress** is post-MVP ([ADR-0409](0409-kollect-ui-deployment.md)).

**Hybrid Kubernetes API (OQ-3):** the UI **may** call the Kubernetes API for **CRD conditions and
metadata** (`KollectTarget`, `KollectInventory`, `KollectScope`, etc.) alongside the Read API. It must
**never** use kube list/watch for inventory **rows** at scale ([FR-READ-1](../REQUIREMENTS.md)). See
[ADR-0411](0411-read-api-extensions-for-ui.md) for the recommended MVP split.

### 2. Distinct `InventoryReader` — not sink `Backend`

Behind the Read API sits a **backing-store adapter** — a distinct **`InventoryReader`** interface,
**not** the sink `Backend` inverted ([ADR-0406](0406-sink-registry.md), OQ-11). Sink backends write
projections; the reader reads them (or the in-memory canonical store) through one contract:

| Adapter | Use | Trade-off |
| --- | --- | --- |
| **`memory`** (default for first UI) | Operator's in-memory `Store` — live, single-cluster, **zero extra infra** | No history; bounded by operator memory/availability |
| **`postgres`** | Scale + **history/drift** | Needs a DB |
| **`parquet`** (DuckDB/object store) | Scale, queryable, **no DB server** | Snapshot granularity; compaction |

The same indirection lets one SPA serve a live console *or* a scale portal by swapping the adapter — no
UI rewrite. Portal mode reads Postgres/Parquet **through** the Read API adapter, never ad-hoc SQL from
the browser.

### 3. Real-time updates (OQ-4)

| Adapter | Mechanism |
| --- | --- |
| **`memory`** | **SSE** via `GET /v1alpha1/inventory/watch` (operator `Store` watch) |
| **`postgres`** / **`parquet`** | **Poll** (default 30 s); SSE optional later |

The SPA uses TanStack Query cache invalidation on SSE events or poll interval.

### 4. SPA is static, read-only, and separately deployed (OQ-1, OQ-12)

A React single-page app in monorepo **`ui/`** (OQ-5) that:

- Talks to the Read API for **inventory rows** and export metadata.
- May talk to the Kubernetes API for **CRD status/conditions only** (hybrid).
- Is **read-only in v0.2** — observability console; no Target create/apply forms; onboarding is
  copy-YAML + docs links.
- Ships as a **separate static container image** `ghcr.io/konih/kollect-ui` — **not** embedded in the
  operator binary for MVP ([ADR-0409](0409-kollect-ui-deployment.md)). Operator-embedded UI remains a
  deferred option if maintainers reopen OQ-1.

Stack, testing pyramid, and bundle budget: [ADR-0410](0410-ui-engineering-and-quality-gates.md).

### 5. Ingress and exposure (OQ-10)

Inventory HTTP and the UI are **off ingress by default**. Operators enable Read API + UI ingress
explicitly when cluster-network access or auth offload is configured. Dev workflows use port-forward.

### 6. Phasing

Aligned with [ROADMAP.md](../ROADMAP.md) and engineering spec §12:

| Milestone | Backend prerequisite | Frontend deliverable |
| --- | --- | --- |
| **v0.1.0** | Harden Read API: filters, `schemaVersion`, export status in OpenAPI ([ADR-0411](0411-read-api-extensions-for-ui.md)) | Monorepo `ui/` scaffold; MSW mocks; contract + unit tests |
| **v0.2.0** | Memory adapter complete; Read API SAR-gated | **Read-only MVP SPA** in separate **`kollect-ui` image**: Overview, Inventory, Targets/Sinks **lists + detail drawers** (no Target create forms); onboarding = copy-YAML + docs; **no auth UI** |
| **v0.3.0** | Postgres Read adapter; hub merged metadata API | Portal mode, drift chart, multi-cluster picker; **oauth2-proxy at ingress** + cross-cluster auth spike |
| **Phase 3** | `KollectScope` deniedNamespaces UI API | Scope comparison; policy violation explainer; optional BFF |
| **Phase 4** | Custom resource metrics in Read API | Metrics sparklines in Target detail |

## Consequences

### Positive

- The UI is decoupled from storage and from unbounded apiserver inventory reads — the read-model thesis
  holds at every tier.
- Separate `kollect-ui` image decouples UI release cadence from the controller binary.
- Hybrid CRD status keeps condition parity with `kubectl describe` without bloating the Read API for MVP.
- The biggest pre-UI investment is the **Read API + `InventoryReader` interface**, not the SPA.

### Negative

- **Drift-over-time requires a historical store** — headline portal differentiator lands on
  `postgres`/`parquet` adapters only.
- Hybrid K8s API calls require a browser-accessible apiserver endpoint (or post-MVP BFF) for CRD status;
  cluster-network / port-forward assumptions apply in MVP.
- Read API extensions ([ADR-0411](0411-read-api-extensions-for-ui.md)) block v0.2 feature work until
  shipped.

## Resolved questions (formerly open)

| # | Question | Resolution |
| --- | --- | --- |
| OQ-1 | Embed UI in operator vs separate image? | **Separate `kollect-ui` image** — [ADR-0409](0409-kollect-ui-deployment.md) |
| OQ-2 | Browser auth in production? | **oauth2-proxy at ingress** post-MVP; MVP **no auth in frontend** |
| OQ-3 | UI calls Kubernetes API for CRD status? | **Hybrid allowed** — CRD conditions/metadata only |
| OQ-4 | Real-time updates? | **SSE** (memory) + **poll** (postgres) |
| OQ-5 | Monorepo vs separate repo? | **Monorepo `ui/`** — [ADR-0410](0410-ui-engineering-and-quality-gates.md) |
| OQ-6 | CSS strategy? | **Tailwind v4** — [ADR-0410](0410-ui-engineering-and-quality-gates.md) |
| OQ-7 | Bundle budget enforcement? | **Fail CI** on breach — [ADR-0410](0410-ui-engineering-and-quality-gates.md) |
| OQ-8 | Visual regression in CI? | **Nightly required** — [ADR-0410](0410-ui-engineering-and-quality-gates.md) |
| OQ-9 | Cross-cluster portal auth? | **Deferred** — no auth in MVP UI |
| OQ-10 | Inventory HTTP on ingress by default? | **Off** — explicit opt-in |
| OQ-11 | Reader vs sink `Backend`? | **Distinct `InventoryReader`** |
| OQ-12 | Target create form in MVP? | **Read-only UI** — no create forms |

## See also

- [ADR-0404: Inventory HTTP API authentication](0404-inventory-api-auth.md)
- [ADR-0405: Export data contract and schema versioning](0405-export-data-contract.md)
- [ADR-0409: Kollect UI deployment](0409-kollect-ui-deployment.md)
- [ADR-0410: UI engineering and quality gates](0410-ui-engineering-and-quality-gates.md)
- [ADR-0411: Read API extensions for UI](0411-read-api-extensions-for-ui.md)
- [ADR-0504: Operator runtime modes](0504-operator-runtime-modes-ha-leader-election.md) — optional
  `kollect-server` split at scale
- [ROADMAP.md](../ROADMAP.md) § Read API + UI console
