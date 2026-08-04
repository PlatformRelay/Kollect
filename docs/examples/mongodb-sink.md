# Export inventory to MongoDB

## Prerequisites

A running operator, MongoDB, and the `inventory-mongodb-uri` Secret described in the sample.

## Apply

```sh
kubectl apply -f config/samples/advanced/kollect_v1alpha1_kollectdatabasesink_mongodb.yaml
```

## Verify

```sh
kubectl wait --for=condition=ConnectionVerified kdb/mongodb-inventory-demo -n default --timeout=90s
kubectl describe kdb mongodb-inventory-demo -n default
```

## If it didn't work

Check the URI Secret, database permissions, TLS trust, and `provisioning.mode`.

## Cleanup

```sh
kubectl delete -f config/samples/advanced/kollect_v1alpha1_kollectdatabasesink_mongodb.yaml
```

## Further reading

[MongoDB ADR](../adr/0417-mongodb-database-sink.md) ·
[Database sink reference](../crds/kollectdatabasesink.md)
