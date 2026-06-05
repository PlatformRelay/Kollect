# ADR-0016: Namespaced multi-tenancy and operator watch scope

## Status

Accepted (2026-06-05)

## Context

Platform teams need kollect to run safely alongside many tenant teams on one cluster. Prior art
([ADR-0013](0013-prior-art.md)) compares:

- **external-secrets** — one cluster-scoped controller reconciling both `ClusterSecretStore` and
  namespaced `SecretStore`, plus optional **per-namespace controller** installs via Helm
  `controller.watchNamespaces` / `scopedNamespace`.
- **Argo CD** — `AppProject` tenancy boundary with a single cluster-scoped controller.

kollect already ships **namespaced** `KollectTarget`, `KollectInventory`, and `KollectScope`
([ADR-0004](0004-crd-model.md)). The open question was whether tenancy enforcement and operator
deployment scope should wait until Phase 3.

## Decision

**Namespaced multi-tenancy is Phase 1 priority (ASAP), not Phase 3.**

Support **both** deployment models:

| Model | When | Mechanism |
| --- | --- | --- |
| **Cluster-scoped manager (default)** | Platform team runs one operator | Manager watches all namespaces; **`KollectScope`** in each tenant namespace governs allowed GVKs, workload namespaces, and `KollectSink` refs |
| **Namespaced manager per team** | Strong isolation or delegated installs | Helm release in tenant namespace with `watchNamespaces: [team-a]` and `tenantMode: true` (namespaced `Role` RBAC) — mirrors ESO scoped controller installs |

### Operator watch scope

- **Default:** empty `watchNamespaces` / unset `--watch-namespaces` → controller-runtime cache watches
  **all namespaces** (cluster-scoped operator).
- **Team-scoped:** non-empty list → `cache.Options{DefaultNamespaces}` restricts informers and
  reconcilers to those namespaces only ([controller-runtime cache options](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/cache#Options)).

Helm values:

```yaml
watchNamespaces: []          # empty = all namespaces
tenantMode: false            # true → Role instead of ClusterRole for the manager SA
```

Manager flag: `--watch-namespaces=team-a,team-b` (comma-separated).

### `KollectScope` (namespaced, static)

- **Scope:** namespaced ([ADR-0004](0004-crd-model.md)); one object per tenant namespace.
- **Validation:** validating webhook rejects invalid GVK entries, duplicate `sinkRefs`, and blank
  allowlist entries at admission ([ADR-0015](0015-static-vs-reconciled.md)).
- **Enforcement:** runtime checks against scope (targets/inventories referencing allowed sinks and
  GVKs) follow in Phase 1 reconciler work — webhook is the minimal Phase 1 gate.

`KollectClusterScope` remains reserved for platform-wide policy when namespaced scope is
insufficient ([ADR-0004](0004-crd-model.md)).

## Consequences

### Positive

- Teams can adopt kollect early with explicit tenancy boundaries.
- Platform can offer either shared or per-team operator installs without forking the binary.
- Aligns with ESO and Flux Helm patterns operators already understand.

### Negative

- Two deployment models require chart and doc clarity to avoid misconfigured RBAC.
- `tenantMode` `Role` cannot grant cluster-scoped `KollectProfile` access — platform must bind a
  separate read-only `ClusterRole` for profiles or pre-provision shared profiles.
- Scope enforcement in reconcilers is follow-up work beyond admission validation.

## Open questions

- **OPEN:** Dedicated `KollectClusterSink` + namespaced `KollectSink` split — still deferred;
  `KollectScope.sinkRefs` allowlists cluster sink names for now.
- **OPEN:** Reconciler-time scope enforcement vs webhook-only in Phase 1 MVP?
