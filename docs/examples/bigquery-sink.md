# Export inventory to BigQuery

## Prerequisites

A running operator, a Google Cloud project with a dataset, and Workload Identity or the credentials
Secret referenced by the validated sample.

## Apply

Review the project, dataset, table, region, and cadence, then apply:

```sh
kubectl apply -f config/samples/kollect_v1alpha1_kollectdatabasesink_bigquery.yaml
```

## Verify

```sh
kubectl wait --for=condition=ConnectionVerified kdb/bigquery-inventory-demo -n default --timeout=90s
kubectl describe kdb bigquery-inventory-demo -n default
```

## If it didn't work

Check ADC or Secret resolution, dataset location, IAM permissions, and API enablement.

## Cleanup

```sh
kubectl delete -f config/samples/kollect_v1alpha1_kollectdatabasesink_bigquery.yaml
```

## Further reading

[BigQuery ADR](../adr/0420-bigquery-database-sink.md) ·
[Database sink reference](../crds/kollectdatabasesink.md)
