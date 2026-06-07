# Command reference

Quick lookup for **kubectl** shortcuts, **Task** targets, and **Helm** commands used with Kollect.
For narrative walkthroughs, see [Quick start](QUICKSTART.md) and [Development setup](DEVELOPMENT.md).

!!! tip "Assumptions"
    Commands assume a working cluster and kubectl context. Local kind bootstrap:
    `task kind-dev-up` ([DEVELOPMENT.md](DEVELOPMENT.md)).

## kubectl — custom resources

Kollect CRDs register short names for faster typing ([CR-REFERENCE.md](CR-REFERENCE.md)).

| Kind | Short name | Example |
| --- | --- | --- |
| `KollectProfile` | `kprof` | `kubectl get kprof -n team-a` |
| `KollectSnapshotSink` | `ksnap` | `kubectl get ksnap -A` |
| `KollectDatabaseSink` | `kdb` | `kubectl get kdb -A` |
| `KollectEventSink` | `kevt` | `kubectl get kevt -A` |
| `KollectTarget` | `ktgt` | `kubectl get ktgt -A` |
| `KollectInventory` | `kinv` | `kubectl get kinv -A` |
| `KollectScope` | `kscope` | `kubectl get kscope -A` |
| `KollectConnectionTest` | `kconntest` | `kubectl get kconntest -A` |
| `KollectClusterProfile` | `kcprof` | `kubectl get kcprof` |
| `KollectClusterTarget` | `kctgt` | `kubectl get kctgt` |
| `KollectClusterInventory` | `kcinv` | `kubectl get kcinv` |

### Pipeline status

```sh
kubectl get kprof,ksnap,kdb,kevt,ktgt,kinv,kscope -n <namespace>
kubectl apply -k config/samples/
kubectl explain kollectinventory.spec
```

### Conditions and describe

```sh
kubectl describe kollectsnapshotsink <name> -n <namespace>
kubectl describe kollectdatabasesink <name> -n <namespace>
kubectl describe kollecteventsink <name> -n <namespace>
kubectl describe kollectinventory <name> -n <namespace>
kubectl get kinv <name> -n <namespace> -o jsonpath='{.status.conditions}'
```

### Wait for probe

```sh
kubectl wait --for=condition=ConnectionVerified kollectdatabasesink/<name> \
  -n <namespace> --timeout=60s
kubectl wait --for=condition=ConnectionVerified kollectconnectiontest/<name> \
  -n <namespace> --timeout=60s
```

### Connection test annotation

```sh
kubectl annotate kollectsnapshotsink <name> -n <namespace> \
  kollect.dev/test-connection=true --overwrite
```

See [Annotations and labels](ANNOTATIONS-LABELS.md).

### Operator and webhooks

```sh
kubectl -n kollect-system rollout status deployment/kollect-controller-manager
kubectl -n kollect-system logs deployment/kollect-controller-manager -f --tail=200
kubectl get pods -n kollect-system -l app.kubernetes.io/name=kollect
```

## Task targets

Run from repository root with [Task](https://taskfile.dev/). Full list: `task --list-all`.

### Daily development

| Task | Purpose |
| --- | --- |
| `task dev-up` | Bootstrap: build, kind cluster, Helm install, samples |
| `task kind-dev-up` | Create `kollect-dev` cluster and deploy operator |
| `task kind-dev-down` | Delete `kollect-dev` cluster |
| `task kind-dev-status` | Show cluster and operator health |
| `task build` | Compile manager binary |
| `task test` | Unit tests + envtest (race) |
| `task lint` | golangci-lint |
| `task verify` | Codegen drift gate |
| `task lint:markdown` | Markdownlint on `docs/` |

!!! note "Minimal kind install"
    Skip ingress/Grafana addons: `KOLLECT_DEV_MINIMAL=1 task dev-up`.

### Install and deploy

| Task | Purpose |
| --- | --- |
| `task install:crds` | Apply CRD bundle to current context |
| `task docker:build` | Build `kollect-controller-manager:dev` image |
| `task docker:build:local` | Build `ghcr.io/konih/kollect:local` for kind/minikube load |
| `task docker:push:local` | Build and push maintainer-only tag to GHCR (default `test-<short-sha>`) |
| `task deploy:operator` | Helm install to `kollect-system` |
| `task kind-dev-load` | Load dev image into kind |

### Quality and release

| Task | Purpose |
| --- | --- |
| `task scrub` | Scan staged diff for private strings |
| `task test-integration` | Sink integration tests (Docker) |
| `task helm-test` | `helm lint` + helm-docs drift + chart unit tests |
| `task helm-docs` | Regenerate `charts/kollect/README.md` |
| `task helm-docs:verify` | Fail if chart README drift |
| `task release-dry-run` | Build `dist/install.yaml` and chart tarball |
| `task changelog:verify` | Fail if `CHANGELOG.md` drift |

### Documentation

```sh
pip install -r docs/requirements-docs.txt
mkdocs serve          # local preview
mkdocs build --strict # CI-equivalent build
```

## Helm

### Install and upgrade

<!-- markdownlint-disable MD046 -->

=== "From repository"

    ```sh
    helm install kollect ./charts/kollect -n kollect-system --create-namespace
    helm upgrade kollect ./charts/kollect -n kollect-system -f values.yaml
    ```

=== "From GHCR (OCI)"

    ```sh
    helm install kollect oci://ghcr.io/konih/kollect --version 0.1.0 \
      -n kollect-system --create-namespace
    ```

!!! warning "CRD upgrades"
    Helm does **not** upgrade CRDs on `helm upgrade`. Apply CRDs separately:

```sh
kubectl apply -f dist/install-crds.yaml
helm upgrade kollect ./charts/kollect -n kollect-system -f values.yaml
```

See [Operator manual — Upgrade](operator-manual/upgrading.md).

### Per-team install

```sh
helm install kollect ./charts/kollect -n kollect-system --create-namespace \
  --set tenantMode=true \
  --set-json 'watchNamespaces=["team-a"]' \
  --set mode=single
```

Or use a values file — see [Operator manual — Per-team install](OPERATOR-MANUAL.md#per-team-install-recommended-default).

### Key values

| Key | Default | Notes |
| --- | --- | --- |
| `mode` | `single` | Only supported runtime mode ([ADR-0501](adr/0501-multi-cluster-fleet.md)) |
| `tenantMode` | `false` | Namespaced RBAC for team installs |
| `watchNamespaces` | `[]` | Restrict informer cache |
| `featureGates.inventoryHttp.enabled` | `false` | Debug HTTP API |
| `image.tag` | chart default | Pin in production |

Full list: [`charts/kollect/values.yaml`](../charts/kollect/values.yaml). Validation:
`task helm-test`.

### Uninstall

!!! warning "CRD retention"
    `helm uninstall` removes the operator but **not** CRDs or tenant CRs. CRD deletion garbage-collects
    all custom resources — avoid in production.

```sh
helm uninstall kollect -n kollect-system
```

## Related

- [CR reference](CR-REFERENCE.md) · [Operator manual](OPERATOR-MANUAL.md)
- [Troubleshooting](TROUBLESHOOTING.md) · [Annotations and labels](ANNOTATIONS-LABELS.md)
- [ADR-0704: Helm chart lifecycle](adr/0704-helm-chart-crd-lifecycle.md)
