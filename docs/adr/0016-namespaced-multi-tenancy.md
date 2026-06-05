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
| **Per-team manager (default for now)** | Delegated / team-owned installs | Helm with `watchNamespaces: [team-a]` and **`tenantMode: true`** — namespaced Role RBAC; namespaced Profile/Sink/Target/Inventory in team namespace ([ADR-0032](0032-platform-architecture-pivot.md)) |
| **Cluster-scoped manager** | Platform team operates one shared operator | Watches all namespaces; **`KollectScope`** per tenant namespace governs GVKs, workload namespaces, and sink refs |

### Operator watch scope

- **Documented default for new installs (owner preference):** `tenantMode: true` + non-empty
  `watchNamespaces` for per-team Helm releases.
- **Platform option:** empty `watchNamespaces` → watches **all namespaces** (cluster-scoped operator).
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
- **Enforcement (Phase 1):** **both** validating webhook **and** reconciler-time checks — reject
  `KollectTarget` / `KollectInventory` that reference disallowed GVKs, workload namespaces, or
  `KollectSink`s per scope allowlists. Webhook alone is insufficient for multi-tenant isolation.

`KollectClusterScope` remains reserved for platform-wide policy when namespaced scope is
insufficient ([ADR-0004](0004-crd-model.md)).

## Consequences

### Positive

- Teams can adopt kollect early with explicit tenancy boundaries.
- Platform can offer either shared or per-team operator installs without forking the binary.
- Aligns with ESO and Flux Helm patterns operators already understand.

### Negative

- Two deployment models require chart and doc clarity to avoid misconfigured RBAC.
- **Resolved ([ADR-0031](0031-namespaced-profiles.md)):** `KollectProfile` moves to **namespaced**
  scope so `tenantMode` installs do not need cluster profile RBAC; `KollectClusterProfile` reserved
  for platform-shared schemas.
- Reconciler enforcement adds controller complexity and must stay consistent with webhook rules.

## Open questions

- **DEFERRED (Phase 3):** `KollectClusterSink` + namespaced `KollectSink` split — `KollectScope.sinkRefs`
  allowlists cluster sink names until then ([ROADMAP.md](../ROADMAP.md)).
