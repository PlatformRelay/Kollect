# KollectClusterScope

Cluster-scoped tenancy **ceiling** for platform operators ([ADR-0207](../adr/0207-target-collection-filtering.md)).

## Spec

| Field | Role |
| --- | --- |
| `allowedGVKs` | Cap on GVKs cluster targets may collect |
| `allowedNamespaces` | Cap on workload namespaces |
| `deniedNamespaces` | Platform blacklist — not overridable by Targets |
| `sinkRefs` | Permitted cluster sink names for export |

Static config only — no status subresource ([ADR-0202](../adr/0202-static-vs-reconciled.md)).

## Example

A cluster-wide ceiling that caps platform collection to `Deployment`/`Service`, blocks
`kube-system`, and allows export only to a named cluster sink:

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectClusterScope
metadata:
  name: platform-ceiling   # cluster-scoped — no namespace
spec:
  allowedGVKs:
    - group: apps
      version: v1
      kind: Deployment
    - group: ""
      version: v1
      kind: Service
  deniedNamespaces:
    - kube-system           # platform blacklist — targets cannot override
  sinkRefs:
    - platform-warehouse
```

The namespaced [`KollectScope`](kollectscope.md) sample
([`config/samples/kollect_v1alpha1_kollectscope_team-a.yaml`](https://github.com/konih/kollect/blob/main/config/samples/kollect_v1alpha1_kollectscope_team-a.yaml))
shows the same fields scoped to a single namespace.

## See also

- [KollectScope](kollectscope.md) — namespaced ceiling
- [KollectClusterTarget](kollectclustertarget.md) — collection intent
