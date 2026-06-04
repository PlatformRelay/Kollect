# ADR-0014: Event-driven dynamic informers

## Status

Accepted

## Context

kollect must watch **arbitrary GVKs** defined by `KollectProfile`, not a fixed built-in schema.
The legacy fleet-inventory-collector used batch listing on an interval — high API load, latent updates,
and poor fit for an operator.

**kube-state-metrics** registers generic informers per configured GVK and reacts to add/update/delete
events. **external-secrets** uses controller-runtime watches with dynamic scope. **Flux**
source-controller reconciles on spec changes and artifact polling intervals — a hybrid, but still
informer-backed for Kubernetes objects.

Polling the API on a short `RequeueAfter` loop would duplicate informer work and violate GUIDELINES.

## Decision

1. **Dynamic informer registration** per active `KollectProfile` GVK using controller-runtime cache
   (`unstructured.Unstructured`) and/or `client-go` `dynamicinformer`.
2. **Level-based reconcile:** each event enqueues `KollectTarget`; reconcile computes desired export
   from current cache state — idempotent, safe to repeat.
3. **Scoped watches:** honor `namespaceSelector`, `labelSelector`, and name lists on `KollectTarget`
   to bound cache memory.
4. **Long resync period** as correctness backstop only (missed events, bookmark gaps) — not a
   freshness knob.
5. **`RequeueAfter`** reserved for **external sink freshness** or time-based doc sync, never for
   watching in-cluster objects.
6. **Secondary watches:** Profile/Sink/Scope changes enqueue dependent Targets/Inventories via
   enqueue mappers.
7. **SAR-gated degradation:** if cluster-scoped list is forbidden, degrade to namespace scope and
   record `skipped:forbidden` in status (port collector logic).

## Consequences

### Positive

- Near-real-time updates with ~99% fewer API reads vs batch polling (per collector experience).
- Matches controller-runtime best practices and kube-state-metrics informer model.
- Natural fit for multi-GVK profiles without codegen per type.

### Negative

- Dynamic informer lifecycle (register on Profile add, tear down on remove) is more complex than
  static typed informers.
- Memory scales with watched objects — requires selector discipline and profiling.

## Open questions

- **OPEN:** Single shared informer per GVK across Targets, or per-Target scoped caches?
  (Prefer shared per GVK for memory.)
- **OPEN:** Maximum concurrent GVK watches before sharding or warning?
