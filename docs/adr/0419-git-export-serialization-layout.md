# ADR-0419: Git export serialization and layout

> Produce readable, deterministic Git snapshots by default while preserving explicit format and
> tree-layout controls.

**Theme:** 04 · Export & sinks · **Status:** Current

## Context

One compact JSON document is deterministic but produces noisy reviews and does not resemble the
resource-oriented trees platform teams use in Git. Git is a projection of Kollect's canonical
in-memory snapshot, so its representation can prioritize reviewability without changing collection,
checksums, or other sink outputs.

## Decision

Git and GitLab are admitted `KollectSnapshotSink` types. A sink with only `type`, `endpoint`, and
optional credentials writes a YAML inventory document at:

```text
inventory/{namespace}/{name}.yaml
```

Git/GitLab support **`yaml` (default), `json`, and `ndjson`**. S3/GCS support
**`json` (default), `parquet`, and `csv`**. Parquet is an object-store format, not a Git layout or
an additional sink type.

The canonical snapshot and debounce checksum stay JSON-normalized. Format encoding and path
projection happen only in the selected backend.

### Document and tree modes

`spec.layout` is optional and valid only for Git/GitLab:

| Mode | Output | Default pruning |
| --- | --- | --- |
| `document` | One inventory file at `spec.pathTemplate` | off |
| `perResource` | One file per collected item at `layout.pathTemplate` | on |
| `split` | A summary index plus the per-resource tree | on |

The default inventory path is `inventory/{namespace}/{name}{extension}`. The default per-resource
path is `{cluster}/{sourceNamespace}/{kind}/{sourceName}{extension}`. Supported placeholders also
include inventory/target identity, API group, UID, and generation. Every segment is sanitized;
`..`, path separators, unsafe characters, and silent path collisions are rejected.

### Content selection

`layout.content` controls tree-file contents:

- `item` writes the complete Kollect item;
- `attributes` writes only its extracted attributes; and
- `manifest` writes the pruned Kubernetes object from Resource export mode.

When a referenced profile uses `export.mode: Resource`, an omitted layout resolves to
`perResource` with `manifest` content. Setting `layout.mode: document` explicitly retains one file.
For `perResource` and `split`, pruning removes files for resources no longer present in the current
snapshot.

### Effective defaults

| Field | Git/GitLab default |
| --- | --- |
| `serialization.format` | `yaml` |
| `serialization.compression` | `none` |
| `layout.mode` | `document`, unless Resource export selects `perResource` |
| `layout.content` | `item`, or `manifest` for Resource export trees |
| `layout.index.enabled` | on only for `split` |
| `layout.filename.groupInPath` | `auto` |
| `layout.filename.lowercaseKind` | `true` |
| `layout.filename.maxSegmentLength` | `63` |
| `git.prune` | off for `document`; on for `perResource` and `split` |

`{extension}` follows the effective serialization format. Users retaining the earlier JSON document
shape set `serialization.format: json`; path templates with a literal `.json` remain honored when
the selected format is JSON.

### Determinism and preview

YAML uses stable map ordering and Kubernetes-compatible field names. NDJSON writes one item per
line. Layout projection is pure and rejects duplicate output paths before writing. The
`kollect.dev/preview: "true"` annotation exposes the effective mode, content, pruning behavior,
document path, and sample resource paths without modifying the repository.

### Example

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectSnapshotSink
metadata:
  name: inventory-git
  namespace: team-a
spec:
  type: git
  endpoint: ssh://git.example.com/platform/inventory.git
  secretRef:
    name: git-credentials
  layout:
    mode: perResource
```

The same layout and serializers are shared by GitLab export. Commit policy, author identity, and
message templates remain the concerns of [ADR-0415](0415-git-sink-commit-ergonomics.md).

## Consequences

- Zero-field Git configuration produces human-readable diffs.
- JSON and NDJSON remain explicit alternatives for automation.
- Per-resource trees improve review granularity but increase file count and Git work; export cadence
  and inventory size must be tuned accordingly.
- Pruning is part of tree-mode correctness so deleted resources disappear from the latest snapshot.

## Related

- [ADR-0405](0405-export-data-contract.md) — canonical item contract
- [ADR-0407](0407-git-object-store-layout.md) — repository and path behavior
- [ADR-0415](0415-git-sink-commit-ergonomics.md) — commit behavior
- [ADR-0416](0416-sink-config-layering.md) — shared format configuration
- [ADR-0306](0306-full-resource-export-pruning.md) — Resource export content
