# Implementation Plan: Error-redaction choke-point

**Branch**: `fm/kollect-b5-redaction` (feature id `001-error-redaction-choke-point`) | **Spec**: [spec.md](./spec.md)

## Summary

Three register findings share one root: credential-bearing text reaches persisted Kubernetes objects (status conditions, Events) without a single choke-point, because parse sites `%w`-wrap raw `*url.ParseError` and the controller's status/Event writers persist `err.Error()` verbatim. Fix: one shared `internal/redact` package (userinfo, query, header and DSN patterns + verbatim secret masking), applied at the message sinks of the controller package; static, endpoint-free messages at the fifteen error-returning `url.Parse` sites; NATS rejects URL userinfo at config time; the collect scrubber adds `authorization`/`bearer` stem matching.

## Technical Context

- **Language**: Go 1.26 (repo pin), controller-runtime project (kubebuilder layout)
- **Testing**: stdlib `testing` + existing fake-client controller tests; no new deps
- **Target**: operator binary; no API/CRD schema change (the `NatsSpec.URL` field description changes to document the userinfo rejection)
- **Arch rules**: `.go-arch-lint.yml` — `internal/redact` joins the `shared` component (leaf, no kollect deps); `collect`, `sink`, `controller` may all depend on `shared` via `commonComponents`

## Key Changes

1. **`internal/redact/redact.go` (new)** — the shared contract:
   - `Text(msg string, secrets ...string) string`: masks `scheme://user[:pass]@` userinfo for any scheme (spaces and Go-escaped quotes allowed inside a quoted URL), credential query-parameter values (including GitLab `private_token`), `Authorization: Bearer|Basic|Token|Digest` credentials and key=value DSN `password`/`passwd`/`pwd` values; replaces non-empty secret values verbatim. RE2 only; idempotent.
   - `Error(err error, secrets ...string) error`: nil-preserving; returned error's `Error()` is the redacted text, `Unwrap()` keeps the original so `errors.Is/As`, `ClassOf`, `IsTerminal`, `apierrors.Is*` are unchanged.
2. **Controller choke-point (writers)** — route message text through `redact.Text` at the final setters only (one place per surface):
   - `internal/controller/events.go` (`recordWarning`, `recordNormal`) — every Event.
   - `internal/controller/sink_status.go` (`setSinkReachableCondition`, `setSyncedCondition`).
   - `internal/controller/per_sink_export.go` (`setSinkExportSynced`).
   - `internal/controller/family_sink_connection.go` (`setConnectionFailed`, `setConnectionVerified`).
   - `internal/controller/kollectconnectiontest_controller.go` (`setProbeSucceeded`, `setProbeFailed`).
   - Degraded setters: `kollectinventory_controller.go` `setInventoryDegraded` and the all-sinks-failed branch of `updateStatus`, `kollectclusterinventory_controller.go` `setDegraded`.
   - Target setters: `conditions.go` `setTargetCondition` (redacts before its unchanged-condition skip check) and `kollectclustertarget_controller.go` `setClusterTargetCondition`.
   - `reconcile_guard.go` emits `ReconcilePanic` through `recordWarning`.
3. **Static parse messages (K-23 leak vector)** — replace `fmt.Errorf("parse endpoint: %w", err)` with static sentinel errors (host/userinfo never echoed):
   - `internal/sink/git/config.go` (`ConfigFromSpec`, `parseEndpoint`), `internal/sink/git/export.go` (`parseRemote`), `internal/sink/git/cli_resolve.go` (`guardResolution`), `internal/sink/git/validate.go` (`validateCloneURL`, `parseFileGitBarePath`, `canonicalCloneURL`), `internal/sink/git/connection.go` (`TestConnection`), `internal/sink/git/auth.go` (`buildAuthMethod`, `buildAuthMethodWithForce`), `internal/sink/git/gogit_ssh_guard.go` (`pinGoGitSSHResolution`), `internal/sink/gitlab/config.go` (`ConfigFromSpec`), `internal/sink/gitlab/client.go` (`APIBaseURL`), `internal/sink/gitlab/mr.go` (`ResolveProjectRef`).
4. **NATS (K-25)** — `internal/sink/nats/config.go`: `url.Parse` the resolved URL; parse failure → static error; non-empty `u.User` → static "must not embed credentials" error. `connect.go`: wrap connect error via `redact.Error` with the resolved token and password; the dialer is a parameter (`connectWith`) so a test can prove it.
5. **Scrubber (K-24)** — `internal/collect/scrub.go`: case/separator-insensitive normalization (strip `-`, `_`, `.`), add exact keys `authorization`, `bearer`, and a high-risk stem set (`authorization`, `bearer`) matched by prefix, closing `authorizationHeader`-style keys. Existing suffix rules unchanged.

## Scope decisions

- `git/redact.go` keeps its narrower http(s)-only regex and behaviour (ssh userinfo preserved by design, pinned by `redact_test.go`); the shared package uses the wider all-scheme regex for persisted status text. Duplication is deliberate (different trust contexts); the test lock pins both.
- Log lines (`log.Error(exportErr, ...)`) keep the raw error: scope of K-23 is status/Events; postgres/git already redact at source.
- Verbatim resolved-secret masking at the controller writers is NOT plumbed (secret values are not on the writer path); the register's regex-first fix plus static parse messages closes the proven vector. Sink-side secret masking stays where the values are known (git).
- No coordination with other batches needed; B4's kafka changes do not overlap these files.

## Testing Lock

- `internal/redact/redact_contract_test.go`: `TestText_contract` (every scheme with userinfo, quoted URLs, query/header/DSN carriers, multiple occurrences, multiline, secret values, empty passthrough, byte-identical no-op, prose controls), `TestText_idempotent`, `TestText_noCredentialSurvives`, and the `TestError_*` identity tests.
- `internal/controller/error_redaction_test.go`: one `Test<Writer>_redacts*` per writer listed in FR-002, each feeding a terminal error that embeds `https://user:s3cr3t@...` and asserting the persisted condition or Event is redacted with its reason unchanged.
- `internal/sink/git/static_parse_test.go` + `internal/sink/gitlab/static_parse_test.go`: one `Test<Site>_malformedEndpointIsStatic` per parse site, including a password with a space that `redact.Text` alone cannot mask.
- `internal/sink/nats/static_parse_test.go`: userinfo-in-URL and in-server-list rejected at `ConfigFromSpec`; malformed URL → static text. `internal/sink/nats/connect_test.go`: `TestConnectWith_redactsResolvedCredentials`.
- `internal/collect/scrub_test.go`: `TestScrubber_bearerCarriers` (`authorization`, `Authorization`, `X-Authorization`, `authorizationHeader`, `bearerToken` redacted; benign keys survive).

## Gates

`go build ./...`, `task lint`, `task verify`, `go test -race ./cmd/... ./internal/...` (with envtest assets), `task scrub`.
