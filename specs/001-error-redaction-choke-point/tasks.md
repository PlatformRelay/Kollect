# Tasks: Error-redaction choke-point

**Input**: `specs/001-error-redaction-choke-point/{spec,plan}.md` — findings K-23, K-25, K-24.

**Tests**: REQUIRED (test lock: shared redaction contract + static parse messages). Tests written first per task.

## Phase 1: Foundational — shared contract

- [x] T001 [P] Write contract test first, then implement `internal/redact` (`Text`, `Error` with identity-preserving `Unwrap`) in `internal/redact/redact.go` + `internal/redact/redact_contract_test.go`; register `internal/redact/**` under the `shared` component in `.go-arch-lint.yml`.

**Checkpoint**: `go test ./internal/redact/...` green; contract pinned.

## Phase 2: User Story 1 (P1) — choke-point at every persisted surface (K-23)

- [x] T002 [P] [US1] Route `recordWarning`/`recordNormal` messages through `redact.Text` in `internal/controller/events.go` + test in `internal/controller/events_redact_test.go`.
- [x] T003 [US1] Redact messages in `setSinkReachableCondition`/`setSyncedCondition` (`internal/controller/sink_status.go`) and `setSinkExportSynced` (`internal/controller/per_sink_export.go`) + tests asserting a `https://user:s3cret@host` payload never reaches the condition message and classification (reason, `IsTerminal`) is unchanged.
- [x] T004 [P] [US1] Redact messages in `familySinkConnection.setConnectionFailed`/`setConnectionVerified` (`internal/controller/family_sink_connection.go`) + test.
- [x] T005 [P] [US1] Redact messages in `KollectConnectionTestReconciler.setProbeFailed` (`internal/controller/kollectconnectiontest_controller.go`) + test.
- [x] T006 [P] [US1] Redact messages in `setInventoryDegraded` (`internal/controller/kollectinventory_controller.go`) and `KollectClusterInventoryReconciler.setDegraded` (`internal/controller/kollectclusterinventory_controller.go`) + tests.

**Checkpoint**: no writer named in K-23 persists unredacted error text; `go test ./internal/controller/...` green.

## Phase 3: User Story 2 (P2) — static parse messages (K-23/K-25)

- [x] T007 [P] [US2] Static sentinel errors at git parse sites: `internal/sink/git/config.go` (ConfigFromSpec, parseEndpoint), `internal/sink/git/export.go` (parseRemote), `internal/sink/git/cli_resolve.go` (guardResolution) + tests with `https://user:tok@bad host` asserting no echo.
- [x] T008 [P] [US2] Static sentinel errors at gitlab parse sites: `internal/sink/gitlab/config.go` (ConfigFromSpec), `internal/sink/gitlab/client.go` (APIBaseURL) + tests.
- [x] T009 [US2] NATS: reject userinfo-bearing and malformed URLs statically in `internal/sink/nats/config.go` (K-25) and wrap connect errors via `redact.Error` in `internal/sink/nats/connect.go` + tests in `internal/sink/nats/config_test.go`/`connect_test.go`.

**Checkpoint**: the seven `url.ParseError` `%w` sites return static text.

## Phase 4: User Story 3 (P3) — scrubber stems (K-24)

- [x] T010 [US3] Separator-insensitive key normalization + stems `authorization`/`bearer` matched exact/prefix in `internal/collect/scrub.go` + table test in `internal/collect/scrub_test.go` (benign control keys survive).

## Phase 5: Polish

- [x] T011 Run repo gates: `task fmt vet lint` (or golangci-lint), `go-arch-lint check`, `go build ./...`, full `go test ./...` unit suite.

## Dependencies

- T001 blocks T002–T009. T002–T006 parallel; T007–T009 parallel; T010 independent of 2–4. T011 last.
