# Example: Kind local lab

!!! tip "Fastest local path"
    `task kind-dev-up` provisions the `kollect-dev` cluster, builds the operator image, and deploys
    Helm in one step — faster than the manual sequence in [QUICKSTART.md](../QUICKSTART.md).

## Install the operator

=== "kind (local dev)"

    One-shot from the repo root (builds image, loads into kind, Helm deploy):

    ```sh
    task kind-dev-up
    kubectl apply -k config/samples/
    ```

    Manual steps (same as [QUICKSTART](../QUICKSTART.md)):

    ```sh
    kind create cluster --name kollect-dev
    task build && task install:crds && task docker:build
    kind load docker-image kollect-controller-manager:dev --name kollect-dev
    task deploy:operator
    kubectl apply -k config/samples/
    ```

=== "Helm"

    On an existing cluster (including kind):

    ```sh
    helm install kollect ../../charts/kollect -n kollect-system --create-namespace
    kubectl -n kollect-system rollout status deployment/kollect-controller-manager --timeout=120s
    kubectl apply -k config/samples/
    ```

    Production values and upgrades: [Operator manual](../OPERATOR-MANUAL.md).

!!! note "No Postgres required"
    For a minimal e2e without a database, apply `config/samples/e2e/team-inventory.yaml` (`sinkRefs: []`).
    See [QUICKSTART.md](../QUICKSTART.md) and the [Examples index](README.md).
