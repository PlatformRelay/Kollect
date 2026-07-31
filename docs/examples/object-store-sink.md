# Export snapshots to object storage

## Prerequisites

A running operator, an S3-compatible bucket or GCS bucket, and workload identity or a credentials
Secret. The repository ships an executable S3 sample; GCS uses the same family CRD and is documented
in the generated [snapshot sink schema](../crds/kollectsnapshotsink.md).

## Apply

```sh
kubectl apply -f config/samples/kollect_v1alpha1_kollectsnapshotsink_s3.yaml
```

Set the sample endpoint, bucket, and Secret reference before using a non-local service. For GCS,
select `type: gcs` and use only fields admitted by the generated CRD schema.

## Verify

```sh
kubectl get ksnap -n default
kubectl describe ksnap s3-inventory-demo -n default
```

## If it didn't work

Check bucket permissions, region, endpoint DNS/TLS, and the resolved credential Secret.

## Cleanup

```sh
kubectl delete -f config/samples/kollect_v1alpha1_kollectsnapshotsink_s3.yaml
```

## Further reading

[Object-store layout](../adr/0407-git-object-store-layout.md) ·
[Security architecture](../security/security-architecture.md)
