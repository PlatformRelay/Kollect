# Example: Spoke cluster inventory

Default MVP: Profile → Target → Inventory → namespaced Sink ([ADR-0703](../adr/0703-platform-architecture-pivot.md)).

```yaml
tenantMode: true
watchNamespaces: [team-a]
mode: single
```

Samples: `kollect_v1alpha1_kollectprofile.yaml`, `kollectsink_postgres.yaml`, `kollecttarget.yaml`, `kollectinventory.yaml`.

E2e without backends: `config/samples/e2e/team-inventory.yaml` (`sinkRefs: []`).

See [Hub mode](hub-mode.md) for multi-cluster.
