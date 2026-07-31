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
KOLLECT_DEV_MINIMAL=1 task dev-up
kubectl -n kollect-system rollout status \
  deployment/kollect-controller-manager --timeout=120s
```

`task dev-up` builds the manager, creates the `kollect-dev` kind cluster, installs the CRDs and
operator, and applies the repository samples. The default samples include optional backends; the
next guide applies a separate Git-only pipeline.

Verify that the operator is ready:

```sh
kubectl get pods -n kollect-system
kubectl api-resources --api-group=kollect.dev
```

## Helm on an existing cluster

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

## Next step

Continue to [Your first inventory](first-inventory.md) to collect Deployments and verify the
export in Git.

## Clean up

For the local path:

```sh
task dev-down
```

If installation fails, check the manager logs and the
[troubleshooting guide](../operator-manual/troubleshooting.md).
