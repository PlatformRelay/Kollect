# Example: Multi-tenant watch scope

## Prerequisites

A running operator, a tenant namespace, and permission to create a `KollectScope` there.

## Apply

```sh
kubectl apply -f config/samples/kollect_v1alpha1_kollectscope_team-a.yaml
```

Apply the target and workload shown below only after their namespaces match the scope policy.

## Verify

```sh
kubectl get kscope -n team-a
kubectl describe kscope -n team-a
```

## If it didn't work

Check allowed GVKs, namespaces, family sinks, watch labels, and the target's degraded reason.

## Cleanup

```sh
kubectl delete -f config/samples/kollect_v1alpha1_kollectscope_team-a.yaml
```

## Further reading

[Multi-tenancy](../concepts/multi-tenancy.md) · [Annotations and labels](../ANNOTATIONS-LABELS.md)

!!! tip "Platform operator setup"
    Combine Helm `tenantMode: true` with `watchNamespaces` so each team namespace gets an isolated
    collection boundary. Add `KollectScope` when you need GVK, namespace, or sink allow-lists.

`tenantMode` + `watchNamespaces` + `KollectScope` ([ADR-0203](../adr/0203-namespaced-multi-tenancy.md)).

**Team-owned install:** use chart profile [`values-minimal-rbac.yaml`](https://github.com/platformrelay/kollect/blob/main/charts/kollect/values-minimal-rbac.yaml)
and the full walkthrough in [Team-owned operator (minimal RBAC)](../examples/team-operator.md).
Apply `kubectl apply -k config/samples/team-operator/` after the Helm install.

Scope sample: `kollect_v1alpha1_kollectscope_team-a.yaml` or `config/samples/team-operator/`.
Opt-in: `kollecttarget_opt-in.yaml`.

!!! note "Watch labels"
    Teams can opt out individual namespaces or resources with `kollect.dev/watch` and
    `kollect.dev/namespace-watch` without changing Helm values
    ([ADR-0205](../adr/0205-watch-labels.md)).

Watch labels: `kollect.dev/watch`, `kollect.dev/namespace-watch` ([ADR-0205](../adr/0205-watch-labels.md)).

![Three tenant namespaces inside one cluster, each bounded by KollectScope policy, with separate inventory pipelines exporting to allowed sinks and one namespace denied by scope rules.](../assets/illustrations/multi-tenant-scope-boundaries-dark.webp){ .kollect-illus .kollect-illus--wide width="800" }
