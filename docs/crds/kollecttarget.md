# KollectTarget

**Scope:** Namespace · **Reconciled:** Yes · **Short name:** `ktgt`

Binds a namespaced `KollectProfile` to workload namespaces and label selectors. Registers a shared
informer per GVK ([ADR-0014](../adr/0014-event-driven-informers.md),
[ADR-0029](../adr/0029-watch-labels.md)).

## Spec fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `spec.profileRef` | string | Yes | `KollectProfile` name in same namespace |
| `spec.namespaceSelector` | labelSelector | No | Restrict collected workload namespaces |
| `spec.labelSelector` | labelSelector | No | Restrict collected resources |
| `spec.names[]` | list | No | Explicit resource names |
| `spec.suspend` | bool | No | Pause reconciliation |
| `spec.watchMode` | enum | No | `All` (default) or `OptIn` — see watch labels |

## Status conditions

| Type | When set | Meaning |
| --- | --- | --- |
| `Ready` | Collection healthy | Target registered with engine |
| `Degraded` | Scope or profile error | Collection blocked |
| `SinkReachable` | Inventory export path | Reflects sink reachability for linked inventory |

## RBAC

| Verb | Resource | Notes |
| --- | --- | --- |
| `get`, `list`, `watch` | `kollecttargets`, `kollectprofiles` | Controller reads profile |
| `get`, `list`, `watch` | Target GVK resources | Dynamic client per profile |
| `update`, `patch` | `kollecttargets/status` | Controller writes status |

## Samples

- [`config/samples/kollect_v1alpha1_kollecttarget.yaml`](../../config/samples/kollect_v1alpha1_kollecttarget.yaml)
- [`config/samples/kollect_v1alpha1_kollecttarget_argo-applications.yaml`](../../config/samples/kollect_v1alpha1_kollecttarget_argo-applications.yaml)
- [`config/samples/kollect_v1alpha1_kollecttarget_opt-in.yaml`](../../config/samples/kollect_v1alpha1_kollecttarget_opt-in.yaml)

## Failure modes

> **TODO:** Document `ScopeGVKDenied`, profile not found, SAR forbidden on target GVK, and suspended
> target behavior.
