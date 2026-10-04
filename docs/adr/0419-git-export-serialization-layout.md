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

### Exact file ownership for pruning (2026-10-02)

Directory depth cannot identify a kind directory in an arbitrary layout template.
Expanding pruning to its siblings can delete a different inventory's files, even
when the original directory-scoped implementation kept those trees separate.

Tree exports therefore persist versioned JSON ownership records at
`.kollect-prune/<sha256(owner)>.json`. The owner is the kind-qualified JSON array
`["v2", kind, cluster, namespace, name]` from the identity the export request carries
([ADR-0422](0422-inventory-export-identity.md)): cluster empty means `default`, namespace is
empty for a `KollectClusterInventory`, and name is the base inventory name. Generation and
multipart suffixes do not change ownership. A git layout export without an identity, or with
one that disagrees with its object path, is refused. Git and GitLab pass the same owner through
both engines. The previous record minus the final current union is the deletion
set; record updates, exported files, and removals land in one Git commit.

Missing metadata adopts only current paths and preserves unknown history. No
heuristic migration sweeps directories. The old directory-scoped helpers remain
for legacy callers without an owner; they never expand into sibling directories.
Record parsing bounds aggregate bytes, owner count, and path count; it rejects
noncanonical paths, Git internal paths, symlinks, and overlapping owner claims
before either engine writes or removes files. A record is repository-controlled
state, not a cryptographic proof against writers who can alter Git history.

Distinct inventories may share a repository with disjoint projected paths. Two
sinks exporting the same inventory identity to the same branch still share one
record: use separate branches or repositories for independent copies. Identity
changes preserve the old owner's files until explicitly cleaned up. A supported
empty tree export commits an empty record and removes its prior owned paths;
explicit tree mode is needed when no rows remain for content-based auto-detection.
A cluster inventory and a namespaced inventory in namespace `cluster` with the same
name render the same object path but have different owners, so each prunes its own
stale files and neither deletes the other's. Colliding projected file paths still get
no independent storage: whichever exports second is rejected, as for any two
inventories. The rejection names the owning inventory and its record file. Before
a commit, the engine also refuses ownership records that would claim one path twice.

Non-final multipart parts cannot advance ownership. The final union includes its
completeness manifest where applicable; every non-final part claim-checks that
manifest path, so a set whose manifest belongs to another inventory is rejected on
part 1, before any part is committed. The existing completeness marker remains
multipart-only; the ownership record is cleanup state, not a second completeness
signal. Interrupted exports can leave previously unrecorded partial files for
manual cleanup. Payload coalescing keeps only the latest operation per repository,
branch, and owner. Its value includes the payload checksum, prune intent, written
paths, and complete keep-set, so returning to an earlier snapshot still replays it.
An operation without a checksum invalidates the previous cached value. Owner
identity preserves suffix-shaped inventory names; only the exact suffix matching
a multipart envelope’s index and total is removed.

Limits: 16 MiB aggregate metadata per branch, 1,024 owners, 100,000 paths per owner,
and 4,096 bytes per path or owner identifier. Invalid metadata is a terminal
configuration error; filesystem failures propagate for retry.

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
