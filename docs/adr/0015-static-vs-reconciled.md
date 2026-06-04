# ADR-0015: Static config vs reconciled CRDs

## Status

Accepted

## Context

Operators differ on whether configuration CRDs get their own reconciler:

- **Flux notification-controller `Provider` and `Alert`** have **no status subresource** and no
  dedicated controller — they are referenced by reconciled `Receiver` and event dispatch logic.
- **external-secrets `SecretStore`** is reconciled (validates provider, writes status conditions).
- **Flux source-controller `GitRepository`** is fully reconciled with rich status (artifact revision).

kollect has configuration objects (`KollectProfile`, `KollectSink`) that change infrequently and
work objects (`KollectTarget`, `KollectInventory`) that drive continuous collection and export.

## Decision

| Category | Kinds | Controller | Status | Validation |
| --- | --- | --- | --- | --- |
| Static config | `KollectProfile`, `KollectSink`, `KollectScope` | None | None (or minimal metadata only) | CEL `x-kubernetes-validations`, validating webhook |
| Reconciled | `KollectTarget`, `KollectInventory`, `KollectPublication` | Yes | Full conditions + `observedGeneration` | Same + runtime SAR checks |

Rationale (Flux-aligned):

- Cuts controllers and status write churn for rarely changing config.
- Profile/Sink edits still trigger dependent reconciles via secondary watches on referencing objects.
- `spec.suspend` on **reconciled** kinds only; static objects are always "active" when referenced.

**Explicitly reject** reconciling `KollectProfile`/`KollectSink` like ESO SecretStore — the
validation benefit does not justify status/etcd churn for kollect's read-mostly config.

## Consequences

### Positive

- Fewer moving parts, fewer leader-election reconciler loops.
- Clear mental model: config CRDs are like Flux Providers; workload CRDs are like GitRepositories.

### Negative

- Invalid sink credentials are discovered at export time (Target/Inventory reconcile), not at
  Profile/Sink apply time — mitigate with optional dry-run validation webhook later.
- `kubectl wait --for=condition=Ready` does not apply to Profile/Sink.

## Open questions

- **OPEN:** Optional **connection test** Job or `KollectSink` annotation trigger for pre-flight
  validation without a full reconciler?
- **OPEN:** Should static kinds get a **generation annotation** bump when dependents must re-export?
