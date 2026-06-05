# Operator manual

Production-oriented guide for **platform teams** installing and operating **Kollect** for tenant
workloads. If you are evaluating locally, start with [Quick start](QUICKSTART.md) or
[Kind local lab](examples/kind-local-lab.md) first.

!!! tip "Assumptions"
    This guide assumes Helm 3, kubectl, and a working Kubernetes cluster. New to Kollect CRDs,
    sink roles, or watch scope? Read [Understand the basics](UNDERSTAND-THE-BASICS.md) and
    [Platform decisions](PLATFORM-DECISIONS.md) before changing production values.

!!! warning "Pre-beta API"
    `v1alpha1` fields and defaults may change until the first release candidate. Check
    [ROADMAP](ROADMAP.md) before production rollout.

## Install

Kollect ships as a **Helm chart** (`charts/kollect`) with CRDs in `crds/` and the controller
Deployment in `templates/`. Chart structure and install modes are documented in
[ADR-0704: Helm chart and CRD lifecycle](adr/0704-helm-chart-crd-lifecycle.md).

### From the repository

```sh
helm install kollect ./charts/kollect -n kollect-system --create-namespace
```

### From GHCR (OCI)

Published releases push the chart to GHCR ([ADR-0705](adr/0705-release-supply-chain.md)):

```sh
helm install kollect oci://ghcr.io/konih/kollect --version 0.1.0 -n kollect-system --create-namespace
```

Pin `image.tag` to the release image when not using the chart default.

### CRDs (first install)

Helm installs CRDs from `crds/` on **first install only**. Apply the release CRD bundle explicitly
when installing from raw manifests or when you need a known schema version:

```sh
kubectl apply -f dist/install-crds.yaml
```

