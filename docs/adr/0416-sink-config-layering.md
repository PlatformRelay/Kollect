# ADR-0416: Sink configuration layering

> Keep common destination policy consistent across the three namespaced sink families while
> retaining typed backend configuration.

**Theme:** 04 · Export & sinks · **Status:** Current

## Context

Connection, serialization, destination ownership, and vendor-specific settings cut across backend
types. Repeating them in every typed backend block would make defaults inconsistent and make the
public API harder to extend safely.

## Decision

`KollectSnapshotSink`, `KollectDatabaseSink`, and `KollectEventSink` share common fields that
normalize to an internal adapter before registry lookup. Public resources remain family-specific
and namespaced; inventories reference them through `snapshotSinkRefs`, `databaseSinkRefs`, and
`eventSinkRefs`.

The admitted backend set is:

- snapshot: `git`, `gitlab`, `s3`, and `gcs`;
- database: `postgres`, `mongodb`, and `bigquery`; and
- event: `kafka` and `nats`.

### Connection and identity

`endpoint`, `secretRef`, TLS trust, `connectionTest`, `cluster`, `pathTemplate`, and
`exportMinInterval` have the same meaning across families where applicable. Credentials are always
resolved through `secretRef`; secret-like keys are rejected from generic options.

### Serialization

`spec.serialization` selects an on-wire format and compression. Admission checks the selection
against the backend capability:

| Admitted type | Supported `serialization.format` |
| --- | --- |
| `git`, `gitlab` | `yaml` (default), `json`, `ndjson` |
| `s3`, `gcs` | `json` (default), `parquet`, `csv` |
| `postgres`, `mongodb`, `bigquery`, `kafka`, `nats` | `json` |

For object stores, `serialization.format` overrides the older `objectStore.format`; admission emits
a warning when both are set. Unsupported combinations fail admission rather than being ignored.

### Provisioning ownership

`spec.provisioning.mode` makes destination ownership explicit:

- `ensure` is the default and permits safe create-if-missing behavior implemented by the backend;
- `existing` forbids destination creation and preflights that the destination already exists.

Optional `provisioning.naming.template` uses the shared placeholder grammar. Neither mode permits
destructive replacement of an existing destination.

### Backend options

`spec.options` is a non-secret `map[string]string` for long-tail backend flags. Admission rejects
keys that look credential-bearing, including password, token, secret, API-key, private-key, and
credential variants. Widely used settings can graduate to typed fields without changing the
meaning of existing manifests.

### Preview

The `kollect.dev/preview: "true"` annotation requests a side-effect-free `status.preview`. It shows
the resolved format, provisioning mode, destination/path implications, and backend-specific output
such as Git commit/layout samples. Preview uses the same effective-config helpers as export.

## Consequences

- A small common layer keeps defaults, validation, and security rules consistent.
- Typed backend blocks still describe settings that are meaningful only to one destination.
- Unsupported formats and secret-like generic options fail before reconciliation.
- Internal normalization does not create another public Kubernetes kind.

## Related

- [ADR-0414](0414-sink-family-crds.md) — namespaced family CRDs and admitted types
- [ADR-0406](0406-sink-registry.md) — registry and backend adapters
- [ADR-0419](0419-git-export-serialization-layout.md) — Git serialization and layout
- [ADR-0104](0104-security-model.md) — credential and transport boundaries
