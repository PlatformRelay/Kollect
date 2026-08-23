# KollectClusterScope

Cluster-scoped tenancy **ceiling** for platform operators ([ADR-0207](../adr/0207-target-collection-filtering.md)).

## Spec

| Field | Role |
| --- | --- |
| `allowedGVKs` | Cap on GVKs cluster targets may collect |
| `allowedNamespaces` | Cap on workload namespaces |
| `deniedNamespaces` | Platform blacklist — not overridable by Targets |
| `snapshotSinkRefs` | Permitted `KollectSnapshotSink` names for export |
| `databaseSinkRefs` | Permitted `KollectDatabaseSink` names for export |
| `eventSinkRefs` | Permitted `KollectEventSink` names for export |

The three allowlists are independent, one per sink family ([ADR-0414](../adr/0414-sink-family-crds.md)).
**An empty allowlist is not a deny — it is "no restriction" for that family**
(`internal/scope/scope.go`), so a ceiling that means to constrain database export must populate
`databaseSinkRefs` explicitly.

Static config only — no status subresource ([ADR-0202](../adr/0202-static-vs-reconciled.md)).

## Example

A cluster-wide ceiling that caps platform collection to `Deployment`/`Service`, blocks
`kube-system`, and caps export on **all three** sink families. All three lists are populated on
purpose: leaving one out would leave that family unrestricted, not denied.

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
  snapshotSinkRefs:
    - platform-audit-git
  databaseSinkRefs:
    - platform-warehouse
  eventSinkRefs:
    - platform-events
```

The namespaced [`KollectScope`](kollectscope.md) sample
([`config/samples/kollect_v1alpha1_kollectscope_team-a.yaml`](https://github.com/platformrelay/kollect/blob/main/config/samples/kollect_v1alpha1_kollectscope_team-a.yaml))
shows the same fields scoped to a single namespace.

## Enforcement

The ceiling is checked twice ([ADR-0207](../adr/0207-target-collection-filtering.md)):

| Stage | Object | Checks | On violation |
| --- | --- | --- | --- |
| Admission | [`KollectClusterTarget`](kollectclustertarget.md) | `allowedGVKs` (profile `targetGVK` and `resourceRules`), `allowedNamespaces`, `deniedNamespaces`, `allowedStaticRefNamespaces` | Create/update rejected |
| Reconcile | `KollectClusterTarget` | `allowedGVKs`, `allowedStaticRefNamespaces` | Informers unregistered; `Degraded=True` with `ScopeGVKDenied` or `ScopeNamespaceDenied` |
| Reconcile | [`KollectClusterInventory`](kollectclusterinventory.md) | `snapshotSinkRefs` / `databaseSinkRefs` / `eventSinkRefs`, `allowedStaticRefNamespaces` on sink refs | `Degraded=True` with `ScopeSinkDenied` or `SinkNamespaceDenied` |

Reconcile is the backstop for objects admitted before the ceiling existed or was tightened, and for
targets created while `profileRef` did not yet resolve — a missing profile makes the profile
`targetGVK` unknowable at admission time. `allowedNamespaces` and `deniedNamespaces` additionally cap
`status.effectiveNamespaces` at collect time, so a selector that matches a denied namespace filters it
out rather than degrading the target.

## See also

- [KollectScope](kollectscope.md) — namespaced ceiling
- [KollectClusterTarget](kollectclustertarget.md) — collection intent
