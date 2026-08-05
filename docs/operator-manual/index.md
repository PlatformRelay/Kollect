# Operator manual

Production-oriented guide for **platform teams** installing and operating **Kollect** for tenant
workloads. If you are evaluating locally, start with [Quick start](../getting-started/install.md) or
[Kind local lab](../getting-started/install.md) first.

!!! tip "Assumptions"
    This guide assumes Helm 3, kubectl, a working Kubernetes cluster, and cert-manager for the
    default webhook-enabled chart. New to **Kollect** CRDs,
    sink roles, or watch scope? Read [Understand the basics](../concepts/resource-model.md) and
    [Platform decisions](../PLATFORM-DECISIONS.md) before changing production values.

!!! warning "Pre-beta API"
    `v1alpha1` fields and defaults may change until the first release candidate. Check
    [ROADMAP](../ROADMAP.md) before production rollout.

## In this manual

| Topic | Page |
| --- | --- |
| Install (Helm, OCI, CRDs, tenant mode) | [Install](#install) (below) |
| Version upgrades (CRD + operator two-step) | [Upgrading Kollect](upgrading.md) |
| Helm values (production knobs) | [Helm values reference](helm-values.md) |
| Prometheus metrics and alerts | [Operator metrics](metrics.md) |
| Sink and webhook secrets | [Secrets](#secrets) (below) |
| Informer scope and tenancy | [Watch scope](#watch-scope) (below) |
| Replicas and leader election | [High availability](#high-availability) (below) |
| Lab evidence publish contract | [Lab evidence bundle](lab-evidence-bundle.md) |
| Adaptive local / existing-cluster lab | [Local lab runbook](local-lab-runbook.md) |
| Local backend / emulator fidelity | [Lab backend fidelity](lab-backend-fidelity.md) |

## Install

**Kollect** ships as a **Helm chart** (`charts/kollect`) with CRDs in `crds/` and the controller
Deployment in `templates/`. Chart structure and install modes are documented in
[ADR-0704: Helm chart and CRD lifecycle](../adr/0704-helm-chart-crd-lifecycle.md).

### From the repository

```sh
helm install kollect ./charts/kollect -n kollect-system --create-namespace
```

### From GHCR (OCI)

Published releases push the chart to GHCR ([ADR-0705](../adr/0705-release-supply-chain.md)):

```sh
helm install kollect oci://ghcr.io/platformrelay/kollect -n kollect-system --create-namespace
```

Omitting `--version` installs the latest published chart. In production, pin a specific version
(e.g. `--version 0.5.0` — see the [releases page](https://github.com/platformrelay/kollect/releases)).

Pin `image.tag` to the release image when not using the chart default — see
[Helm values reference](helm-values.md).

!!! note "Lab clusters where GHCR is denied"
    A live OCI pull is the preferred path. When package ACLs return **403** on a
    disconnected lab (for example a Talos driving-range), use the fallback in
    [Lab image bootstrap when GHCR is unavailable (403)](#lab-image-bootstrap-when-ghcr-is-unavailable-403).

### CRDs (first install)

Helm installs CRDs from `crds/` on **first install only**. Apply the release CRD bundle explicitly
when installing from raw manifests or when you need a known schema version:

```sh
kubectl apply -f dist/install-crds.yaml
```

!!! note "Two install artifacts"
    Day-2 upgrades treat **CRD schema** and **operator Deployment** as separate steps — see
    [Upgrading Kollect](upgrading.md). Full operator manifest: `dist/install.yaml`.

### Per-team install (recommended default)

For tenant isolation, enable namespaced RBAC and restrict the informer cache
([ADR-0203](../adr/0203-namespaced-multi-tenancy.md), [ADR-0201](../adr/0201-crd-model.md)):

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

Full value reference: [Helm values](helm-values.md).

## Secrets

### Sink credentials

Never put passwords or tokens on family sink CRs (`KollectSnapshotSink`, `KollectDatabaseSink`,
`KollectEventSink`). Reference Kubernetes Secrets instead:

| Sink family | Backends | Secret keys | Field |
| --- | --- | --- | --- |
| `KollectDatabaseSink` | Postgres, MongoDB | `dsn`, `url`, `connectionString`, or `DATABASE_URL` | `spec.postgres.databaseRef` / MongoDB ref |
| `KollectSnapshotSink` | Git, GitLab, S3, GCS | deploy key or token | `spec.secretRef` |
| `KollectEventSink` | Kafka, NATS | broker credentials | `spec.secretRef` |

Maturity tiers (Core / Beta / Planned): [Roadmap — Supported & planned sinks](../ROADMAP.md#supported-planned-sinks).

Walkthrough: [Postgres state store](../examples/postgres-state-store.md).

!!! warning "Credentials in Secrets only"
    Inline credentials on CRs are rejected by policy and leak in `kubectl get -o yaml`. Store DSNs
    and tokens in Secrets; grant the operator ServiceAccount read access via RBAC.

TLS trust for sinks uses `caBundle` or `caSecretRef` on the sink spec — same resolution as export
and connection probes ([ADR-0403](../adr/0403-connection-test.md)).

### Git snapshot sinks (`spec.git.engine`)

| `spec.git.engine` | Runtime needs | Notes |
| --- | --- | --- |
| `go-git` (default) | None beyond the manager binary | Pure Go transport; works on minimal images |
| `cli` | `git` and `openssh-client` in `PATH` | Native clone/commit/push; SSH uses `GIT_SSH_COMMAND` with `openssh-client` |

The published operator image (`ghcr.io/platformrelay/kollect`) ships **Debian bookworm-slim** with
`git`, `openssh-client`, and `ca-certificates` on UID/GID **65532**. Default `go-git` export is
unchanged; the image change enables `engine: cli` and full `git ls-remote` connection probes.
Custom images built from an older distroless base must install `git` (and `openssh-client` for SSH)
when using `engine: cli`.

### Webhook serving certificate

Validating webhooks require a TLS serving cert mounted on every manager replica
([ADR-0105](../adr/0105-webhook-serving-cert-management.md)):

### Webhook TLS

- **Default:** cert-manager `Certificate` in `webhook-certmanager.yaml`. cert-manager is required
  for the default chart values.
- **Operator-provided certificates:** set `webhooks.certManager.create: false` only after arranging
  the named serving Secret and CA trust for the `ValidatingWebhookConfiguration`. This setting
  suppresses cert-manager resources; it does not generate a certificate.
- **Development only:** `webhooks.enabled: false` avoids the TLS dependency by disabling validating
  admission. Do not use this as the production workaround.

Example: [Cert-manager webhooks](index.md#webhook-tls).

### Production connection tests

Production sink manifests should use **`spec.connectionTest: false`** (chart default) and trigger
probes with the **`kollect.dev/test-connection: "true"`** annotation when needed. CI and samples may
set `connectionTest: true` ([ADR-0403](../adr/0403-connection-test.md)).

## Watch scope

**Kollect** collection scope is controlled at three layers:

### Helm: `watchNamespaces` and `tenantMode`

| Setting | Effect |
| --- | --- |
| `watchNamespaces: []` | Informer cache watches all namespaces (cluster-wide install) |
| `watchNamespaces: [team-a, team-b]` | Cache restricted to listed namespaces |
| `tenantMode: true` | Namespaced `Role`/`RoleBinding` instead of `ClusterRole` |

Use **`tenantMode: true` + `watchNamespaces`** for per-team operator installs
([ADR-0203](../adr/0203-namespaced-multi-tenancy.md)). Team path profile:
[Team-owned operator (minimal RBAC)](../examples/team-operator.md). Example:
[Multi-tenant watch scope](../examples/multi-tenant-watch-namespaces.md).

Helm keys: [Helm values reference](helm-values.md).

### `KollectScope` allow-lists

Optional `KollectScope` CRs enforce GVK, namespace, and sink allow-lists. Violations set
`Degraded` on affected pipelines ([ADR-0203](../adr/0203-namespaced-multi-tenancy.md)).

### Watch labels and annotations

Teams can opt individual namespaces or resources in or out without changing Helm values
([ADR-0205](../adr/0205-watch-labels.md)):

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
| Memory at scale | `resourcesProfile` default | Use `resourcesProfile: large` for 100k-row design target ([ADR-0603](../adr/0603-performance-scalability.md)) |

!!! note "Multi-node lab evidence (v0.14.0)"
    On a Talos lab with 1 control plane and 2 workers, published **v0.14.0** ran with
    `replicaCount: 2` on distinct workers and completed a leader failover without unexpected
    restarts in that schedule. That is **ready-with-conditions** evidence for HA wiring on that
    pin — not a load, NetPol, or drain proof. See
    [testing · multi-node lab evidence](../development/testing.md#multi-node-lab-evidence).

!!! info "Single-cluster mode only"
    The operator runs `mode: single`. Multi-cluster fleets run **one single-mode operator per
    cluster** exporting to a shared sink, partitioned by `spec.cluster` — there is no hub/spoke
    runtime tier ([ADR-0501](../adr/0501-multi-cluster-fleet.md)).

## Lab image bootstrap when GHCR is unavailable (403)

!!! warning "Fallback only — not a production path"
    A **live OCI pull from GHCR is the preferred install path** (see [Install](#install)). Use this
    bootstrap **only** when package ACLs deny access — for example a disconnected Talos
    driving-range where `ghcr.io/platformrelay/kollect:<tag>` returns **403** to anonymous *and*
    authenticated pulls, and a personal `ghcr.io` push is also denied (no `write:packages`). It
    stands up a plain-HTTP lab registry and a temporary Talos registry mirror; both are insecure by
    design and **must be torn down after the run**.

The bootstrap has three parts: install the **chart** from the GitHub Release asset, rebuild the
**manager image** from the release git tag into a lab registry, and point a temporary Talos
**registry mirror** at that registry so nodes resolve `ghcr.io` pulls locally.

Set these placeholders for your lab (do not hard-code a single lab's addresses):

```sh
export LAB_HOST_IP=192.168.100.1                 # lab host IP reachable by Talos nodes
export TALOS_NODES=192.168.100.10,192.168.100.11,192.168.100.12  # all control-plane + workers
export VERSION=0.10.0                             # target release, matches the chart/image tag
```

### 1. Chart from the GitHub Release asset

Install the chart from the release `.tgz` instead of the OCI `oci://ghcr.io/platformrelay/kollect`
pull:

```sh
curl -sSLO https://github.com/platformrelay/kollect/releases/download/v${VERSION}/kollect-${VERSION}.tgz
helm install kollect ./kollect-${VERSION}.tgz -n kollect-system --create-namespace -f values.yaml \
  --set image.tag=${VERSION}
```

The chart keeps its default `ghcr.io/platformrelay/kollect` **repository** — step 3 makes nodes
resolve that host from the lab registry, so no `image.repository` override is needed. You **must**,
however, set `image.tag=${VERSION}`: the chart's default tag resolves to the v-prefixed release
image (`:v${VERSION}`), but step 2 below rebuilds and pushes only the bare `:${VERSION}` to the lab
registry (the mirror rewrites the host, not the tag), so a default-tag install would try to pull a
`:v${VERSION}` image that this lab flow never pushed and fail with `ImagePullBackOff`.

### 2. Rebuild the manager image into a dual-bound lab registry

Run a host registry bound to **both** loopback (for an insecure local push — Docker treats
`127.0.0.1` as insecure automatically) and the node-facing lab IP (so Talos nodes can pull):

```sh
docker run -d --restart=always --name kollect-dr-registry \
  -p 127.0.0.1:5000:5000 -p ${LAB_HOST_IP}:5000:5000 registry:2
```

Rebuild the manager image from the **release git tag** and push it under the `platformrelay/kollect`
repository path (so the mirror in step 3 resolves the same path GHCR would):

```sh
git fetch --tags
git checkout v${VERSION}
docker build -t 127.0.0.1:5000/platformrelay/kollect:${VERSION} .
docker push 127.0.0.1:5000/platformrelay/kollect:${VERSION}
```

!!! note "Rebuilt digest differs from the release"
    This image is rebuilt from source, so its digest will **not** match the published release digest
    and cosign verification against the release will not apply. That is acceptable for a lab; never
    ship a self-rebuilt image to production.

### 3. Temporary Talos registry mirror (with mandatory revert)

Point the `ghcr.io` mirror at the lab registry on **every** node. Applying the whole
`/machine/registries` block (absent by default) keeps the revert a clean wholesale remove:

```sh
cat > /tmp/registries.patch.yaml <<EOF
- op: add
  path: /machine/registries
  value:
    mirrors:
      ghcr.io:
        endpoints:
          - http://${LAB_HOST_IP}:5000
EOF
talosctl -n ${TALOS_NODES} patch mc --patch @/tmp/registries.patch.yaml
```

!!! warning "If nodes already define `machine.registries`"
    The `add` above assumes the block is absent (the driving-range default). If your nodes already
    carry a `machine.registries` block, **merge** the `ghcr.io` mirror into it instead — and on
    revert restore the prior block rather than removing the whole path.

**Cleanup / revert is mandatory.** After the run, remove the mirror and confirm no node still
references the lab host:

```sh
# revert: remove the entire block added above
talosctl -n ${TALOS_NODES} patch mc --patch '[{"op":"remove","path":"/machine/registries"}]'

# verify machine.registries no longer references the lab host (expect no output)
talosctl -n ${TALOS_NODES} get mc v1alpha1 -o yaml | grep -F "${LAB_HOST_IP}:5000"

# tear down the lab registry container
docker rm -f kollect-dr-registry
```

Then uninstall the release and delete the lab namespace as usual (`helm uninstall kollect -n
kollect-system`). Once the org GHCR ACL is fixed, revert to the [live OCI pull](#from-ghcr-oci) —
this bootstrap should not outlive the access outage that forced it.

## See also

- [Upgrading Kollect](upgrading.md) · [Helm values](helm-values.md)
- [Common errors](troubleshooting.md) — full catalog: conditions, metrics, and fixes
- [FAQ](troubleshooting.md) — symptom-oriented troubleshooting
- [Quick start](../getting-started/install.md) · [Development setup](../development/setup.md)
- [Chart README](https://github.com/platformrelay/kollect/blob/main/charts/kollect/README.md) — inventory HTTP auth at source
- [RELEASE](../RELEASE.md) — version bumps and release artifacts
- [ADR-0704: Helm chart and CRD lifecycle](../adr/0704-helm-chart-crd-lifecycle.md)
- [ADR-0501: Multi-cluster fleet](../adr/0501-multi-cluster-fleet.md)
- [ADR-0104: Security model](../adr/0104-security-model.md)
