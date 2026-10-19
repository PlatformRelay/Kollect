# Design

Required by `openspec/config.yaml` (credential and compatibility change: a sink that set a CA
changes behaviour on upgrade).

## One pool rule

A new helper in `internal/sink/sinktls` returns `x509.SystemCertPool()` (or an empty pool when the
platform has none) with the sink's CA appended, and an error when the PEM adds no certificate. It is
the same rule go-git applies to Git/GitLab `CABundle` (`plumbing/transport/http/common.go:262-275`),
except that unparseable PEM is an error here where go-git ignores the return value. `sinktls.FromSpec`
(Kafka, NATS, Git probe) is not changed (proposal non-goal).

## CA resolution, shared

`internal/sink` gets one exported function that picks the CA from a `TLSSpec` plus the referenced
Secret's data: rejects `caBundle` together with `caSecretRef`; returns `caBundle`; else the first
non-empty key of `tls.crt`, `ca.crt`, `ca.pem`; else an error naming the Secret and the expected
keys (never the values). `resolveCAPEM` in `build_context.go:215-245` (operator) and
`internal/pipeline/wire.go:143-171` (CLI) both call it after fetching the Secret their own way.
Errors from it are `kollecterrors.Terminal`.

The both-set and missing-key errors apply to every sink type, because the resolver is shared and
the same silent fallback exists for all of them; this is release-noted. PEM parsing stays per type
(SCT-4 for the four types here).

## Signatures

The CA reaches each package through its existing constructor and probe entry points:
`s3.NewBackend`, `s3.TestConnection`, `gcs.NewBackend`, `gcs.TestConnection`,
`postgres.NewBackend`, `postgres.TestConnection`, `mongodb.NewBackend`, `mongodb.TestConnection`
gain a `caPEM []byte` parameter, and `registry.go` / `probe.go` pass `ctx.CAPEM` /
`buildCtx.CAPEM`. Validation of the CA against the DSN/URI happens in each package's
`ConfigFromSpec` path, so probe and export reject the same input with the same message.

## Postgres

After `pgxpool.ParseConfig(dsn)` (`pool.go:17`), when a CA is configured:

1. Reject when the DSN sets `sslrootcert` (keyword or URL form), checked on the raw DSN keys before
   parsing so the message can name the parameter; the error names `sslrootcert` and
   `spec.tls`, never the DSN.
2. Classify the parsed config: `TLSConfig == nil` (sslmode `disable`) or any fallback with a nil
   `TLSConfig` (`allow`, `prefer`, unset) → reject, naming the effective sslmode class and the fix
   (`sslmode=verify-full`, `verify-ca` or `require`).
3. For the main config and every fallback with a TLS config: set `RootCAs` to the appended pool. A
   config with `InsecureSkipVerify` and no `VerifyPeerCertificate` (`require` without a root) gets
   the `verify-ca` chain check installed (chain against the pool, no hostname), as pgx does for
   `require` with `sslrootcert` (`pgconn/config.go:945-955`). `verify-ca` keeps pgx's callback,
   which reads `RootCAs` at handshake time. `verify-full` keeps `ServerName`.

`PGSSLMODE` / `PGSSLROOTCERT` in the process environment feed pgx's defaults (the CLI runs in CI
environments). Step 2 sees the effective mode after env defaults; step 1 also rejects a parsed
config whose `TLSConfig.RootCAs` is non-nil before kollect sets it (a CA from `PGSSLROOTCERT`).

`redactedConnectError` (`pool.go:69-76`) keeps its static text but adds a static suffix when the
cause chain holds an `x509` verification error, so a wrong CA is diagnosable without the host.

## MongoDB

Parse the URI's TLS keys (the driver's `connstring` parse) before connecting, when a CA is
configured: reject `tlsCAFile`/`sslCertificateAuthorityFile`, reject TLS off (no `tls=true`/`ssl=true`
on `mongodb://`, or `tls=false`/`ssl=false` on either scheme), reject `tlsInsecure=true`. Then
`ApplyURI`, take the driver-built `TLSConfig`, clone it and set `RootCAs` to the appended pool,
keeping client-certificate settings from the URI. Error text names the URI parameter, never the URI.

## S3 and GCS

`newClient` (`s3/backend.go:124-149`) clones the netguard transport (`netguard.HTTPClient(0)`, which
keeps the guarded `DialContext` and `Proxy = nil`) and sets `TLSClientConfig.RootCAs` to the
appended pool with `MinVersion: tls.VersionTLS12`. An endpoint with scheme `http` and a CA is
rejected in `ConfigFromSpec`. `s3://` endpoints use the AWS default HTTPS endpoint and accept the
CA.

## Compatibility

| Before (v0.21.x) | After (v0.22.0) |
| --- | --- |
| CA ignored; private-CA server fails verification (S3, GCS, MongoDB with TLS, Postgres `verify-*`) | Verifies with system roots plus the CA |
| Postgres `sslmode` unset/`prefer`/`require` + CA: TLS without verification | `require`: chain verified; unset/`prefer`/`allow`/`disable`: terminal error |
| `sslrootcert` / `tlsCAFile` in the connection string + CA: the connection string's CA used | Terminal error |
| Secret with no CA key: no CA, system roots (all types) | Terminal error (all types) |
| `caBundle` and `caSecretRef` both set: `caBundle` used (Git/GitLab rejected it already) | Terminal error (all types) |

Rollback to v0.21.x restores the old behaviour; nothing is persisted.
