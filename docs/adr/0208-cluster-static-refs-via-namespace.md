# ADR-0208: Cluster reconciled kinds reference namespaced static config

> Drop `KollectClusterProfile` and `KollectCluster*Sink`; cluster targets and inventories resolve
> profiles and family sinks by explicit namespace + name.

**Theme:** 02 · API & tenancy · **Status:** Accepted (implemented 2026-06-10)

## Context

Kollect ships **parallel static-config CRD pairs** for platform vs team scope:

| Static config | Namespaced | Cluster variant |
| --- | --- | --- |
| Extraction schema | `KollectProfile` | `KollectClusterProfile` |
| Snapshot sink | `KollectSnapshotSink` | `KollectClusterSnapshotSink` |
| Database sink | `KollectDatabaseSink` | `KollectClusterDatabaseSink` |
| Event sink | `KollectEventSink` | `KollectClusterEventSink` |

Cluster **reconciled** kinds (`KollectClusterTarget`, `KollectClusterInventory`) already lean on
namespaced objects in practice:

- `KollectClusterTarget` resolves `spec.profileRef` to `KollectClusterProfile` first, then falls back
  to a `KollectProfile` in the platform namespace (`kollect-system`) — see
  `resolveClusterTargetProfile`.
- `KollectClusterInventory` resolves family sink refs in `spec.sinkNamespace` (default
  `kollect-system`) and only then tries cluster-scoped sinks — see `loadClusterInventorySink`.

That dual-resolution path duplicates OpenAPI, webhooks, connection-test controllers, RBAC rules, golden
schema tests, and Helm CRD bundles for four cluster static kinds whose spec bodies are identical to
their namespaced counterparts.

Prior ADRs reserved or retained cluster static kinds to mirror **external-secrets**
(`SecretStore` / `ClusterSecretStore`) and to support platform-wide shared backends
([ADR-0204](0204-namespaced-profiles.md), [ADR-0414](0414-sink-family-crds.md)). Unlike
`ClusterSecretStore`, Kollect cluster static kinds today carry **no namespace allowlist or
conditions** — any cluster inventory can reference any cluster sink by name alone, which weakens the
tenancy story [ADR-0203](0203-namespaced-multi-tenancy.md) intended to provide.

**Risk:** cross-namespace refs without explicit RBAC checks and observability have already caused
silent partial collection elsewhere in the operator (SAR-gated workload reads per
[ADR-0104](0104-security-model.md), [ADR-0602](0602-error-taxonomy.md)). Cluster static refs need
the same rigor before we expand platform rollups.

## Decision

**Remove cluster-scoped static config CRDs.** Platform operators publish shared profiles and sinks as
**namespaced** objects (typically in `kollect-system` or a dedicated export namespace). Cluster
reconciled kinds reference them with **explicit namespace + name** — no implicit fallback, no
cluster-scoped duplicate kinds.

### CRDs removed (pre-GA clean break)

| Removed kind | Replacement |
| --- | --- |
| `KollectClusterProfile` | `KollectProfile` in `spec.profileRef.namespace` |
| `KollectClusterSnapshotSink` | `KollectSnapshotSink` in sink ref namespace |
| `KollectClusterDatabaseSink` | `KollectDatabaseSink` in sink ref namespace |
| `KollectClusterEventSink` | `KollectEventSink` in sink ref namespace |

**Retained:** all namespaced static kinds; all cluster **reconciled** kinds (`KollectClusterTarget`,
`KollectClusterInventory`, reserved `KollectClusterScope`).

### API shape

Introduce a shared **`NamespacedObjectReference`** (name required; namespace optional with
documented default):

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectClusterTarget
metadata:
  name: platform-deployments
spec:
  profileRef:
    name: deployment-baseline
    namespace: kollect-system   # required on cluster kinds (no implicit default)
  namespaceSelector:
    matchLabels:
      kollect.dev/collect: "true"
---
apiVersion: kollect.dev/v1alpha1
kind: KollectClusterInventory
metadata:
  name: platform-rollup
spec:
  sinkNamespace: kollect-system          # default for refs omitting namespace
  snapshotSinkRefs:
    - name: git-backup                   # resolves in sinkNamespace
    - name: team-git
      namespace: team-a                  # explicit cross-namespace export
  databaseSinkRefs:
    - name: warehouse
      namespace: kollect-system
  targetRefs:
    - platform-deployments
