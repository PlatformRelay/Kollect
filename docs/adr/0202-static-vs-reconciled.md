# ADR-0202: Static config vs reconciled CRDs

> Config kinds (`Profile`, `Scope`) are static with no controller; work kinds are reconciled.
> Family sinks get a minimal connection-probe reconciler only ([ADR-0414](0414-sink-family-crds.md)).

**Theme:** 02 · API & tenancy · **Status:** Current

## Context

Operators differ on whether configuration CRDs get their own reconciler:

- **Flux notification-controller `Provider` and `Alert`** have **no status subresource** and no
  dedicated controller — they are referenced by reconciled `Receiver` and event dispatch logic.
- **external-secrets `SecretStore`** is reconciled (validates provider, writes status conditions).
- **Flux source-controller `GitRepository`** is fully reconciled with rich status (artifact revision).

Kollect has configuration objects (`KollectProfile`, family sinks) that change infrequently and
work objects (`KollectTarget`, `KollectInventory`) that drive continuous collection and export.

The former unified `KollectSink` CRD is **not** a public kind — it was removed in favour of typed
family sinks ([ADR-0414](0414-sink-family-crds.md)). The Go-only `KollectSinkSpec` adapter is an
internal registry/probe helper, not a Kubernetes resource.

At **large fleet scale**, shared GVK definitions may be duplicated per namespace or published via future
`KollectClusterProfile`; per-target overrides may be needed later without forking profiles.

## Decision

| Category | Kinds | Controller | Status | Validation |
| --- | --- | --- | --- | --- |
| Static config | `KollectProfile`, `KollectScope` (namespaced) | None | None | CEL `x-kubernetes-validations`, **validating webhook** ([ADR-0203](0203-namespaced-multi-tenancy.md)) |
| Static + probe | Family sinks: `KollectSnapshotSink`, `KollectDatabaseSink`, `KollectEventSink` | **Minimal** — connection test only ([ADR-0403](0403-connection-test.md)) | `ConnectionVerified`, `TLSInsecure`, `Degraded` | Webhook + probe reconciler |
| Reconciled | `KollectTarget`, `KollectInventory` | Yes | Full conditions + `observedGeneration` | Same + runtime SAR checks |

Rationale (Flux-aligned):

- Cuts controllers and status write churn for rarely changing config.
- Profile edits still trigger dependent reconciles via secondary watches on referencing objects.
- `spec.suspend` on **reconciled** kinds only; static objects are always "active" when referenced.

**Reject** full reconciliation of `KollectProfile` like ESO `SecretStore`. **Family sinks** are the
narrow exception: a lightweight reconciler for connectivity only ([ADR-0403](0403-connection-test.md),
[ADR-0414](0414-sink-family-crds.md)).

### Shared GVK, optional per-target overrides

- **Default:** `KollectTarget.spec.profileRef` names a `KollectProfile` in the **same namespace**
  ([ADR-0204](0204-namespaced-profiles.md)).
- **Future door:** optional inline attribute overrides or `profileRef` + patch fields on Target —
  design keeps API evolvable without breaking shared profiles ([ADR-0201](0201-crd-model.md)).

### Concurrent GVK watches

Research and document an informed default for **maximum concurrent GVK informers** before memory/API
pressure. Expose manager configuration:

- `maxConcurrentWatches` — soft limit with warning Event when approached
- Tune with envtest/load tests; document default in Helm `values.yaml` comments

Prefer **one shared informer per GVK** across Targets ([ADR-0301](0301-event-driven-informers.md)).

### Connection test (first-class)

See **[ADR-0403](0403-connection-test.md)** — probes attach to **family sink** CRDs (and optional
`KollectConnectionTest`); there is no unified `KollectSink` probe kind.

| Mechanism | Behavior |
| --- | --- |
| **`spec.connectionTest: true`** on a family sink | Probe on create/update |
| **Annotation `kollect.dev/test-connection: "true"`** | One-shot re-test on the family sink |
| **`ConnectionVerified` on the family sink** | `kubectl wait --for=condition=ConnectionVerified=...` |
| **Pipeline conditions (follow-up)** | `SinkReachable` (or export conditions) on `KollectInventory` / `KollectTarget` |

Connection tests run from the operator with the same TLS trust as export ([ADR-0201](0201-crd-model.md)
`caBundle` / `caSecretRef`). Errors are **visible and informative** (HTTP status, DNS, TLS handshake)
— sanitized, no secrets in messages.

### Collected object generation annotation

When exporting or summarizing a source object, record the source `metadata.generation` on collected
rows or export metadata via annotation:

- `kollect.dev/collectedGeneration: "<n>"`

Enables consumers to detect stale inventory vs live object without full payload in status.

```mermaid
sequenceDiagram
  participant User
  participant API as API server
  participant Op as kollect
  User->>API: apply family sink (connectionTest or test-connection annotation)
  API->>Op: family sink reconciler
  Op->>Op: probe sink with CA trust
  Op->>API: patch sink ConnectionVerified
  User->>API: kubectl wait ConnectionVerified
```

## Consequences

### Positive

- Fewer moving parts, fewer leader-election reconciler loops.
- Clear mental model: config CRDs are like Flux Providers; workload CRDs are like GitRepositories.
- Connection test gives human-user-0 fast feedback via minimal family-sink reconciler
  ([ADR-0403](0403-connection-test.md)).

### Negative

- Invalid sink credentials may still first appear at export unless user runs connection test.
- `kubectl wait --for=condition=Ready` does not apply to Profile or family sinks.
- `maxConcurrentWatches` tuning is cluster-dependent — wrong default causes silent memory pressure.

## Open questions

- **RESOLVED :** Connection probes live on family sinks + optional `KollectConnectionTest`
  ([ADR-0403](0403-connection-test.md)); unified `KollectSink` removed ([ADR-0414](0414-sink-family-crds.md)).
- **OPEN:** Per-target profile override API shape — inline map vs `KollectProfilePatch` kind?
