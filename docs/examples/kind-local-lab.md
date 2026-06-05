# Example: Kind local lab

!!! tip "Fastest local path"
    `task kind-dev-up` provisions the `kollect-dev` cluster, builds the operator image, and deploys
    Helm in one step — faster than the manual sequence in [QUICKSTART.md](../QUICKSTART.md).

```sh
task kind-dev-up
kubectl apply -k config/samples/
```

!!! note "No Postgres required"
    For a minimal e2e without a database, apply `config/samples/e2e/team-inventory.yaml` (`sinkRefs: []`).
    See [QUICKSTART.md](../QUICKSTART.md) and the [Examples index](README.md).
