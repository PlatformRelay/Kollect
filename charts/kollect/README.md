# Kollect Helm chart

Installs the [Kollect](https://github.com/konih/kollect) operator and CRDs.

## Install

```bash
helm install kollect ./charts/kollect -n kollect-system --create-namespace
```

## Values

| Key | Description | Default |
| --- | --- | --- |
| `image.repository` | Controller image | `ghcr.io/konih/kollect` |
| `featureGates.inventoryHttp.enabled` | Expose `GET /inventory` (debug/small install only) | `false` |
| `mode` | Operator mode: `single`, `hub`, or `spoke` (same image — no `KollectHub` CRD) | `single` |
| `tenantMode` | Namespaced Role RBAC; per-team install story | `false` (set `true` + `watchNamespaces` for team installs) |
| `watchNamespaces` | Restrict informer cache to these namespaces | `[]` (all) |
| `transport.type` | Hub/spoke transport backend | `inprocess` |

Export debouncing is configured per **`KollectInventory.spec.exportMinInterval`** (CRD default
**30s**). The chart does not pass the deprecated manager `--export-debounce` flag; use the Inventory
CR or `extraArgs` only for local debugging.

See `values.yaml` for the full list. Critical values are validated by
[`values.schema.json`](values.schema.json); CI runs `task helm-test` (`helm lint` + `helm-unittest`).

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

Namespaced `KollectProfile`, `KollectSink`, `KollectTarget`, and `KollectInventory` live in the
team namespace. Portal read path: **Postgres or Kafka sink export** — not spoke HTTP.

### Hub mode (no `KollectHub` CRD)

Multi-cluster hub/spoke uses **Helm values on the same image** — there is **no `KollectHub` CRD**
on the product roadmap ([ADR-0703](../../docs/adr/0703-platform-architecture-pivot.md)). Existing
`KollectHub` was removed from the repo; do not create hub Deployments via CR.

**Spoke cluster:**

```yaml
mode: spoke
transport:
  type: inprocess   # only default until external backend passes integration proof
featureGates:
  inventoryHttp:
    enabled: false
```

Spoke operators collect inventory, export to local Postgres/Kafka (or push summaries to hub ingest
per [ADR-0503](../../docs/adr/0503-hub-cluster-auth-istio-pattern.md)).

**Hub cluster:**

```yaml
mode: hub
replicaCount: 2   # horizontal hub consumers when transport scales out
transport:
  type: inprocess
hub:
  name: platform
  platformNamespace: kollect-system
  exportNamespace: kollect-system
  sinkRefs:
    - hub-postgres
    - hub-kafka
  remoteClusters:
    - spoke-a
    - spoke-b
  ingestPort: 8083
  ingestAuthMode: kubernetes
controller:
  maxConcurrentReconciles:
    hub: 4
featureGates:
  inventoryHttp:
    enabled: false   # hub portal read uses merged Postgres/Kafka later
```

Hub merge runs via `internal/hub/` library + ingest HTTP (`POST /hub/v1alpha1/reports`). Register
spokes with namespaced **`KollectRemoteCluster`** objects ([ADR-0503](../../docs/adr/0503-hub-cluster-auth-istio-pattern.md)).
`hub.sinkRefs` resolve namespaced **`KollectSink`** objects in `hub.exportNamespace`; post-merge
export fans out to **Postgres and Kafka in parallel**. `hub.remoteClusters` sets
`KOLLECT_REMOTE_CLUSTERS` (fail-closed when present, even if empty).

| Hub value | Env | Role |
| --- | --- | --- |
| `hub.exportNamespace` | `KOLLECT_HUB_EXPORT_NAMESPACE` | Namespace for hub sink CRs |
| `hub.sinkRefs[]` | `KOLLECT_HUB_SINK_REFS` | Parallel Postgres/Kafka export targets |
| `hub.remoteClusters[]` | `KOLLECT_REMOTE_CLUSTERS` | Spoke registration allowlist |
| `hub.platformNamespace` | `KOLLECT_PLATFORM_NAMESPACE` | SAR namespace for `kollectremoteclusters` |

| Key | Description | Default |
| --- | --- | --- |
| `oauth2Proxy.enabled` | Optional OIDC sidecar in front of inventory HTTP | `false` |
| `webhooks.enabled` | Validating webhook for profiles | `true` |
| `sinkDefaults.connectionTest` | Default for sample `KollectSink` probes | `false` (prod); CI/dev overlays use `true` |

### Connection test (`KollectSink`)

Production sink manifests should use **`spec.connectionTest: false`** (default) and trigger probes with
the **`kollect.dev/test-connection: "true"`** annotation when needed ([ADR-0403](../../docs/adr/0403-connection-test.md)).
CI/samples may set `connectionTest: true`.

### Hub transport

Hub/spoke transport defaults to **`inprocess`** until an external backend passes integration tests.
Do not enable Redis/NATS/Kafka in chart values without explicit ops choice ([ADR-0502](../../docs/adr/0502-lean-queue-transport.md)).

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
