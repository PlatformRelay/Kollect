# ADR-0207: Target collection filtering — intent vs Scope ceiling

> **Theme:** 02 · API & tenancy · **Status:** Current

## Context

Operators need **whitelist and blacklist** collection policy at namespace, GVK, label, and
attribute levels without patching Namespace objects or watching every object a Profile can
extract. ADR-0201 deferred target-side filters; ADR-0205 watch labels address tenant consent,
not platform exclusion or Trivy severity gates.

Maintainer decisions (rev 3, Option 1):

- **Target / ClusterTarget** — collection intent (`includedNamespaces`, `excludedNamespaces`,
  `namespaceExcludeSelector`, `resourceRules[]`, per-rule `matchPolicy` CEL).
- **Scope / ClusterScope** — hard ceiling only (`allowedGVKs`, `allowedNamespaces`,
  `deniedNamespaces`); no `resourceRules` on Scope.
- **Profile** — extraction only; same Profile reusable across broad vs narrow Targets.
- **Inventory** — no pre-store filters (export-time row filter remains deferred).

## Decision

### Phase 1 — namespace intent + Scope deny

| Field | CRD |
| --- | --- |
| `includedNamespaces`, `excludedNamespaces`, `namespaceExcludeSelector` | `KollectTarget`, `KollectClusterTarget` |
| `deniedNamespaces` | `KollectScope`, `KollectClusterScope` |

### Phase 2 — `resourceRules[]`

Structured rules on Target: `gvk`, optional `namespaceSelector`, `matchLabels` /
`matchExpressions`. Empty `resourceRules` **falls back** to Profile `targetGVK` + Target
`labelSelector` / `names` (backward compatible).

Multiple rules are **OR-unioned** (label rule OR CEL severity for Trivy).

### Phase 3 — CEL `matchPolicy`

Optional per-rule CEL evaluated **after informer event, before store insert** (not export-only).
Reuses the collect engine CEL environment with `object` bound to the resource.

### Evaluation order

![Stacked filter layers showing how Kubernetes resources pass through operator watch scope, Scope denials, Target include and exclude rules, resource rules, CEL match policy, and watch labels before becoming inventory rows.](../assets/illustrations/collection-filter-layers-dark.webp){ .kollect-illus .kollect-illus--portrait width="360" }

1. Helm `watchNamespaces` / operator cache boundary
2. Scope `deniedNamespaces` (non-overridable)
3. Target include intent (`includedNamespaces` ∩ `namespaceSelector`)
4. Target exclude (`excludedNamespaces`, `namespaceExcludeSelector`)
5. Scope `allowedNamespaces` cap
6. `resourceRules[]` union (OR)
7. Per-rule `matchPolicy` CEL
8. Scope `allowedGVKs` (admission + reconcile)
9. `ShouldCollect` watch labels (ADR-0205)

Effective namespaces:

`(Target intent) ∩ (Scope allowedNamespaces if set) \ (Target exclude ∪ Scope denied ∪ Helm exclude defaults)`

Helm `defaultIncludedNamespaces` / `defaultExcludedNamespaces` apply when CRD fields are empty;
CRD overrides chart defaults.

### Status

Targets expose `status.matchedNamespaces`, `status.effectiveNamespaces`, `status.activeResourceRules`.

### Admission

Validating webhooks reject Target `includedNamespaces` outside Scope `allowedNamespaces`, any
intent namespace in Scope `deniedNamespaces`, and GVKs outside `allowedGVKs`. Reconcile degrades
as backstop (`ScopeNamespaceDenied`, `ScopeGVKDenied`).

## Consequences

- Platform can deny `kube-system` via Scope without Namespace patch RBAC.
- Teams express Trivy HIGH-only collection without a dedicated Profile per severity slice.
- `KollectCollectionRule` CRD deferred until inline rules prove insufficient.
- RBAC generation should eventually union GVKs from `resourceRules` (follow-up).

## See also

- [ADR-0201](0201-crd-model.md) · [ADR-0203](0203-namespaced-multi-tenancy.md)
- [ADR-0205](0205-watch-labels.md) · [ADR-0302](0302-cel-jsonpath-extraction.md)
