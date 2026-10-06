# KollectSnapshotSink

**Scope:** Namespace · **Reconciled:** Yes (connection test) · **Short name:** `ksnap`

Platform-shared backends: publish a `KollectSnapshotSink` in `kollect-system` and reference it from a
`KollectClusterInventory` sink ref by `name` + `namespace` — there is no cluster-scoped sink kind
([ADR-0208](../adr/0208-cluster-static-refs-via-namespace.md)).

## What it is for

A `KollectSnapshotSink` configures **snapshot-store** export backends — Git, GitLab, S3, and GCS
([ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md)). Inventories reference
snapshot sinks via `KollectInventory.spec.snapshotSinkRefs`.

## Spec highlights

| Field | Purpose |
| --- | --- |
| `spec.type` | Backend: `git`, `gitlab`, `s3`, `gcs` |
| `spec.endpoint` | Repository URL or bucket URI |
| `spec.git` / `spec.gitlab` / `spec.objectStore` | Type-specific settings |
| `spec.git.engine` | Git export backend: `go-git` (default, pure Go) or `cli` (native `git` binary). `cli` is required for some SSH/KEX edge cases; shipped operator image includes `git` and `openssh-client` |
| `spec.serialization.format` | Output format. **Git/GitLab: `yaml` (default), `json`, or `ndjson`**; **S3/GCS: `json` (default), `parquet`, or `csv`** ([ADR-0419](../adr/0419-git-export-serialization-layout.md), [ADR-0416](../adr/0416-sink-config-layering.md)) |
| `spec.pathTemplate` | Inventory document path; `{extension}` resolves from the format (e.g. `.yaml`) |
| `spec.layout` | **Git/GitLab only** — document shape and folder layout (`document`/`perResource`/`split`) ([ADR-0419](../adr/0419-git-export-serialization-layout.md)) |
| `spec.exportMinInterval` | Default per-ref debounce when inventory ref omits override |
| `spec.connectionTest` | Automatic probe on create/update (default `true`) |
| `spec.deletionPolicy` | What inventory deletion does to this sink's exported objects: `Retain` (default) leaves them, `Delete` retracts them ([ADR-0421](../adr/0421-snapshot-sink-deletion-policy.md)) |

## Example

