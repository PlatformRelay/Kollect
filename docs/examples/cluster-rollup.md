# Example: Cluster-scoped rollup

## Prerequisites

A running operator with cluster-scope RBAC and existing namespaced profiles and family sinks.

## Apply

```sh
kubectl apply -f config/samples/kollect_v1alpha1_kollectclustertarget.yaml
kubectl apply -f config/samples/kollect_v1alpha1_kollectclusterinventory.yaml
```

## Verify

```sh
kubectl get kctgt,kcinv
kubectl describe kcinv cluster-inventory
```

## If it didn't work

Check structured name/namespace references, cluster RBAC, selectors, and status conditions.

## Cleanup

```sh
kubectl delete -f config/samples/kollect_v1alpha1_kollectclusterinventory.yaml
kubectl delete -f config/samples/kollect_v1alpha1_kollectclustertarget.yaml
```

## Further reading

[Cluster inventory reference](../crds/kollectclusterinventory.md) · [Multi-cluster](../concepts/multi-cluster.md)

!!! tip "Prerequisites"
    Platform cross-namespace collection requires cluster-scoped RBAC and labeled workload namespaces.
    For team-scoped e2e, use [deployment-inventory.md](../getting-started/first-inventory.md) first.

A namespaced `KollectProfile` (in `kollect-system`) + `KollectClusterTarget` + `KollectClusterInventory`
roll up platform-wide rows and export to sinks resolved per ref namespace.

`KollectClusterTarget.spec.profileRef` requires explicit `name` + `namespace`; cluster-inventory sink
refs resolve by `name` + `namespace`, defaulting to `spec.sinkNamespace` (`kollect-system`) when a ref
omits `namespace` — no cluster-scoped static config kinds
([ADR-0208](../adr/0208-cluster-static-refs-via-namespace.md)).

Samples in `config/samples/kustomization.yaml`.

Dedupe: `spec.dedupe` — `keepAll` (default) or `byResourceUID` ([ADR-0305](../adr/0305-aggregation-dedupe.md)).
