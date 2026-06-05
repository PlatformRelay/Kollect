# KollectScope

**Scope:** Namespace · **Reconciled:** No (enforced by other controllers) · **Short name:** `kscope`

Namespaced tenancy boundary — AppProject-inspired policy for allowed GVKs, workload namespaces, and
sinks ([ADR-0016](../adr/0016-namespaced-multi-tenancy.md)).

## Spec fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `spec.allowedGVKs[]` | list | No | Permitted target resource kinds (`group`, `version`, `kind`) |
| `spec.allowedNamespaces[]` | list | No | Permitted workload namespaces (empty = any allowed by targets) |
| `spec.sinkRefs[]` | list | No | Permitted `KollectSink` names for export |

## Status conditions

| Type | When set | Meaning |
| --- | --- | --- |
| *(none)* | — | Static CR — violations surface on Target/Inventory status |

## RBAC

| Verb | Resource | Notes |
| --- | --- | --- |
| `get`, `list`, `watch` | `kollectscopes` | Target/Inventory controllers read scope |
| `create`, `update`, `patch`, `delete` | `kollectscopes` | Platform/team admins |

## Samples

- [`config/samples/kollect_v1alpha1_kollectscope_team-a.yaml`](../../config/samples/kollect_v1alpha1_kollectscope_team-a.yaml)

## Failure modes

> **TODO:** Document hard degrade reasons: `ScopeGVKDenied`, `ScopeNamespaceDenied`, `ScopeSinkDenied`,
> and missing scope when enforcement expected.
