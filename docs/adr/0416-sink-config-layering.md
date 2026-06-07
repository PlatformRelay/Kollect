# ADR-0416: Sink configuration layering

> Cross-cutting `serialization`, `provisioning`, and `options` blocks on every sink family — zero
> config by default, enterprise-overridable, without exploding the CRD surface per backend.

**Theme:** 04 · Export & sinks · **Status:** Current

## Context

Per-backend settings grow combinatorially: `GitSpec` already has 11 fields, `ObjectStoreSpec`
carries `format` + `hotAttributes`, NATS has stream/subject. Each new backend (S3/GCS/Azure Blob,
MongoDB, RabbitMQ, CosmosDB) adds another typed sub-struct. Three concerns are tangled and only one
is cross-cutting today:

1. **Connection** — endpoint/secret/TLS — already cross-cutting via `SinkCommonFields`.
2. **Serialization & schema** — what bytes go on the wire — ad-hoc (`ObjectStoreSpec.format` only).
3. **Provisioning ownership** — who creates the destination topic/table/bucket — implicit and
   inconsistent (Postgres auto-`CREATE TABLE`, Kafka relies on broker auto-create, S3 assumes the
   bucket exists), with no user control.

Two personas use one CRD: a kind user wants `kubectl apply` to just work; a platform team wants to
point kollect at a resource **they** provisioned with their schema and partitioning, and forbid
auto-create.

## Decision

Add two cross-cutting blocks plus a generic escape hatch to `SinkCommonFields` (inlined into every
family CRD, namespaced and cluster-scoped) and normalize them into `KollectSinkSpec`.

### `serialization` (Option 4)

```yaml
serialization:
  format: parquet        # json (default) | parquet | csv | ndjson
  compression: zstd      # none | gzip | snappy | zstd
```

`json` is the zero-config default. The backend **capability matrix** gates which formats are
honored; the validating webhook rejects nonsensical combinations at admission:

| Family | Supported `serialization.format` |
| --- | --- |
| object store (s3/gcs/azureblob) | json, parquet, csv |
| http | json, ndjson |
| event (kafka/nats) | json (avro/protobuf deferred to v1beta1) |
| database (postgres/bigquery) | native rows — only json accepted |

`serialization.format` takes precedence over the legacy `objectStore.format`; the webhook emits a
warning when both are set.

### `provisioning` (Option 5)

```yaml
provisioning:
  mode: existing         # ensure (default) | existing
  naming:
    template: "{cluster}.inventory.{namespace}"
```

- **ensure** (default): create-if-missing with safe defaults — never destructive (CS-1).
- **existing**: kollect never issues create/admin/DDL calls; preflight verifies the resource exists
  (CS-2, CS-4). The webhook emits a warning so the implication is visible in GitOps diffs.

`adopt` is explicitly deferred. Per maintainer guidance (RFC Q5), `ensure` is positioned for
test/PoC; enterprises pre-provision and run `existing`, so kollect needs **no** advanced admin
automation.

### `options` (Option 2)

```yaml
options:
  storageClass: GLACIER_IR
  acks: all
```

A non-secret `map[string]string` pass-through for long-tail vendor flags, so backends evolve
without an API bump. **Guardrail:** the webhook rejects secret-like keys (`password`, `token`,
`secret`, `apikey`, `*key`, `credential`, …); credentials only ever travel via `secretRef`
([ADR-0104](0104-security-model.md)).

### Preview annotation (foundation, §8 of RFC)

`kollect.dev/preview: "true"` is reserved to opt a sink into `status.preview` rendering of its
export implications (DDL, sample commit, object path) with no side effects. The preview **surface**
ships in a later minor; this ADR only locks the annotation key and the
`EffectiveSerializationFormat`/`EffectiveProvisioningMode` projection helpers that guarantee
preview == reality.

## Consequences

- New backends slot into `{connection, serialization, provisioning, ≤~5 hot fields, options}` — no
  new structural concept per backend.
- The CRD surface grows by three small, validated, defaulted blocks, not by N×backend fields.
- Zero-config is preserved: every block is optional and defaults to json / ensure.
- Hot-field promotion path: a popular `options` key can graduate to a typed field in a later minor
  (additive, non-breaking).

## Deferred (future ADRs / versions)

| Item | Notes |
| --- | --- |
| `serialization.schemaRegistry` + Avro/Protobuf | v1beta1 — Confluent first |
| `configRef` ConfigMap for verbose config / DDL | v1beta1 |
| `provisioning.mode: adopt` + additive settings | v1beta1 |
| `status.preview` rendering + `kubectl kollect explain-sink` | next minor (preview surface) |
| Resource-lifecycle CRDs (Crossplane-style) | rejected — point to Crossplane/Terraform |

## Related

- [ADR-0414](0414-sink-family-crds.md) — family CRDs + `SinkCommonFields` + `ToKollectSinkSpec`
- [ADR-0406](0406-sink-registry.md) — registry/`Backend`; `Capabilities()` is the matrix home
- [ADR-0401](0401-sink-taxonomy-state-vs-stream.md) — Parquet hybrid schema precedent
- [ADR-0104](0104-security-model.md) — secrets only via `secretRef` (options guardrail)
- [ADR-0407](0407-git-object-store-layout.md) — path/name template grammar reused by `naming`
