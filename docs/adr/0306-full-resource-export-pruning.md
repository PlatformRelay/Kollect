# ADR-0306: Full resource export with path pruning

> Export the whole target object (minus noise) from a profile, using Argo CD–style path exclusions
> and the existing redaction stack — without hand-authoring every field.

**Theme:** 03 · Collection & extraction · **Status:** Accepted (Phase 1 shipped)

## Context

`KollectProfile` today requires an explicit `spec.attributes[]` list: each row in export is a map of
named JSONPath/CEL extractions ([ADR-0302](0302-cel-jsonpath-extraction.md)). That model is ideal for
**curated inventory** (image tags, chart versions, ingress hosts) but painful for:

- **Audit / drift snapshots** — “give me the Deployment as YAML-ish JSON, minus controller noise.”
- **Exploratory profiles** — new GVKs where the author does not yet know which fields matter.
- **GitOps debugging** — compare live `spec` + selective `status` without maintaining 40 paths.

Authors can approximate full export today with a single attribute `path: "cel:object"`, but that
**bypasses** pruning, size governance, and admission gates. It also embeds identity/metadata the
export contract already carries on the `Item` envelope ([ADR-0405](0405-export-data-contract.md)).

**Prior art**

| Project | Pattern | Relevance |
| --- | --- | --- |
| **Argo CD** | `spec.ignoreDifferences[]` with `jsonPointers` (RFC 6901) and `jqPathExpressions` | Familiar “ignore this subtree” UX for GitOps users |
| **kubectl** | `last-applied-configuration`, `managedFields` noise in live objects | Default exclusions candidates |
| **Flux / ESO** | Never persist secret bytes in status | Reinforces scrub-at-extraction ([ADR-0104](0104-security-model.md)) |
| **Kollect today** | Explicit attributes + `scrubKeys` / `Secret.data` webhook guard ([ADR-0303](0303-helm-release-inventory.md)) | Security must apply to full-object mode too |

The design question is not *whether* to support wide export — some teams need it — but how to make it
**safe, bounded, and ergonomic** without breaking the `Item` contract or the explicit-attribute default.

## Decision

Add an optional **`spec.export`** block on `KollectProfile` / `KollectClusterProfile`. When
`export.mode: Resource`, the collector serializes a **pruned copy** of the informer object into export
instead of (or in addition to) hand-picked attributes.

### CRD shape (proposed)

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectProfile
metadata:
  name: deployment-snapshot
  namespace: team-a
  annotations:
    # Required when targetGVK is Secret or export.mode is Resource on sensitive kinds
    kollect.dev/allow-full-resource-export: "true"
spec:
  targetGVK:
    group: apps
    version: v1
    kind: Deployment

  export:
    # Attributes | Resource — default Attributes (current behaviour)
    mode: Resource

    # Attribute key for the embedded object in Item.attributes (default: resource)
    as: resource

    # Which top-level object sections to include (default: SpecAndStatus)
    include: SpecAndStatus   # MetadataOnly | SpecOnly | StatusOnly | SpecAndStatus | All

    # Path-based pruning — Argo CD–compatible + Kollect JSONPath
    prune:
      # Apply built-in noise exclusions (managedFields, last-applied-configuration, …)
      defaults: true

      # RFC 6901 JSON Pointers — same mental model as Argo CD ignoreDifferences
      jsonPointers:
        - /metadata/resourceVersion
        - /metadata/generation
        - /metadata/managedFields
        - /metadata/annotations/kubectl.kubernetes.io~1last-applied-configuration
        - /metadata/annotations/argocd.argoproj.io~1tracking-id
        - /status/conditions

      # Optional kubectl/JSONPath exclusions for authors already using profile paths
      jsonPaths:
        - '$.metadata.labels["pod-template-hash"]'

      # Key-name denylist at any depth — merges with operator scrubKeys ([ADR-0303](0303-helm-release-inventory.md))
      scrubKeys:
        - password
        - token

      # Advanced: CEL predicates evaluated against object; true => drop matched value
      # (Phase 2 — webhook validates compile; Phase 1 omit)
      cel:
        - 'cel:has(object.status) && has(object.status.observedGeneration)'

  # When mode: Attributes (default), attributes[] is required (unchanged).
  # When mode: Resource, attributes[] is optional — use for computed fields on top of the blob.
  attributes:
    - name: containerCount
      path: 'cel:size(object.spec.template.spec.containers)'
      type: int
