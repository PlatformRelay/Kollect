# ADR-0405: Export data contract and schema versioning

> The serialized inventory shape every sink and consumer depends on: the `Item` row, its ordering,
> and how the contract is versioned.

**Theme:** 04 · Export & sinks · **Status:** Current (schema versioning: Exploring)

## Context

kollect's external value is the **exported inventory payload**. Portals, SQL queries, Git diffs,
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
- A missing explicit `schemaVersion` is a real gap (see open questions).

## Open questions

- **DECIDED (2026-06-05):** Add an explicit **`schemaVersion`** to the export envelope (not per-row),
  aligned with the spoke→hub report `apiVersion` ([ADR-0502](0502-lean-queue-transport.md)). Consumers
  branch on it; bumped only on a breaking contract change.
- **DECIDED (2026-06-05):** Attributes stay **`map[string]any`** in the contract; stronger typing is a
  **sink-side** concern — the Parquet sink promotes a hot-attribute allowlist to typed columns while
  keeping a JSON `attributes` column ([ADR-0401](0401-sink-taxonomy-state-vs-stream.md)).
- **OPEN:** Publish a JSON Schema / OpenAPI for the `Item` array next to `openapi/v1alpha1/inventory.yaml`?
