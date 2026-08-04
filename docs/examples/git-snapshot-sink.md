# Export snapshots to Git

## Prerequisites

A running operator, a writable Git repository, and a credentials Secret. Start with
[Your first inventory](../getting-started/first-inventory.md) for the full credential flow.

## Apply

Use the validated minimal sample, then set its endpoint and Secret reference for your repository:

```sh
kubectl apply -f config/samples/advanced/kollect_v1alpha1_kollectsnapshotsink_git_minimal.yaml
```

## Verify

```sh
kubectl wait --for=condition=ConnectionVerified ksnap/git-inventory-minimal -n default --timeout=90s
kubectl get ksnap/git-inventory-minimal -n default -o yaml
```

## If it didn't work

Describe the sink and check repository permissions, TLS trust, branch, and Secret keys.

## Cleanup

```sh
kubectl delete -f config/samples/advanced/kollect_v1alpha1_kollectsnapshotsink_git_minimal.yaml
```

## Further reading

[Snapshot sink reference](../crds/kollectsnapshotsink.md) ·
[Git layout ADR](../adr/0419-git-export-serialization-layout.md)
