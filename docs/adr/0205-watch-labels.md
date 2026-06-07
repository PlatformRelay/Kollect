# ADR-0205: Watch opt-in and opt-out labels

> `kollect.dev/watch` labels and `watchMode: All|OptIn` decide which objects get collected.

**Theme:** 02 · API & tenancy · **Status:** Current

## Context

Platform teams need a **discoverable, kubectl-friendly** way to exclude noisy namespaces or
workloads from inventory without editing every `KollectTarget`, and to run **opt-in** collection
in shared clusters where most tenants should be ignored by default.

Flux uses a similar pattern on Kustomizations:

```yaml
metadata:
  annotations:
    kustomize.toolkit.fluxcd.io/reconcile: disabled
```

Kollect needs parallel semantics for **collection watch** — explicit enable/disable signals on
namespaces and resources that the collection engine honors before attribute extraction.

## Decision

### Label and annotation keys

| Key | On | Values | Meaning |
| --- | --- | --- | --- |
| `kollect.dev/watch` | Namespace, namespaced resource | `enabled`, `disabled` | Resource/namespace watch opt-in or opt-out |
| `kollect.dev/namespace-watch` | Namespace (annotation) | `enabled`, `disabled` | Applies to **all resources** in the namespace unless a resource label overrides |

Constants live in `api/v1alpha1/constants.go`.

### `KollectTarget.spec.watchMode`

| Mode | Default | Behavior |
| --- | --- | --- |
| `All` | yes | Collect objects matching selectors **except** those explicitly `disabled` |
| `OptIn` | no | Collect **only** objects/namespaces explicitly `enabled` |

Validated by CEL enum on the CRD and the `KollectTarget` validating webhook.

### Precedence (engine `ShouldCollect`)

1. Resource label `kollect.dev/watch: disabled` — **always skip** (wins over everything).
2. Resource label `kollect.dev/watch: enabled` — **collect** (overrides namespace disabled).
3. Namespace label `kollect.dev/watch: disabled` or annotation `kollect.dev/namespace-watch: disabled` — skip all resources in namespace (unless step 2).
4. `watchMode: OptIn` — require namespace or resource `enabled`; otherwise skip.
5. `watchMode: All` (default) — collect when selectors match and no opt-out applies.

The collection engine evaluates watch labels **after** namespace/label selectors and **before**
attribute extraction. Items removed from the watch set are dropped from the in-memory store on
update events.

### Examples

**Opt-out a namespace** (All mode target still matches selectors):

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: kube-system
  annotations:
    kollect.dev/namespace-watch: disabled
```

**Opt-in cluster** (target uses `watchMode: OptIn`):

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectTarget
spec:
  watchMode: OptIn
  profileRef: deployment-images
```

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: team-a
  labels:
    kollect.dev/watch: enabled
```

**Opt-out one Deployment in an otherwise watched namespace**:

```yaml
metadata:
  labels:
    kollect.dev/watch: disabled
```

## Consequences

- Operators can document a single label key for both namespaces and workloads.
- Namespace annotations avoid relabeling every resource for bulk opt-out.
- Opt-in mode supports multi-tenant clusters without per-namespace target forks.
- Namespace metadata is cached when targets register; label/annotation changes propagate on
  target reconcile and informer resync (12h), not via a dedicated Namespace watch (future
  improvement if needed).
- **Collection policy** (namespace allow/deny, `resourceRules`, CEL `matchPolicy`) is evaluated
  **before** watch labels — see [ADR-0207](0207-target-collection-filtering.md). Watch labels remain
  tenant consent, not platform or attribute filtering.

## Open questions

- **DECIDED :** Under `watchMode: OptIn`, cluster-scoped resources honor a
  **target-level default opt-in** (e.g. `clusterScopedDefault: Include` on the
  `KollectTarget`/`KollectClusterTarget`), since they have no namespace to inherit from. A per-object
  **opt-out label still wins**. Platform targets can inventory Nodes/PVs without labeling each object.
- **OPEN:** Dedicated Namespace informer to refresh watch metadata without waiting for target reconcile?
