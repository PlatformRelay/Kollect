# ADR-0405: Export data contract and schema versioning

> The serialized inventory shape every sink and consumer depends on: the `Item` row, its ordering,
> and how the contract is versioned.

**Theme:** 04 · Export & sinks · **Status:** Current (schema versioning: Exploring)

## Context

Kollect's external value is the **exported inventory payload**. Portals, SQL queries, Git diffs,
Kafka consumers, and the HTTP API all read this contract — it is the most stability-sensitive surface
in the project, yet it had no ADR. The shape is implemented in `internal/collect/store.go` but its
guarantees (field set, ordering, null handling, versioning) were never written down.

A data contract must be: explicit, stable-ordered (for diffable Git and golden tests —
[ADR-0103](0103-etcd-limit.md)), bounded (no full payload in etcd status), and **versioned** so
consumers can detect breaking changes.

## Decision

### Row shape (`Item`)

One collected resource = one `Item` (`internal/collect/store.go`):

```json
{
  "targetNamespace": "team-a",
  "targetName": "deployments",
  "namespace": "team-a",
  "name": "api",
  "group": "apps",
  "version": "v1",
  "kind": "Deployment",
  "uid": "…",
  "attributes": { "image": "…", "images": ["…"] }
}
```

- **Identity fields** (`group/version/kind`, `namespace`, `name`, `uid`) locate the source object;
  `targetNamespace`/`targetName` record which `KollectTarget` produced the row.
- `attributes` is the profile-defined extraction result (`map[string]any`); JSONPath `[*]` yields a
  JSON array ([ADR-0302](0302-cel-jsonpath-extraction.md)).
- `group` is `omitempty` (core kinds); all other identity fields are always present.

### Aggregated payload

- **Default export** = a JSON array of `Item` for the inventory's scope (`MarshalNamespaceJSON`).
- **HTTP** = `NamespaceSummary { namespace, itemCount, items }`.
- **Sink projections** derive from this canonical snapshot ([ADR-0401](0401-sink-taxonomy-state-vs-stream.md)):
  Postgres rows keyed by `(inventory_namespace, inventory_name, target_name, source_uid)` + `cluster`;
  Kafka keyed by `{cluster}:{ns}/{name}`; Git/object-store as the whole JSON document.

### Export metadata

Carried **alongside** the payload (status + sink columns/headers), not inside each row:
`schemaVersion` (envelope contract version), `checksum` (SHA-256 of payload — `aggregate.ContentHash`),
source `generation`, `itemCount`, `exportedAt`, and `cluster`. These drive debounce/coalesce
([ADR-0305](0305-aggregation-dedupe.md)) and staleness detection without bloating rows.

### Multipart completeness marker (REL-02)

When a snapshot exceeds `maxExportBytes` it is sharded across several bounded envelopes
(`export.PartitionEnvelopes`). Sharded writes are **not atomic**: parts are persisted one by one, so a
mid-write failure — or a stale generation-`N-1` shard left beside fresh generation-`N` shards — can
leave a **torn** set that would otherwise masquerade as complete. To make torn sets detectable, each
part of a multipart **JSON `ExportEnvelope`** carries a completeness marker:

- `partIndex` — 1-based position of this part.
- `partTotal` — total number of parts in the set.
- `generation` — already present; identical across all parts of one set.

**Scope of the guarantee (important).** These markers live in the JSON `ExportEnvelope` header. They
are present, and payload-level torn-set detection applies, only where the envelope itself is the
persisted payload: the state-sink JSON contract — non-git object stores (S3/GCS/Azure/HTTP) and the
Git/GitLab **document + `serialization.format: json`** case, which writes the canonical envelope
unchanged.

The default **Git/GitLab sink serializes to YAML** via the human-readable layout projection
([ADR-0419](0419-git-export-serialization-layout.md)), which **intentionally emits bare `Item` rows and
carries NO `ExportEnvelope` metadata** — no `schemaVersion`, `checksum`, `itemCount`, `generation`, or
`partIndex`/`partTotal` inside the data files. Payload-level (per-row) completeness detection therefore
does not apply to YAML sinks. Instead, a multipart YAML set carries its completeness marker in a
**per-set manifest sidecar** (REL-02-FUP), giving YAML consumers the same torn-set / stale-set guarantee
JSON consumers already have.

**Per-set manifest sidecar (YAML layout, REL-02-FUP).** A prune-bearing layout export
(`layout.mode: perResource`/`split`) that shards into **more than one part** writes exactly one manifest
sidecar for the whole set:

- **Path** — deterministic and per-*set*: `inventory/{namespace}/{name}.manifest.json`. It carries **no**
  `{generation}` or part placeholders, so a re-export at a new generation **replaces** the manifest in
  place (replaced-not-orphaned) rather than accumulating stale siblings. The `.manifest.json` extension
  is distinct from the `.yaml`/`.ndjson` data files, so an existing consumer globbing
  `inventory/{ns}/*.yaml` **skips it** (graceful-ignore).
