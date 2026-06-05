# Example: Cluster-scoped rollup

!!! tip "Prerequisites"
    Platform cross-namespace collection requires cluster-scoped RBAC and labeled workload namespaces.
    For team-scoped e2e, use [deployment-inventory.md](deployment-inventory.md) first.

`KollectClusterProfile` + `KollectClusterTarget` + `KollectClusterInventory` roll up platform-wide
rows and export to sinks in `spec.sinkNamespace`.

Samples in `config/samples/kustomization.yaml`. `sinkNamespace: kollect-system` resolves sink CRs.

Dedupe: `spec.dedupe` — `keepAll` (default) or `byResourceUID` ([ADR-0305](../adr/0305-aggregation-dedupe.md)).
