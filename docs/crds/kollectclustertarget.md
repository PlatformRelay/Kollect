# KollectClusterTarget

**Scope:** Cluster · **Reconciled:** Webhook only (Phase 1) · **Short name:** `kctgt`

Platform operator collection across namespaces. Pairs with reserved **`KollectClusterInventory`**
for rollup export ([ADR-0032](../adr/0032-platform-architecture-pivot.md)).

## Spec fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `spec.profileRef` | string | Yes | `KollectClusterProfile` or platform-namespace profile |
| `spec.namespaceSelector` | labelSelector | No | Restrict collected workload namespaces |
| `spec.suspend` | bool | No | Pause reconciliation (reserved) |

## Status conditions

| Type | When set | Meaning |
| --- | --- | --- |
| *(reserved)* | Future controller | `Ready`, `Degraded` — not wired in Phase 1 |

## RBAC

| Verb | Resource | Notes |
| --- | --- | --- |
| `get`, `list`, `watch` | `kollectclustertargets` | Cluster-scoped — platform RBAC |
| `create`, `update`, `patch`, `delete` | `kollectclustertargets` | Platform admins |

## Samples

- [`config/samples/kollect_v1alpha1_kollectclustertarget.yaml`](../../config/samples/kollect_v1alpha1_kollectclustertarget.yaml)

## Failure modes

> **TODO:** Document webhook validation (invalid profileRef, namespaceSelector), and future scope
> interaction when cluster controller lands.
