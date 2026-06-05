# ADR-0704: Helm chart and CRD lifecycle

> How kollect is packaged and installed: the Helm chart layout, tenant/hub install modes, webhook
> certs, and the CRD upgrade story (Helm's notorious footgun).

**Theme:** 07 · Project & meta · **Status:** Current

## Context

kollect is operator software; **install and upgrade** are first-class product decisions. The chart
(`charts/kollect`) is the primary install path and was built day-1, but the packaging decisions —
especially **CRD lifecycle**, which Helm handles poorly — were never recorded. This ADR captures them.

## Decision

### Chart structure

`charts/kollect` ships:

- **`crds/`** — the seven CRDs, generated from `api/v1alpha1` and kept drift-free by `task verify`.
- **`templates/`** — `deployment`, `serviceaccount`, RBAC (`clusterrole`/`role` + bindings),
  `role-leader-election` (single lease), webhook config + `webhook-service`, `webhook-certmanager`,
  and `manager-service`.
- **`values.schema.json`** — JSON-schema-validated values (fails fast on bad input).
- **`tests/`** + **`ci/`** values — `helm unittest` snapshots for tenant-mode, hub, and webhook wiring.

### Install modes (driven by `values.yaml`)

- **Cluster-wide** (default): `ClusterRole`/`ClusterRoleBinding`, operator watches all namespaces.
- **Tenant mode**: `watchNamespaces` set → namespaced `Role`/`RoleBinding` only, no cluster-wide read
  ([ADR-0203](0203-namespaced-multi-tenancy.md), [ADR-0104](0104-security-model.md)).
- **Hub mode**: optional hub deployment values for the aggregation tier ([ADR-0501](0501-multi-cluster-sync-rfc.md)).
- **Feature gates** — see [Feature gate strategy](#feature-gate-strategy) below.

### Feature gate strategy

Optional HTTP/debug surfaces are **off by default** and controlled from Helm `featureGates.*` → manager
flags. There is **no** Kubernetes upstream `FeatureGate` API — gates are chart values + CLI flags only
(D4: folded here; no standalone ADR).

| Gate | Helm values | Manager flags | Default | ADR |
| --- | --- | --- | --- | --- |
| Inventory HTTP API | `featureGates.inventoryHttp.enabled` | `--inventory-http-enabled`, port, auth mode | **false** | [ADR-0404](0404-inventory-api-auth.md) |
| pprof | `pprof.enabled` | `--enable-pprof`, bind address | **false** | [GUIDELINES.md](https://github.com/konih/kollect/blob/main/GUIDELINES.md) §5 |
| Validating webhooks | `webhooks.enabled` | `--validating-webhooks-enabled` | **true** | [ADR-0105](0105-webhook-serving-cert-management.md) |

**Adding a new gate:** (1) Helm value under `featureGates` or a clearly named top-level block, (2) manager
flag wired in `deployment.yaml`, (3) snapshot in `charts/kollect/tests/`, (4) document here and in the
feature ADR. Production Helm defaults stay conservative; dev/CI overlays (`ci/dev-values.yaml`) may enable
gates for smoke tests.

Future gates (planned): Read API / embedded SPA ([ADR-0408](0408-read-api-ui-architecture.md)), optional
`oauth2Proxy` sidecar ([ADR-0404](0404-inventory-api-auth.md)).

### Webhook certificates

- Default path uses **cert-manager** (`webhook-certmanager.yaml`) for serving + rotation; cert-manager is
  a documented soft dependency for the webhook ([ADR-0201](0201-crd-model.md), [ADR-0105](0105-webhook-serving-cert-management.md)).
- **Fallback:** self-signed bootstrap when `webhooks.certManager.create: false` — see [ADR-0105](0105-webhook-serving-cert-management.md) (Q4).
- The chart's webhook wiring is snapshot-tested (`tests/deployment_webhooks_test.yaml`).

### CRD lifecycle (the explicit decision)

Helm **installs** CRDs from `crds/` but does **not upgrade or delete** them on `helm upgrade`. We accept
this deliberately and provide a **second, authoritative path**:

- The release publishes a standalone **`dist/install-crds.yaml`** ([ADR-0705](0705-release-supply-chain.md)).
- **CRD upgrades are applied explicitly** (`kubectl apply -f install-crds.yaml`) out of band of
  `helm upgrade`, documented in `docs/RELEASE.md` / quickstart.
- CRDs are **never deleted** by tooling (deleting a CRD garbage-collects all CRs).

This avoids the silent "Helm won't update my CRD schema" failure mode and the dangerous "Helm deletes
my CRDs" one.

### Distribution

- Chart and controller image publish to **GHCR as OCI** ([ADR-0705](0705-release-supply-chain.md));
  `helm push`/`helm install oci://…`. No separate chart-repo index to maintain.

## Consequences

- Two intentional install artifacts: Helm (operator + RBAC + webhooks) and a CRD manifest (schema
  lifecycle). Clear, if it requires documenting the two-step upgrade.
- Tenant/hub/feature-gate behavior is values-driven and snapshot-tested, so packaging regressions fail in
  CI ([ADR-0706](0706-testing-merge-gate-architecture.md)).
- cert-manager as the default adds a dependency; self-signed bootstrap is documented in [ADR-0105](0105-webhook-serving-cert-management.md).

## Open questions

- **DECIDED (2026-06-05, Q4):** Self-signed/bootstrap webhook-cert option for clusters without
  cert-manager — see [ADR-0105](0105-webhook-serving-cert-management.md) (cert-manager remains recommended).
- **OPEN:** Should the chart optionally manage CRDs via a templated `crds/` (with a guarded
  `upgradeCRDs` value) to offer a one-command upgrade for non-GitOps users? (Default: keep out-of-band
  `install-crds.yaml`.)
- **OPEN:** Kustomize overlay parity for users who don't use Helm (currently `dist/install.yaml`).
