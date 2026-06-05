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
- **Feature gates**: off-by-default surfaces (e.g. inventory HTTP API — [ADR-0404](0404-inventory-api-auth.md))
  are gated in values.

### Webhook certificates

- Default path uses **cert-manager** (`webhook-certmanager.yaml`) for serving + rotation; cert-manager is
  a documented soft dependency for the webhook ([ADR-0201](0201-crd-model.md)).
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
  CI ([ADR-0706 testing — planned]).
- cert-manager as the default adds a dependency; a self-signed fallback is an open item.

## Open questions

- **DECIDED (2026-06-05):** Add a **self-signed/bootstrap webhook-cert option** for clusters without
  cert-manager (cert-manager stays the recommended default). To be detailed in planned **ADR-0105**
  (webhook serving & cert management).
- **OPEN:** Should the chart optionally manage CRDs via a templated `crds/` (with a guarded
  `upgradeCRDs` value) to offer a one-command upgrade for non-GitOps users? (Default: keep out-of-band
  `install-crds.yaml`.)
- **OPEN:** Kustomize overlay parity for users who don't use Helm (currently `dist/install.yaml`).
