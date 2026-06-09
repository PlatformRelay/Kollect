# KollectSnapshotSink

**Scope:** Namespace · **Reconciled:** Yes (connection test) · **Short name:** `ksnap`

Cluster-scoped variant: **`KollectClusterSnapshotSink`** (`kcsnap`).

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
| `spec.serialization.format` | Output format. **Git/GitLab default `yaml`**; object stores default `json` ([ADR-0419](../adr/0419-git-export-serialization-layout.md)) |
| `spec.pathTemplate` | Inventory document path; `{extension}` resolves from the format (e.g. `.yaml`) |
| `spec.layout` | **Git/GitLab only** — document shape and folder layout (`document`/`perResource`/`split`) ([ADR-0419](../adr/0419-git-export-serialization-layout.md)) |
| `spec.exportMinInterval` | Default per-ref debounce when inventory ref omits override |
| `spec.connectionTest` | Automatic probe on create/update (default `true`) |

## Example

A Git snapshot store with per-cluster path partitioning
([`config/samples/kollect_v1alpha1_kollectsnapshotsink.yaml`](https://github.com/konih/kollect/blob/main/config/samples/kollect_v1alpha1_kollectsnapshotsink.yaml)):

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
  #   namespace: kollect-system
```

S3 object-store variant:
[`config/samples/kollect_v1alpha1_kollectsnapshotsink_s3.yaml`](https://github.com/konih/kollect/blob/main/config/samples/kollect_v1alpha1_kollectsnapshotsink_s3.yaml).

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
[`..._git_layout.yaml`](https://github.com/konih/kollect/blob/main/config/samples/kollect_v1alpha1_kollectsnapshotsink_git_layout.yaml)
(explicit override) and
[`..._git_resource_tree.yaml`](https://github.com/konih/kollect/blob/main/config/samples/kollect_v1alpha1_kollectsnapshotsink_git_resource_tree.yaml)
(Resource-mode tree). Cross-refs: [ADR-0407](../adr/0407-git-object-store-layout.md),
[ADR-0415](../adr/0415-git-sink-commit-ergonomics.md), [ADR-0416](../adr/0416-sink-config-layering.md).

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
