# Example: Multi-tenant watch scope

!!! tip "Platform operator setup"
    Combine Helm `tenantMode: true` with `watchNamespaces` so each team namespace gets an isolated
    collection boundary. Add `KollectScope` when you need GVK, namespace, or sink allow-lists.

`tenantMode` + `watchNamespaces` + `KollectScope` ([ADR-0203](../adr/0203-namespaced-multi-tenancy.md)).

Scope sample: `kollect_v1alpha1_kollectscope_team-a.yaml`. Opt-in: `kollecttarget_opt-in.yaml`.

!!! note "Watch labels"
    Teams can opt out individual namespaces or resources with `kollect.dev/watch` and
    `kollect.dev/namespace-watch` without changing Helm values
    ([ADR-0205](../adr/0205-watch-labels.md)).

Watch labels: `kollect.dev/watch`, `kollect.dev/namespace-watch` ([ADR-0205](../adr/0205-watch-labels.md)).
