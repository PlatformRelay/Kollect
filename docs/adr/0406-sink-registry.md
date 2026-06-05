# ADR-0406: Sink registry and the Backend interface

> How sink backends are abstracted, registered, and constructed — one `Backend` interface behind a
> type-keyed factory, with no vendor SDK in reconcilers.

**Theme:** 04 · Export & sinks · **Status:** Current

## Context

Kollect exports to many backends (Git, GitLab, S3, GCS, Postgres, Kafka — [ADR-0402](0402-sink-backends-database-kafka.md),
[ADR-0401](0401-sink-taxonomy-state-vs-stream.md)). Reconcilers must not import vendor SDKs directly,
and adding a backend must not touch controller code. This decision was implemented in
`internal/sink/registry.go` but never recorded — earlier ADRs even cited a non-existent "ADR-0005"
for it. This ADR fills that gap.

Pattern precedent: external-secrets' provider registry ([ADR-0102](0102-prior-art.md)).

## Decision

### `Backend` interface

```go
type Backend interface {
    Type() string
    Capabilities() Capabilities // snapshot vs stream, supports-delete
    Export(ctx context.Context, payload []byte, path string) error
}
```

Minimal by design: a backend takes a serialized payload ([ADR-0405](0405-export-data-contract.md)) and
an object path, and exports. `Capabilities()` lets the inventory controller choose whole-snapshot vs
delete-reconciliation behavior per backend ([ADR-0401](0401-sink-taxonomy-state-vs-stream.md)).
Connectivity probing is a parallel concern ([ADR-0403](0403-connection-test.md)).

### Factory + registry

- `Factory func(spec KollectSinkSpec, ctx BuildContext) (Backend, error)`.
- `Registry` maps `spec.type` → `Factory`; built-ins registered in `NewRegistry()`
  (`git`, `gitlab`, `s3`, `gcs`, `postgres`, `kafka`).
- `NewBackend(spec, ctx)` resolves the factory or returns `unknown sink type %q`.
- **`BuildContext`** carries resolved, non-spec material — `CAPEM`, `SecretData`,
  `DatabaseSecretData` — so backends never read Kubernetes secrets themselves and reconcilers stay
  free of vendor SDKs ([ADR-0104](0104-security-model.md)).

### Rules (binding)

1. **No vendor SDK above `internal/sink/<backend>/`** — controllers and the registry import only the
   `Backend` interface.
2. **`spec.type` is a webhook-validated enum** ([ADR-0201](0201-crd-model.md), [ADR-0602](0602-error-taxonomy.md));
   the registry is the single source of which types exist.
3. **A backend ships only when integration/e2e-testable** (testcontainers or kind sidecar —
   [ADR-0402](0402-sink-backends-database-kafka.md)); do not register a type without a backend
   (the GitLab/`nats`/Parquet enum lesson).
4. **Idempotent export** — `Export` is safe to retry; at-least-once semantics ([ADR-0502](0502-lean-queue-transport.md)).

## Consequences

- New backends = one package + one `Register` line; zero controller changes.
- Capability differences (snapshot store vs event emitter — [ADR-0401](0401-sink-taxonomy-state-vs-stream.md))
  are currently implicit in the backend; a capability flag may be needed (open question).
- The interface is sync/blocking; long exports rely on context deadlines and the circuit breaker
  ([ADR-0602](0602-error-taxonomy.md)).

## Open questions

- **DECIDED (2026-06-05):** Add a **`Capabilities()`** method (snapshot vs stream, supports-delete) so
  the inventory controller picks delete-reconciliation vs whole-snapshot behavior per
  [ADR-0401](0401-sink-taxonomy-state-vs-stream.md).
- **OPEN:** Out-of-tree backend registration (plugin) — or keep the registry compile-time only?
- **OPEN:** Should `Export` take the structured snapshot instead of `[]byte` so backends choose their
  own serialization (Parquet, row batches) without re-parsing JSON? (Revisit with the Parquet sink.)
