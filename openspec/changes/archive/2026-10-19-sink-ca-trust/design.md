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

`internal/sink` gets two exported functions, called in this order by `resolveCAPEM` in
`build_context.go:215-245` (operator) and `internal/pipeline/wire.go:143-171` (CLI):

1. a spec check, before any Secret is fetched: `caBundle` together with `caSecretRef` is a terminal
   error (so a both-set spec never fails on a missing Secret first);
2. key selection, after each caller fetched the Secret its own way: the first non-empty key of
   `tls.crt`, `ca.crt`, `ca.pem`, else a terminal error naming the Secret and the expected keys
   (never the values).

Parity between operator and CLI is defined at these two functions: they return the same error
values, and each caller may wrap them with its own context (the CLI prefixes, `wire.go:496`,
`:506`). Secret-lookup errors (not found) stay source-specific and are not part of the parity.

The both-set and missing-key errors apply to every sink type, because the resolver is shared and
the same silent fallback exists for all of them; this is release-noted. PEM parsing stays per type
(SCT-4 for the four types here).

## Signatures

The CA reaches each package through its existing constructor and probe entry points:
`s3.NewBackend`, `s3.TestConnection`, `gcs.NewBackend`, `gcs.TestConnection`,
`postgres.NewBackend`, `postgres.TestConnection`, `mongodb.NewBackend`, `mongodb.TestConnection`
gain a `caPEM []byte` parameter, and `registry.go` / `probe.go` pass `ctx.CAPEM` /
`buildCtx.CAPEM`. Each package's `ConfigFromSpec` also gains `caPEM`, and every CA rule that needs
no parsed client config (PEM, S3 spec scheme, DSN/URI keys) runs there; the rules that need the
parsed config (S3 effective endpoint, pgx sslmode classification, MongoDB backstop) run in the one
client-building function both probe and export call (`newClient`, `newGuardedPool`, a new MongoDB
options builder). So probe and export reject the same input with the same error.

## Postgres

After `pgxpool.ParseConfig(dsn)` (`pool.go:17`), when a CA is configured:

1. Before parsing, reject a root certificate from any source pgx would read, naming the source
   (`sslrootcert` in the DSN, keyword or URL form; `sslrootcert` in the service-file entry the DSN
   or `PGSERVICE` selects, from `PGSERVICEFILE` or pgx's default service file, merged at
   `pgconn/config.go:449-452`; `PGSSLROOTCERT`; `~/.postgresql/root.crt` when it exists,
   `pgconn/defaults.go:58`) and `spec.tls`, never the DSN. Checking first means an unreadable or
   malformed root file is reported as the conflict, not as the static `invalid connection string`
   (`pool.go:46-55`). The backstop in step 2 stays for any source this check misses.
2. Before parsing, read the sslmode from the DSN keys (keyword or URL form) and `PGSSLMODE`
   (orchestrator decision 2026-10-19, option B). Unset in both → add `sslmode=verify-full` to the
   DSN (keyword: append the pair; URL: add the query parameter) before `pgxpool.ParseConfig`, so
   pgx's default `prefer` and its `sslnegotiation=direct` rewrite never apply. Explicit `disable`,
   `allow` or `prefer` → reject, naming the mode, its source (DSN or `PGSSLMODE`) and the fix
   (`sslmode=verify-full`, `verify-ca` or `require`; `verify-ca` when the DSN host does not match
   the certificate SAN). An accepted mode is pinned the same way, in front of the DSN keys
   (stage-B fix 8.5): pgx parses `require` and `prefer` with `sslnegotiation=direct` to the same
   shape, so no backstop could tell them apart if the key read and pgx disagreed. A later DSN mode
   the key read missed still wins in pgx; it is never weaker than the pinned one (plaintext is
   rejected below, `prefer`+`direct` gets the `require` chain check, and verify-full rejects a
   config without the hostname check). Then parse. Backstops: a parsed `TLSConfig == nil` or a fallback with a nil
   `TLSConfig` → reject (a weak mode the key read missed); a parsed `TLSConfig.RootCAs` that is
   non-nil before kollect sets it → reject as a conflict.