A Git snapshot store with per-cluster path partitioning
([`config/samples/advanced/kollect_v1alpha1_kollectsnapshotsink.yaml`](https://github.com/platformrelay/kollect/blob/main/config/samples/advanced/kollect_v1alpha1_kollectsnapshotsink.yaml)):

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectSnapshotSink
metadata:
  name: git-inventory-demo
  namespace: default
spec:
  type: git
  endpoint: https://github.com/konih/kollect-inventory-demo.git
  # Fleet: partition paths per cluster to reduce repo lock contention (ADR-0407, ADR-0501).
  pathTemplate: clusters/{cluster}/inventory/{namespace}/{name}.json
  cluster: lab-west
  connectionTest: true
  git:
    branch: main
    pushPolicy: Commit
    auth:
      type: token
  # secretRef:                  # required for private repos
  #   name: git-push-credentials
```

> **Secret references are namespace-scoped (K-04).** A `secretRef` (and `caSecretRef`, `git.auth.secretRef`,
> per-backend `databaseRef`/`secretRef`) must name a Secret in the sink's **own** namespace. Admission
> rejects a reference to any other namespace unless that namespace is in the operator's process-wide
> `allowSecretRefNamespaces` allowlist (`--allow-secret-ref-namespaces`); the allowlist is a
> cluster-admin install setting, never a CRD field. An empty `namespace` means the sink's own namespace.

S3 object-store variant:
[`config/samples/advanced/kollect_v1alpha1_kollectsnapshotsink_s3.yaml`](https://github.com/platformrelay/kollect/blob/main/config/samples/advanced/kollect_v1alpha1_kollectsnapshotsink_s3.yaml).

`azureblob` and `http` remain reserved API constants but are **not accepted by admission** and have
no shipped backend. Parquet is not a separate sink type: select `s3` or `gcs` and set
`spec.serialization.format: parquet`.

## Git serialization & layout (ADR-0419)

Git and GitLab sinks need only `type` + `endpoint` to produce a **human-readable YAML inventory**.
The canonical in-memory snapshot stays JSON-normalized; YAML and folder layout are applied only at
Git write time ([ADR-0419](../adr/0419-git-export-serialization-layout.md)).

Defaults (Git/GitLab) — every row is optional:

| Field | Default | Effect |
| --- | --- | --- |
| `serialization.format` | `yaml` | One inventory file as a YAML list of `Item` rows (set `json` to pin pre-0419 behaviour) |
| `pathTemplate` | `inventory/{namespace}/{name}{extension}` | `{extension}` follows the format → `inventory/team-a/api.yaml` |
| `layout.mode` | `document` | Single inventory file; `perResource` = one file per `Item`; `split` = index + tree |
| `layout.content` | `item` | `attributes` = attribute map only; `manifest` = native object (auto with `export.mode: Resource`) |
| `layout.pathTemplate` | `{cluster}/{sourceNamespace}/{kind}/{sourceName}{extension}` | Per-resource path (modes `perResource`/`split`) |
| `git.prune` | `false` in `document`; `true` in `perResource`/`split` | Stale files removed automatically in tree layouts |

When the referenced profile uses **`export.mode: Resource`** ([ADR-0306](../adr/0306-full-resource-export-pruning.md)),
a zero-field Git sink auto-upgrades to a `perResource` manifest tree (`content: manifest`, pruning on).
Set `layout.mode: document` explicitly to keep a single inventory file.

Minimal per-resource tree (one field beyond `type`/`endpoint`):

```yaml
# kollect-doc: fragment KollectSnapshotSink
spec:
  type: git
  endpoint: https://git.example.com/platform/inventory.git
  cluster: prod-west
  layout:
    mode: perResource
```

produces (default path template):

```text
prod-west/team-a/deployment/api.yaml
prod-west/team-a/deployment/web.yaml
```

See samples
[`..._git_layout.yaml`](https://github.com/platformrelay/kollect/blob/main/config/samples/advanced/kollect_v1alpha1_kollectsnapshotsink_git_layout.yaml)
(explicit override) and
[`..._git_resource_tree.yaml`](https://github.com/platformrelay/kollect/blob/main/config/samples/advanced/kollect_v1alpha1_kollectsnapshotsink_git_resource_tree.yaml)
(Resource-mode tree). Cross-refs: [ADR-0407](../adr/0407-git-object-store-layout.md),
[ADR-0415](../adr/0415-git-sink-commit-ergonomics.md), [ADR-0416](../adr/0416-sink-config-layering.md).

## Inventory deletion (`spec.deletionPolicy`)

When a `KollectInventory` or `KollectClusterInventory` bound to this sink is deleted,
`spec.deletionPolicy` decides what happens to the objects the sink holds for it
([ADR-0421](../adr/0421-snapshot-sink-deletion-policy.md)). On git and gitlab sinks both policies
also release the inventory's ownership record `.kollect-prune/<sha256(owner)>.json`
([ADR-0422](../adr/0422-inventory-export-identity.md)), so another inventory can take over its
paths. The finalizer releases once the cleanup ran or was announced as retained; a terminal backend
failure keeps it (`CleanupTerminal`, re-checked every 5 minutes) until the sink
is fixed or `kollect.dev/force-cleanup: "true"` is set, and a transient failure retries:

| Policy | Effect on deletion | Event |
| --- | --- | --- |
| `Retain` (default) | Exported objects are left in place. S3/GCS are not contacted, so broken credentials cannot block deletion; git/gitlab get one commit that removes only the inventory's ownership record (none when it has no record), so a broken git credential keeps the finalizer until fixed | `Normal` `CleanupRetainedByPolicy` |
| `Delete` | Retracts the inventory's export: git/gitlab deletion commit (on the merge-request feature branch in `branchMR` mode) that also removes every file the inventory's ownership record lists and the record itself, never a file another inventory's record lists; S3/GCS object deletion — the document, its `.part-NNNN-of-NNNN` siblings, layout sidecars, and for parquet the inventory's hive partitions including multipart ones | none when the retraction is provably complete; `Warning` `CleanupRetained` when it cannot be proven (layout trees, `{generation}` templates, an unmerged deletion MR, a changed `pathTemplate`/format, exports recorded before `lastExportPaths` existed) |

```yaml
# kollect-doc: fragment KollectSnapshotSink
spec:
  type: s3
  endpoint: s3://inventory-bucket/kollect
  deletionPolicy: Delete   # opt in: retract this inventory's objects on deletion
```

With `Delete`, a `KollectClusterInventory` `X` and a `KollectInventory` `X` in a namespace called
`cluster` share the export identity `inventory/cluster/X`; deleting either skips the retraction while
the other exists and records `CleanupSharedIdentity`.

## Status

`status.conditions` includes `ConnectionVerified` after the family sink reconciler runs an optional
connectivity probe ([ADR-0403](../adr/0403-connection-test.md)).

### Preview (`status.preview`)

Annotate a sink with `kollect.dev/preview: "true"` to render a side-effect-free preview under
`status.preview` ([ADR-0416](../adr/0416-sink-config-layering.md) §8): the resolved object path and,
for `git`/`gitlab`, a sample commit subject and body rendered from the configured templates plus the
resolved `status.preview.layout` (mode, content, prune, and sample resource paths —
[ADR-0419](../adr/0419-git-export-serialization-layout.md)). Removing the annotation clears
`status.preview`.

See [ADR-0414](../adr/0414-sink-family-crds.md) for the family CRD model, and
[ADR-0415](../adr/0415-git-sink-commit-ergonomics.md) for commit message ergonomics.
