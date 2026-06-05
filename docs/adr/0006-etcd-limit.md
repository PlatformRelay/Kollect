# ADR-0006: Data storage and etcd size limit

## Status

Accepted (amended 2026-06-05 by [ADR-0032](0032-platform-architecture-pivot.md) — HTTP optional, not core)

## Context

Kubernetes etcd imposes a **~1.5 MB** limit per object (request size). Inventory operators that
store full collected payloads in CRD `.status` will eventually hit admission failures and destabilize
the apiserver.

OSS validation:

- **kube-state-metrics** never persists collected state in etcd — it projects informer cache into
  Prometheus metrics or serves them from memory.
- **Flux** source-controller stores **artifact metadata** in status (revision, digest, conditions),
  not artifact bytes.
- **external-secrets** stores sync status and references, not secret values, in status.
- **Argo CD** Application status holds sync/health summaries and revision metadata, not full manifests
  (those live in Git or the live cluster).

kollect aggregates attributes from many resources; a naive "put everything in status" design fails
at scale. Developer portals also need a **read path** without scraping Git — addressed here via HTTP.

## Decision

1. **`KollectInventory.status` holds metadata only:** item counts, per-target summaries,
   `metav1.Condition`, `observedGeneration`, `lastExportTime`, and **references** to last export
   (commit SHA, object key, page ID) — never the full payload.
2. **Collected payload flows to sinks** (Postgres/Kafka primary; Git/S3 for audit/archive) on
   debounced export cycles. In-memory aggregation during reconcile is bounded and not persisted to etcd.
3. **Stable ordering** of serialized output (sort keys, deterministic iteration) so Git diffs and
   golden tests are reproducible.
4. **Bounded lists:** paginate API `List` calls; scope informer caches with namespace/label selectors.
5. **Status patch discipline:** patch status only when changed; avoid hot loops writing large status.
6. **Read-only HTTP inventory API (optional):** expose aggregated inventory via operator HTTP
   for **debug and small installs only** — feature-gated, **off in production Helm defaults**
   ([ADR-0032](0032-platform-architecture-pivot.md)). Scalable portal read uses **sink export**
   (Postgres/Kafka) and hub merged store — not spoke HTTP at fleet scale. Same schema as sink
   export where possible when enabled.
   - **Paths (when enabled):** **`GET /v1alpha1/inventory`** (namespace index or caller-scoped
     list); optional **`GET /v1alpha1/inventory/{namespace}/{name}`** for a single inventory.
   - **OpenAPI:** ship **`openapi/v1alpha1/inventory.yaml`** beside the handler (Phase 1 when HTTP
     ships) so portals have a stable schema.
   - **Auth (primary):** delegate to Kubernetes API auth — **TokenReview** + **SubjectAccessReview**;
     callers use standard `Authorization: Bearer` service account tokens;
     `--inventory-auth-mode=kubernetes` (default). See [ADR-0024](0024-inventory-api-auth.md).
   - **Auth (optional):** **oauth2-proxy** Helm sidecar/subchart for OIDC browser access —
     `oauth2Proxy.enabled: false` by default; documented, not required for service-to-service.
   - **TODO:** Async push to clients — **SSE** or **watch** endpoint when inventory changes, not only GET snapshot.
7. **Optional PVC buffer:** when in-memory aggregate exceeds **`maxExportBytes`**, spill full payload
   to a mounted volume for export and HTTP serve — still not written to etcd status.
8. **`maxExportBytes` / aggregate bounds:** **global manager default** (~**1.5 MiB**, etcd safety
   margin) plus optional **`KollectInventory.spec.maxExportBytes`** override. Validating webhook
   **rejects** per-inventory override **greater than** the global cap. In-memory hot path and
   status summaries stay bounded; full payload only to PVC/sink/HTTP body.

```mermaid
flowchart LR
  Inv[KollectInventory reconcile]
  Mem[In-memory aggregate]
  Status[status: counts + conditions]
  PVC[(optional PVC)]
  HTTP[HTTP /v1alpha1/inventory]
  Sink[Git / S3 / ...]
  Inv --> Mem
  Mem --> Status
  Mem -->|over maxExportBytes| PVC
  Mem --> HTTP
  Mem --> Sink
```

## Consequences

### Positive

- Safe at scale for clusters with thousands of collected objects.
- Aligns with how mature operators treat status as **observed state summary**, not a database.
- **Postgres/Kafka sinks** are the **system of record** for portals; Git/S3 remain audit/diff paths.
- Optional HTTP API enables small-install debugging when feature-gated on — not fleet-scale portal read.

### Negative

- HTTP surface adds auth, TLS, and network policy obligations.
- PVC spill path adds storage class and backup considerations.
- Consumers must still use sink or HTTP for full payload — `kubectl get kinv -o yaml` is not enough.

## Open questions

- **RESOLVED (2026-06-05):** HTTP paths **`GET /v1alpha1/inventory`** (+ optional
  `{namespace}/{name}`); OpenAPI at **`openapi/v1alpha1/inventory.yaml`** when HTTP enabled.
- **RESOLVED (2026-06-05):** Global default ~**1.5 MiB** + per-Inventory **`spec.maxExportBytes`**
  override capped by webhook — see item 8 above.
- **RESOLVED (2026-06-05):** Optional Helm sidecar/subchart for oauth2-proxy; K8s-native auth is
  primary — [ADR-0024](0024-inventory-api-auth.md).
