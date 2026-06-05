# ADR-0502: Event-emitter transport for hub fan-in

> One event-emitter abstraction — NATS JetStream is the lean default, Kafka/Redpanda the enterprise
> opt-in, `inprocess` the dev/test default. It is the same abstraction as the event sink.

**Theme:** 05 · Multi-cluster · **Status:** Current · **Evolution:** [ADR-0401](0401-sink-taxonomy-state-vs-stream.md)
elevated NATS over the earlier Redis-spike framing and unified the transport with the Kafka/NATS event
sink — a spoke publishing to a shared subject *is* the fan-in. Hub deployment model superseded by
[ADR-0703](0703-platform-architecture-pivot.md) — **`mode: hub` Helm values**, not `KollectHub` CRD.

## Context

Multi-cluster hub aggregation ([ADR-0501](0501-multi-cluster-sync-rfc.md)) needs a transport between
**spoke** operators (per cluster) and the **hub** (same image with Helm **`mode: hub`** — see
[ADR-0703](0703-platform-architecture-pivot.md)). Requirements:

- **Low operational burden** for a Phase 1–2 hub prototype (personal/small-platform scale first).
- **Pluggable** — no hard dependency on Kafka or a specific vendor; teams that standardize on Kafka
  must be able to select it via configuration without forking kollect.
- **At-least-once** delivery acceptable; hub merge is idempotent on `(cluster, namespace, name, uid)`.
- Payloads are **summarized inventory JSON** (not full object dumps) per [ADR-0103](0103-etcd-limit.md).
- Every shipped backend must be **provable in integration or e2e tests** with reasonable effort
  (testcontainers or kind-sidecar).

## Options evaluated

| Transport | Ops footprint | Ordering / retention | testcontainers-go | Fit |
| --- | --- | --- | --- | --- |
| **In-process channel** | None (single process) | In-memory only; lost on restart | N/A (no container) | **Dev/test default** — unit, envtest, local kind |
| **Redis Streams** | Often already present; single Deployment | `XREADGROUP`, trimming, persistence | `modules/redis` — mature, fast CI spin-up | **Phase 2 spike candidate only** — not production default |
| **NATS JetStream** | Small binary; single server or K8s Deployment | Streams, consumer groups, replay | `modules/nats` — available | **Second lean driver** — same interface, config-selected |
| **Kafka** | Cluster + KRaft, topic ops | Durable log, enterprise tooling | Heavier (KRaft or Redpanda module) | **Optional enterprise backend** — only when team needs it |

### Redis Streams (Phase 2 spike pick)

- Pros: `testcontainers-go/modules/redis` is lightweight and widely used in CI; many platforms already
  run Redis; `XREADGROUP` gives consumer groups and at-least-once semantics.
- Cons: Redis not universal; memory pressure if retention unbounded — configure `MAXLEN` / trimming.

### NATS JetStream

- Pros: purpose-built messaging; small footprint; good Go client; stream retention policies.
- Cons: another service if not already in the estate; TLS/auth configuration.

### In-process channel

- Pros: zero deps; fastest TDD loop for hub merge logic inside one binary.
- Cons: no cross-pod or cross-cluster delivery — only for **prototype wiring** before external bus.

### Kafka

- Pros: enterprise standard, long retention, ecosystem — matches teams that already operate Kafka.
- Cons: heavy for Phase 1; topic/partition design; **must remain optional** and config-pluggable.

## Decision

1. **Transport abstraction:** all hub and in-operator messaging goes through a **`Transport` interface**
   (`Publisher` / `Subscriber` in `internal/transport/`). Backends implement the same contract; no
   controller imports a vendor SDK directly.

2. **Configurable backend selection** (Helm `transport.type` and/or manager flags — **not** a
   `KollectHub` CRD; see [ADR-0703](0703-platform-architecture-pivot.md)):

   | `transport.type` (Helm) | When |
   | --- | --- |
   | `inprocess` | **Default everywhere** until an external backend passes integration/e2e proof |
   | `redis` | Phase 2 **spike** backend — testcontainers validation; explicit opt-in only |
   | `nats` | Alternative lean backend — same interface, explicit opt-in |
   | `kafka` | Optional enterprise backend — never required for install or CI |

   **No transport type is a silent production default except `inprocess`.** Helm chart and hub/spoke
   samples must not pre-select Redis/NATS/Kafka.

   Connection details (URL, credentials, stream/topic names) live under Helm `transport` values or
   equivalent env vars / secret refs — exact shape is an implementation detail.

