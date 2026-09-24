# Implementation Plan: Namespaced Scope Enforcement (Batch B3)

**Branch**: `fm/kollect-b3-scope` | **Spec**: `specs/001-namespaced-scope-enforcement/spec.md`

## Summary

Four small, independent fixes on the namespaced scope path, each landed with a
behavioural test from the batch test lock. No API change, no CRD change, no new
dependencies.

## Technical Context

- Go 1.x operator (controller-runtime), envtest unit suite (`make test` /
  `task test`), golangci-lint + go-arch-lint gates.
- Touches: `internal/controller` (K-05, K-08), `internal/collect` (K-06),
  `internal/webhook/v1alpha1` (K-09). No cross-layer imports added
  (webhook → `internal/scope` + `internal/validation` already exist).

## Design

### K-05 — unregister on scope deny (`kollecttarget_controller.go`)

In the `enforceTarget` deny branch (currently only `setDegraded`), call
`r.Engine.UnregisterTarget(target.Namespace, target.Name)` before returning,
nil-safe, mirroring the suspend branch and the cluster
`unregisterAll`-before-degrade pattern. This covers all three deny reasons
(lookup failure included: fail closed).

### K-06 — empty effective set fails closed under a ceiling (`internal/collect`)

- Add `scopeEnforced bool` to `targetState`.
- In `RegisterTarget`, set it from the supplied ceiling
  (`len(Allowed)+len(Denied) > 0`): a ceiling with both lists empty restricts
  nothing, so today's selector semantics remain correct there.
- `namespaceMatches` gains the flag: with `effective` empty and
  `scopeEnforced` true, only the `metadata.name` pin (cluster-synthetic path)
  can match; everything else returns false. Without enforcement the existing
  fallback is unchanged (FR-5).
- No fingerprint change needed: the flag is derived from inputs already inside
  the registration path (ceiling is not hashed today; effective set is, and a
  ceiling change that empties the set changes it).

### K-08 — scope watch (`kollecttarget_controller.go`)

`Watches(&KollectScope{}, handler.EnqueueRequestsFromMapFunc(r.mapScopeToTargets))`.
Mapper: type-assert the scope, list `KollectTarget`s in `scope.Namespace`,
enqueue all of them (any scope write can change which one `scope.Load` picks —
same no-filter rationale as `mapClusterScopeToClusterTargets`).
RBAC for scopes already exists; the degraded-no-requeue gap closes because
scope edits/deletes now enqueue.

### K-09 — sink-ref allowlist at admission (`kollectinventory_webhook.go`)

Inside the existing `binding.Enforced` block, after the interval-floor check:
`scope.ValidateInventoryFamilySinkRefs(binding.Scope, kollectdevv1alpha1.CollectInventorySinkBindings(&inv.Spec))`
→ `validation.InventoryInvalid(inv.Name, validation.ScopeViolationErrors(err))`,
mirroring `kollectclusterinventory_webhook.go`.

## Test plan (test lock)

1. **register→deny→no items** — envtest on the target reconciler: Ready with
   no scope + engine registered ⇒ create denying scope ⇒ reconcile ⇒ engine
   unregistered, `ItemCount == 0`.
2. **empty-effective fail closed** — `engine_helpers_test.go` rewritten to the
   new `namespaceMatches` contract (enforced-empty ⇒ deny, pin kept); plus a
   `RegisterTarget`-level test: ceiling excluding all matched namespaces ⇒
   dispatch matches nothing.
3. **webhook sink-ref** — validator unit test: enforced scope with a family
   allowlist, off-list ref ⇒ denied; on-list ⇒ allowed; non-enforced ⇒
   allowed.
4. **K-08 watch mapper** — unit test: scope event enqueues only same-namespace
   targets; wrong type ⇒ nil.

## Risks

- K-06 changes a behaviour pinned as intended in `engine_helpers_test.go` —
  the register explicitly re-pins it (tenancy bypass beats selector
  convenience under an enforced ceiling). Ceiling-free behaviour unchanged.
- `scopeEnforced` derived from non-empty ceiling lists, not from
  `binding.Enforced`: an enforced scope with empty lists is permissive by
  construction, so the distinction cannot leak. Noted for reviewers.
