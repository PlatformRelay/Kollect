# ADR-0006: Data storage and etcd size limit

## Status

Accepted

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
at scale.

## Decision

1. **`KollectInventory.status` holds metadata only:** item counts, per-target summaries,
   `metav1.Condition`, `observedGeneration`, `lastExportTime`, and **references** to last export
   (commit SHA, object key, page ID) — never the full payload.
2. **Collected payload flows directly to sinks** (Git commit, S3 object, etc.) on each reconcile
   export cycle. In-memory aggregation during reconcile is bounded and not persisted to etcd.
3. **Stable ordering** of serialized output (sort keys, deterministic iteration) so Git diffs and
   golden tests are reproducible.
4. **Bounded lists:** paginate API `List` calls; scope informer caches with namespace/label selectors.
5. **Status patch discipline:** patch status only when changed; avoid hot loops writing large status.

## Consequences

### Positive

- Safe at scale for clusters with thousands of collected objects.
- Aligns with how mature operators treat status as **observed state summary**, not a database.
- Git/S3 exports become the **auditable source of truth** for stakeholders and developer portals.

### Negative

- Consumers cannot `kubectl get kinv -o yaml` and see full inventory — they must read the sink
  (Git file, HTTP API if added, or metrics in Phase 4).
- Reconcile must tolerate sink-unavailable periods without stuffing fallback data into status.

## Open questions

- **OPEN:** Should Phase 1 include an optional **read-only HTTP `/inventory` endpoint** (in-memory
  or sink-backed cache) for developer portal frontends, avoiding CRD reads? Adds attack surface and
  ops burden; ESO/Flux do not expose inventory HTTP by default.
- **OPEN:** Max in-memory aggregate size per Inventory before spilling to temp object storage
  mid-reconcile?
