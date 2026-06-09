# Operator manual

Production-oriented guide for **platform teams** installing and operating **Kollect** for tenant
workloads. If you are evaluating locally, start with [Quick start](QUICKSTART.md) or
[Kind local lab](examples/kind-local-lab.md) first.

!!! tip "Assumptions"
    This guide assumes Helm 3, kubectl, and a working Kubernetes cluster. New to **Kollect** CRDs,
    sink roles, or watch scope? Read [Understand the basics](UNDERSTAND-THE-BASICS.md) and
    [Platform decisions](PLATFORM-DECISIONS.md) before changing production values.

!!! warning "Pre-beta API"
    `v1alpha1` fields and defaults may change until the first release candidate. Check
    [ROADMAP](ROADMAP.md) before production rollout.

## In this manual

| Topic | Page |
| --- | --- |
| Install (Helm, OCI, CRDs, tenant mode) | [Install](#install) (below) |
| Version upgrades (CRD + operator two-step) | [Upgrading Kollect](operator-manual/upgrading.md) |
| Helm values (production knobs) | [Helm values reference](operator-manual/helm-values.md) |
| Prometheus metrics and alerts | [Operator metrics](operator-manual/metrics.md) |
| Sink and webhook secrets | [Secrets](#secrets) (below) |
| Informer scope and tenancy | [Watch scope](#watch-scope) (below) |
| Replicas and leader election | [High availability](#high-availability) (below) |
| Read-only UI (early adopter preview) | [Read-only UI](operator-manual/ui.md) · [ADR-0409](adr/0409-kollect-ui-deployment.md) · [UI local dev (mock)](examples/ui-local-development.md) |

## Install

**Kollect** ships as a **Helm chart** (`charts/kollect`) with CRDs in `crds/` and the controller
Deployment in `templates/`. Chart structure and install modes are documented in
[ADR-0704: Helm chart and CRD lifecycle](adr/0704-helm-chart-crd-lifecycle.md).

### From the repository

```sh
helm install kollect ./charts/kollect -n kollect-system --create-namespace
```

### From GHCR (OCI)

Published releases push the chart to GHCR ([ADR-0705](adr/0705-release-supply-chain.md)):

```sh
helm install kollect oci://ghcr.io/konih/kollect -n kollect-system --create-namespace
```

Omitting `--version` installs the latest published chart. In production, pin a specific version
(e.g. `--version 0.5.0` — see the [releases page](https://github.com/konih/kollect/releases)).

Pin `image.tag` to the release image when not using the chart default — see
[Helm values reference](operator-manual/helm-values.md).

### CRDs (first install)

Helm installs CRDs from `crds/` on **first install only**. Apply the release CRD bundle explicitly
when installing from raw manifests or when you need a known schema version:

```sh
kubectl apply -f dist/install-crds.yaml
```

!!! note "Two install artifacts"
    Day-2 upgrades treat **CRD schema** and **operator Deployment** as separate steps — see
    [Upgrading Kollect](operator-manual/upgrading.md). Full operator manifest: `dist/install.yaml`.

### Per-team install (recommended default)

For tenant isolation, enable namespaced RBAC and restrict the informer cache
([ADR-0203](adr/0203-namespaced-multi-tenancy.md), [ADR-0201](adr/0201-crd-model.md)):

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

Namespaced `KollectProfile`, family sinks, `KollectTarget`, and `KollectInventory` live in the team
namespace. Portal read path uses **Postgres or Kafka sink export** — not direct operator HTTP.

Full value reference: [Helm values](operator-manual/helm-values.md).

## Secrets

### Sink credentials

Never put passwords or tokens on family sink CRs (`KollectSnapshotSink`, `KollectDatabaseSink`,
`KollectEventSink`). Reference Kubernetes Secrets instead:

| Sink family | Backends | Secret keys | Field |
| --- | --- | --- | --- |
| `KollectDatabaseSink` | Postgres, MongoDB | `dsn`, `url`, `connectionString`, or `DATABASE_URL` | `spec.postgres.databaseRef` / MongoDB ref |
| `KollectSnapshotSink` | Git, GitLab, S3, GCS | deploy key or token | `spec.secretRef` |
| `KollectEventSink` | Kafka | broker credentials | `spec.secretRef` |

Walkthrough: [Postgres state store](examples/postgres-state-store.md).

!!! warning "Credentials in Secrets only"
    Inline credentials on CRs are rejected by policy and leak in `kubectl get -o yaml`. Store DSNs
    and tokens in Secrets; grant the operator ServiceAccount read access via RBAC.

TLS trust for sinks uses `caBundle` or `caSecretRef` on the sink spec — same resolution as export
and connection probes ([ADR-0403](adr/0403-connection-test.md)).

### Git snapshot sinks (`spec.git.engine`)

| `spec.git.engine` | Runtime needs | Notes |
| --- | --- | --- |
| `go-git` (default) | None beyond the manager binary | Pure Go transport; works on minimal images |
| `cli` | `git` and `openssh-client` in `PATH` | Native clone/commit/push; SSH uses `GIT_SSH_COMMAND` with `openssh-client` |

The published operator image (`ghcr.io/konih/kollect`) ships **Debian bookworm-slim** with
`git`, `openssh-client`, and `ca-certificates` on UID/GID **65532**. Default `go-git` export is
unchanged; the image change enables `engine: cli` and full `git ls-remote` connection probes.
Custom images built from an older distroless base must install `git` (and `openssh-client` for SSH)
when using `engine: cli`.

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

**Kollect** collection scope is controlled at three layers:

### Helm: `watchNamespaces` and `tenantMode`

| Setting | Effect |
| --- | --- |
| `watchNamespaces: []` | Informer cache watches all namespaces (cluster-wide install) |
| `watchNamespaces: [team-a, team-b]` | Cache restricted to listed namespaces |
| `tenantMode: true` | Namespaced `Role`/`RoleBinding` instead of `ClusterRole` |

Use **`tenantMode: true` + `watchNamespaces`** for per-team operator installs
([ADR-0203](adr/0203-namespaced-multi-tenancy.md)). Team path profile:
[Team-owned operator (minimal RBAC)](deployment/team-operator.md). Example:
[Multi-tenant watch scope](examples/multi-tenant-watch-namespaces.md).

Helm keys: [Helm values reference](operator-manual/helm-values.md).

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

Controller HA relies on controller-runtime leader election: scale `replicaCount` and let one elected
leader own reconciliation while standby replicas wait.

Summary for operators:

| Concern | Default chart | Production guidance |
| --- | --- | --- |
| Controller replicas | `replicaCount: 1` | `replicaCount: 2+`, `leaderElection.enabled: true` |
| Duplicate exports | Prevented by leader election | **Never** set `replicaCount > 1` with `leaderElection.enabled: false` |
| Webhooks | Served on every ready replica | Apiserver targets webhook `Service`; not gated by leader election |
| Memory at scale | `resourcesProfile` default | Use `resourcesProfile: large` for 100k-row design target ([ADR-0603](adr/0603-performance-scalability.md)) |

!!! info "Single-cluster mode only"
    The operator runs `mode: single`. Multi-cluster fleets run **one single-mode operator per
    cluster** exporting to a shared sink, partitioned by `spec.cluster` — there is no hub/spoke
    runtime tier ([ADR-0501](adr/0501-multi-cluster-fleet.md)).

## See also

- [Upgrading Kollect](operator-manual/upgrading.md) · [Helm values](operator-manual/helm-values.md)
- [Common errors](operator-manual/common-errors.md) — full catalog: conditions, metrics, and fixes
- [FAQ](FAQ.md) — symptom-oriented troubleshooting
- [Quick start](QUICKSTART.md) · [Development setup](DEVELOPMENT.md)
- [Chart README](../charts/kollect/README.md) — inventory HTTP auth at source
- [RELEASE](RELEASE.md) — version bumps and release artifacts
- [ADR-0704: Helm chart and CRD lifecycle](adr/0704-helm-chart-crd-lifecycle.md)
- [ADR-0501: Multi-cluster fleet](adr/0501-multi-cluster-fleet.md)
- [ADR-0104: Security model](adr/0104-security-model.md)