```

Apply the same `NamespacedObjectReference` to:

- `KollectClusterTarget.spec.profileRef`
- `KollectClusterInventory.spec.profileRef` (optional rollup override)
- `InventorySinkRef.namespace` (optional; inherits `sinkNamespace` on cluster inventory only)
- `KollectClusterScope` allowlists when that kind ships (mirror namespaced `KollectScope`)

**Namespaced** `KollectTarget` / `KollectInventory` keep same-namespace resolution (namespace field
omitted or forbidden on namespaced kinds at webhook).

**Breaking:** plain string `profileRef` and cluster-sink fallback resolution are **removed** — no
conversion webhook in v1alpha1 ([ADR-0206](0206-api-versioning-conversion.md)).

### Resolution rules

1. **Cluster kinds:** `profileRef.namespace` is **required** at admission.
2. **Cluster inventory sinks:** resolve `namespace/name` per ref; default namespace =
   `spec.sinkNamespace` when ref omits `namespace`.
3. **Never** attempt cluster-scoped sink or profile GET after this change.
4. **Secrets and TLS** for a sink resolve in the **sink object's namespace** (unchanged semantics;
   `SinkNamespaceForResolved`).

### RBAC and authorization

Cross-namespace reads must be **explicit, checked, and observable** — not inferred from CRD scope.

#### Operator ServiceAccount RBAC

- **ClusterRole** (platform golden path): `get/list/watch` on namespaced
  `kollectprofiles`, `kollectsnapshotsinks`, `kollectdatabasesinks`, `kollecteventsinks` **across
  all namespaces** the operator may reference — same as today for workload informers. Chart
  documents which namespaces platform teams should use (`kollect-system`, export NS).
- **tenantMode** ([ADR-0203](0203-namespaced-multi-tenancy.md)): Role scoped to watched namespaces
  only; cluster kinds are **unsupported** in tenantMode installs (admission rejects
  `KollectClusterTarget` / `KollectClusterInventory` when `tenantMode: true`).

#### Reconcile-time checks

| Check | When | Denied behavior |
| --- | --- | --- |
| **SelfSubjectAccessReview (SSAR)** | Before reading a profile or sink outside the operator's home namespace | `ErrForbidden` — degrade cluster target/inventory; condition `Ready=False`, `Reason=Forbidden`, message names `namespace/kind/name` |
| **SSAR** | Before listing/watching workload GVK in a tenant namespace | Existing collection path ([ADR-0203](0203-namespaced-multi-tenancy.md)) — per-target `skipped:forbidden` |
| **`KollectClusterScope` allowlist** (when shipped) | Profile/sink ref namespace ∈ allowed set | `Degraded=True`, `Reason=ScopeNamespaceDenied` or `ScopeSinkDenied` |

Do **not** escalate privileges. Missing permission → degrade + status, never a blind retry loop
([ADR-0602](0602-error-taxonomy.md)).

#### Admission validation

Validating webhook (dry-run GET where SA has permission):

- Target profile and inventory sink refs must **exist** in the declared namespace (terminal error at
  apply time when GET succeeds and object is wrong kind).
- Reject `profileRef` without namespace on cluster kinds.
- Reject references to removed cluster-scoped kinds (CRD absent after migration).

When webhook SA lacks GET on a referenced namespace, admission may pass with a **warning** (SSAR at
reconcile is authoritative) — same pattern as optional CEL compile against live CRDs.

### Observability (required before merge)

All paths below must have **unit or envtest coverage** asserting label values / condition reasons
([ADR-0706](0706-testing-merge-gate-architecture.md)).

#### Conditions and events

| Reason | Type | When |
| --- | --- | --- |
| `ProfileForbidden` | `Ready=False` | SSAR denies `get` on referenced profile |
| `ProfileNotFound` | `Degraded=True` | Profile missing (terminal) |
| `SinkForbidden` | `Ready=False` | SSAR denies `get` on referenced sink |
| `SinkNotFound` | `Degraded=True` | Sink missing in declared namespace (terminal) |
| `SinkNamespaceDenied` | `Degraded=True` | Ref namespace outside `KollectClusterScope` allowlist |

Emit **Warning** events on first transition to forbidden/degraded with `namespace`, `name`, and
`resource` (no secret data).

#### Prometheus metrics

Extend existing counters ([ADR-0602](0602-error-taxonomy.md)) — do not explode cardinality:

| Metric | Labels | Notes |
| --- | --- | --- |
| `kollect_reconcile_errors_total` | `kind`, `error_class=forbidden` | Increment on SSAR denial resolving static refs |
| `kollect_static_ref_resolution_total` | `kind`, `ref_type` (`profile`/`snapshot`/`database`/`event`), `result` (`ok`/`not_found`/`forbidden`) | New; bounded enum labels |
| `kollect_sink_exports_total` | existing + `sink_namespace` | Low-cardinality: only namespaces referenced by cluster inventories |

#### Status fields

- `KollectClusterInventory.status.sinkExports[].name` continues to use `family/name` key; add
  **`namespace`** field for portal/debug.
- `KollectClusterTarget.status` (when collection controller ships): record resolved
  `profileNamespace/profileName` in status for supportability.

### Testing and merge gates

Implementation is **not complete** until all tiers pass:

| Tier | Scope | Required cases |
| --- | --- | --- |
| **L0 unit** | `internal/sink/resolver`, profile resolver | Namespace-qualified resolve; no cluster fallback; forbidden error wrapping |
| **L1 envtest** | Webhooks + reconcilers | Cluster target/inventory with refs in `kollect-system`; cross-ns sink ref; SSAR denied → condition + metric |
| **L2 golden** | OpenAPI / sample YAML | Remove cluster static CRD golden files; update cluster inventory/target samples |
| **L3 integration** | Export with namespaced sink only | Cluster inventory → Git sink in `kollect-system` |
| **L4 e2e** | `hack/e2e/multitenant.sh` or new `cluster-rollup.sh` | Platform operator + cluster inventory; assert export; RBAC-trimmed SA → `SinkForbidden` visible in status |
| **Q16 RBAC audit** | `hack/audit-rbac.sh` | Updated `config/rbac/role.yaml` — no rules for removed cluster static resources |

Add **`task verify`** gate: no generated manifests or RBAC referencing removed CRD plurals
(`kollectclusterprofiles`, `kollectclustersnapshotsinks`, …).

### Migration (pre-GA)

1. Delete four cluster static CRD YAMLs from `config/crd/bases/` and `charts/kollect/crds/`.
2. Move any sample cluster profiles/sinks to namespaced manifests under `kollect-system`.
3. Rewrite `KollectClusterTarget` / `KollectClusterInventory` refs to `NamespacedObjectReference`.
4. Remove cluster family sink reconcilers' cluster-scoped branches; keep namespaced connection tests.
5. Sweep docs (`CR-REFERENCE`, cluster rollup example, topology matrix).

No dual-write window — same policy as [ADR-0414](0414-sink-family-crds.md) clean break.

## Alternatives considered

| Option | Verdict |
| --- | --- |
| **Keep cluster static CRDs** (status quo) | Rejected — duplicate surface, ambiguous fallback, weak tenancy |
| **Keep cluster sinks only, drop cluster profile** | Rejected — asymmetric; sinks and profiles share the same resolution problem |
| **`ClusterSecretStore`-style namespace conditions on cluster sinks** | Rejected — adds complexity without reducing CRD count; explicit refs are simpler |
| **Single unified `KollectSink` again** | Rejected — [ADR-0414](0414-sink-family-crds.md) family split stays |

## Consequences

### Positive

- **Four fewer CRDs** in the Helm bundle and CR reference — lower adoption friction.
- One webhook + controller path per family; no `clusterScoped` boolean in `ResolveOptions`.
- Namespace on refs makes tenancy explicit and auditable; aligns with namespaced Profile/Sink
  decision ([ADR-0204](0204-namespaced-profiles.md)).
- Platform shared backends remain: publish sinks once in `kollect-system`, reference from cluster
  inventory — same ops model as today without duplicate kinds.
- Forces RBAC + observability work before platform rollups scale.

### Negative / trade-offs

- Cluster refs require namespace discipline — typos fail at reconcile instead of "magic" platform
  namespace fallback.
- Platform GitOps must place shared profiles/sinks in a known namespace (documented convention).
- **Breaking** for early adopters of cluster static kinds (expected pre-GA).
- Operator ClusterRole still needs cross-namespace GET on static kinds — scope reduction is in **API
  clarity**, not operator privilege (workload collection already required it).

## Supersedes / amends

This ADR **supersedes**:

- [ADR-0204](0204-namespaced-profiles.md) — decision §2 (`KollectClusterProfile` reserved)
- [ADR-0414](0414-sink-family-crds.md) — § "Cluster sink kinds — retained"
- [ADR-0201](0201-crd-model.md) — reserved cluster profile + cluster family sink rows

Amends [ADR-0203](0203-namespaced-multi-tenancy.md) (tenantMode vs cluster kinds) and
[ADR-0104](0104-security-model.md) (static ref SSAR requirements).

## Open questions

- **Default namespace field:** require explicit `profileRef.namespace` on cluster kinds vs default
  `kollect-system` with webhook warning when omitted?
- **Per-ref vs global `sinkNamespace`:** keep both (proposed) or require namespace on every cluster
  inventory sink ref?
- **Promotion timing:** land API + webhook first, or single atomic release with CRD deletion?

## References

- [ADR-0201](0201-crd-model.md) — CRD model
- [ADR-0203](0203-namespaced-multi-tenancy.md) — tenancy and SAR
- [ADR-0204](0204-namespaced-profiles.md) — namespaced profiles
- [ADR-0414](0414-sink-family-crds.md) — sink family CRDs
- [ADR-0104](0104-security-model.md) — security model
- [ADR-0602](0602-error-taxonomy.md) — error classes and metrics
- [ADR-0706](0706-testing-merge-gate-architecture.md) — merge gates
