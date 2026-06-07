# KollectSnapshotSink

**Scope:** Namespace · **Reconciled:** Yes (connection test) · **Short name:** `ksnap`

Cluster-scoped variant: **`KollectClusterSnapshotSink`** (`kcsnap`).

## What it is for

A `KollectSnapshotSink` configures **snapshot-store** export backends — Git, GitLab, S3, GCS, Azure
Blob, and HTTP/webhook ([ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md)). Inventories reference
snapshot sinks via `KollectInventory.spec.snapshotSinkRefs`.

## Spec highlights

| Field | Purpose |
| --- | --- |
| `spec.type` | Backend: `git`, `gitlab`, `s3`, `gcs`, `azureblob`, `http` |
| `spec.endpoint` | Repository URL, bucket URI, or webhook URL |
| `spec.git` / `spec.gitlab` / `spec.objectStore` / `spec.http` | Type-specific settings |
| `spec.git.engine` | Git export backend: `go-git` (default, pure Go) or `cli` (native `git` binary). `cli` is required for some SSH/KEX edge cases; shipped operator image includes `git` and `openssh-client` |
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

## Status

`status.conditions` includes `ConnectionVerified` after the family sink reconciler runs an optional
connectivity probe ([ADR-0403](../adr/0403-connection-test.md)).

### Preview (`status.preview`)

Annotate a sink with `kollect.dev/preview: "true"` to render a side-effect-free preview under
`status.preview` ([ADR-0416](../adr/0416-sink-config-layering.md) §8): the resolved object path and,
for `git`/`gitlab`, a sample commit subject and body rendered from the configured templates. Removing
the annotation clears `status.preview`.

See [ADR-0414](../adr/0414-sink-family-crds.md) for the family CRD model, and
[ADR-0415](../adr/0415-git-sink-commit-ergonomics.md) for commit message ergonomics.
