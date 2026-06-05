# Example: Connection test

Sink probe: `spec.connectionTest: true` (samples) or `kollect.dev/test-connection: "true"` annotation.

CR: `config/samples/kollect_v1alpha1_kollectconnectiontest.yaml` — `sinkRef: postgres-inventory-demo`.

Conditions: `ConnectionVerified` on sink; `SinkReachable` on inventory ([ADR-0403](../adr/0403-connection-test.md)).
