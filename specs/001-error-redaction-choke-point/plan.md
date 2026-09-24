# Implementation Plan: Error-redaction choke-point

**Branch**: `fm/kollect-b5-redaction` (feature id `001-error-redaction-choke-point`) | **Spec**: [spec.md](./spec.md)

## Summary

Three register findings share one root: credential-bearing text reaches persisted Kubernetes objects (status conditions, Events) without a single choke-point, because parse sites `%w`-wrap raw `*url.ParseError` and five writers persist `err.Error()` verbatim. Fix: one shared `internal/redact` package (userinfo regex for every scheme + verbatim secret masking), applied at the message sinks of the controller package; static, endpoint-free messages at the seven `url.Parse` sites; NATS rejects URL userinfo at config time; the collect scrubber adds `authorization`/`bearer` stem matching.

## Technical Context

- **Language**: Go 1.26 (repo pin), controller-runtime project (kubebuilder layout)
- **Testing**: stdlib `testing` + existing fake-client controller tests; no new deps
- **Target**: operator binary; no API/CRD schema change
- **Arch rules**: `.go-arch-lint.yml` — `internal/redact` joins the `shared` component (leaf, no kollect deps); `collect`, `sink`, `controller` may all depend on `shared` via `commonComponents`

## Key Changes

1. **`internal/redact/redact.go` (new)** — the shared contract:
   - `Text(msg string, secrets ...string) string`: masks `scheme://user[:pass]@` userinfo for any scheme; replaces non-empty secret values verbatim.
   - `Error(err error, secrets ...string) error`: nil-preserving; returned error's `Error()` is the redacted text, `Unwrap()` keeps the original so `errors.Is/As`, `ClassOf`, `IsTerminal`, `apierrors.Is*` are unchanged.
2. **Controller choke-point (writers)** — route message text through `redact.Text` at the final setters only (one place per surface):
   - `internal/controller/events.go` (`recordWarning`, `recordNormal`) — every Event.
   - `internal/controller/sink_status.go` (`setSinkReachableCondition`, `setSyncedCondition`).
   - `internal/controller/per_sink_export.go` (`setSinkExportSynced`).
   - `internal/controller/family_sink_connection.go` (`setConnectionFailed`, `setConnectionVerified`).
   - `internal/controller/kollectconnectiontest_controller.go` (`setProbeFailed`).
   - Degraded setters: `kollectinventory_controller.go` `setInventoryDegraded`, `kollectclusterinventory_controller.go` `setDegraded`.
3. **Static parse messages (K-23 leak vector)** — replace `fmt.Errorf("parse endpoint: %w", err)` with static sentinel errors (host/userinfo never echoed):
   - `internal/sink/git/config.go` (`ConfigFromSpec`, `parseEndpoint`), `internal/sink/git/export.go` (`parseRemote`), `internal/sink/git/cli_resolve.go` (`guardResolution`), `internal/sink/gitlab/config.go` (`ConfigFromSpec`), `internal/sink/gitlab/client.go` (`APIBaseURL`).
4. **NATS (K-25)** — `internal/sink/nats/config.go`: `url.Parse` the resolved URL; parse failure → static error; non-empty `u.User` → static "must not embed credentials" error. `connect.go`: wrap connect error via `redact.Error`.
5. **Scrubber (K-24)** — `internal/collect/scrub.go`: case/separator-insensitive normalization (strip `-`, `_`, `.`), add exact keys `authorization`, `bearer`, and a high-risk stem set (`authorization`, `bearer`) matched by prefix, closing `authorizationHeader`-style keys. Existing suffix rules unchanged.

## Scope decisions

- `git/redact.go` keeps its narrower http(s)-only regex and behaviour (ssh userinfo preserved by design, pinned by `redact_test.go`); the shared package uses the wider all-scheme regex for persisted status text. Duplication is deliberate (different trust contexts); the test lock pins both.
- Log lines (`log.Error(exportErr, ...)`) keep the raw error: scope of K-23 is status/Events; postgres/git already redact at source.
- Verbatim resolved-secret masking at the controller writers is NOT plumbed (secret values are not on the writer path); the register's regex-first fix plus static parse messages closes the proven vector. Sink-side secret masking stays where the values are known (git).
- No coordination with other batches needed; B4's kafka changes do not overlap these files.

## Testing Lock

- `internal/redact/redact_contract_test.go`: table-driven contract — every scheme with userinfo, multiple occurrences, multiline, secret values, empty/nil passthrough, byte-identical no-op.
- `internal/controller` test: each choke-point setter receives an error embedding `https://user:s3cret@host` and a `ClassError` — message on the persisted object is redacted; classification/reason unchanged.
- `internal/sink/git` + `internal/sink/gitlab` parse-site tests: malformed credential-bearing endpoint → static message, no echo.
- `internal/sink/nats`: userinfo-in-URL rejected at `ConfigFromSpec`; malformed URL → static text.
- `internal/collect/scrub_test.go` cases: `authorization`, `Authorization`, `X-Authorization`, `authorizationHeader`, `bearerToken` redacted; benign keys survive.

## Gates

`go build ./...`, `go test ./...` (unit, no envtest), `golangci-lint run`, `go-arch-lint check`, `gofmt`/`go vet` per repo Taskfile.
