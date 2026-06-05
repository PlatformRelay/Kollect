# kollect Helm chart

Installs the [kollect](https://github.com/konih/kollect) operator and CRDs.

## Install

```bash
helm install kollect ./charts/kollect -n kollect-system --create-namespace
```

## Values

| Key | Description | Default |
| --- | --- | --- |
| `image.repository` | Controller image | `ghcr.io/konih/kollect` |
| `featureGates.inventoryHttp.enabled` | Expose `GET /inventory` | `false` |
| `oauth2Proxy.enabled` | Optional OIDC sidecar in front of inventory HTTP | `false` |
| `webhooks.enabled` | Validating webhook for profiles | `true` |
| `transport.type` | Hub/spoke transport backend | `inprocess` |
| `sinkDefaults.connectionTest` | Default for sample `KollectSink` probes | `false` (prod); CI/dev overlays use `true` |

See `values.yaml` for the full list.

### Connection test (`KollectSink`)

Production sink manifests should use **`spec.connectionTest: false`** (default) and trigger probes with
the **`kollect.dev/test-connection: "true"`** annotation when needed ([ADR-0030](../docs/adr/0030-connection-test.md)).
CI/samples may set `connectionTest: true`.

### Hub transport

Hub/spoke transport defaults to **`inprocess`** until an external backend passes integration tests.
Do not enable Redis/NATS/Kafka in chart values without explicit ops choice ([ADR-0023](../docs/adr/0023-lean-queue-transport.md)).

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
  the intended pattern per [ADR-0024](../../docs/adr/0024-inventory-api-auth.md).

### Local development

For kind smoke tests and local debugging only, `--inventory-auth-mode=disabled` skips auth
(startup warning logged). Do not use in production.

## See also

- [ADR-0024: Inventory HTTP API authentication](../../docs/adr/0024-inventory-api-auth.md)
- [ADR-0006: Data storage and etcd limit](../../docs/adr/0006-etcd-limit.md)
