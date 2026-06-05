# Example: Cluster-scoped rollup

!!! warning "Controller not wired (Phase 1)"
    Cluster-scoped CRs validate at admission but **do not reconcile** until the platform controller
    milestone lands. Use namespaced `KollectTarget` + `KollectInventory` for working e2e today.

`KollectClusterProfile` + `KollectClusterTarget` + `KollectClusterInventory`.

Samples in `config/samples/kustomization.yaml`. `sinkNamespace: kollect-system` resolves sink CRs.

Dedupe: `spec.dedupe` — `keepAll` (default) or `byResourceUID` ([ADR-0305](../adr/0305-aggregation-dedupe.md)).