- **Shape** — a `layout.SetManifest`, always serialised as **JSON** (a machine-readable control file,
  independent of `serialization.format`). It self-identifies via `kind: KollectExportSetManifest` and
  `schemaVersion` (== `ExportSchemaVersion`, additive-only per rule 2), and declares `generation`,
  `partTotal`, the per-part identifiers `parts: [1..partTotal]`, and `paths` — the **union** of every
  part's projected data-file paths.
- **Distinct from the split `Index`** — `layout.Index` is the per-*projection* split-mode sidecar (one
  projection's rows, for CI gating); `SetManifest` spans **every part** of one multipart set. When a
  split-mode export is also multipart the two coexist at different paths and self-identify by `kind`.
- **Scope** — the sidecar targets prune-bearing tree layouts, where parts occupy **distinct** paths and a
  torn set is possible. It is **not** emitted for single-part exports (byte-compatible with today, no
  sidecar) nor for `document` mode (all parts overwrite one path — a degenerate set; `document` is
  `prune: off`). Its prune interaction is specified in [ADR-0419](0419-git-export-serialization-layout.md).

**Consumer validation (YAML sidecar contract).** Writer/verifier split: the controller **writes** the
manifest (it never reads it back), and **completeness detection is the consumer's responsibility** — the
marker is a *detection* contract, not a controller-side guarantee. A consumer confirms a YAML set is
complete by reading the manifest and checking that **every path in `paths` exists on disk** (a missing
path = a torn set, a part failed to persist) and that `generation` matches the generation it expects
(from the CR status or commit metadata — the same expected-generation signal the JSON contract uses; a
mismatch = a stale set left by a torn export that never rewrote the manifest). `layout.VerifySet` is a
**consumer-side helper** (it has no production callers inside the operator) that a Go consumer can vendor
to apply exactly this rule — and, for generation-scoped path templates, it additionally surfaces
prior-generation orphan data files; its `present` argument may be a raw directory listing — the
`.manifest.json` sidecar is an expected member of the set, never counted as a stale extra. A single-part
export writes **no** sidecar and is complete on its own, byte-identical to the pre-marker shape (additive
evolution, rule 2).

**Consumer validation (JSON envelope contract).** A consumer reassembling a set from JSON envelopes
MUST verify it holds every index `1..partTotal`, that the count equals `partTotal`, and that
`generation` is **uniform** across the parts; a missing index or a mixed generation means the set is
torn or stale and MUST NOT be treated as complete. The **absence** of `partTotal` (the legacy/
`omitempty` form) denotes a standalone single-part document that is complete on its own — single-part
exports stay byte-identical to the pre-marker shape, so existing consumers are unaffected (additive
evolution, rule 2). The marker is a *detection* contract only; manifest-last write ordering /
staged-commit / GC of orphaned parts is deliberately out of scope here and tracked separately.

### Stability rules (binding)

1. **Deterministic ordering** — stable key order on serialize so Git diffs and golden tests are
   reproducible.
2. **Additive evolution preferred** — new attributes/fields are additive; removals/renames are
   breaking and gated by the API versioning policy ([ADR-0206](0206-api-versioning-conversion.md)).
3. **No secrets, ever** — redaction happens before export ([ADR-0303](0303-helm-release-inventory.md),
   [ADR-0104](0104-security-model.md)).
4. **Bounded size** — spill over `maxExportBytes` to object store; never to etcd ([ADR-0103](0103-etcd-limit.md)).

## Consequences

- Consumers have one documented schema across all sinks.
- Golden/contract tests can assert the shape; breaking it fails CI.
- Consumers can branch on `schemaVersion` without coupling to CRD API versions.

## Implementation status (schemaVersion milestone)

| Export path | `schemaVersion` envelope | Status |
| --- | --- | --- |
| Kafka `EventEnvelope` | Yes — `internal/sink/kafka/backend.go` | **Shipped** |
| Inventory / cluster inventory sink export | No — bare `[]Item` JSON array (`MarshalNamespaceJSON`) | **Pre-beta gap** |
| Git / Postgres / S3 / GCS object payloads | No — canonical array only | **Pre-beta gap** |
| Read API HTTP responses | No — `NamespaceSummary` without envelope | **Pre-beta gap** — align with OpenAPI `openapi/v1alpha1/inventory.yaml` |

Contract value: `kollect.dev/v1alpha1` ([ADR-0206](0206-api-versioning-conversion.md)).

**Pre-beta milestone:** wrap all sink exports and Read API responses in a versioned envelope
(`schemaVersion`, `items`, metadata) so consumers decouple from CRD API versions
([ADR-0206](0206-api-versioning-conversion.md)). Until then, schema versioning remains **Exploring**.

## Open questions

- **PARTIAL:** Explicit **`schemaVersion`** on Kafka event envelopes — inventory and state-sink JSON
  exports still emit bare arrays; milestone tracked above.
- **DECIDED :** Attributes stay **`map[string]any`** in the contract; stronger typing is a
  **sink-side** concern — the Parquet sink promotes a hot-attribute allowlist to typed columns while
  keeping a JSON `attributes` column ([ADR-0401](0401-sink-taxonomy-state-vs-stream.md)).
- **PARTIAL :** OpenAPI extensions (pagination, filters, envelope, `exportStatus`) tracked in
  the Read API OpenAPI contract; publish JSON Schema for `Item` alongside OpenAPI when
  envelope milestone closes.
