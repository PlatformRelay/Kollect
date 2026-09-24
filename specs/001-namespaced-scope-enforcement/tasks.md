# Tasks: Namespaced Scope Enforcement (Batch B3)

**Input**: `specs/001-namespaced-scope-enforcement/`
**Tests**: mandatory per finding — the batch test lock names the behavioural test.

## Foundational (read-only orientation)

- [X] T001 Read cited code paths (done during planning; see plan.md Design)

## User Story 1 — K-05 scope-deny unregisters

- [x] T002 [P] [US1] Failing test: register namespaced target Ready with real engine (fake clients), create denying `KollectScope`, reconcile ⇒ engine unregistered, store emptied, late object not stored — `internal/controller/kollecttarget_scope_unregister_test.go`
- [x] T003 [US1] `r.Engine.UnregisterTarget(target.Namespace, target.Name)` (nil-engine safe) in the `enforceTarget` deny branch before `setDegraded` — `internal/controller/kollecttarget_controller.go`; T002 green

## User Story 2 — K-06 empty effective fails closed

- [x] T004 [P] [US2] Re-pinned `namespaceMatches` contract in `internal/collect/engine_helpers_test.go`: enforced+empty ⇒ deny except pin; non-enforced ⇒ unchanged selector/pin fallback
- [x] T005 [US2] `scopeEnforced` on `targetState`, set in `RegisterTarget` via `CeilingRestrictsNamespaces(opts.ScopeCeiling)`, honoured in `namespaceMatches` — `internal/collect/engine.go`, `internal/collect/collection_filter.go`; T004 green
- [x] T006 [P] [US2] Engine-level lock: unrestricted collect ⇒ re-register under ceiling with empty effective ⇒ count drops to 0 and stays 0 — `internal/collect/engine_scope_ceiling_test.go`

## User Story 3 — K-08 scope watch

- [x] T007 [P] [US3] Unit test for `mapScopeToTargets`: scope event ⇒ enqueues only same-namespace targets; foreign object type ⇒ nil — `internal/controller/kollecttarget_map_test.go`
- [x] T008 [US3] `Watches(&KollectScope{}, ...mapScopeToTargets)` in `SetupWithManager` + mapper with no-filter rationale (lowest-name rule) — `internal/controller/kollecttarget_controller.go`; T007 green

## User Story 4 — K-09 webhook sink-ref allowlist

- [x] T009 [P] [US4] Failing webhook test: enforced scope + `snapshotSinkRefs` allowlist + off-list ref ⇒ denied naming the ref; on-list ⇒ admitted; non-enforced ns ⇒ admitted — `internal/webhook/v1alpha1/kollectinventory_webhook_test.go`
- [x] T010 [US4] `scope.ValidateInventoryFamilySinkRefs` + `validation.ScopeViolationErrors` in `validate()` under `binding.Enforced` — `internal/webhook/v1alpha1/kollectinventory_webhook.go`; T009 green

## Polish / gates

- [x] T011 `go build ./...`, `go vet`, and `go test` green for `internal/collect`, `internal/controller` (envtest incl.), `internal/webhook/...`, `internal/scope`, `internal/validation` (KUBEBUILDER_ASSETS via `make echo-kubebuilder-assets`)
- [x] T012 `make lint` / `task arch-lint` clean; gofmt
- [x] T013 Adversarial self-review of the diff; fix findings
- [x] T014 Push branch, open PR (ready), report

## Dependencies

- T003 after T002; T005 after T004; T008 after T007; T010 after T009.
- US1/US2/US3/US4 are independent of each other (single lane, done in order).
