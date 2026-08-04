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

### Per-set manifest sidecar (multipart torn-set detection)

The YAML/tree projection emits bare `Item` rows with no envelope metadata, so a size-sharded
(multipart) export could otherwise leave a **torn** set (a mid-write failure) or a **stale** set
(generation-`N-1` files beside generation-`N`) that a YAML consumer cannot distinguish from a complete
one. To close that gap a prune-bearing layout (`perResource`/`split`) that shards into **more than one
part** writes one **per-set manifest sidecar** at a deterministic, generation-stable path —
`inventory/{namespace}/{name}.manifest.json` — declaring `generation`, `partTotal`, the per-part
identifiers, and the **union** of every part's projected data-file paths. The manifest shape, schema
versioning, and the consumer validation rule are specified in
[ADR-0405](0405-export-data-contract.md) (`layout.SetManifest` / `layout.VerifySet`); a `document`-mode
or single-part export writes no sidecar.

**Prune interaction (the reason this needs the multipart union-prune).** The sidecar lives inside the
managed directory, so it must be a member of the single union-prune keep-set or the final part's prune
would orphan it. The manifest is written on the **final part** — the same part that runs the one
union-prune — and its path is appended to `PruneKeepPaths` alongside every data path. As a result:

- **Every part's data files AND the sidecar survive** the single union-prune (they are all in the
  keep-set); nothing an earlier part wrote is lost.
- On a **new-generation re-export** the manifest path is unchanged (no `{generation}` placeholder), so
  the fresh manifest **replaces** the old one in place — replaced-not-orphaned.
- In `document` mode (`prune: off`) there is a single overwritten path and no distinct part files to
  reconcile, so no sidecar is emitted; this depends on and builds directly on the multipart union-prune
  established for tree modes.

**Shared-repo-path constraint (one inventory per managed path).** The sidecar path
`inventory/{namespace}/{name}.manifest.json` places it inside the pruned managed directory. Each export
prunes against **only its own** part union, so if two sibling inventories in the same namespace export to
the **same repo path**, inventory B's prune would delete inventory A's manifest (a false-torn signal and
marker flapping). With **disjoint** collection scopes the data files under
`{cluster}/{sourceNamespace}/{kind}/` are keyed distinctly and typically survive while the completeness
marker does not; with **overlapping** scopes sharing a repo path, sibling data files in the shared
managed directory can also be pruned. A shared repo path across sibling inventories is therefore
**unsupported** for the manifest: give each inventory its own repo path (distinct `endpoint`/subpath), or
rely on the distinct `{name}` component keeping manifests from colliding while accepting that a
co-located sibling's prune may still remove them. This mirrors the existing per-inventory ownership
assumption of the managed directory ([ADR-0407](0407-git-object-store-layout.md)).

## Consequences

- Zero-field Git configuration produces human-readable diffs.
- JSON and NDJSON remain explicit alternatives for automation.
- Per-resource trees improve review granularity but increase file count and Git work; export cadence
  and inventory size must be tuned accordingly.
- Pruning is part of tree-mode correctness so deleted resources disappear from the latest snapshot.
- Multipart (size-sharded) exports prune EXACTLY ONCE, against the union of every part's projected
  paths, on the final part — never per-part, which would let part N's prune delete part N-1's files
  (last-part-wins data loss). `ExportFilesOptions.SuppressPrune` authoritatively forces prune off on
  non-final parts and overrides an explicit `git.prune: true`, so per-part pruning can never
  re-enable; the final part carries `PruneKeepPaths` = the union keep-set.
- A multipart `perResource`/`split` set carries a per-set `*.manifest.json` sidecar so YAML consumers
  can detect a torn or stale set from the output alone — the sidecar rides the final part's union-prune
  keep-set, so it survives partial writes and is replaced-not-orphaned on regeneration.

## Related

- [ADR-0405](0405-export-data-contract.md) — canonical item contract
- [ADR-0407](0407-git-object-store-layout.md) — repository and path behavior
- [ADR-0415](0415-git-sink-commit-ergonomics.md) — commit behavior
- [ADR-0416](0416-sink-config-layering.md) — shared format configuration
- [ADR-0306](0306-full-resource-export-pruning.md) — Resource export content
