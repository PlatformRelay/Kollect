# ADR-0206: API versioning and conversion strategy

> How Kollect's CRD API evolves: the pre-beta break-freely policy, the storage version, and the path to
> a stable conversion-webhook'd API.

**Theme:** 02 · API & tenancy · **Status:** Exploring

## Context

Kollect ships a growing CRD surface (`KollectProfile`, `KollectTarget`, `KollectInventory`, `KollectSink`,
`KollectScope`, cluster-scoped variants, `KollectConnectionTest` — [ADR-0201](0201-crd-model.md),
[ADR-0203](0203-namespaced-multi-tenancy.md)). All are `v1alpha1` today with a single served/stored
version and **no conversion webhook**. We have a stated break-freely posture for pre-beta
([ADR-0201](0201-crd-model.md)) but no recorded plan for how and when the API
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
  `task verify` ([ADR-0201](0201-crd-model.md)).

### Validation is webhook-first

Field-level guarantees come from CEL/declarative validation and the validating webhook
([ADR-0201](0201-crd-model.md), [ADR-0602](0602-error-taxonomy.md)) rather than from version skew — so
most "shape" changes are validation tightening, not version bumps.

### Path to beta (trigger decided 2026-06-05)

**Cut `v1beta1` at the v0.10 presentation gate** (or shortly after) — not before. Until then, `v1alpha1` churns freely.

1. Freeze the `v1alpha1` field set; introduce `v1beta1` as a new served version.
2. Set `v1beta1` as storage version; keep `v1alpha1` served for one release window.
3. Add a **conversion webhook** (controller-runtime conversion or hub-and-spoke conversion funcs) for
   `v1alpha1 ↔ v1beta1`; round-trip fuzz tests gate it.
4. Deprecate, then drop, `v1alpha1` after the documented window.

### Export data contract alignment (Read API)

The **export data contract** ([ADR-0405](0405-export-data-contract.md)) versions **independently** of
CRD API versions. Consumers (portals, SQL, Kafka subscribers, Read API) branch on envelope
`schemaVersion`, not on `apiVersion` of Kollect CRDs.

| Surface | Versioning today | Pre-beta target |
| --- | --- | --- |
| CRD schemas (`v1alpha1`) | Break freely until **v0.10 presentation gate** | `v1beta1` + conversion webhook |
| Wire envelopes (| Sink JSON + Read API | Bare `[]Item` / `NamespaceSummary` — **no envelope yet** | Versioned envelope per ADR-0405 milestone |
| Read API routes | `/v1alpha1/…` path prefix ([ADR-0408](0408-read-api-ui-architecture.md)) | Stable OpenAPI; response body carries `schemaVersion` |

Read API work ([ADR-0408](0408-read-api-ui-architecture.md)) must return the same envelope contract
as sink exports — never a divergent shape. HTTP path version (`/v1alpha1/`) and envelope
`schemaVersion` are orthogonal: path version tracks API route stability; envelope version tracks
payload semantics.

## Consequences

- Fast iteration now (no conversion burden) at the cost of forcing reinstalls for adopters of pre-beta
  builds — acceptable while there are no production adopters.
- The export **data contract** ([ADR-0405](0405-export-data-contract.md)) versions independently of the
  CRD API; a CRD bump need not break consumers and vice versa.
- Deferring conversion means the first beta is the moment we take on conversion-test infrastructure.

## Open questions

- **DECIDED :** Cut `v1beta1` at the **v0.10 presentation gate** milestone (revised from v0.1.0).
- **PARTIAL :** Wire envelopes (  Read API responses still pending the ADR-0405 pre-beta milestone.
- **OPEN:** Conversion approach: controller-runtime hub-spoke conversion vs none-with-storage-migration?
- **OPEN:** Do `status` subresource fields carry their own compatibility guarantees, or follow spec
  versioning? (Leaning: follow spec versioning.)
