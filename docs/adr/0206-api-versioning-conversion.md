# ADR-0206: API versioning and conversion strategy

> How kollect's CRD API evolves: the pre-beta break-freely policy, the storage version, and the path to
> a stable conversion-webhook'd API.

**Theme:** 02 · API & tenancy · **Status:** Exploring

## Context

kollect ships a growing CRD surface (`KollectProfile`, `KollectTarget`, `KollectInventory`, `KollectSink`,
`KollectScope`, cluster-scoped variants, `KollectHub`, `KollectConnectionTest` — [ADR-0201](0201-crd-model.md),
[ADR-0203](0203-namespaced-multi-tenancy.md)). All are `v1alpha1` today with a single served/stored
version and **no conversion webhook**. We have a stated break-freely posture for pre-beta
([ADR-0703](0703-platform-architecture-pivot.md)) but no recorded plan for how and when the API
stabilizes, how breaking changes are signaled, and how conversion will work. This ADR makes the policy
explicit and marks the open decisions.

## Decision (current policy)

### Pre-beta: break freely, signal clearly

While on `v1alpha1` and **before any beta tag**:

- CRD schemas may change without backward compatibility; no conversion webhook is maintained.
- Breaking schema changes are **not** marked `!`/`BREAKING CHANGE` in commits unless a tagged release
  already requires migration (per `AGENTS.md` commit policy) — but they **must** be called out in
  `CHANGELOG.md` and release notes.
- `v1alpha1` is the single served and **storage** version; codegen drift (CRDs, deepcopy) is gated by
  `task verify` ([ADR-0703](0703-platform-architecture-pivot.md)).

### Validation is webhook-first

Field-level guarantees come from CEL/declarative validation and the validating webhook
([ADR-0201](0201-crd-model.md), [ADR-0602](0602-error-taxonomy.md)) rather than from version skew — so
most "shape" changes are validation tightening, not version bumps.

### Path to beta (trigger decided 2026-06-05)

**Cut `v1beta1` at the `v0.1.0` feature-freeze** — not before. Until then, `v1alpha1` churns freely.

1. Freeze the `v1alpha1` field set; introduce `v1beta1` as a new served version.
2. Set `v1beta1` as storage version; keep `v1alpha1` served for one release window.
3. Add a **conversion webhook** (controller-runtime conversion or hub-and-spoke conversion funcs) for
   `v1alpha1 ↔ v1beta1`; round-trip fuzz tests gate it.
4. Deprecate, then drop, `v1alpha1` after the documented window.

## Consequences

- Fast iteration now (no conversion burden) at the cost of forcing reinstalls for adopters of pre-beta
  builds — acceptable while there are no production adopters.
- The export **data contract** ([ADR-0405](0405-export-data-contract.md)) versions independently of the
  CRD API; a CRD bump need not break consumers and vice versa.
- Deferring conversion means the first beta is the moment we take on conversion-test infrastructure.

## Open questions

- **DECIDED (2026-06-05):** Cut `v1beta1` at the **`v0.1.0` feature-freeze** milestone.
- **DECIDED (2026-06-05):** The export payload gains an explicit **`schemaVersion`** so consumers
  decouple from CRD versions entirely ([ADR-0405](0405-export-data-contract.md)).
- **OPEN:** Conversion approach: controller-runtime hub-spoke conversion vs none-with-storage-migration?
- **OPEN:** Do `status` subresource fields carry their own compatibility guarantees, or follow spec
  versioning? (Leaning: follow spec versioning.)