3. For the main config and every fallback with a TLS config: set `RootCAs` to the appended pool. A
   config with `InsecureSkipVerify` and no `VerifyPeerCertificate` (`require` without a root) gets
   the `verify-ca` chain check installed (chain against the pool, no hostname), as pgx does for
   `require` with `sslrootcert` (`pgconn/config.go:945-955`). `verify-ca` keeps pgx's callback,
   which reads `RootCAs` at handshake time. `verify-full` keeps `ServerName`.

`PGSSLMODE` / `PGSSLROOTCERT` in the process environment feed pgx's defaults (the operator pod
sets neither; the CLI does not load database sinks, proposal non-goals). Step 2 sees the effective
mode after env defaults, and the upgrade applies only when both are unset.

`redactedConnectError` (`pool.go:69-76`) keeps its static text but adds a static suffix when the
cause chain holds an `x509` verification error, so a wrong CA is diagnosable without the host.

## MongoDB

When a CA is configured, read the URI's query keys without DNS (the driver's `connstring` parse
resolves TXT and SRV records for `mongodb+srv` at `connstring.go:994`, `:1016`, so it is not used
for this check), matching key names case-insensitively as the driver does (`connstring.go:348`)
and treating a repeated key as the driver does (0.1 records which): reject
`tlsCAFile`/`sslCertificateAuthorityFile`; reject TLS off (`mongodb://` without `tls=true`/`ssl=true`
or another option that turns TLS on, e.g. `tlsCertificateKeyFile`, `connstring.go:596-640`; or
`tls=false`/`ssl=false` on either scheme); reject `tlsInsecure`/`sslInsecure` set to anything but
`false` (a malformed value is rejected as the driver would). Then `ApplyURI`, and as a backstop reject a driver-built `TLSConfig` that is nil or already
carries `RootCAs` or `InsecureSkipVerify`. Clone it and set `RootCAs` to the appended pool, keeping
client-certificate settings from the URI. Error text names the URI parameter, never the URI.

## S3 and GCS

`newClient` (`s3/backend.go:124-149`) clones the netguard transport (`netguard.HTTPClient(0)`, which
keeps the guarded `DialContext` and `Proxy = nil`) and sets `TLSClientConfig.RootCAs` to the
appended pool with `MinVersion: tls.VersionTLS12`. A spec endpoint with scheme `http` and a CA is
rejected in `ConfigFromSpec`. For `s3://` endpoints kollect sets no `BaseEndpoint`
(`s3/backend.go:141-145`), so the SDK may take one from `AWS_ENDPOINT_URL`, `AWS_ENDPOINT_URL_S3`
or shared config, and the S3-specific source (`AWS_ENDPOINT_URL_S3`, a `services` section) is
resolved inside `s3.NewFromConfig`, not in the loaded `aws.Config`. With a CA configured, `newClient`
builds the S3 client and then reads `client.Options().BaseEndpoint`, rejecting `http` before it
returns the client. The client's `CheckRedirect` refuses a redirect to a non-HTTPS URL when a
CA is configured.

## Compatibility

| Before (v0.21.x) | After (v0.22.0) |
| --- | --- |
| CA ignored; private-CA server fails verification (S3, GCS, MongoDB with TLS, Postgres `verify-*`) | Verifies with system roots plus the CA |
| MongoDB `tlsCAFile`, TLS off, or `tlsInsecure`/`sslInsecure` + CA: the URI's settings used | Terminal error |
| Postgres `sslmode` unset + CA (pgx default `prefer`): TLS without verification | `verify-full`: chain and hostname verified; a DSN host that does not match the certificate SAN fails (escape hatch: explicit `sslmode=verify-ca`) |
| Postgres explicit `sslmode=require` + CA: TLS without verification | chain verified (libpq `verify-ca` rule) |
| Postgres explicit `sslmode=prefer`/`allow`/`disable` + CA: CA ignored | Terminal error |
| `sslrootcert` / `tlsCAFile` in the connection string + CA: the connection string's CA used | Terminal error |
| Secret with no CA key: no CA; system roots (Git, GitLab, S3, GCS), plaintext (Kafka, NATS without other TLS material), connection-string TLS (Postgres, MongoDB) | Terminal error (all types) |
| `caBundle` and `caSecretRef` both set: `caBundle` used (Git/GitLab rejected it already) | Terminal error (all types) |

Rollback to v0.21.x restores the old behaviour; nothing is persisted.
