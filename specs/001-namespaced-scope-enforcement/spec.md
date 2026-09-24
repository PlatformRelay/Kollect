# Feature Specification: Namespaced Scope Enforcement (Batch B3)

**Feature Branch**: `fm/kollect-b3-scope`

**Created**: 2026-09-24

**Status**: Draft

**Input**: Kollect six-review unify report (register section 4, batch B3 section 6) — findings K-05, K-06, K-08, K-09.

## Background

`KollectScope` is the tenancy ceiling for namespaced collection. Four defects let
collection continue past what the ceiling permits, or delay pickup of ceiling
changes:

- **K-05 (HIGH)** — when `scopeCheck.enforceTarget` denies a namespaced
  `KollectTarget`, the reconciler writes Degraded and returns **without**
  `Engine.UnregisterTarget`. `RegisterTarget` froze `effectiveNamespaces` at
  registration and `matchesTarget` gates dispatch on that frozen set, so a
  scope created/tightened after the target went Ready keeps exporting from
  now-forbidden namespaces. The cluster analogue does unregister
  (`cluster_scope_enforce.go`).
- **K-06 (HIGH)** — `EffectiveNamespaceSet` returns `nil` for an empty result
  and `namespaceMatches` treats nil/empty as "no restriction", falling back to
  selector matching **without re-applying the ceiling**. A selector that matches
  nothing at registration, or a ceiling that excludes every matched namespace,
  silently disables the ceiling and lets denied namespaces reach the sink. The
  cluster-scoped path is fail-closed.
- **K-08 (MEDIUM)** — the namespaced target controller watches
  `KollectTarget`/`KollectProfile` but not `KollectScope`; scope changes are
  picked up only by the 60s Ready requeue, and a K-05-degraded target has no
  requeue at all.
- **K-09 (MEDIUM)** — the namespaced `KollectInventory` webhook validates
  intervals against the scope floor but never calls
  `scope.ValidateInventoryFamilySinkRefs`; the reconcile path and the
  cluster-scoped webhook both do. Inventories referencing sinks outside the
  allowlist are admitted and only degraded later.

## User Scenarios & Testing

### US1 — Scope tightening stops collection immediately (P1, K-05)

As a platform operator, after I create or tighten a `KollectScope` in a tenant
namespace, the already-Ready `KollectTarget` there must stop collecting and
exporting from the denied namespaces — no items may continue to reach sinks.

**Independent test**: envtest — register target Ready with no scope, engine
has items; create denying scope; reconcile; assert engine reports no
registration (`ItemCount == 0`, target absent) and no further items reach the
sink path. (Batch test lock: "register→deny→no items".)

### US2 — Empty effective set fails closed under a ceiling (P1, K-06)

As a platform operator, when the namespace ceiling excludes every namespace a
target's selector matches (effective set becomes empty), the target must
collect **nothing** — an empty effective set must never fall back to
unrestricted selector matching.

**Independent test**: unit over `namespaceMatches`/`RegisterTarget` contract —
ceiling supplied + empty effective ⇒ no dispatch match for any namespace,
including previously denied ones; no-ceiling targets keep selector semantics.
(Batch test lock: "empty-effective fail closed".)

### US3 — Scope changes re-trigger reconcile (P2, K-08)

As a platform operator, creating, editing or deleting a `KollectScope` must
enqueue the `KollectTarget`s in that scope's namespace immediately, not on the
next 60s resync (and not never, once degraded).

**Independent test**: unit over the watch map function — scope event enqueues
exactly the targets in that namespace (cluster mapper mirrored).

### US4 — Admission rejects off-allowlist sink refs (P2, K-09)

As a platform operator, a `KollectInventory` created or updated in a scope-
enforced namespace whose family sink refs violate the scope allowlist must be
rejected at admission, not admitted-then-degraded.

**Independent test**: webhook validator unit/envtest — violating ref ⇒ denied
with scope-violation error; allowed ref and non-enforced namespaces ⇒ admitted.
(Batch test lock: "webhook sink-ref".)

## Requirements

- **FR-1 (K-05)**: the namespaced target reconcile deny branch must call
  `Engine.UnregisterTarget` (all deny reasons: lookup failure, GVK denied,
  namespace denied) mirroring the cluster-scoped enforcement behaviour.
- **FR-2 (K-06)**: `targetState` must carry whether a scope ceiling was
  supplied; with a ceiling and an empty effective set, dispatch matching
  returns no match. The `metadata.name` pin stays valid for the
  cluster-synthetic path; ceiling-free targets keep today's selector fallback.
- **FR-3 (K-08)**: `KollectTargetReconciler.SetupWithManager` must
  `Watches(&KollectScope{}, …)` with a mapper enqueueing targets in the scope's
  namespace (mirrors `mapClusterScopeToClusterTargets`; no filtering to the
  lowest-named scope since any scope write can change which one `scope.Load`
  picks).
- **FR-4 (K-09)**: namespaced `KollectInventory` create/update validation must
  call `scope.ValidateInventoryFamilySinkRefs` under an enforced binding and
  return `validation.ScopeViolationErrors`.
- **FR-5**: no behaviour change for ceiling-free (no-scope) targets or the
  cluster-scoped registration path.

## Test Lock (batch B3)

1. register→deny→no items (US1)
2. empty-effective fail closed (US2)
3. webhook sink-ref (US4)

## Out of scope

K-07 (multiple scopes per namespace) is batch B12 (captain call C-8);
cross-namespace secretRef (K-04) and inventory auth mode (K-12) are batch B2.
