# ADR-0004: CRD model — prefixed kinds, static vs reconciled split

## Status

Accepted

## Context

kollect replaces a hardcoded batch collector schema with CRD-driven configuration. Operators in
this space use different tenancy and config patterns:

- **external-secrets** splits `SecretStore` (namespaced) vs `ClusterSecretStore` (cluster) with
  namespace `conditions` on the cluster variant; provider config is a discriminated union in spec.
- **Flux notification-controller** keeps `Provider` and `Alert` as **static** objects (no status
  subresource, no dedicated reconciler) while `Receiver` is reconciled.
- **Argo CD** uses `AppProject` as a tenancy boundary and `Application` as the reconciled unit.

kollect must combine generic attribute selection, resource selection, aggregation, multi-backend
export, and (later) doc-sync — no single OSS project covers all of this.

## Decision

API group `kollect.dev/v1alpha1`. All kinds are **prefixed** (`Kollect*`) to avoid collisions.

### Static config (validated, no controller)

| Kind | Scope | Role |
| --- | --- | --- |
| `KollectProfile` | Cluster | Reusable extraction schema: GVK + named CEL/JSONPath attributes |
| `KollectSink` | Cluster | Backend config: `type` + endpoint + `secretRef`; resolved via Go registry |
| `KollectScope` | Cluster | Tenancy boundary (Phase 3): allowed GVKs, namespaces, sinks |

### Reconciled (controller + dynamic informers)

| Kind | Scope | Role |
| --- | --- | --- |
| `KollectTarget` | Namespaced | `profileRef` + selectors + optional CEL predicate; drives collection |
| `KollectInventory` | Cluster | Aggregates targets; dispatches to sinks |
| `KollectPublication` | Namespaced | Phase 2: template render + doc-backend sync |

### Reserved (designed, not built yet)

- `KollectReceiver` — inbound webhook → trigger (Flux Receiver pattern).
- `KollectTargetSet` — generator templating many Targets (ApplicationSet pattern).

Short names: `kprof`, `ksink`, `kscope`, `ktgt`, `kinv`, `kpub`.

All reconciled kinds support `spec.suspend` and `kollect.dev/requestedAt` manual-trigger annotation.

## Consequences

### Positive

- Static Profile/Sink cuts moving parts (validated at admission, read at reconcile time).
- Namespaced Targets align with team ownership; cluster Profiles/Sinks enable shared platform config.
- Prefix naming is grep-friendly and avoids generic kind collisions in multi-operator clusters.

### Negative

- Cluster-scoped `KollectInventory` may be awkward for strict multi-tenant isolation (see open questions).
- `KollectSink` is cluster-scoped only today — no namespaced sink variant unlike ESO's Store split.

## Open questions

- **OPEN:** Should `KollectInventory` be **namespaced** (team-owned rollup) with optional cluster
  aggregator, mirroring `SecretStore` vs `ClusterSecretStore`? Cluster scope simplifies a single
  portal view but widens RBAC blast radius.
- **OPEN:** Add `KollectClusterSink` + namespaced `KollectSink` split in Phase 1 or defer to Phase 3
  with `KollectScope`?
- **OPEN:** Is `KollectPublication` the right Phase 2 entry, or ship **plain JSON export to Git**
  in Phase 1 and defer templating/Confluence to Phase 2? (See ADR-0013 and PLAN Phase 1/2.)