!!! note "Two install artifacts"
    Day-2 upgrades treat **CRD schema** and **operator Deployment** as separate steps — see
    [Upgrade](#upgrade) below. Full operator manifest: `dist/install.yaml`.

### Per-team install (recommended default)

For tenant isolation, enable namespaced RBAC and restrict the informer cache
([ADR-0203](adr/0203-namespaced-multi-tenancy.md), [ADR-0703](adr/0703-platform-architecture-pivot.md)):

```yaml
tenantMode: true
watchNamespaces:
  - team-a
mode: single
featureGates:
  inventoryHttp:
    enabled: false
```

```sh
helm install kollect ./charts/kollect -n kollect-system --create-namespace -f values-team.yaml
```

Namespaced `KollectProfile`, `KollectSink`, `KollectTarget`, and `KollectInventory` live in the team
namespace. Portal read path uses **Postgres or Kafka sink export** — not spoke HTTP.

## Upgrade

Helm **does not upgrade or delete CRDs** on `helm upgrade`. Kollect accepts this deliberately and
documents a **two-step upgrade** ([ADR-0704](adr/0704-helm-chart-crd-lifecycle.md)):

1. **Apply CRD schema** out of band:

   ```sh
   kubectl apply -f dist/install-crds.yaml
   ```

2. **Upgrade the operator** (image, RBAC, webhooks):

   ```sh
   helm upgrade kollect ./charts/kollect -n kollect-system -f values.yaml
   ```

!!! warning "Never delete CRDs"
    Deleting a CRD garbage-collects all custom resources. CRD upgrades are **apply-only**;
    tooling must never remove them.

!!! note "Webhook certificates"
    Default webhook serving uses **cert-manager** (`webhooks.certManager.create: true`). Clusters
    without cert-manager can use the self-signed bootstrap path — see
    [ADR-0105](adr/0105-webhook-serving-cert-management.md).

Release artifacts (`install-crds.yaml`, OCI chart, pinned image) publish on each GitHub Release —
see [RELEASE](RELEASE.md).

## Helm values

Key values are validated by [`values.schema.json`](../charts/kollect/values.schema.json); CI runs
`task helm-test`. See [`charts/kollect/values.yaml`](../charts/kollect/values.yaml) for the full list.

| Key | Description | Default |
| --- | --- | --- |
| `image.repository` | Controller image | `ghcr.io/konih/kollect` |
| `image.tag` | Image tag | `latest` (pin in production) |
| `replicaCount` | Manager pod replicas | `1` |
| `leaderElection.enabled` | Controller-runtime leader election | `true` |
| `mode` | Operator mode: `single`, `hub`, or `spoke` | `single` |
| `tenantMode` | Namespaced Role RBAC for per-team installs | `false` |
| `watchNamespaces` | Restrict informer cache to these namespaces | `[]` (all) |
| `featureGates.inventoryHttp.enabled` | Expose `GET /inventory` (debug/small install only) | `false` |
| `webhooks.enabled` | Validating webhook for profiles | `true` |
| `sinkDefaults.connectionTest` | Default for sample `KollectSink` probes | `false` |
| `transport.type` | Hub/spoke transport backend | `inprocess` |

Export debouncing is configured per **`KollectInventory.spec.exportMinInterval`** (CRD default
**30s**). The chart does not pass the deprecated manager `--export-debounce` flag.

### Hub and spoke values

Multi-cluster hub/spoke uses **Helm `mode`** on the same image — there is **no `KollectHub` CRD**
([ADR-0703](adr/0703-platform-architecture-pivot.md)). Spoke: `mode: spoke`. Hub: `hub.sinkRefs`,
`hub.remoteClusters`, `hub.exportNamespace`. Walkthrough: [Hub mode example](examples/hub-mode.md).

!!! warning "Pre-beta hub transport"
    Hub ingest and spoke push paths are still maturing. Default transport is `inprocess` until an
    external backend passes integration proof ([ADR-0502](adr/0502-lean-queue-transport.md)).

### Feature gates

Optional HTTP and debug surfaces are **off by default** and map from Helm `featureGates.*` to manager
flags ([ADR-0704](adr/0704-helm-chart-crd-lifecycle.md)):

| Gate | Helm values | Default |
| --- | --- | --- |
| Inventory HTTP API | `featureGates.inventoryHttp.enabled` | **false** |
| pprof | `pprof.enabled` | **false** |
| Validating webhooks | `webhooks.enabled` | **true** |

Inventory HTTP auth uses Kubernetes bearer tokens by default
([ADR-0404](adr/0404-inventory-api-auth.md)). Optional `oauth2Proxy` sidecar is for browser/OIDC only.

## Secrets

### Sink credentials

Never put passwords or tokens on `KollectSink` CRs. Reference Kubernetes Secrets instead:

| Sink type | Secret keys | Field |
| --- | --- | --- |
| Postgres | `dsn`, `url`, `connectionString`, or `DATABASE_URL` | `spec.postgres.databaseRef` |
| Git / GitLab | deploy key or token | `spec.secretRef` |
| S3 / GCS | access credentials | `spec.secretRef` or provider-specific refs |
| Kafka / NATS | broker credentials | `spec.secretRef` |

Walkthrough: [Postgres state store](examples/postgres-state-store.md).

!!! warning "Credentials in Secrets only"
    Inline credentials on CRs are rejected by policy and leak in `kubectl get -o yaml`. Store DSNs
    and tokens in Secrets; grant the operator ServiceAccount read access via RBAC.

TLS trust for sinks uses `caBundle` or `caSecretRef` on the sink spec — same resolution as export
and connection probes ([ADR-0403](adr/0403-connection-test.md)).

### Webhook serving certificate

Validating webhooks require a TLS serving cert mounted on every manager replica
([ADR-0105](adr/0105-webhook-serving-cert-management.md)):

- **Default:** cert-manager `Certificate` in `webhook-certmanager.yaml` (soft dependency).
- **Fallback:** self-signed bootstrap when `webhooks.certManager.create: false`.

Example: [Cert-manager webhooks](examples/cert-manager-webhook.md).

### Production connection tests

Production sink manifests should use **`spec.connectionTest: false`** (chart default) and trigger
probes with the **`kollect.dev/test-connection: "true"`** annotation when needed. CI and samples may
set `connectionTest: true` ([ADR-0403](adr/0403-connection-test.md)).

## Watch scope

Kollect collection scope is controlled at three layers:

### Helm: `watchNamespaces` and `tenantMode`

| Setting | Effect |
| --- | --- |
| `watchNamespaces: []` | Informer cache watches all namespaces (cluster-wide install) |
| `watchNamespaces: [team-a, team-b]` | Cache restricted to listed namespaces |
| `tenantMode: true` | Namespaced `Role`/`RoleBinding` instead of `ClusterRole` |

Use **`tenantMode: true` + `watchNamespaces`** for per-team operator installs
([ADR-0203](adr/0203-namespaced-multi-tenancy.md)). Example:
[Multi-tenant watch scope](examples/multi-tenant-watch-namespaces.md).

### `KollectScope` allow-lists

Optional `KollectScope` CRs enforce GVK, namespace, and sink allow-lists. Violations set
`Degraded` on affected pipelines ([ADR-0203](adr/0203-namespaced-multi-tenancy.md)).

### Watch labels and annotations

Teams can opt individual namespaces or resources in or out without changing Helm values
([ADR-0205](adr/0205-watch-labels.md)):

| Key | Values | Effect |
| --- | --- | --- |
| `kollect.dev/watch` (label) | `enabled` / `disabled` | Opt in or out a namespace or resource |
| `kollect.dev/namespace-watch` (annotation) | `enabled` / `disabled` | Opt in or out all resources in a namespace |

`KollectTarget.spec.watchMode` defaults to `All`. Set `watchMode: OptIn` to collect only
`enabled` namespaces/resources.

## High availability

Controller HA, leader election, webhook serving, and hub consumer scaling are documented in
[ADR-0504: Operator runtime modes, HA, and leader election](adr/0504-operator-runtime-modes-ha-leader-election.md).

Summary for operators:

| Concern | Default chart | Production guidance |
| --- | --- | --- |
| Controller replicas | `replicaCount: 1` | `replicaCount: 2+`, `leaderElection.enabled: true` |
| Duplicate exports | Prevented by leader election | **Never** set `replicaCount > 1` with `leaderElection.enabled: false` |
| Webhooks | Served on every ready replica | Apiserver targets webhook `Service`; not gated by leader election |
| Hub ingest | Single pod acceptable for MVP | Scale `replicaCount`; shard transport per ADR-0502 before RAM limits |

!!! info "Same image, three modes"
    `mode: single` (default), `mode: spoke`, and `mode: hub` use one container image configured by
    Helm values — no forked binaries ([ADR-0501](adr/0501-multi-cluster-sync-rfc.md)).

## See also

- [FAQ](FAQ.md) — symptom-oriented troubleshooting
- [Quick start](QUICKSTART.md) · [Development setup](DEVELOPMENT.md)
- [Chart README](../charts/kollect/README.md) — values reference at source
- [RELEASE](RELEASE.md) — version bumps and release artifacts
- [ADR-0704: Helm chart and CRD lifecycle](adr/0704-helm-chart-crd-lifecycle.md)
- [ADR-0504: Runtime modes and HA](adr/0504-operator-runtime-modes-ha-leader-election.md)
- [ADR-0104: Security model](adr/0104-security-model.md)
