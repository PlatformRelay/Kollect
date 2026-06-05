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
`KOLLECT_TRANSPORT_TLS_CLIENT_KEY_FILE`, `KOLLECT_TRANSPORT_TLS_INSECURE_SKIP_VERIFY` (ADR-0503).

Optional wire ACL hints (hub consumer validates `cluster_id` when set):

- `KOLLECT_TRANSPORT_ACL_ALLOWED_CLUSTERS` — comma-separated spoke cluster IDs; hub consumer
  rejects wire messages whose `cluster_id` is not listed (`transport.ACLSettings.ValidateClusterID`)
- `KOLLECT_TRANSPORT_ACL_PUBLISH_SUBJECTS` — documented publish subjects for Helm values (broker ACL
  provisioning is external to the operator)
- `KOLLECT_TRANSPORT_ACL_SUBSCRIBE_SUBJECTS` — documented subscribe subjects for Helm values

Production TLS/ACL wiring is **stub-level in the operator**: client TLS for Redis/NATS is supported;
broker-side stream/subject ACLs and NATS account limits must be configured out-of-band (Helm docs +
`ACLSettingsFromEnv`). Spoke identity on the wire still flows through report payload + hub
`KOLLECT_REMOTE_CLUSTERS` allowlist (ADR-0503).

### Broker setup (operator + external)

| Layer | Redis Streams | NATS JetStream |
| --- | --- | --- |
| **Client TLS** | `KOLLECT_TRANSPORT_TLS_*` on spoke + hub pods | same |
| **Wire cluster id** | `cluster_id` stream field (`transport.WireClusterID`) | `X-Kollect-Cluster-Id` header |
| **Operator publish guard** | `KOLLECT_TRANSPORT_ACL_PUBLISH_SUBJECTS` | same |
| **Operator subscribe guard** | `KOLLECT_TRANSPORT_ACL_SUBSCRIBE_SUBJECTS` | same |
| **Hub consumer cluster guard** | `KOLLECT_TRANSPORT_ACL_ALLOWED_CLUSTERS` + `KOLLECT_REMOTE_CLUSTERS` | same |
| **Broker ACL (external)** | Redis ACL `+@write` on stream key only; deny `KEYS *` | NATS account publish/subscribe on `inventory.>` |

Helm values: set `hub.transport.acl.*` env mirrors when chart exposes them; otherwise patch the hub
Deployment env from the table above. Redis: create a dedicated user with `XADD` on `kollect.hub` and
`XREADGROUP` for the hub consumer group. NATS: create a JetStream-enabled account limited to
`inventory/reports` publish (spoke) and durable consume (hub).

Use cases: spoke → hub inventory reports (ADR-0501), debounced export triggers, and optional
decoupling of collection from export workers.
