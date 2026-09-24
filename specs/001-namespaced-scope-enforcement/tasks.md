# Tasks: Namespaced Scope Enforcement (Batch B3)

**Input**: `specs/001-namespaced-scope-enforcement/`
**Tests**: mandatory per finding — the batch test lock names the behavioural test.

## Foundational (read-only orientation)

- [X] T001 Read cited code paths (done during planning; see plan.md Design)

## User Story 1 — K-05 scope-deny unregisters

- [ ] T002 [P] [US1] Add failing envtest: register namespaced target Ready with engine, create denying `KollectScope`, reconcile ⇒ engine unregistered and no items — `internal/controller/kollecttarget_scope_envtest_test.go`
- [ ] T003 [US1] Call `r.Engine.UnregisterTarget(target.Namespace, target.Name)` in the `enforceTarget` deny branch (nil-engine safe) — `internal/controller/kollecttarget_controller.go:119-127`; T002 goes green

## User Story 2 — K-06 empty effective fails closed

- [ ] T004 [P] [US2] Re-pin `namespaceMatches` contract in `internal/collect/engine_helpers_test.go`: enforced+empty ⇒ no match for any namespace incl. pin-only case per plan; non-enforced ⇒ unchanged selector/pin fallback (failing before impl)
- [ ] T005 [US2] Add `scopeEnforced` to `targetState`, set it in `RegisterTarget` from non-empty `ScopeCeiling`, and honour it in `namespaceMatches` (enforced+empty ⇒ deny except `metadata.name` pin) — `internal/collect/engine.go`; T004 goes green
- [ ] T006 [P] [US2] Engine-level test: register with ceiling excluding all matched namespaces ⇒ dispatch of an object in a denied namespace stores no item — `internal/collect/engine_test.go`

## User Story 3 — K-08 scope watch

- [ ] T007 [P] [US3] Failing unit test for `mapScopeToTargets`: scope event ⇒ enqueues only targets in the scope's namespace; foreign object type ⇒ nil — `internal/controller/kollecttarget_scope_envtest_test.go` (or unit file)
- [ ] T008 [US3] Add `Watches(&KollectScope{}, ...mapScopeToTargets)` to `SetupWithManager` and implement the mapper (mirror `mapClusterScopeToClusterTargets` rationale) — `internal/controller/kollecttarget_controller.go`; T007 goes green

## User Story 4 — K-09 webhook sink-ref allowlist

- [ ] T009 [P] [US4] Failing webhook test: enforced scope + family allowlist + off-list sink ref ⇒ denied; on-list ⇒ admitted; non-enforced ⇒ admitted — `internal/webhook/v1alpha1/kollectinventory_webhook_test.go`
- [ ] T010 [US4] Call `scope.ValidateInventoryFamilySinkRefs` + `validation.ScopeViolationErrors` in `validate()` under `binding.Enforced` — `internal/webhook/v1alpha1/kollectinventory_webhook.go`; T009 goes green

## Polish / gates

- [ ] T011 `go build ./...` + `go test ./internal/collect/... ./internal/controller/... ./internal/webhook/...` green (envtest via `make test` if assets available)
- [ ] T012 `make lint` / `task arch-lint` clean; gofmt
- [ ] T013 Adversarial self-review of the diff; fix findings
- [ ] T014 Push branch, open PR (ready), report

## Dependencies

- T003 after T002; T005 after T004; T008 after T007; T010 after T009.
- US1/US2/US3/US4 are independent of each other (single lane, done in order).