```

**OpenAPI sketch** (Go types):

```go
type ExportSpec struct {
    // +kubebuilder:validation:Enum=Attributes;Resource
    // +kubebuilder:default=Attributes
    Mode string `json:"mode,omitempty"`

    // +kubebuilder:default=resource
    As string `json:"as,omitempty"`

    // +kubebuilder:validation:Enum=MetadataOnly;SpecOnly;StatusOnly;SpecAndStatus;All
    // +kubebuilder:default=SpecAndStatus
    Include string `json:"include,omitempty"`

    Prune *PruneSpec `json:"prune,omitempty"`
}

type PruneSpec struct {
    // +kubebuilder:default=true
    Defaults *bool `json:"defaults,omitempty"`

    JSONPointers []string `json:"jsonPointers,omitempty"`
    JSONPaths    []string `json:"jsonPaths,omitempty"`
    ScrubKeys    []string `json:"scrubKeys,omitempty"`
    CEL          []string `json:"cel,omitempty"` // Phase 2
}
```

### Semantics

| Topic | Rule |
| --- | --- |
| **Default** | `export` omitted ⇒ `mode: Attributes`; existing profiles unchanged |
| **Mutual requirement** | `mode: Attributes` ⇒ `spec.attributes` non-empty; `mode: Resource` ⇒ `attributes` optional |
| **Export payload** | Pruned object stored under `Item.attributes[<as>]` (default key `resource`) |
| **Identity dedup** | When `include` contains metadata, strip fields already on `Item` (`uid`, `namespace`, `name`, GVK) from the embedded copy to avoid triple storage — configurable `dedupeIdentity: true` (default) |
| **Ordering** | Deep-sort map keys on serialize so Git diffs stay stable ([ADR-0405](0405-export-data-contract.md)) |
| **Size** | Full-object rows count toward `maxExportBytes`; oversize rows truncate with `ErrTerminal` + target condition — never spill raw objects to etcd status ([ADR-0103](0103-etcd-limit.md)) |
| **Metrics** | `spec.metrics[].path` may reference `<as>` only when the metric extractor supports nested JSONPath (Phase 2); Phase 1: metrics disabled or require flat `attributes` |

### Built-in defaults (`prune.defaults: true`)

Applied before user `jsonPointers` / `jsonPaths`:

| Pointer | Rationale |
| --- | --- |
| `/metadata/managedFields` | SSA noise; huge and unstable |
| `/metadata/resourceVersion` | Churn on every watch event |
| `/metadata/generation` | Churn; rarely needed in inventory |
| `/metadata/annotations/kubectl.kubernetes.io~1last-applied-configuration` | Legacy apply blob |
| `/metadata/annotations/argocd.argoproj.io~1tracking-id` | Argo tracking noise (optional in preset `gitops`) |

**Not** in defaults: `/status` (often valuable), `/metadata/labels`, `/metadata/annotations` (teams prune explicitly).

Future: named presets via `prune.preset: kubernetes | gitops | none` instead of a boolean.

### Security and governance

Full-object export inherits the **redaction-at-extraction** model ([ADR-0104](0104-security-model.md)):

1. **Global operator `scrubKeys[]`** always runs (Phase 2 per [ADR-0303](0303-helm-release-inventory.md)).
2. **Profile `prune.scrubKeys`** merges with global list (case-insensitive key match at any depth).
3. **`Secret` GVK** — `mode: Resource` rejected unless `kollect.dev/allow-full-resource-export: "true"` on the profile **and** `KollectScope` (or cluster policy) allows wide export for that GVK.
4. **`Secret.data`** — always scrubbed to `{"redacted": true, "reason": "secret-data"}` even with opt-in annotation (same as summary profiles).
5. **Helm storage** — raw `data.release` / rendered manifest never exported ([ADR-0303](0303-helm-release-inventory.md)).

Admission webhook validates:

- RFC 6901 pointer syntax for `jsonPointers`
- JSONPath parse for `jsonPaths` (warn-only Phase 1, reject Phase 2 — [ADR-0302](0302-cel-jsonpath-extraction.md))
- `export.as` is a valid attribute name and does not collide with explicit `attributes[].name`
- CEL compile for `prune.cel` (Phase 2)

### Example: Argo CD Application (audit-friendly)

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectProfile
metadata:
  name: argo-application-snapshot
  namespace: platform
spec:
  targetGVK:
    group: argoproj.io
    version: v1alpha1
    kind: Application
  export:
    mode: Resource
    as: application
    include: SpecAndStatus
    prune:
      defaults: true
      jsonPointers:
        - /status/operationState
        - /status/reconciledAt
        - /status/sync.comparedTo
  attributes:
    - name: chartVersion
      path: '$.status.sync.revision'
      type: string
```

