# Fan out one inventory to multiple sinks

## Prerequisites

A running operator and configured Git and Postgres destinations. The repository sample is the
schema-validated source for the family references and their independent cadences.

## Apply

Create the referenced sinks first, then apply the validated inventory:

```sh
# Dual-cadence inventory lives on the advanced (opt-in) path — create Secrets first.
kubectl apply -k config/samples/advanced/
# or apply the inventory alone after sinks exist:
kubectl apply -f config/samples/advanced/kollect_v1alpha1_kollectinventory.yaml
```

Its `snapshotSinkRefs` and `databaseSinkRefs` export the same canonical rows independently.

## Verify

```sh
kubectl get kinv team-inventory -n default -o jsonpath='{.status.sinkExports}' ; echo
```

## If it didn't work

Describe the inventory. A missing family sink or one failed backend appears per destination; it
does not roll back a successful sibling export.

## Cleanup

```sh
kubectl delete -f config/samples/advanced/kollect_v1alpha1_kollectinventory.yaml
```

## Further reading

[Export pipeline](../concepts/export-pipeline.md) ·
[Per-sink scheduling ADR](../adr/0413-export-interval-scheduling.md)
