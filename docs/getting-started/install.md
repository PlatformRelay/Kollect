# Install Kollect

Install the operator on a local kind cluster, then continue to a Git-backed inventory you can
inspect and diff.

!!! note "Before you start"
    You need Docker, [kind](https://kind.sigs.k8s.io/), `kubectl`, Git, Go, and
    [Task](https://taskfile.dev/). The API is `v1alpha1`; pin a release for shared environments.

## Local evaluation

From a fresh checkout:

```sh
git clone https://github.com/platformrelay/kollect.git
cd kollect
task build
task kind-dev-up
kubectl -n kollect-system rollout status \
  deployment/kollect-controller-manager --timeout=120s
```

`task kind-dev-up` creates the `kollect-dev` cluster and installs only the operator and its CRDs.
It deliberately does not apply `config/samples/`, so the next guide starts with an empty Kollect
API and a collision-free Git-only pipeline.

Verify that the operator is ready:

```sh
kubectl get pods -n kollect-system
kubectl api-resources --api-group=kollect.dev
```

## Helm on an existing cluster

The default chart enables validating webhooks and creates cert-manager `Issuer` and `Certificate`
resources. Install [cert-manager](https://cert-manager.io/docs/installation/) first and verify its
CRDs are available:

```sh
kubectl get crd certificates.cert-manager.io issuers.cert-manager.io
```

Install the published OCI chart, or use the chart in your checkout:

```sh
helm install kollect oci://ghcr.io/platformrelay/kollect \
  --namespace kollect-system --create-namespace
kubectl -n kollect-system rollout status \
  deployment/kollect-controller-manager --timeout=120s
```

For production values, restricted watch scope, secrets, and webhook TLS, use the
[operator manual](../operator-manual/index.md). See the [release page](../RELEASE.md) before
upgrading a pinned installation.

If cert-manager cannot be installed, either provide the webhook serving Secret and CA injection
yourself before installing with `webhooks.certManager.create=false`, or explicitly disable webhooks
for a constrained development environment. The chart does not generate certificates when
`webhooks.certManager.create=false`; disabling admission is not the recommended production path.

## Next step

Continue to [Your first inventory](first-inventory.md) to collect Deployments and verify the
export in Git.

## Clean up

For the local path:

```sh
task kind-dev-down
```

If installation fails, check the manager logs and the
[troubleshooting guide](../operator-manual/troubleshooting.md).
