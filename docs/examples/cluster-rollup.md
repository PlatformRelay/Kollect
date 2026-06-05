# Example: Cluster-scoped rollup

`KollectClusterProfile` + `KollectClusterTarget` + `KollectClusterInventory`.

Samples in `config/samples/kustomization.yaml`. `sinkNamespace: kollect-system` resolves sink CRs.

Dedupe: `spec.dedupe` — `keepAll` (default) or `byResourceUID` ([ADR-0305](../adr/0305-aggregation-dedupe.md)).