Export row (conceptual):

```json
{
  "targetNamespace": "platform",
  "targetName": "argo-apps",
  "namespace": "argocd",
  "name": "guestbook",
  "group": "argoproj.io",
  "version": "v1alpha1",
  "kind": "Application",
  "uid": "…",
  "attributes": {
    "application": {
      "apiVersion": "argoproj.io/v1alpha1",
      "kind": "Application",
      "spec": { "…": "…" },
      "status": { "sync": { "status": "Synced" } }
    },
    "chartVersion": "abc123"
  }
}
```

### Alternatives considered

| Option | Verdict |
| --- | --- |
| **`cel:object` attribute only** | Too easy to foot-gun; no defaults, no pointer pruning, weak governance |
| **New top-level `Item.resource` field** | Cleaner typing but breaks additive-only contract assumptions in early consumers; defer until envelope milestone ([ADR-0405](0405-export-data-contract.md)) |
| **`attributes: [{ name: "*", path: … }]` wildcard** | Magic name; hard to validate and to document in OpenAPI |
| **Target-side pruning** | Wrong layer — pruning is schema concern on Profile, not collection scope ([ADR-0207](0207-target-collection-filtering.md)) |
| **jqPathExpressions (Argo)** | Useful Phase 2 alias; start with `jsonPointers` + `jsonPaths` to reuse existing parsers |

## Consequences

### Positive

- One profile covers “export almost everything” with familiar Argo CD pointer syntax.
- Explicit attributes remain the default — no behaviour change for curated inventories.
- Hybrid mode (`Resource` + extra attributes) supports checksums / derived fields without duplicating paths.
- Security stays at extraction time; sinks remain secret-free by construction.

### Negative

- Payload size and Git diff noise increase — authors must opt in and tune `prune`.
- Deep scrub walks add CPU per object; needs benchmarks in [PERFORMANCE.md](../PERFORMANCE.md).
- Nested-object metrics and SQL column promotion need follow-up ([ADR-0304](0304-custom-resource-aggregation-rfc.md), [ADR-0401](0401-sink-taxonomy-state-vs-stream.md)).

## Implementation phases

1. **Phase 1 (MVP):** `export.mode`, `as`, `include`, `prune.defaults`, `prune.jsonPointers`, `prune.jsonPaths`, `prune.scrubKeys`; webhook + unit tests; no `prune.cel`.
2. **Phase 2:** `prune.cel`, `prune.preset`, jqPathExpressions alias, nested metrics paths, scope-level `allowResourceExport`.
3. **Phase 3:** Optional `Item` envelope field `resource` if consumers outgrow `attributes.resource`.

## Open questions

- **OPEN:** Should `include: MetadataOnly` be allowed, or always fold identity into `Item` and omit metadata from the blob?
- **OPEN:** Preset name `gitops` vs documenting Argo pointers only in samples?
- **OPEN:** Hybrid default — when both `Resource` and `attributes` are set, fail if an attribute path reads outside pruned tree?
- **OPEN:** Parquet / Postgres sinks — promote top-level `spec.*` columns automatically in Resource mode, or stay JSON blob only?

## References

- [ADR-0201](0201-crd-model.md) · [ADR-0302](0302-cel-jsonpath-extraction.md) · [ADR-0303](0303-helm-release-inventory.md)
- [ADR-0405](0405-export-data-contract.md) · [ADR-0104](0104-security-model.md)
- [Argo CD ignoreDifferences](https://argo-cd.readthedocs.io/en/stable/user-guide/diffing/)
