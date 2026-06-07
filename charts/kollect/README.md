# Kollect Helm chart

Installs the [Kollect](https://github.com/konih/kollect) operator and CRDs.

![Version: 0.5.0-rc.1](https://img.shields.io/badge/Version-0.5.0--rc.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.5.0-rc.1](https://img.shields.io/badge/AppVersion-0.5.0--rc.1-informational?style=flat-square)

## Install

```bash
helm install kollect ./charts/kollect -n kollect-system --create-namespace
```

### Container image

The default image is **Debian bookworm-slim** (UID/GID 65532) with `git` and `openssh-client` so
`spec.git.engine: cli` and `git ls-remote` connection probes work out of the box. Default
`spec.git.engine: go-git` does not require those binaries. Pod `securityContext` defaults are
unchanged (`readOnlyRootFilesystem: true`, capabilities dropped, `/tmp` `emptyDir` for git workdirs).

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| controller.collectDispatchQueueSize | int | `512` | Collection informer dispatch queue depth. |
| controller.collectDispatchWorkers | int | `4` | Collection informer dispatch worker count (PERF-03). |
| controller.collectMetricsSampleInterval | string | `"30s"` |  |
| controller.informerResyncPeriod | string | `"12h"` |  |
| controller.maxConcurrentReconciles.inventory | int | `3` | Max concurrent reconciles for KollectInventory. |
| controller.maxConcurrentReconciles.target | int | `5` | Max concurrent reconciles for KollectTarget. |
| controller.reconcileRateLimit | string | `""` |  |
| createNamespace | bool | `false` | Create the release namespace if it does not exist. |
| defaultExcludedNamespaces | list | `[]` | Default namespace denylist for Target collection intent (CRD fields on KollectTarget override). |
| defaultIncludedNamespaces | list | `[]` | Default namespace allowlist for Target collection intent (CRD fields on KollectTarget override). |
| extraArgs | list | `[]` | Extra arguments passed to the manager container. |
| featureGates.inventoryHttp.authMode | string | `"kubernetes"` | Inventory HTTP auth mode: kubernetes (TokenReview/SAR) or disabled (dev only). |
| featureGates.inventoryHttp.enabled | bool | `false` | Expose read-only GET /inventory HTTP API (debug/small installs only). |
| featureGates.inventoryHttp.port | int | `8082` | Inventory HTTP listen port. |
| fullnameOverride | string | `""` | Override the full resource name prefix (defaults to release name + chart name). |
| image.pullPolicy | string | `"IfNotPresent"` | Controller image pull policy. |
| image.repository | string | `"ghcr.io/konih/kollect"` | Controller container image repository. |
| image.tag | string | `"latest"` | Controller image tag (defaults to chart appVersion when empty). |
| imagePullSecrets | list | `[]` | Image pull secrets for private registries. |
| largeCluster.controller.collectDispatchQueueSize | int | `2048` |  |
| largeCluster.controller.collectDispatchWorkers | int | `8` |  |
| largeCluster.controller.maxConcurrentReconciles.inventory | int | `10` |  |
| largeCluster.controller.maxConcurrentReconciles.target | int | `10` |  |
| largeCluster.resources.limits.cpu | string | `"2"` |  |
| largeCluster.resources.limits.memory | string | `"4Gi"` |  |
| largeCluster.resources.requests.cpu | string | `"500m"` |  |
| largeCluster.resources.requests.memory | string | `"2Gi"` |  |
| leaderElection.enabled | bool | `true` | Enable controller-runtime leader election. |
| metrics.bindAddress | string | `":8443"` | Metrics bind address. |
| metrics.enabled | bool | `true` | Expose Prometheus metrics endpoint. |
| metrics.prometheusRule.additionalRules | list | `[]` | Extra PrometheusRule entries appended to the kollect.rules group. |
| metrics.prometheusRule.enabled | bool | `false` | Create a PrometheusRule with starter kollect alerts. |
| metrics.prometheusRule.labels | object | `{}` | Labels merged onto PrometheusRule (must match Prometheus ruleSelector). |
| metrics.secure | bool | `true` | Serve metrics over HTTPS with bearer token auth. |
| metrics.serviceMonitor | object | `{"enabled":false,"interval":"30s","labels":{},"scrapeTimeout":"10s"}` | Prometheus Operator ServiceMonitor integration (requires monitoring.coreos.com CRDs). |
| metrics.serviceMonitor.enabled | bool | `false` | Create a ServiceMonitor resource. |
| metrics.serviceMonitor.interval | string | `"30s"` | Scrape interval. |
| metrics.serviceMonitor.labels | object | `{}` | Labels merged onto ServiceMonitor (must match Prometheus serviceMonitorSelector). |
| metrics.serviceMonitor.scrapeTimeout | string | `"10s"` | Scrape timeout. |
| mode | string | `"single"` | Operator deployment mode (single-cluster only). |
| nameOverride | string | `""` | Override the chart name used in labels and resource names. |
| nodeSelector | object | `{}` |  |
| oauth2Proxy | object | `{"enabled":false}` | Optional oauth2-proxy sidecar for browser/OIDC access (not rendered yet). |
| oauth2Proxy.enabled | bool | `false` | Enable oauth2-proxy sidecar in front of inventory HTTP. |
| podSecurityContext.runAsNonRoot | bool | `true` |  |
| podSecurityContext.seccompProfile.type | string | `"RuntimeDefault"` |  |
| pprof.bindAddress | string | `":6060"` | pprof bind address. |
| pprof.enabled | bool | `false` | Enable Go pprof debug endpoint on the manager. |
| rbac.create | bool | `true` | Create RBAC roles/bindings for the manager. |
| replicaCount | int | `1` | Manager Deployment replica count. |
| resources.limits.cpu | string | `"500m"` |  |
| resources.limits.memory | string | `"256Mi"` |  |
| resources.requests.cpu | string | `"10m"` |  |
| resources.requests.memory | string | `"64Mi"` |  |
| resourcesProfile | string | `"default"` |  |
| securityContext.allowPrivilegeEscalation | bool | `false` |  |
| securityContext.capabilities.drop[0] | string | `"ALL"` |  |
| securityContext.readOnlyRootFilesystem | bool | `true` |  |
| serviceAccount.annotations | object | `{}` | Annotations added to the ServiceAccount. |
| serviceAccount.create | bool | `true` | Create a ServiceAccount for the manager pod. |
| serviceAccount.name | string | `""` | ServiceAccount name (generated when empty). |
| sinkDefaults | object | `{"connectionTest":false}` | Defaults for KollectSink connection probes in bundled samples (CRD default is true). |
| sinkDefaults.connectionTest | bool | `false` | Default spec.connectionTest for sample sinks (CI/dev overlays often set true). |
| tenantMode | bool | `false` | When true, render namespaced Role/RoleBinding instead of ClusterRole/ClusterRoleBinding. |
| tolerations | list | `[]` |  |
| ui | object | `{"enabled":false,"image":{"repository":"ghcr.io/konih/kollect-ui","tag":""},"ingress":{"enabled":false},"readApiUrl":"http://kollect:8082"}` | Optional kollect-ui subchart (static React SPA — default off). |
| ui.enabled | bool | `false` | Enable the kollect-ui subchart. |
| ui.image.repository | string | `"ghcr.io/konih/kollect-ui"` | kollect-ui container image repository. |
| ui.image.tag | string | `""` | kollect-ui image tag (defaults to chart appVersion when empty). |
| ui.ingress.enabled | bool | `false` | Expose kollect-ui via Ingress. |
| ui.readApiUrl | string | `"http://kollect:8082"` | Read API base URL injected into the UI bundle. |
| watchNamespaces | list | `[]` | Restrict the manager informer cache to these namespaces (empty = all namespaces). Use for per-team operator installs. |
| webhooks.certManager.create | bool | `true` | Create cert-manager Certificate/Issuer for webhook serving cert. |
| webhooks.certManager.secretName | string | `"webhook-server-cert"` | Secret name for webhook TLS material. |
| webhooks.enabled | bool | `true` | Enable validating admission webhooks for Kollect CRDs. |

Export debouncing is configured per **`KollectInventory.spec.exportMinInterval`** (CRD default
**30s**). Set the field on the Inventory CR; the chart does not expose a global manager flag.

Critical values are validated by [`values.schema.json`](values.schema.json); CI runs `task helm-test`
(`helm lint`, `helm-unittest`, and `helm-docs` drift check).

### Per-team install (default story)

Documented default for new team installs ([ADR-0203](../../docs/adr/0203-namespaced-multi-tenancy.md),
[ADR-0703](../../docs/adr/0703-platform-architecture-pivot.md)):

```yaml
tenantMode: true
watchNamespaces:
  - team-a
mode: single
featureGates:
  inventoryHttp:
    enabled: false
```

Namespaced `KollectProfile`, sink family CRs (`KollectSnapshotSink`, `KollectDatabaseSink`,
`KollectEventSink`), `KollectTarget`, and `KollectInventory` live in the team namespace.
Portal read path: **Postgres or event sink export** (or Git snapshot for audit).

### Multi-cluster fleet

Fleet scale uses **one operator per cluster** writing to a **shared sink** (Postgres, Git, S3/GCS,
Kafka). Label exports with **`spec.cluster`** on each `KollectInventory` so the sink can merge rows
from many clusters ([ADR-0501](../../docs/adr/0501-multi-cluster-fleet.md),
[fleet scaling](../../docs/operator-manual/scaling-and-fleet.md)).

```yaml
# Each cluster — same chart, different release/namespace as needed
mode: single
tenantMode: true
watchNamespaces:
  - team-a
resourcesProfile: large   # optional: 2–4 GiB for large inventories
```

Install the chart once per cluster; configure sink endpoints to point at the shared backend.
There is **no hub/spoke Helm mode** and no `KollectRemoteCluster` CR.

### Connection test (sink CRs)

Production sink manifests should use **`spec.connectionTest: false`** (default) and trigger probes with
the **`kollect.dev/test-connection: "true"`** annotation when needed ([ADR-0403](../../docs/adr/0403-connection-test.md)).
CI/samples may set `connectionTest: true`.

## Prometheus Operator monitoring

Requires Prometheus Operator CRDs (e.g. **kube-prometheus-stack**). Metrics scrape and alerts are **off by default**
— enable when your cluster runs the operator.

```yaml
metrics:
  enabled: true
  secure: true
  serviceMonitor:
    enabled: true
    interval: 30s
    scrapeTimeout: 10s
    labels:
      release: kube-prometheus-stack   # must match Prometheus serviceMonitorSelector
  prometheusRule:
    enabled: true
    labels:
      release: kube-prometheus-stack   # must match Prometheus ruleSelector
    additionalRules: []                # optional extra rules in kollect.rules group
```

The `ServiceMonitor` targets Service `<release>-kollect-controller-manager` port **`metrics`**
(HTTPS with bearer token when `metrics.secure: true`). Bind a **metrics-reader** `ClusterRole`
to your Prometheus service account so SAR succeeds on `/metrics`.

Starter alerts (group `kollect.rules`): reconcile errors, inventory export errors, sink export
failures, connection test failures, high export latency, workqueue backlog
(all expressions use `kollect_*` metrics only). See [operator metrics reference](../../docs/operator-manual/metrics.md).

CI overlay: [`ci/monitoring-values.yaml`](ci/monitoring-values.yaml).

## Inventory HTTP authentication

When `featureGates.inventoryHttp.enabled` is `true`, the operator serves a read-only inventory API.
Production auth uses **Kubernetes-native delegation** — not a custom API-key scheme.

### Primary: Kubernetes bearer token auth (default)

- Clients send **`Authorization: Bearer <token>`** with a valid Kubernetes service account token
  (or other token accepted by the apiserver).
- The operator validates the token via **`TokenReview`** and checks permission via
  **`SubjectAccessReview`** against inventory read RBAC.
- Manager flag: **`--inventory-auth-mode=kubernetes`** (default when HTTP is enabled).
- Grant consumers a Role/ClusterRole that allows reading inventory in their namespace scope; bind to
  the caller's ServiceAccount.

Example (conceptual — exact SAR resource TBD in implementation):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kollect-inventory-reader
  namespace: team-a
rules:
  - apiGroups: ["kollect.dev"]
    resources: ["kollectinventories"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: portal-reader
  namespace: team-a
subjects:
  - kind: ServiceAccount
    name: portal
    namespace: team-a
roleRef:
  kind: Role
  name: kollect-inventory-reader
  apiGroup: rbac.authorization.k8s.io
```

Automated clients (portals, CI, other operators) should use projected service account tokens and
call the operator Service directly on the inventory port.

### Optional: oauth2-proxy sidecar (OIDC / browser)

For **human browser access** via an identity provider (OIDC), enable the documented optional
oauth2-proxy sidecar pattern:

```yaml
featureGates:
  inventoryHttp:
    enabled: true
    port: 8082

oauth2Proxy:
  enabled: true
  # provider URL, client ID, cookie secret — see values.yaml when implemented
```

- **`oauth2Proxy.enabled: false` by default** — no extra container unless you opt in.
- When enabled, oauth2-proxy terminates OIDC login and forwards authenticated requests to the
  operator inventory port (typically via Ingress).
- **Service-to-service callers should not route through oauth2-proxy** — use bearer tokens against
  the operator Service directly.
- Sidecar implementation is reserved for when the HTTP API ships; values and this README document
  the intended pattern per [ADR-0404](../../docs/adr/0404-inventory-api-auth.md).

### Local development

For kind smoke tests and local debugging only, `--inventory-auth-mode=disabled` skips auth
(startup warning logged). Do not use in production.

## See also

- [ADR-0404: Inventory HTTP API authentication](../../docs/adr/0404-inventory-api-auth.md)
- [ADR-0103: Data storage and etcd limit](../../docs/adr/0103-etcd-limit.md)
- [ADR-0704: Helm chart and CRD lifecycle](../../docs/adr/0704-helm-chart-crd-lifecycle.md)

---

Autogenerated from [`values.yaml`](values.yaml) via [helm-docs](https://github.com/norwoodj/helm-docs).
Edit [`README.md.gotmpl`](README.md.gotmpl) for narrative sections; run `task helm-docs` after changing values comments.
