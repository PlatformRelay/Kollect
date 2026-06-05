# ADR-0031: Namespaced KollectProfile

## Status

Accepted (2026-06-05)

## Context

`KollectTarget` and `KollectInventory` are **namespaced**. `KollectProfile` is **cluster-scoped**
today ([ADR-0004](0004-crd-model.md)). Per-team operator installs with `tenantMode: true` and
namespaced `Role` RBAC cannot manage cluster profiles without extra platform `ClusterRole` bindings
([ADR-0016](0016-namespaced-multi-tenancy.md)).

external-secrets solves this with **namespaced `SecretStore`** + optional **`ClusterSecretStore`**.
The same split fits kollect extraction schemas.

## Decision

1. **`KollectProfile` becomes namespaced** (breaking API change — schedule with CRD versioning
   notes in release changelog).

2. **Reserve `KollectClusterProfile`** (cluster-scoped) for platform-wide shared schemas — same
   relationship as `SecretStore` / `ClusterSecretStore`. Not required for single-cluster MVP;
   design + CRD stub when platform rollup needs shared GVK definitions.

3. **`KollectTarget.spec.profileRef`** resolves a profile in the **same namespace** as the Target
   by default. Optional future field `profileNamespace` (or `profileRef` as
   `namespace/name`) only if cross-namespace refs are proven necessary — defer until requested.

4. **Platform model:** teams own Profile + Target + Inventory in their namespace; platform may
   publish read-only `KollectClusterProfile` objects for standard schemas (Deployment baseline,
   Helm summary) that tenants copy or reference via documented GitOps pattern until cluster profile
   kind ships.

5. **`KollectSink` stays cluster-scoped for Phase 1** — **`KollectClusterSink` deferred Phase 3**
  ([ROADMAP.md](../ROADMAP.md)); `KollectScope.sinkRefs` allowlists cluster sink names.

## Consequences

### Positive

- `tenantMode` installs work without awkward cluster profile RBAC.
- Tenancy boundary aligns: team namespace owns schema + collection + rollup.
- Platform can still offer shared schemas via `KollectClusterProfile` later.

### Negative

- Breaking change from cluster `KollectProfile` — migration: re-apply profiles per namespace.
- Duplicated profile YAML across namespaces unless platform uses GitOps templating or cluster
  profile kind.
- `profileRef` resolution rules must be webhook-validated.

## Open questions

- **OPEN:** Implement namespaced profile in one breaking release vs dual-write transition period?
- **OPEN:** Short name `kprof` remains; reserve `kcprof` for `KollectClusterProfile`?
- **DEFERRED (Phase 3):** `KollectClusterSink` + namespaced sink split.
