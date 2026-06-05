# Example: Multi-tenant watch scope

`tenantMode` + `watchNamespaces` + `KollectScope` ([ADR-0203](../adr/0203-namespaced-multi-tenancy.md)).

Scope sample: `kollect_v1alpha1_kollectscope_team-a.yaml`. Opt-in: `kollecttarget_opt-in.yaml`.

Watch labels: `kollect.dev/watch`, `kollect.dev/namespace-watch` ([ADR-0205](../adr/0205-watch-labels.md)).
