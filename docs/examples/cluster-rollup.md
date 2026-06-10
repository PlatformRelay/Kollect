# Example: Cluster-scoped rollup

!!! tip "Prerequisites"
    Platform cross-namespace collection requires cluster-scoped RBAC and labeled workload namespaces.
    For team-scoped e2e, use [deployment-inventory.md](deployment-inventory.md) first.

A namespaced `KollectProfile` (in `kollect-system`) + `KollectClusterTarget` + `KollectClusterInventory`
roll up platform-wide rows and export to sinks resolved per ref namespace.

`KollectClusterTarget.spec.profileRef` requires explicit `name` + `namespace`; cluster-inventory sink
refs resolve by `name` + `namespace`, defaulting to `spec.sinkNamespace` (`kollect-system`) when a ref
omits `namespace` — no cluster-scoped static config kinds
([ADR-0208](../adr/0208-cluster-static-refs-via-namespace.md)).

Samples in `config/samples/kustomization.yaml`.

Dedupe: `spec.dedupe` — `keepAll` (default) or `byResourceUID` ([ADR-0305](../adr/0305-aggregation-dedupe.md)).
