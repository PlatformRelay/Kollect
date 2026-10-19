# Proposal

## Why

A sink's `spec.tls.caBundle` / `spec.tls.caSecretRef` is resolved for every sink type and then
ignored by four of them (backlog SINK-CA-UNUSED-01, release inventory 2026-10-19 item 11):

- The operator resolves the CA in `BuildContextFromSpec` (`internal/sink/build_context.go:38`,
  resolver at `:215-245`), so a missing CA Secret already fails reconcile. The CLI
  (`kollect-pipeline`) resolves it the same way in its own copy (`internal/pipeline/wire.go:143-171`,
  called at `:257`).
- `newS3Backend`, `newGCSBackend`, `newPostgresBackend` and `newMongoBackend`
  (`internal/sink/registry.go:120-157`) never pass `ctx.CAPEM`. Neither do the connection probes
  (`internal/sink/probe.go:91`, `:115`, `:151`, `:163`). No `RootCAs` or `sslrootcert` handling exists
  in `internal/sink/s3`, `gcs`, `postgres` or `mongodb`. The S3 client uses
  `netguard.HTTPClient(0)` with the default root pool (`internal/sink/s3/backend.go:127`); Postgres
  and MongoDB take TLS only from the DSN/URI (`internal/sink/postgres/pool.go:16-24`,
  `internal/sink/mongodb/backend.go:97`, `internal/sink/mongodb/connection.go:34`).
- So a private-CA S3/GCS endpoint or Postgres/MongoDB server fails verification, or (Postgres with
  the default `sslmode=prefer`) connects without verifying anything, while the sink's spec says it
  trusts a CA. `docs/REQUIREMENTS.md:90` (FR-EXP-7) claims CA trust for Postgres, which is false.
- Two more silent paths in the same resolver: a `caSecretRef` Secret that holds none of the CA keys
  resolves to no CA at all (`build_context.go:244`, `wire.go:170`), and a sink that sets both
  `caBundle` and `caSecretRef` gets `caBundle` (`build_context.go:225`), although the CRD says not
  to set both (`api/v1alpha1/kollectsink_types.go:342`) and the shared TLS helper's comment claims
  the opposite precedence (`internal/sink/sinktls/sinktls.go:24-26`).

Operator rulings: SINK-CA-UNUSED-01 → A, honour it (2026-10-04), and the CA semantics, outcome A
(2026-10-19, `agent-context/decisions.md`): append the CA to the system roots for all four types,
consistent with Git/GitLab; for Postgres and MongoDB `caSecretRef` wins, a DSN/URI that also names
a CA file is rejected with a clear message, DSN `sslmode` and verify settings still apply, and
verification is never turned off. The 2026-10-19 ruling supersedes the 2026-10-04 wording "trusts
that CA and only that CA": append never pins.

## What Changes

- SCT-1: S3 and GCS (the GCS sink is the S3 client, `internal/sink/gcs/backend.go:26-40`,
  `connection.go:13-21`) build their HTTPS transport with a root pool of system roots plus the
  sink's CA, on the netguard transport, for export and probe alike. An `http://` endpoint with a CA
  configured is rejected: the CA would never be checked.
- SCT-2: Postgres applies the CA to every TLS config pgx builds from the DSN (main and fallbacks),
  as system roots plus the CA. `sslmode=verify-full` and `verify-ca` keep their meaning;
  `sslmode=require` verifies the chain as `verify-ca` does, which is libpq's documented behaviour
  when a root certificate is configured. `disable`, `allow`, `prefer` and an unset `sslmode` with a
  CA configured are rejected (the CA would never be checked). A DSN that sets `sslrootcert` is
  rejected when the sink also sets a CA.
- SCT-3: MongoDB applies the CA the same way to the driver's TLS config. A URI that sets
  `tlsCAFile` / `sslCertificateAuthorityFile` is rejected when the sink sets a CA; so is a URI on
  which TLS is off (`mongodb://` without `tls=true`/`ssl=true`, or `tls=false`) or `tlsInsecure=true`.
- SCT-4: a CA that is not usable is a terminal error at backend build: PEM with no parseable
  certificate (the four types here), and, for every sink type because the resolver is shared, a
  `caSecretRef` Secret with none of the keys `tls.crt`, `ca.crt`, `ca.pem`, and `caBundle`
  together with `caSecretRef` (Git/GitLab already reject both, `internal/sink/git/tls.go:23-25`;
  Kafka and NATS change and are release-noted). A valid CA that does not sign a private
  server fails the connection with a message that names TLS verification and the configured CA,
  without the DSN, URI, host or credentials.
- SCT-5: the operator (reconcile export and `KollectConnectionTest` probe) and the CLI
  `kollect-pipeline` resolve the CA through one shared function and build the same client.
