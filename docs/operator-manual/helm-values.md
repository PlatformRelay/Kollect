# Helm values reference

Key **Kollect** chart values for platform operators. This page summarizes the most common production
knobs; the authoritative full list lives in the chart tree.

!!! tip "Assumptions"
    This guide assumes Helm 3 and a chart install path ([Install](../OPERATOR-MANUAL.md#install)).
    For CRD vs operator upgrade ordering, see [Upgrading Kollect](upgrading.md).

!!! warning "Pre-beta API"
    `v1alpha1` fields may change. Check [ROADMAP](../ROADMAP.md) before locking production values.

## Source of truth

| File | Purpose |
| --- | --- |
| [`charts/kollect/values.yaml`](../../charts/kollect/values.yaml) | All defaults |
| [`charts/kollect/values.schema.json`](../../charts/kollect/values.schema.json) | JSON Schema validation (CI: `task helm-test`) |
| [`charts/kollect/README.md`](../../charts/kollect/README.md) | Hub/spoke YAML, inventory HTTP auth, connection-test detail |

!!! note "Do not duplicate the chart README"
    Hub ingest env mapping, oauth2-proxy sidecar layout, and inventory HTTP RBAC examples are maintained
    in the chart README only — link there instead of copying large blocks into docs.

## Core values

| Key | Description | Default |
| --- | --- | --- |
| `image.repository` | Controller image | `ghcr.io/konih/kollect` |
| `image.tag` | Image tag | `latest` (**pin in production**) |
| `replicaCount` | Manager pod replicas | `1` |
| `leaderElection.enabled` | Controller-runtime leader election | `true` |
| `mode` | Operator mode: `single`, `hub`, or `spoke` | `single` |
| `tenantMode` | Namespaced Role RBAC for per-team installs | `false` |
| `watchNamespaces` | Restrict informer cache to these namespaces | `[]` (all) |
| `transport.type` | Hub/spoke transport backend | `inprocess` |
| `webhooks.enabled` | Validating webhook for profiles | `true` |
| `webhooks.certManager.create` | cert-manager `Certificate` for webhook TLS | `true` |
| `sinkDefaults.connectionTest` | Default for sample `KollectSink` probes | `false` |

Export debouncing is configured per **sink ref** on `KollectInventory` / `KollectClusterInventory`
([ADR-0413](../adr/0413-export-interval-scheduling.md)): ref override → sink default → inventory
default **30s** → scope floor.

Enable Prometheus Operator scraping and alerts:

```yaml
metrics:
  serviceMonitor:
    enabled: true
    labels:
      release: kube-prometheus-stack
  prometheusRule:
    enabled: true
    labels:
      release: kube-prometheus-stack
```

See [Metrics](metrics.md) for the alert catalog and [chart README — monitoring](../../charts/kollect/README.md#prometheus-operator-monitoring).

## Per-team install (recommended default)

For tenant isolation, enable namespaced RBAC and restrict the informer cache
([ADR-0203](../adr/0203-namespaced-multi-tenancy.md), [ADR-0703](../adr/0703-platform-architecture-pivot.md)):

```yaml
tenantMode: true
watchNamespaces:
  - team-a
mode: single
featureGates:
  inventoryHttp:
    enabled: false
```

Namespaced `KollectProfile`, `KollectSink`, `KollectTarget`, and `KollectInventory` live in the team
namespace. Portal read path uses **Postgres or Kafka sink export** — not spoke HTTP.

Example walkthrough: [Multi-tenant watch scope](../examples/multi-tenant-watch-namespaces.md).

## Hub and spoke

Multi-cluster hub/spoke uses **Helm `mode`** on the same image — there is **no `KollectHub` CRD**
([ADR-0703](../adr/0703-platform-architecture-pivot.md)).

| Mode | Key values | Notes |
| --- | --- | --- |
| Spoke | `mode: spoke`, `transport.type: inprocess` | Collect locally; optional hub push per [ADR-0503](../adr/0503-hub-cluster-auth-istio-pattern.md) |
| Hub | `mode: hub`, `hub.sinkRefs`, `hub.remoteClusters`, `hub.exportNamespace` | Merge + parallel Postgres/Kafka export |

Spoke and hub value blocks, env var mapping, and `KollectRemoteCluster` registration are documented in
the [chart README — Hub mode](../../charts/kollect/README.md#hub-mode-no-kollecthub-crd).

Walkthrough: [Hub mode example](../examples/hub-mode.md).

!!! warning "Pre-beta hub transport"
    Default transport is `inprocess` until an external backend passes integration proof
    ([ADR-0502](../adr/0502-lean-queue-transport.md)). Do not enable Redis/NATS/Kafka in chart values
    without explicit ops sign-off.

## Feature gates

Optional HTTP and debug surfaces are **off by default** ([ADR-0704](../adr/0704-helm-chart-crd-lifecycle.md)):

| Gate | Helm values | Default |
| --- | --- | --- |
| Inventory HTTP API | `featureGates.inventoryHttp.enabled` | **false** |
| pprof | `pprof.enabled` | **false** |
| Validating webhooks | `webhooks.enabled` | **true** |

Inventory HTTP auth uses Kubernetes bearer tokens by default ([ADR-0404](../adr/0404-inventory-api-auth.md)).
Optional `oauth2Proxy` sidecar is for browser/OIDC only — see the
[chart README — Inventory HTTP authentication](../../charts/kollect/README.md#inventory-http-authentication).

## Connection tests

Production sink manifests should use **`spec.connectionTest: false`** (chart default) and trigger probes
with the **`kollect.dev/test-connection: "true"`** annotation when needed. CI and samples may set
`connectionTest: true` ([ADR-0403](../adr/0403-connection-test.md)).

## Resources, metrics, and webhooks

| Key | Description | Default |
| --- | --- | --- |
| `resources` | CPU/memory requests and limits | See `values.yaml` |
| `metrics.enabled` | Prometheus metrics listener | `true` |
| `metrics.serviceMonitor.enabled` | Prometheus Operator `ServiceMonitor` | `false` |
| `metrics.prometheusRule.enabled` | Default `PrometheusRule` alerts | `false` |
| `controller.maxConcurrentReconciles.*` | Per-controller concurrency | See `values.yaml` |
| `extraArgs` | Additional manager flags (debug only) | `[]` |

Webhook serving certificates: cert-manager default or self-signed bootstrap —
[ADR-0105](../adr/0105-webhook-serving-cert-management.md) ·
[Cert-manager webhooks example](../examples/cert-manager-webhook.md).

## See also

- [Operator manual](../OPERATOR-MANUAL.md) · [Upgrading Kollect](upgrading.md) · [Metrics](metrics.md)
- [ADR-0704: Helm chart and CRD lifecycle](../adr/0704-helm-chart-crd-lifecycle.md)
- [High availability](../OPERATOR-MANUAL.md#high-availability) — `replicaCount` and leader election
