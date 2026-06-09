# Upgrading Kollect

Production upgrade path for **Kollect** when moving between chart or image versions. For first-time
install, see [Install](../OPERATOR-MANUAL.md#install) in the operator manual.

!!! tip "Assumptions"
    This guide assumes Helm 3, kubectl, and an existing **Kollect** release. New to CRD lifecycle or
    the two-artifact install model? Read [Understand the basics](../UNDERSTAND-THE-BASICS.md) and
    [ADR-0704: Helm chart and CRD lifecycle](../adr/0704-helm-chart-crd-lifecycle.md) first.

!!! warning "Pre-beta API"
    `v1alpha1` fields and defaults may change until the first release candidate. Check
    [ROADMAP](../ROADMAP.md) before production rollout.

## Why two steps

Helm **installs** CRDs from `charts/kollect/crds/` on first install but **does not upgrade or delete**
them on `helm upgrade`. **Kollect** accepts this deliberately ([ADR-0704](../adr/0704-helm-chart-crd-lifecycle.md)):

| Artifact | Lifecycle | Tooling |
| --- | --- | --- |
| CRD schema | Apply-only, never deleted | `kubectl apply -f dist/install-crds.yaml` |
| Operator (Deployment, RBAC, webhooks) | Helm-managed | `helm upgrade` |

!!! warning "Never delete CRDs"
    Deleting a CRD garbage-collects all custom resources of that kind. CRD upgrades are **apply-only**;
    release tooling and runbooks must never remove them.

!!! note "Two install artifacts"
    Each GitHub Release publishes `install-crds.yaml` (schema) and `install.yaml` (full operator manifest)
    plus the OCI Helm chart — see [Release process](../RELEASE.md).

## Standard upgrade procedure

Apply CRD schema **before** upgrading the operator Deployment so the manager and apiserver agree on
stored object shape.

### 1. Fetch release assets

Download `install-crds.yaml` from the target [GitHub Release](https://github.com/konih/kollect/releases)
or build locally:

```sh
VERSION=0.1.0 task release-dry-run
```

Verify image digest or tag with cosign when adopting from GHCR ([ADR-0705](../adr/0705-release-supply-chain.md)).

### 2. Apply CRD schema

```sh
kubectl apply -f install-crds.yaml
```

`kubectl apply` is idempotent. Review server-side apply conflicts if you customized CRD annotations.

!!! warning "Cluster-scoped CRD changes"
    Schema changes that affect stored versions may require apiserver conversion webhooks or manual
    field migration. Read release notes and [ADR-0206](../adr/0206-api-versioning-conversion.md) before
    skipping minor bumps.

### 3. Upgrade the operator

**Chart from repository:**

```sh
helm upgrade kollect ./charts/kollect -n kollect-system -f values.yaml
```

**OCI chart (GHCR):**

```sh
# pin the target release version, e.g. --version 0.5.0
helm upgrade kollect oci://ghcr.io/konih/kollect \
  --version <chart-version> \
  -n kollect-system \
  -f values.yaml
```

**Raw manifests:**

```sh
kubectl apply -f install.yaml
```

Pin `image.tag` (or use the release-pinned `install.yaml`) in production — do not rely on `latest`.

### 4. Wait for rollout

```sh
kubectl -n kollect-system rollout status deployment/kollect-controller-manager --timeout=300s
```

Confirm validating webhooks are **Ready** if `webhooks.enabled: true` (default). cert-manager must
have issued or rotated the serving certificate ([ADR-0105](../adr/0105-webhook-serving-cert-management.md)).

## Values and behaviour changes

Review [Helm values](helm-values.md) and the [chart README](../../charts/kollect/README.md) when
bumping versions. Common upgrade touchpoints:

| Area | Check |
| --- | --- |
| `tenantMode` / `watchNamespaces` | RBAC shape changes require reconciling Role vs ClusterRole |
| `mode` | Single-cluster only; remove legacy hub/spoke values from overlays |
| `featureGates.*` | New gates default off; dev overlays may differ from production values |
| `webhooks.certManager.create` | Clusters without cert-manager need the self-signed bootstrap path |

!!! info "Export debouncing"
    Debounce interval is per **`KollectInventory.spec.exportMinInterval`** (CRD default **30s**).

## GitOps and CI/CD

For Argo CD, Flux, or similar:

1. Commit or sync **`install-crds.yaml`** in a separate wave or Job **before** the Helm release.
2. Keep CRD manifests out of the same Helm hook that upgrades the Deployment unless you accept
   Helm's CRD non-upgrade semantics.
3. Pin chart `version` and image digest in values; use OCI `oci://ghcr.io/konih/kollect` with an
   immutable tag.

!!! note "Open question"
    A guarded `upgradeCRDs` Helm value remains **undecided** ([ADR-0704](../adr/0704-helm-chart-crd-lifecycle.md)).
    Default stays out-of-band `install-crds.yaml` for explicit operator control.

## Rollback

| Layer | Action |
| --- | --- |
| Operator Deployment | `helm rollback kollect <revision>` or re-apply prior `install.yaml` |
| CRD schema | **Do not downgrade** CRDs if new fields were persisted — restore etcd backup or migrate data |
| Custom resources | Unaffected by operator rollback if CRD schema is backward compatible |

If a bad operator image breaks reconciliation, roll back the Deployment first. CRD schema rollback is
a last resort and may require maintenance windows.

## Verify after upgrade

```sh
kubectl get crd | grep kollect.dev
kubectl -n kollect-system get deploy,pod
kubectl get kollectinventories.kollect.dev -A
```

Check `Ready` conditions on sample `KollectInventory` objects and sink export timestamps. In a
multi-cluster fleet, repeat per cluster and confirm rows land in the shared sink under each
`spec.cluster` partition.

## See also

- [Operator manual](../OPERATOR-MANUAL.md) · [Helm values](helm-values.md)
- [Release process](../RELEASE.md) — artifacts and tagging
- [ADR-0704: Helm chart and CRD lifecycle](../adr/0704-helm-chart-crd-lifecycle.md)
- [Cert-manager webhooks example](../examples/cert-manager-webhook.md)