3. **Phase order:**
   - **Phase 1 / dev:** `inprocess` — validate merge + export without external infra.
   - **Phase 2 spike:** **Redis Streams** — prove hub fan-in via testcontainers; **not** promoted
     to production default until load test at target spoke count; operators choose backend explicitly.
   - **Phase 2+:** NATS JetStream driver behind the same factory (config, not compile-time).
   - **Enterprise optional:** Kafka driver — ship only when integration-tested; never a hard dependency.

4. **Backend admission rule:** **do not merge a transport backend** unless it can be exercised in an
   integration or e2e test with reasonable effort (testcontainers module or documented kind sidecar).
   In-process is exempt (no container). Kafka is deferred until that bar is met.

5. Spoke → hub message schema (sketch, not API):

```json
{
  "apiVersion": "kollect.dev/v1alpha1",
  "schemaVersion": "kollect.dev/v1alpha1",
  "cluster": "prod-eu-1",
  "inventoryRef": { "namespace": "team-a", "name": "team-inventory" },
  "generation": 42,
  "summary": { "itemCount": 120, "checksum": "sha256:..." },
  "payloadRef": "optional object-store key for large bodies"
}
```

## Transport factory (reference)

```mermaid
flowchart TB
  subgraph config [Configuration]
    Helm["Helm transport.type + env"]
    Flags["Manager flags"]
  end

  subgraph factory [Transport factory]
    F["NewTransport(type, config)"]
  end

  subgraph backends [Pluggable backends]
    IP[inprocess]
    RD[redis]
    NT[nats]
    KF[kafka]
  end

  subgraph consumers [Hub wiring]
    Spoke[spoke operator mode: spoke]
    Hub[hub operator mode: hub]
    Merge[merge + dedupe]
    Export[Postgres / Kafka / S3]
  end

  Helm --> F
  Flags --> F
  F -->|inprocess| IP
  F -->|redis| RD
  F -->|nats| NT
  F -->|kafka| KF

  Spoke -->|publish report| F
  Hub -->|consume| F
  F --> Merge
  Merge --> Export
```

## Hub wiring (reference)

```mermaid
flowchart LR
  Spoke[spoke mode: spoke] -->|publish report| Q[Transport]
  HelmHub[Helm mode: hub] --> HubProc[hub ingest + merge]
  HubProc -->|consume| Q
  Q --> Merge[merge + dedupe]
  Merge --> Export[Postgres / Kafka / S3]
```

Hub Deployments are rendered by the Helm chart when **`mode: hub`** — no `KollectHub` CRD spawns
hub workloads ([ADR-0703](0703-platform-architecture-pivot.md)). Shard routing uses
`hash(clusterName) % shardCount` from Helm values / env.

## Consequences

### Positive

- Clear spike order: in-process → Redis → NATS (config) → optional Kafka.
- Hub collector testable in envtest with in-process transport; Redis provable via testcontainers.
- Enterprise Kafka teams select backend via Helm — no fork required.
- Factory pattern keeps vendor SDKs out of reconcilers.

### Negative

- Multiple lean backends (Redis + NATS) may both need maintenance if both ship — none are implicit
  defaults; `inprocess` remains fallback until ops selects an external type.
- Message schema versioning requires discipline — `apiVersion` and `schemaVersion` fields are mandatory
  in wire payloads ([ADR-0405](0405-export-data-contract.md)).

## Open questions

- **DECIDED (2026-06-05):** **At-least-once + idempotent** delivery (effectively-once for state, since
  exports are identity-keyed idempotent upserts). Consumers dedupe by `(identity, contentHash,
  generation)`; JetStream `Nats-Msg-Id` (= content hash) gives free in-window dedupe. Exactly-once is a
  **non-goal** unless a regulatory event-count requirement appears.
- **OPEN:** Hub pulls from queue vs queue pushes to hub webhook sidecar?

## See also

- [ADR-0501: Multi-cluster sync topology](0501-multi-cluster-sync-rfc.md)
- [ADR-0503: Hub cluster authentication](0503-hub-cluster-auth-istio-pattern.md)
- [ADR-0703: Platform architecture pivot](0703-platform-architecture-pivot.md)
