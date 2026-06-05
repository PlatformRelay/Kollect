# ADR-0503: Hub cluster authentication — push-first (Istio remote-secret pattern)

> When the hub tier is used, spoke→hub ingest is push-first: TokenReview + SAR `create` on
> `kollectremoteclusters`; an Istio-style remote credential `Secret` covers hub-pull.

**Theme:** 05 · Multi-cluster · **Status:** Current

## Context

Multi-cluster inventory fan-in ([ADR-0501](0501-multi-cluster-sync-rfc.md)) requires **authenticated
spoke → hub** channels at hub scale. Inventory HTTP read auth is already settled
([ADR-0404](0404-inventory-api-auth.md)); hub/spoke transport auth is a **separate concern** —
spokes push summarized deltas, hubs validate identity before merge.

[Istio multicluster](https://istio.io/latest/docs/setup/install/multicluster/) solves a related
problem: a **primary** control plane must reach **remote** Kubernetes API servers. The established
pattern is:

| Istio mechanism | Purpose |
| --- | --- |
| **Remote secret** | `Secret` in primary cluster with base64 kubeconfig fragment (API URL + SA token + CA) |
| **Label** `istio/multiCluster: "true"` | Tells Istiod to watch and register the secret |
| **Annotation** `networking.istio.io/cluster: <name>` | Stable cluster identity for routing and discovery |
| **`istioctl create-remote-secret`** | GitOps-friendly generator from remote-cluster SA credentials |
| **Trust** | Shared root or federated CA; **mTLS** for east-west workload traffic |
| **Topologies** | Primary-remote (one control plane) and multi-primary (peered control planes) |

kollect maps to **hub-and-spoke** ([ADR-0501](0501-multi-cluster-sync-rfc.md)): hub aggregates;
spokes stay lightweight. We do **not** need Istio's full mesh trust plane for inventory deltas, but
the **credential registration model** transfers cleanly.

Options considered:

| Approach | Pros | Cons |
| --- | --- | --- |
| **Istio-style remote credential `Secret` + `KollectRemoteCluster` CR** | GitOps-friendly; optional hub pull; familiar to platform teams | Secret lifecycle; hub must list/watch secrets |
| **Push-only Bearer SA token + TokenReview** | Scales to many spokes; no hub API reach into spokes; reuses [ADR-0404](0404-inventory-api-auth.md) | Spokes need routable hub ingress; token rotation |
| **mTLS client certs per spoke** | Strong transport identity | Cert ops at hub scale; CSR/bootstrap complexity |
| **OIDC / static API keys** | Simple for non-K8s spokes | Parallel identity stack; rotation burden |

## Decision

Adopt a **hybrid** model aligned with Istio's remote-secret **registration** pattern and kollect's
**push-first** scale target.

### 1. `KollectRemoteCluster` CR (namespaced on hub)

Declarative registration of a spoke cluster in the hub namespace (platform team scope):

| Field | Role | Istio parallel |
| --- | --- | --- |
| `spec.clusterName` | DNS-1123 spoke identity | `networking.istio.io/cluster` annotation |
| `spec.credentialsSecretRef` | Optional kubeconfig fragment for **hub pull** | `istio-remote-secret-*` data key |
| `spec.apiServerURL` | Optional spoke API URL (pull / health only) | kubeconfig `server` field |
| `spec.trustBundle` | PEM CA for spoke API or future mTLS | kubeconfig `certificate-authority-data` |

Status stub: `Connected` condition (minimal; full reconciler deferred).

### 2. Istio-like credential `Secret` (optional pull path)

For GitOps and hub-pull scenarios, spokes (or a bootstrap Job) apply a labeled secret to the hub:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kollect-remote-secret-spoke-a
  namespace: platform
  labels:
    kollect.dev/multiCluster: "true"
  annotations:
    kollect.dev/cluster: spoke-a
type: Opaque
data:
  spoke-a: <base64-kubeconfig-fragment>
```

`KollectRemoteCluster.spec.credentialsSecretRef` points at this secret — same ergonomics as
`istioctl create-remote-secret | kubectl apply` ([Istio primary-remote install](https://istio.io/latest/docs/setup/install/multicluster/primary-remote/)).

### 3. Push path (default at hub scale)

Spokes POST summarized `SpokeReport` JSON to hub ingress:

- **`Authorization: Bearer <in-cluster SA token>`** — validated on hub via **`TokenReview`**
  ([ADR-0404](0404-inventory-api-auth.md) pattern), then **`SubjectAccessReview`** for ingest
  permission.
- **`X-Kollect-Cluster-Id: <spec.clusterName>`** — must match `SpokeReport.cluster` body field.
- Hub flag **`--hub-ingest-auth-mode=kubernetes`** (default); `disabled` for dev/CI only.

Lean queue transports (Redis/NATS/Kafka) carry **`cluster_id` wire metadata** in Phase 2; HTTP push
remains the reference **application-auth** channel (TokenReview + SAR). Queue **TLS + hub ACL**
hardening ships as follows (see § Queue wire hardening).

### 4. `KollectHub` references remote clusters

`KollectHub` continues to own transport + Deployment; platform teams pair it with
`KollectRemoteCluster` objects per registered spoke. Full `spec.remoteClusters[]` on `KollectHub`
is **deferred** — discovery via `KollectRemoteCluster` list in the hub namespace is sufficient
for Phase 2.

## Push auth flow (default)

```mermaid
sequenceDiagram
  participant Spoke as Spoke operator
  participant SA as SA token file
  participant Hub as Hub ingest HTTP
  participant TR as TokenReview API
  participant SAR as SubjectAccessReview API
  participant Merge as Hub merger

  Spoke->>SA: read projected token
  Spoke->>Hub: POST /hub/v1alpha1/reports<br/>Authorization: Bearer token<br/>X-Kollect-Cluster-Id: spoke-a
  Hub->>TR: TokenReview(token)
  TR-->>Hub: authenticated SA subject
  Hub->>SAR: authorize ingest (see below)
  SAR-->>Hub: allowed / denied
  alt token valid, SAR allowed, cluster id matches body
    Hub->>Merge: Apply SpokeReport
    Merge-->>Hub: ok
    Hub-->>Spoke: 202 Accepted
  else invalid token
    Hub-->>Spoke: 401 Unauthorized
  else SAR denied
    Hub-->>Spoke: 403 Forbidden
  end
```

**Ingest SAR shape (resolved 2026-06-05 session 4):** hub middleware requires **`create`** on
**`kollectremoteclusters.kollect.dev`** in the hub namespace for the authenticated spoke service
account (after successful `TokenReview` and `X-Kollect-Cluster-Id` match).

Platform teams grant spoke SAs a `Role` or `ClusterRole` with **`create`** on
`kollectremoteclusters` in the hub platform namespace. Pull registration via
`credentialsSecretRef` remains optional and separate.

## Optional pull registration (Istio-style)

```mermaid
sequenceDiagram
  participant GitOps as GitOps / bootstrap Job
  participant HubNS as Hub namespace
  participant CR as KollectRemoteCluster
  participant Hub as Hub operator (future)

  GitOps->>HubNS: apply Secret<br/>label kollect.dev/multiCluster=true<br/>annotation kollect.dev/cluster=spoke-a
  GitOps->>HubNS: apply KollectRemoteCluster<br/>credentialsSecretRef → secret
  Note over Hub: Future reconciler reads kubeconfig<br/>for pull/health; push remains default
  CR-->>Hub: spec.clusterName + trustBundle
```

## Comparison: Istio vs kollect

| Dimension | Istio multicluster | kollect hub-and-spoke |
| --- | --- | --- |
| **Registration** | Labeled `Secret` + cluster annotation | `KollectRemoteCluster` CR + optional same-label `Secret` |
| **Generator** | `istioctl create-remote-secret` | `kollect create-remote-secret` CLI stub / `hack/create-remote-secret.sh` |
| **Default data plane** | mTLS east-west between workloads | Summarized inventory deltas (no workload mesh) |
| **Default control traffic** | Istiod → remote API (pull watches) | Spoke → hub HTTP push (TokenReview) |
| **Identity** | SA token in kubeconfig + mesh CA | SA bearer token + `X-Kollect-Cluster-Id` |
| **Trust** | Shared/federated mesh CA | Hub apiserver TokenReview; optional `trustBundle` for pull/mTLS later |
| **Topology** | Primary-remote / multi-primary | Hub-and-spoke only ([ADR-0501](0501-multi-cluster-sync-rfc.md)) |
| **Scale bias** | Tens of clusters per mesh | many spokes, push-first |

## Consequences

### Positive

- Platform teams already running Istio multicluster recognize the credential secret + cluster name model.
- Push + TokenReview avoids hub→spoke API connectivity requirements at scale.
- Pull path remains available for health checks and future hub-initiated collection without redesign.
- Reuses Kubernetes-native auth from [ADR-0404](0404-inventory-api-auth.md).

### Negative

- Two paths (push auth vs pull secrets) must stay documented to avoid confusion.
- Queue transports need separate TLS/ACL hardening before production multi-tenant hubs.
- `KollectRemoteCluster` reconciler (Connected status, secret rotation) not implemented in this ADR — stub only.

### 5. `kollect create-remote-secret` (stub)

GitOps-friendly generator parallel to `istioctl create-remote-secret`:

```sh
go run ./cmd/kollect create-remote-secret --cluster spoke-a --namespace platform | kubectl apply -f -
# or:
hack/create-remote-secret.sh --cluster spoke-a --api-server https://spoke-a.example:6443
```

Emits a labeled `Secret` with a base64 kubeconfig fragment (`server`, `token`, `certificate-authority-data`
placeholders when flags are omitted). Pair with `KollectRemoteCluster.spec.credentialsSecretRef` for
optional hub pull; push path remains default.

### 6. Queue wire hardening (TLS + hub ACL)

Queue transports are **not** a substitute for HTTP TokenReview — they rely on **network +
registration gates** until a future signed-envelope spike.

| Layer | Mechanism | Config |
| --- | --- | --- |
| **Transport TLS** | `rediss://` / NATS with `nats.Secure` | Shared env: `KOLLECT_TRANSPORT_TLS_CA_FILE`, `KOLLECT_TRANSPORT_TLS_CLIENT_CERT_FILE`, `KOLLECT_TRANSPORT_TLS_CLIENT_KEY_FILE`, `KOLLECT_TRANSPORT_TLS_INSECURE_SKIP_VERIFY` (dev only) |
| **Wire identity** | `cluster_id` field (Redis stream) or `X-Kollect-Cluster-Id` header (NATS) | Spoke sets via `KOLLECT_SPOKE_CLUSTER` publish context |
| **Hub ACL** | Reject reports whose `report.cluster` ∉ `KOLLECT_REMOTE_CLUSTERS` | Populated from `KollectHub.spec.remoteClusters[]` → resolved `KollectRemoteCluster.spec.clusterName` values; **empty allowlist = dev open mode** |

HTTP ingest continues to use TokenReview + SAR; queue consumer uses ACL only. Platform teams run
queue brokers with vendor ACLs (Redis ACL / NATS accounts) in addition to kollect's registration gate.

**Deferred:** signed `SpokeReport` envelopes, Kafka SASL/TLS (same env pattern when wired).

## Open questions

- **RESOLVED (2026-06-05):** Hub/spoke identity model **locked** — this ADR is default; push-first
  TokenReview + `X-Kollect-Cluster-Id`; optional remote `Secret` for pull; mTLS/OIDC/bootstrap not primary.
- **RESOLVED (2026-06-05):** Queue wire hardening — TLS via shared `KOLLECT_TRANSPORT_TLS_*` env;
  hub registration ACL via `KOLLECT_REMOTE_CLUSTERS`; vendor broker ACLs remain operator responsibility.
- **OPEN:** Federated trust / mTLS for HTTP ingress behind non-Kubernetes load balancers?
- **RESOLVED (2026-06-05):** `KollectHub.spec.remoteClusters[]` wires `KOLLECT_REMOTE_CLUSTERS` env
  for hub consumer ACL + shard routing inputs — list discovery alone is insufficient for production hubs.

## See also

- [ADR-0501: Multi-cluster sync topology](0501-multi-cluster-sync-rfc.md)
- [ADR-0502: Lean queue transport](0502-lean-queue-transport.md)
- [ADR-0404: Inventory HTTP API authentication](0404-inventory-api-auth.md)
- [Istio: Install multi-cluster — primary-remote](https://istio.io/latest/docs/setup/install/multicluster/primary-remote/)
- [Istio: `istioctl create-remote-secret`](https://github.com/istio/istio/blob/master/istioctl/pkg/multicluster/remote_secret.go)
