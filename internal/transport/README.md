# transport

Lean publish/subscribe boundary for inventory change notifications inside the operator.

## Backends

| Type | Implementation | Integration test |
| --- | --- | --- |
| `inprocess` | `InProcessBus` (default) | unit |
| `redis` | Redis Streams | testcontainers `modules/redis` |
| `nats` | NATS JetStream | testcontainers `modules/nats` |
| `kafka` | Kafka/Redpanda | testcontainers `modules/redpanda` |

Configure via `transport.Config` or `ConfigFromEnv()` (`KOLLECT_TRANSPORT_TYPE`, backend URLs).
Optional TLS for Redis/NATS: `KOLLECT_TRANSPORT_TLS_CA_FILE`, `KOLLECT_TRANSPORT_TLS_CLIENT_CERT_FILE`,
`KOLLECT_TRANSPORT_TLS_CLIENT_KEY_FILE`, `KOLLECT_TRANSPORT_TLS_INSECURE_SKIP_VERIFY` (ADR-0028).

Optional wire ACL hints (hub consumer validates `cluster_id` when set):

- `KOLLECT_TRANSPORT_ACL_ALLOWED_CLUSTERS` — comma-separated spoke cluster IDs; hub consumer
  rejects wire messages whose `cluster_id` is not listed (`transport.ACLSettings.ValidateClusterID`)
- `KOLLECT_TRANSPORT_ACL_PUBLISH_SUBJECTS` — documented publish subjects for Helm values (broker ACL
  provisioning is external to the operator)
- `KOLLECT_TRANSPORT_ACL_SUBSCRIBE_SUBJECTS` — documented subscribe subjects for Helm values

Production TLS/ACL wiring is **stub-level in the operator**: client TLS for Redis/NATS is supported;
broker-side stream/subject ACLs and NATS account limits must be configured out-of-band (Helm docs +
`ACLSettingsFromEnv`). Spoke identity on the wire still flows through report payload + hub
`KOLLECT_REMOTE_CLUSTERS` allowlist (ADR-0028).

Use cases: spoke → hub inventory reports (ADR-0022), debounced export triggers, and optional
decoupling of collection from export workers.