- SCT-6: docs: the sink reference pages, the upgrade notes, `docs/REQUIREMENTS.md` FR-EXP-7, the
  security architecture TLS section, and a release-note line for v0.22.0.

## Capabilities

### New Capabilities

- `sink-tls-trust`: how a sink's configured CA is applied to its client, for S3, GCS, Postgres and
  MongoDB.

### Modified Capabilities

None. `pipeline-sink-credentials` (archived change `2026-10-04-cli-sink-credential-refs`) requires
that the CLI resolves `tls.caSecretRef` as the operator does; SCT-5 keeps that and adds the
missing-key error to both.

## Impact

- Entry points: `KollectSnapshotSink` (types `s3`, `gcs`) and `KollectDatabaseSink` (types
  `postgres`, `mongodb`) `spec.tls.caBundle` / `spec.tls.caSecretRef`, plus the legacy
  `KollectSink`; reached through the export backend pool (`internal/sink/backend_pool.go:112`,
  `:145`), the connection probe (`internal/controller/kollectconnectiontest_controller.go:91`,
  `internal/controller/family_sink_connection.go:49`), and `kollect-pipeline collect`
  (`internal/pipeline/wire.go:494`, `:523`).
- Code: `internal/sink/registry.go`, `probe.go`, `build_context.go`, `sinktls/`, `s3/`, `gcs/`,
  `postgres/`, `mongodb/`, `internal/pipeline/wire.go`. No CRD schema change, no new dependency.
- Behaviour change, release-noted for v0.22.0: a sink that set a CA starts verifying with it, and a
  sink whose CA, DSN or URI conflicts as above stops exporting until fixed (today it exports while
  the CA is ignored). With append semantics a CA next to a publicly signed endpoint keeps working.
- Tests: unit tests per package; real TLS tests with a private CA per sink, integration-tagged
  because netguard refuses loopback outside that tag (`internal/sink/netguard/dialer.go:242-248`,
  `integration_policy.go:1-14`). `INTEGRATION_PACKAGES` (`Taskfile.yml:20`) already covers
  `./internal/sink/...` and `./internal/pipeline/...`.

## Non-goals

- Strict pinning (trust only the configured CA). Append never excludes public CAs; a separate
  opt-in field could add pinning later, recorded as the counterpoint to the 2026-10-19 decision.
- Kafka, NATS and the Git probe keep their current pool (`sinktls.FromSpec` builds a fresh pool
  with only the CA, `sinktls.go:44`, used by the Git TLS probe at `internal/sink/git/connection.go:56`),
  while Git/GitLab export appends through go-git. That inconsistency is recorded as a follow-up,
  not changed here. BigQuery is out of scope.
- `spec.tls.insecureSkipVerify` stays unsupported for these four types (ignored, as today,
  `docs/security/security-architecture.md:239-243`); this change SHALL NOT start honouring it.
  A DSN/URI that itself disables verification without a CA configured is unchanged (no K-14 gate
  on connection strings).
- Credential and CA rotation without restart (SECRET-ROTATION-01): the CA is still read at backend
  build (`internal/sink/backend_pool.go:263`).
- Client certificates (mTLS) from the sink spec.
- The test-only duplicate resolver `internal/controller/secret_ca.go:19-60` is not on any
  production path; deleting it is allowed but not required.

## Assumptions

- go-git appends `CABundle` to the system pool: `transportWithCABundle` calls
  `x509.SystemCertPool()` and appends (go-git v5.19.3,
  `plumbing/transport/http/common.go:262-275`, read 2026-10-19). This is the Git behaviour SCT-1..3
  match.
- pgx v5.11.0 builds TLS from `sslmode`/`sslrootcert` in `configTLS`
  (`pgconn/config.go:883-993`, read 2026-10-19): `prefer`/`allow` set `InsecureSkipVerify` with a
  plaintext fallback, `require` without a root certificate skips verification and with one falls
  through to `verify-ca`, `verify-ca` verifies the chain in `VerifyPeerCertificate` reading
  `tlsConfig.RootCAs` at handshake time, `verify-full` sets `ServerName`. SCT-2's rules rest on this.
- mongo-driver v1.17.10 turns `tls`/`ssl`, `tlsCAFile`/`sslCertificateAuthorityFile` and
  `tlsInsecure` into `ClientOptions.TLSConfig` in `ApplyURI`
  (`mongo/options/clientoptions.go:560-598`; keys in `x/mongo/driver/connstring/connstring.go:579-640`;
  `mongodb+srv` defaults TLS on at `:1000`); it has no `tlsAllowInvalidCertificates` /
  `tlsAllowInvalidHostnames` option. Read 2026-10-19.
- Go's `x509.SystemCertPool` on Linux reads `SSL_CERT_FILE` once per process; the "still trusts
  public CAs" tests rely on it to stand a test CA in for the public roots, in a subprocess.
