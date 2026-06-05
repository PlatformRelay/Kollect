# ADR-0504: Operator runtime modes, HA, and leader election

> One controller image, three deployment modes (`single`, `hub`, `spoke`), controller-runtime leader
> election for reconcilers, and how HA replicas interact with webhooks, hub ingest, and future read-API
> splits.

**Theme:** 05 · Multi-cluster · **Status:** Current

## Context

Kollect ships as a **single container image** (`ghcr.io/konih/kollect`) configured by Helm values and
manager flags ([ADR-0501](0501-multi-cluster-sync-rfc.md), [ADR-0703](0703-platform-architecture-pivot.md)).
There is **no `KollectHub` CRD** — hub vs spoke is **`values.mode`** (`single` | `hub` | `spoke`) plus
hub/spoke sub-values.

The chart defaults to **`replicaCount: 1`** and **`leaderElection.enabled: true`**
(`--leader-elect`). Reconciler HA, webhook serving, and hub consumer sharding had different
requirements but were never recorded. [ADR-0408](0408-read-api-ui-architecture.md) plans an optional
**`kollect-server` split** so a busy read API / SPA does not share the controller process at scale.

## Decision

### Runtime modes (same binary)

| Mode | Helm `mode` | Manager flags / env | Active subsystems |
| --- | --- | --- | --- |
| **Single cluster** | `single` (default) | Standard controller manager | Dynamic informers, namespaced `KollectInventory` export, connection tests |
| **Spoke** | `spoke` | `--mode=spoke` + spoke transport env | Same collection as single; **push** delta snapshots to hub ingest / shared sink ([ADR-0502](0502-lean-queue-transport.md), [ADR-0503](0503-hub-cluster-auth-istio-pattern.md)) |
| **Hub** | `hub` | `--mode=hub` + `KOLLECT_HUB_*` env | Ingest HTTP, merge/dedupe (`internal/hub/`), export to hub `sinkRefs`; **no spoke-side informers required** on hub for ingest-only installs |

**Rule:** mode selects **which controllers and servers start**; CRDs and export contract stay identical
([ADR-0405](0405-export-data-contract.md)). Single-cluster users never set `hub`/`spoke`.

### Leader election (reconcilers)

- **Controller reconcilers** (Target, Inventory, ConnectionTest, Scope, …) use controller-runtime
  **leader election** on a **single lease** (`charts/kollect/templates/role-leader-election.yaml`).
  Only the elected pod runs reconcilers when `leaderElection.enabled: true` (default).
- **`replicaCount > 1` without leader election is unsupported** for controller work — it would duplicate
  exports and race on sinks.
- **Hub `Runner`** (`internal/hub/runner.go`) returns **`NeedLeaderElection() == false`**: when transport
  is sharded or each pod subscribes to a partition, **multiple hub replicas may consume concurrently**.
  Hub HA is **horizontal consumers**, not a single active reconciler.
- **Optional servers** (inventory HTTP, pprof) also skip leader election when enabled — any replica may
  serve ([ADR-0704](0704-helm-chart-crd-lifecycle.md) feature gates).

### Webhooks and HA

- Validating webhooks run in the **manager process** on every ready replica; apiserver targets the
  webhook `Service`. **Leader election does not gate webhook serving** — standard kubebuilder pattern.
- All replicas mount the same serving cert Secret ([ADR-0105](0105-webhook-serving-cert-management.md)).

### HA posture (defaults vs production)

| Concern | Default chart | Production guidance |
| --- | --- | --- |
| Controller replicas | `replicaCount: 1` | `replicaCount: 2+`, `leaderElection.enabled: true`, **PDB** `minAvailable: 1` (template TBD) |
| Hub ingest | Single pod acceptable for MVP | Scale hub `replicaCount`; shard transport per [ADR-0502](0502-lean-queue-transport.md) before hub RAM becomes bottleneck |
| Spoke | One pod per cluster | Spokes stay **lightweight**; HA within a spoke cluster is optional (leader-elected collector) |
| Read API / UI | Operator-embedded when feature-gated | At scale, split to **`kollect-server`** Deployment ([ADR-0408](0408-read-api-ui-architecture.md)) — separate HA story |

### Graceful lifecycle

- **`/healthz` and `/readyz`** on `:8081`; `terminationGracePeriodSeconds: 10` on the Deployment.
- On SIGTERM, in-flight reconciles respect context cancel; leader releases lease for fast failover.
- Metrics on `:8443` (TLS) when enabled — all replicas expose metrics; scrape any ready pod.

## Consequences

- One image and one Helm chart cover single-cluster, hub, and spoke — no forked binaries.
- Leader election prevents duplicate export from multiple controller replicas; hub scale uses consumer
  sharding instead of a hub-wide lease.
- Webhook + reconciler HA semantics are explicit — adopters know replicas ≠ duplicate collection without
  `--leader-elect`.
- Future `kollect-server` split is anticipated without blocking current embedded feature gates.

## Open questions

- **DECIDED:** **`mode: hub|spoke`** on the same image; no `KollectHub` CRD ([ADR-0501](0501-multi-cluster-sync-rfc.md)).
- **OPEN:** Helm **PDB** template — always render when `replicaCount > 1`, or opt-in values flag?
- **OPEN:** Hub ingest **active/active** behind a Service without sticky sessions — document connection
  limits and merge idempotency ([ADR-0305](0305-aggregation-dedupe.md)).
- **OPEN:** `kollect-server` split milestone — v0.3.0 vs load-test trigger ([ADR-0408](0408-read-api-ui-architecture.md)).
- **OPEN:** Spoke mode with **`replicaCount > 1`** — supported for HA collector, or document single-replica
  spoke only until push dedupe is proven?
