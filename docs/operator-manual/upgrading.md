# Upgrading Kollect

Production upgrade path for **Kollect** when moving between chart or image versions. For first-time
install, see [Install](../operator-manual/index.md#install) in the operator manual.

!!! tip "Assumptions"
    This guide assumes Helm 3, kubectl, and an existing **Kollect** release. New to CRD lifecycle or
    the two-artifact install model? Read [Understand the basics](../concepts/resource-model.md) and
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

Download `install-crds.yaml` from the target [GitHub Release](https://github.com/platformrelay/kollect/releases)
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
helm upgrade kollect oci://ghcr.io/platformrelay/charts/kollect \
  --version <chart-version> \
  -n kollect-system \
  -f values.yaml
```

!!! warning "Upgrading an install made from the old chart coordinate"
    Charts released before ADR-0709 came from `ghcr.io/platformrelay/kollect` (no `charts/`
    segment), the path that also serves the controller image; Artifact Hub indexes one chart per
    repository, so the chart moved to `charts/kollect` while the image stayed put
    ([ADR-0709](../adr/0709-chart-image-oci-path-separation.md)). Upgrading across the move is an
    ordinary `helm upgrade` — same release name, same chart name, no resource churn — because the
    chart history was copied and both paths serve byte-identical manifests at identical digests, so
    **existing installs keep working**. What the old path will not receive is *new versions*:
    repoint every pinned or automated reference (GitOps `HelmRelease`/`Application` sources,
    Renovate/Dependabot rules, CI `helm pull`) or those installs quietly stop tracking releases.
    `image.repository` stays `ghcr.io/platformrelay/kollect` — the image did not move, and changing
    it would break the pull.

**Raw manifests:**

```sh
kubectl apply -f install.yaml
```

Pin `image.tag` to a specific release (or use the release-pinned `install.yaml`) in production. The
chart default resolves to `v<appVersion>` — the image shipped with that chart version — rather than a
floating `latest` tag.

!!! warning "Upgrade chart and image together"
    RBAC is Helm-managed, so a pinned `image.tag` newer than the installed chart runs against the
    older ClusterRole. Releases after **v0.18.0** add `kollectclusterscopes` `get`/`list`/`watch` to
    the manager ClusterRole and make the controller *watch* that type. A manager that cannot watch a
    type it registered fails its cache sync and exits, so pairing the new image with the old
    ClusterRole crash-loops the operator instead of degrading one controller. Bump the chart in the
    same change as the image — or, on the raw-manifest path, re-apply `install.yaml` from the same
    release.

### 4. Wait for rollout

```sh
kubectl -n kollect-system rollout status deployment/kollect-controller-manager --timeout=300s
```

Confirm validating webhooks are **Ready** if `webhooks.enabled: true` (default). cert-manager must
have issued or rotated the serving certificate ([ADR-0105](../adr/0105-webhook-serving-cert-management.md)).

## Values and behaviour changes

Review [Helm values](helm-values.md) and the [chart README](https://github.com/platformrelay/kollect/blob/main/charts/kollect/README.md) when
bumping versions. Common upgrade touchpoints:

| Area | Check |
| --- | --- |
| `tenantMode` / `watchNamespaces` | RBAC shape changes require reconciling Role vs ClusterRole |
| `mode` | Single-cluster only; remove legacy hub/spoke values from overlays |
| `featureGates.*` | New gates default off; dev overlays may differ from production values |
| `webhooks.certManager.create` | `false` requires an operator-provided serving Secret and webhook CA trust; the chart does not generate either |

### Sink CA trust (after v0.21.0)

S3, GCS, Postgres and MongoDB sinks now use `spec.tls.caBundle` / `caSecretRef`, trusted in
addition to the system roots. Before, they ignored it. A CA that cannot take effect now fails the
sink instead of being ignored, and two CA errors apply to every sink type.

| Before (v0.21.x) | After |
| --- | --- |
| CA ignored; a private-CA server fails verification (S3, GCS, MongoDB with TLS, Postgres `verify-ca`/`verify-full`) | Verified against the system roots plus the CA |
| MongoDB `tlsCAFile`, TLS off, or `tlsInsecure`/`sslInsecure` with a CA: the URI's settings used | Terminal error |
| Postgres `sslmode` unset in the DSN, the service-file entry and `PGSSLMODE`, with a CA (pgx default `prefer`): TLS without verification | `verify-full`: chain and hostname verified; a DSN host that does not match the certificate SAN fails. Set `sslmode=verify-ca` explicitly to check the chain only |
| Postgres explicit `sslmode=require` with a CA: TLS without verification | Chain verified (libpq `verify-ca` rule) |
| Postgres explicit `sslmode=prefer`/`allow`/`disable` with a CA: CA ignored | Terminal error |
| `sslrootcert` (Postgres) or `tlsCAFile` (MongoDB) in the connection string with a CA: the connection string's CA used | Terminal error |
| S3/GCS `http` endpoint with a CA: CA ignored | Terminal error |
| `caSecretRef` Secret with none of `tls.crt`, `ca.crt`, `ca.pem`: no CA (system roots for Git, GitLab, S3, GCS; plaintext for Kafka and NATS without other TLS settings; the connection string's TLS for Postgres and MongoDB) | Terminal error, **every sink type** |
| `caBundle` and `caSecretRef` both set: `caBundle` used (Git and GitLab already rejected it) | Terminal error, **every sink type** |

To find affected sinks before upgrading, list every sink that sets `spec.tls`:

```sh
kubectl get kollectsnapshotsinks,kollectdatabasesinks,kollecteventsinks -A -o json \
  | jq -r '.items[] | select(.spec.tls) | "\(.kind) \(.metadata.namespace)/\(.metadata.name)"'
```

For each, check that a `caSecretRef` Secret holds one of the three keys, that only one CA source
is set, and, for Postgres and MongoDB, the connection string against the rules on the
[database sink page](../crds/kollectdatabasesink.md#tls-trust-spectls). After the upgrade, a
failing sink shows the terminal error on its status and connection test.

A failed backend build is not cached, but no controller watches the CA Secret and a terminal
error does not requeue. After fixing a Secret that failed the sink, change any annotation on
the sink, for example `kubectl annotate <sink kind> <name> ca-fixed-at="$(date +%s)" --overwrite`:
the probe runs again, and the inventories that reference the sink reconcile and build its
backend on their next export.

Rollback to v0.21.x restores the old behaviour; nothing is persisted.

### Sink Secret rotation (after v0.21.0)

From v0.22.0 the operator picks up a changed sink Secret without a restart. Each export, and
each cleanup that builds the backend, reads the metadata of every Secret the sink references (credential, database and CA
Secrets) from the API server, not from the informer cache. When one changed since the backend was
built, that export builds a new backend from the current values. Exports already running finish
on the old backend, which is closed after the last of them ends. A deleted or emptied Secret
now fails the export instead of being ignored.

| Before (v0.21.x) | After (v0.22.0) |
| --- | --- |
| Rotated credential or CA Secret: old value used until spec change, sink delete, restart or 48 h idle | New value used from the next export |
| Deleted referenced Secret: exports keep working on the cached backend | Next export fails (retryable), naming the Secret |
| Entirely empty referenced Secret: same as above; S3/GCS fall back to the pod's ambient AWS credentials on the next build | Next export fails (retryable), naming the Secret. A Secret that has other keys but no access key still falls back, as before |
| Cross-namespace `secretRef` under `--watch-namespaces`: build fails ("unknown namespace for the cache") | Export path reads through the API reader and works; the connection probe still reads through the cache |
| Git sink, credentials rotated away, inventory deleted with Retain: release used cached credentials | Release fails and the inventory stays `Terminating` until the Secret is fixed or `kollect.dev/force-cleanup: "true"` is set |
| Kafka/NATS sink with a deleted Secret, inventory deleted | Unchanged: retained without contacting the sink |
| `caSecretRef` Secret emptied entirely: terminal "none of the CA keys" error on the next build | Retryable "has no data" error on the next export (a Secret with other keys but no CA key keeps the terminal error) |
| Spec change while another export used the backend: backend closed under it | Closed after that export releases it |
| Inventory with `exportMinInterval: 0s`, every sink export fails transiently: no timed retry | Retried every 30 s (`ZeroIntervalWatchdog`) until it succeeds; a successful, unchanged `0s` inventory is still not re-exported, and a Secret change alone still does not trigger an export |

What to know before upgrading:

- **When a rotation takes effect.** A Secret change alone never triggers an export. With an
  export interval above `0s` the next export, and so the new value, comes within the interval.
  With `exportMinInterval: 0s` ("export only on material change") it comes with the next material
  change, spec generation change or `kollect.dev/requestedAt` change.
- **Empty means entirely empty.** A Secret counts as emptied when its data holds no non-empty
  value. Only then does S3 or GCS stop falling back to ambient credentials. Check that no S3 or GCS
  sink relies on an empty `secretRef` Secret for ambient credentials; point those at a Secret with
  keys, or remove the `secretRef`.
- **Errors carry no values.** A deleted or emptied Secret gives
  `secret "<name>" not found in namespace "<namespace>"` or `… has no data`, on status, Events
  and logs. `kollect-pipeline` prints the same "has no data" text for an empty Secret manifest
  before it builds a backend.
- **Cleanup.** Kafka and NATS sinks retain on inventory deletion without reading the Secret.
  Git, GitLab, database sinks and S3/GCS cleanup that retracts objects (`deletionPolicy: Delete`,
  identity not shared) need the Secret to clean up: restore it, or set `kollect.dev/force-cleanup: "true"` on the deleting inventory and accept
  that the exported objects stay.
- **Terminal errors still need a nudge.** No controller watches Secrets and a terminal error does
  not requeue. After fixing a Secret behind a terminal error (a Git `401`/`403`, a CA Secret with
  data but no CA key), change any annotation on the sink as in
  [Sink CA trust](#sink-ca-trust-after-v0210).
- **No RBAC change.** The metadata read uses the existing `get` on `secrets`.

Rollback to v0.21.x restores the old behaviour; nothing is persisted.

!!! info "Export debouncing"
    Debounce interval is per **`KollectInventory.spec.exportMinInterval`** (CRD default **30s**).

### Cluster-scope GVK enforcement (after v0.18.0)

Releases after **v0.18.0** enforce [`KollectClusterScope`](../crds/kollectclusterscope.md)
`allowedGVKs` during reconcile, not only at admission — the backstop
[ADR-0207](../adr/0207-target-collection-filtering.md) always specified.

!!! warning "Existing cluster targets can stop collecting"
    A [`KollectClusterTarget`](../crds/kollectclustertarget.md) whose profile `targetGVK` or
    `resourceRules` GVK sits outside a non-empty `allowedGVKs` now unregisters its informers and goes
    `Degraded=True` / `reason=ScopeGVKDenied` on the first reconcile after upgrade. Targets admitted
    **before** the ceiling was created or tightened are the affected set — admission only ran when
    they were last written.

Audit before upgrading, on each cluster that has a `KollectClusterScope`:

```sh
kubectl get kollectclusterscopes.kollect.dev -o yaml | grep -A4 allowedGVKs
kubectl get kollectclustertargets.kollect.dev \
  -o custom-columns='NAME:.metadata.name,PROFILE:.spec.profileRef.name,PROFILE_NS:.spec.profileRef.namespace'
```

Cross-check each target's profile `targetGVK` (plus any `spec.resourceRules[].gvk`) against
`allowedGVKs`. Remediate by widening `allowedGVKs`, repointing `profileRef`, or retiring the target.
After upgrading, the affected targets are listed by:

```sh
kubectl get kollectclustertargets.kollect.dev -o custom-columns=\
'NAME:.metadata.name,DEGRADED:.status.conditions[?(@.type=="Degraded")].status,REASON:.status.conditions[?(@.type=="Degraded")].reason'
```

Widening the ceiling clears the condition on the next reconcile; nothing needs to be recreated.

### Insecure TLS opt-in and Git redirects (after v0.20.0)

Releases after **v0.20.0** make `spec.tls.insecureSkipVerify` a cluster-admin opt-in (K-14) and stop
Git remotes from following HTTP redirects (K-15).

!!! warning "Existing insecure sinks stop exporting"
    Without the manager flag `--allow-insecure-sinks` (Helm value `allowInsecureSinks`, default
    `false`), admission rejects any family sink that sets `spec.tls.insecureSkipVerify`, on create
    and on update, and the Git, GitLab, Kafka and NATS sinks refuse it at construction with
    `tls.insecureSkipVerify is not permitted`. Sinks that already set the field stop connecting and
    exporting on the first reconcile after upgrade. Deleting a sink, or an update that removes the
    field, is still accepted.

Audit before upgrading:

```sh
for kind in kollectsnapshotsinks kollectdatabasesinks kollecteventsinks; do
  kubectl get "${kind}.kollect.dev" -A \
    -o jsonpath='{range .items[?(@.spec.tls.insecureSkipVerify==true)]}{.kind}/{.metadata.namespace}/{.metadata.name}{"\n"}{end}'
done
```

Remediate by supplying the server CA (`spec.tls.caBundle` or `spec.tls.caSecretRef`) and removing
the field, or, for development clusters only, set `allowInsecureSinks: true`. Once enabled, every
family sink using the field carries `TLSInsecure=True`, whether or not its connection test runs.

Git remotes that only work through an HTTP redirect (an `http://` URL upgraded to `https://`, a
moved or renamed project) now fail; set `spec.endpoint` to the final URL. Kafka sinks now honour
`spec.tls`: a sink with a CA bundle switches its broker connection to TLS, so make sure the broker
listener it points at speaks TLS.

### Git export ownership by inventory identity (after v0.21.0)

Git and GitLab layout exports record which inventory owns each file, keyed by kind, cluster,
namespace and name ([ADR-0422](../adr/0422-inventory-export-identity.md)). Cluster inventories
prune their own stale files again, and a cluster inventory and a namespaced inventory in a
namespace called `cluster` never delete each other's files.

!!! warning "Inventories that project the same file are rejected"
    The default resource path (`{cluster}/{sourceNamespace}/{kind}/{sourceName}`) names no
    inventory. When a namespaced inventory and a cluster inventory export the same resource to the
    same Git sink, whichever exports second fails with
    `prune path "<path>" belongs to another inventory: <Kind namespace/name> ...`, naming the owner and
    its record file. Give overlapping inventories separate sinks, or a `spec.layout.pathTemplate`
    that includes the inventory (`{namespace}` and `{name}` are the inventory's, for example
    `{namespace}/{name}/{sourceNamespace}/{kind}/{sourceName}{extension}`), or narrow their
    selection so they do not overlap. A cluster inventory `X` and a namespaced inventory `X` in
    namespace `cluster` render the same `{namespace}/{name}` (`cluster/X`), so give those two
    separate sinks.

In GitLab `merge_request` mode an export is also checked against the target branch's ownership
records, and fails with `... belongs to another inventory on merge target branch "<target>" ...`
when another inventory's merged record claims one of its paths. A cluster inventory's merge requests
now use the branch `<prefix>/_cluster/<name>` instead of `<prefix>/cluster/<name>`, so they no longer
share a branch with a namespaced inventory of the same name in namespace `cluster`. Close merge
requests left open on the old branch and delete the branch: it may still hold the cluster
inventory's record, which would block that namespaced inventory's exports. If a target branch already holds two records claiming one path,
every export to it fails with `duplicate ownership of "<path>": ...`, naming both inventories and
both record files: delete one of the two records.

Ownership records written by builds before this change are not migrated. An inventory whose paths
are listed in an old record is rejected as if another inventory owned them, so delete the
`.kollect-prune/` directory from the export branch when upgrading; the next export records the
current files again.

Deleting an inventory releases its ownership record on every Git and GitLab sink, under both
deletion policies ([ADR-0421](../adr/0421-snapshot-sink-deletion-policy.md)). With `Retain` the
exported files stay and one commit removes only the record; with `Delete` the same commit also
removes the files the record lists. Another inventory can then take over those paths. Because the
release needs the repository, a Git or GitLab sink with a broken credential now keeps a `Retain`
deletion in `Terminating` until the credential is fixed or `kollect.dev/force-cleanup: "true"` is
set (which leaves the record in place).

### Git engine convergence: `engine: cli` removed (after v0.21.0)

The git snapshot sink exports through one engine, `go-git`
([ADR-0803](../adr/0803-git-engine-convergence.md)). `spec.git.engine` accepts only `go-git` (or
omission, the default): the CRD schema, admission validation and backend construction all reject
`cli`. `file://` remotes and `git ls-remote` connection probes keep using the git CLI machinery
they always used, so the operator image keeps `git` and `openssh-client`.

!!! warning "Persisted `engine: cli` sinks stop exporting"
    An existing `KollectSnapshotSink` object that still carries `spec.git.engine: cli` is **not**
    deleted or rewritten by the operator, and admission does not re-run on stored objects. At that
    sink's next export build the construction fails terminally with an error naming `go-git`, and
    the sink reports the failure through its conditions.

Audit before upgrading:

```sh
kubectl get kollectsnapshotsinks.kollect.dev -A \
  -o jsonpath='{range .items[?(@.spec.git.engine=="cli")]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}'
```

Remediate by removing `spec.git.engine` from the sink spec (or setting `go-git`). The go-git
engine serves HTTPS token auth and SSH key auth through the same `spec.auth` modes as before, but
SSH host-key verification differs: the go-git path fails closed without a `known_hosts` key in the
sink's credential secret (or `tls.insecureSkipVerify`, dev only), where the CLI path left host-key
policy to the ambient ssh configuration. SSH key-exchange preferences are otherwise unchanged
apart from two additions x/crypto already implements (`mlkem768x25519-sha256`,
`diffie-hellman-group16-sha512`).

## GitOps and CI/CD

For Argo CD, Flux, or similar:

1. Commit or sync **`install-crds.yaml`** in a separate wave or Job **before** the Helm release.
2. Keep CRD manifests out of the same Helm hook that upgrades the Deployment unless you accept
   Helm's CRD non-upgrade semantics.
3. Pin chart `version` and image digest in values; use OCI
   `oci://ghcr.io/platformrelay/charts/kollect` with an immutable tag. If a `HelmRelease` or
   `Application` still points at the old `ghcr.io/platformrelay/kollect` chart path (no `charts/`
   segment), repoint it — that path no longer receives new chart versions
   ([ADR-0709](../adr/0709-chart-image-oci-path-separation.md)).

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

- [Operator manual](../operator-manual/index.md) · [Helm values](helm-values.md)
- [Release process](../RELEASE.md) — artifacts and tagging
- [ADR-0704: Helm chart and CRD lifecycle](../adr/0704-helm-chart-crd-lifecycle.md)
- [Cert-manager webhooks example](index.md#webhook-tls)
