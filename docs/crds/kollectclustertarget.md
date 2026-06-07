# KollectClusterTarget

**Scope:** Cluster · **Reconciled:** Yes · **Short name:** `kctgt`

!!! tip "Platform vs team scope"
    Use `KollectClusterTarget` for cross-namespace platform collection. Team-scoped flows use
    namespaced `KollectTarget` + `KollectInventory` instead.

## What it is for

A `KollectClusterTarget` is the **platform-operator** variant of `KollectTarget`: it collects
across multiple namespaces using a cluster-scoped object and a required `namespaceSelector`. It
pairs with `KollectClusterInventory` for platform-wide rollup export
([ADR-0201](../adr/0201-crd-model.md)).

The controller registers shared informers per profile GVK and filters events across namespaces
matched by `spec.namespaceSelector`.

## How it fits the pipeline

```mermaid
flowchart TD
  CProf[KollectClusterProfile]
  Profile[KollectProfile in platform NS]
  CTarget[KollectClusterTarget]
  CInv[KollectClusterInventory]
  Sink[KollectSink in sinkNamespace]

  CProf -->|"profileRef (preferred)"| CTarget
  Profile -.->|"profileRef (MVP fallback)"| CTarget
  CTarget -->|rows| CInv
  CInv --> Sink
```

| Relationship | Rule |
| --- | --- |
| Profile | `spec.profileRef` resolves to **`KollectClusterProfile`** (preferred) or `KollectProfile` in **platform namespace** (MVP fallback) |
| Namespaces | `namespaceSelector` **required** — empty selector rejected at admission |
| Namespaced pipeline | Team flows use `KollectTarget` + `KollectInventory` instead |

Walkthrough: [examples/cluster-rollup.md](../examples/cluster-rollup.md).

## Spec fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `spec.profileRef` | string | Yes | Name of `KollectClusterProfile` or platform-namespace `KollectProfile` (name only) |
| `spec.namespaceSelector` | labelSelector | **Yes** | Required — webhook rejects empty selector (no cluster-wide implicit scrape) |
| `spec.suspend` | bool | No | Pause reconciliation (reserved) |

## Sample usage

```sh
# Cluster profile (preferred) or namespaced profile in platform namespace
kubectl apply -f config/samples/kollect_v1alpha1_kollectclusterprofile.yaml
kubectl apply -f config/samples/kollect_v1alpha1_kollectclustertarget.yaml

kubectl get kctgt platform-argo-applications -o yaml
kubectl describe kctgt platform-argo-applications
```

Label namespaces for the sample selector:

```sh
kubectl label namespace argocd kollect.dev/tenant=platform --overwrite
```

```sh
kubectl get kctgt platform-argo-applications -w
kubectl describe kctgt platform-argo-applications
```

## Status conditions

| Type | When set | Meaning | Remediation |
| --- | --- | --- | --- |
| `Ready=True` | Collecting | Profile resolved; informer registered across matched namespaces | None |
| `Degraded=True` | Blocked | See `reason` below | Fix root cause; generation bump re-reconciles |

### Common `Degraded` reasons

| Reason | Cause | Fix |
| --- | --- | --- |
| `Suspended` | `spec.suspend: true` | Set `suspend: false` |
| `ProfileNotFound` | No matching cluster or platform profile | Create `KollectClusterProfile` or platform-namespace `KollectProfile` |
| `InformerRegistrationFailed` | Dynamic client / GVK error | Verify CRD installed; check operator logs |

## RBAC

| Actor | Verbs | Resource | Notes |
| --- | --- | --- | --- |
| Platform admins | `create`, `update`, `patch`, `delete` | `kollectclustertargets` | Cluster-scoped |
| Platform readers | `get`, `list`, `watch` | `kollectclustertargets` | Audit platform config |
| Operator | `get`, `list`, `watch` + target GVK verbs | cluster + dynamic | Cross-namespace list |

Cluster-scoped resources require elevated RBAC — restrict to platform SRE roles.

## Common failure modes

| Symptom | Cause | Fix |
| --- | --- | --- |
| Admission denied | Missing `profileRef` | Set profile name (not `namespace/name`) |
| Admission denied | Missing `namespaceSelector` | Add explicit label selector |
| Admission denied | `profileRef` contains `/` | Use name only — profile lives in platform namespace |
| No collection | Empty `namespaceSelector` match or RBAC denied | Label namespaces; extend operator ClusterRole for target GVK |
| `ProfileNotFound` | No `KollectClusterProfile` or platform profile | Create `kcprof` or namespaced profile in platform namespace |
| `Degraded` / `Forbidden` | SAR denies list in scoped NS | Grant operator read on target GVK in workload namespaces |

## See also

- [KollectClusterProfile](kollectclusterprofile.md) — platform extraction schema
- [KollectClusterInventory](kollectclusterinventory.md) — pairs with this kind
- [KollectTarget](kollecttarget.md) — namespaced equivalent (shipped)
- [CR-REFERENCE.md](../CR-REFERENCE.md) — reserved cluster kinds
- [PLATFORM-DECISIONS.md](../PLATFORM-DECISIONS.md)
- [ADR-0201](../adr/0201-crd-model.md)
