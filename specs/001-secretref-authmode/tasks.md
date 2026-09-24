# Tasks: B2 — Admission secretRef guard + auth-mode enum

**Input**: `specs/001-secretref-authmode/spec.md`, `plan.md`

**Test lock**: webhook envtest cross-ns reject/allow + startup flag enum validation.

## Phase 1 — K-04 admission guard

- [x] **T001** [US1] Add `internal/validation/secret_ref_namespace.go`: process-wide
  allowlist (`SetAllowedSecretRefNamespaces`, `SecretRefNamespaceAllowed`) and
  `ValidateSecretRefNamespaces(spec *KollectSinkSpec, sinkNamespace string)
  field.ErrorList` covering `spec.secretRef`, `spec.tls.caSecretRef`,
  `spec.git.auth.secretRef`, `spec.postgres.databaseRef`,
  `spec.mongodb.databaseRef`, `spec.bigquery.secretRef`, `spec.nats.secretRef`,
  `spec.kafka.secretRef`.
- [x] **T002** [US1] Unit tests `internal/validation/secret_ref_namespace_test.go`:
  empty-namespace allowed, same-namespace allowed, cross-namespace rejected with
  the field path, allowlisted namespace allowed, every reference path covered.
- [x] **T003** [US1,US2] Wire the guard into the three family-sink validators in
  `internal/webhook/v1alpha1/family_sink_webhook.go`.
- [x] **T004** [US1,US2] envtest `internal/webhook/v1alpha1/secret_ref_namespace_envtest_test.go`:
  cross-ns reject (allowlist empty) and allow (namespace allowlisted), driven
  through the real API server + webhook.
- [x] **T005** [US2] Flag `--allow-secret-ref-namespaces` in `cmd/startup_flags.go`;
  wire `validation.SetAllowedSecretRefNamespaces` in `cmd/main.go`.
- [x] **T006** [US2] Helm: `allowSecretRefNamespaces` in `values.yaml`,
  `--allow-secret-ref-namespaces` render in `templates/deployment.yaml`, and a
  helm-unittest.
- [x] **T007** [US1] Docs: note the cross-namespace rejection in
  `docs/crds/kollectsnapshotsink.md` and the allowlist in
  `docs/security/security-architecture.md`.

## Phase 2 — K-12 auth-mode enum

- [x] **T008** [US3] Add `validateInventoryAuthMode` in `cmd/startup_flags.go`
  accepting only `kubernetes`/`disabled`; call it in `cmd/main.go` before use and
  fail startup on an invalid value.
- [x] **T009** [US3] Make `internal/inventory/auth.go` `AuthDisabled()` match only
  `disabled` (drop the undocumented `none` alias).
- [x] **T010** [US3] `cmd/main.go`: `RequireInventoryGet` true for every
  non-disabled mode.
- [x] **T011** [US3] `cmd/startup_flags_test.go`: invalid mode → error; valid modes
  → nil; flag round-trip for `--allow-secret-ref-namespaces`.

## Phase 3 — Verification

- [x] **T012** `gofmt`, `go vet`, `make lint`/golangci-lint, `task arch-lint`.
- [x] **T013** `go test ./internal/validation/... ./internal/webhook/... ./cmd/...
  ./internal/inventory/...` with envtest assets; then the broader `make test` set
  as time allows.
- [x] **T014** Adversarial self-review of the diff; fix findings.
- [x] **T015** Close out task-by-task with evidence.

Evidence: `specs/001-secretref-authmode/evidence/B2.md`.
