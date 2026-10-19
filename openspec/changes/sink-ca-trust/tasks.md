# Tasks

Red first. In each group the test task comes before the implementation and records its red in
`evidence/G<N>.md`: the named test runs (`=== RUN <name>`) and fails on the target assertion, not on
a compile error, a missing Docker daemon or a skip. A stub with the signature from `design.md` and a
`not implemented` body is in scope so the tests compile. Each group closes in one commit (test and
implementation), so the tree is green at every commit and *Known red* stays empty unless a group is
split.

Each case below reads: given / when / then · the failure it exposes.

Test rules for every group:

- Certificates and keys are generated at test time (`crypto/x509`, ECDSA P-256) and never written
  to the repository; nothing under `testdata/` holds a private key (gitleaks). One test helper,
  `internal/sink/sinktls/sinktlstest` (a non-`_test.go` package so the four sink packages and
  `internal/pipeline` share it), issues CA X, CA Y, a stand-in public root P, and server
  certificates for `localhost` and `127.0.0.1` signed by any of them.
- Real TLS tests dial loopback, which netguard refuses outside the `integration` build tag
  (`internal/sink/netguard/dialer.go:242-248`, `integration_policy.go:1-14`). They carry
  `//go:build integration` as their first line, live in packages under `INTEGRATION_PACKAGES`
  (`Taskfile.yml:20`), and handle a missing Docker daemon with
  `integrationtest.IsDockerUnavailable` / `SkipDockerUnavailable` like
  `internal/sink/postgres/export_integration_test.go:34-45`. httptest-based ones need no Docker
  but still run only in that tier. Unit cases (no network) carry no build tag.
- "Still trusts public CAs" cases replace the system pool with root P by re-executing the test
  binary as a subprocess with `SSL_CERT_FILE=<P.pem>` and `SSL_CERT_DIR=` (an empty directory), so
  the rest of the package keeps the real pool; the parent asserts the child's exit status and its
  `=== RUN` line. Probe this in 0.1 before relying on it.
- Error-text assertions check for sentinel values put into the DSN/URI (host `sentinel-host`, user
  `sentinel-user`, password `sentinel-pass`, database `sentinel-db`) and SHALL find none of them.
- Each group records `go test -race -count=3` on the packages it changed.

## 0. Probe

- [ ] 0.1 Verify the load-bearing assumptions in `proposal.md` and record each as verified (command,
  versions, date) in `evidence/T0.1.md`; a failed one stops the loop as a spec finding:
  - pgx v5.11.0: the `verify-ca` callback reads `RootCAs` from the same `*tls.Config` that
    `ConnConfig.TLSConfig` / `Fallbacks[i].TLSConfig` point to after `pgxpool.ParseConfig`
    (`pgconn/config.go:957-990`), and whether fallbacks share or copy that pointer; which parsed
    shape each sslmode yields (TLSConfig nil, fallbacks with nil TLSConfig, `InsecureSkipVerify` with
    or without `VerifyPeerCertificate`); that `PGSSLMODE` / `PGSSLROOTCERT` reach the parse;
  - mongo-driver v1.17.10: a parse of the URI's TLS keys that does no DNS lookup for
    `mongodb+srv` (the SRV resolve at `x/mongo/driver/connstring/connstring.go:995-1002` must not run
    in validation); otherwise SCT-3's check parses the query string itself;
  - Go 1.26 on Linux: `SSL_CERT_FILE` in a child process replaces `x509.SystemCertPool()`;
  - Postgres image `postgres:16-alpine` accepts a test key mounted into the container (key owner
    and mode rules) and MongoDB's image accepts `--tlsMode requireTLS --tlsCertificateKeyFile`;
    record the command that worked.

## 1. Shared pool and CA resolution (SCT-4, SCT-5)

- [ ] 1.1 Tests, red against stubs of `sinktls.AppendedPool(pem []byte) (*x509.CertPool, error)` and
  the shared resolver in `internal/sink` (`design.md`, *CA resolution, shared*):
  - P1 given CA X, when `AppendedPool` builds the pool, then a leaf signed by X verifies · the CA is
    dropped;
  - P2 given CA X and, in a subprocess, `SSL_CERT_FILE` = root P, when a leaf signed by P is
    verified against the pool, then it verifies · replace instead of append (the `sinktls.FromSpec`
    rule, `sinktls.go:44`);
  - P3 given PEM `not a certificate`, an empty string, or a PEM holding only a `PRIVATE KEY` block,
    then an error that names `spec.tls` and does not contain the input · garbage accepted as "no CA";
  - R1 table through the shared resolver: no `tls` → nil; `caBundle` → it; Secret with `ca.crt` →
    it; with `tls.crt` and `ca.crt` → `tls.crt`; with `ca.pem` only → it; with only `other` → error
    naming the Secret and `tls.crt`, `ca.crt`, `ca.pem`, `kollecterrors.IsTerminal` true; `caBundle`
    and `caSecretRef` → terminal error · the silent fallback at `build_context.go:244` and the
    silent `caBundle` precedence at `:225` (both red today);
  - R2 extend `TestCLICredentials_matchOperatorResolver`
    (`internal/pipeline/sink_credentials_test.go:312`): compare error **text**, not only presence
    (`:384`); rename `"inline bundle wins"` (`:356`) to `"both sources rejected"` and expect the
    terminal error on both paths; add `"caSecret ca.pem"` and `"caSecret no key"` (`:354`) expects
    the same terminal error on both paths · operator and CLI drifting apart (record both as *Test
    changes* in `loop.md`).
- [ ] 1.2 Implement `AppendedPool` and the shared resolver; `resolveCAPEM` in
  `internal/sink/build_context.go:215-245` and `internal/pipeline/wire.go:143-171` call it. Fix the
  `sinktls.FromSpec` comment that claims `caSecretRef` precedence (`sinktls.go:24-26`).

## 2. S3 and GCS (SCT-1, SCT-4)

- [ ] 2.1 Tests, red against stubs that accept `caPEM` in `s3.NewBackend`, `s3.TestConnection`,
  `gcs.NewBackend`, `gcs.TestConnection` (registry and probe pass `ctx.CAPEM`):
  - unit S1 given endpoint `http://h/b` and CA X, when `s3.NewBackend` runs, then a terminal error
    naming `spec.tls` and `http`, and the counting transport saw no request · CA ignored over HTTP;
  - unit S2 given garbage CA PEM, then a terminal error before any request · garbage accepted;
  - unit S3 given CA X, when the client is built, then its transport's `DialContext` is netguard's,
    `Proxy` is nil and `TLSClientConfig.RootCAs` is the appended pool; given no CA, `RootCAs` is nil
    (no-op control: no-CA behaviour unchanged) · a CA path that bypasses the SSRF guard or the
    proxy ban;
  - integration I1 (`internal/sink/s3/ca_trust_integration_test.go`): given an
    `httptest.NewUnstartedServer` with a certificate signed by X answering `HeadBucket` and
    `PutObject`, when `NewBackend(...).Export` and `TestConnection` run with CA X, then both succeed
    and the server recorded the object · the CA is not passed (red today: unknown authority);
  - integration I2: server signed by Y, CA X → certificate verification error and no object recorded
    · trust widened to any server;
  - integration I3: subprocess with `SSL_CERT_FILE` = P, server signed by P, CA X → success ·
    replace instead of append;
  - integration G1 (`internal/sink/gcs/ca_trust_integration_test.go`): I1 and I2 through
    `gcs.NewBackend` / `gcs.TestConnection` · the GCS wrapper not passing the CA.
- [ ] 2.2 Implement (`design.md`, *S3 and GCS*): `s3/config.go` rejects `http` with a CA,
  `s3/backend.go:124-149` sets the pool on the cloned netguard transport, `gcs/backend.go:26-40` and
  `gcs/connection.go:13-21` pass the CA, `registry.go:120-126` and `probe.go:151`, `:163` pass it.

## 3. Postgres (SCT-2, SCT-4)

- [ ] 3.1 Tests, red against stubs that accept `caPEM` in `postgres.NewBackend` and
  `postgres.TestConnection`:
  - unit D1 given keyword DSN `host=sentinel-host user=sentinel-user password=sentinel-pass
    dbname=sentinel-db sslmode=verify-full sslrootcert=/x` and CA X, when the config is built, then a
    terminal error naming `sslrootcert` and `spec.tls`, containing no sentinel · silent precedence
    and a DSN leak; D2 the same with URL form `postgres://...?sslrootcert=/x`;
  - unit D3 table, CA X: sslmode unset, `disable`, `allow`, `prefer` → terminal error naming the mode
    and `verify-full`, `verify-ca`, `require`; `require`, `verify-ca`, `verify-full` → accepted ·
    a CA silently unused under `prefer` (today's default);
  - unit D4 `t.Setenv("PGSSLROOTCERT", <file>)` and CA X → rejected as D1; D5
    `t.Setenv("PGSSLMODE", "prefer")` and a DSN without sslmode → rejected as D3 · env defaults
    bypassing the rules (relevant to the CLI in CI);
  - unit D6 given CA X and a two-host `verify-full` DSN, when the pool config is built, then the main
    config and every fallback's `TLSConfig.RootCAs` is the appended pool; with `require`,
    `VerifyPeerCertificate` is set; given no CA, the parsed config equals today's (no-op control) ·
    a fallback left on system roots, `require` left unverified;
  - unit D7 garbage PEM → terminal error before any dial;
  - unit D8 given a `*pgconn.ConnectError` wrapping `x509.UnknownAuthorityError`, when
    `redactedConnectError` runs, then the static text plus a static TLS-verification suffix naming
    `spec.tls`, no sentinel · a wrong CA indistinguishable from a network fault;
  - integration PI1 (`internal/sink/postgres/ca_trust_integration_test.go`): given
    `postgres:16-alpine` with `ssl=on` and a server certificate signed by X, when a sink with CA X
    and `sslmode=verify-full` exports and probes, then both succeed · CA not applied (red today);
  - integration PI2: server signed by Y, CA X, `sslmode=require` → verification error · `require`
    left unverified (red today: it connects);
  - integration PI3: subprocess with `SSL_CERT_FILE` = P, server signed by P, CA X, `verify-full`
    → success · replace instead of append;
  - integration PI4: through `acquireBackend` (`internal/sink/backend_pool.go:94`) with a fake
    client, a DSN Secret that sets `sslrootcert` fails; after the Secret is corrected, the next
    acquisition builds and exports, sink spec unchanged · a cached failure.
- [ ] 3.2 Implement (`design.md`, *Postgres*): `postgres/pool.go:16-24` and `:69-76`,
  `postgres/config.go`, `backend.go:53`, `connection.go:24`, `registry.go:132-139`, `probe.go:91`.

## 4. MongoDB (SCT-3, SCT-4)

- [ ] 4.1 Tests, red against stubs that accept `caPEM` in `mongodb.NewBackend` and
  `mongodb.TestConnection`:
  - unit M1 given URI `mongodb://sentinel-user:sentinel-pass@sentinel-host/?tls=true&tlsCAFile=/x`
    and CA X, then a terminal error naming `tlsCAFile` and `spec.tls`, no sentinel · silent
    precedence and a URI leak; M2 the same with `sslCertificateAuthorityFile`;
  - unit M3 table, CA X: `mongodb://h/` (no tls), `mongodb://h/?tls=false`,
    `mongodb+srv://sentinel-host.invalid/?tls=false` → terminal error naming TLS off; and the srv
    case does no DNS lookup (it returns before the SRV resolve; asserted with a resolver that fails
    the test if called, or by timing against an unroutable resolver as 0.1 decides) · CA unused on
    plaintext, validation that dials;
  - unit M4 `tlsInsecure=true` → terminal error · verification off next to a CA;
  - unit M5 garbage PEM → terminal error before any connect;
  - unit M6 given `tls=true&tlsCertificateKeyFile=<test file>` and CA X, when client options are
    built, then `TLSConfig.RootCAs` is the appended pool and the client certificate is kept; given no
    CA, options equal today's (no-op control) · URI TLS settings dropped;
  - integration MI1 (`internal/sink/mongodb/ca_trust_integration_test.go`): given a MongoDB
    container with `--tlsMode requireTLS` and a certificate signed by X, when a sink with CA X and
    `tls=true` exports and probes, then both succeed · CA not applied (red today);
  - integration MI2: server signed by Y → verification error; MI3: subprocess with
    `SSL_CERT_FILE` = P, server signed by P → success · replace instead of append.
- [ ] 4.2 Implement (`design.md`, *MongoDB*): `mongodb/config.go`, `backend.go:97`,
  `connection.go:34`, `registry.go:150-157`, `probe.go:115`.

## 5. Operator and CLI parity (SCT-5)

These guard tests may pass when written, because groups 1-4 route both paths through the same
resolver and constructors; their red is a mutation instead (5.1 names it).

- [ ] 5.1 Tests:
  - C1 given a `postgres` sink with CA X and a DSN without sslmode in a config directory, when
    `kollect-pipeline collect` runs (`internal/pipeline`, as `sink_credentials_test.go` does), then
    the context result is fatal, the CLI exits 2 (`cmd/cli/exitcode.go:17`), the sink saw no write,
    and the error text equals the operator's from `BuildContextFromSpec` + `NewBackend` · a CLI path
    with its own validation;
  - C2 given the same sink, when `RunConnectionTest` (`internal/sink/probe.go:37`) and
    `Registry.NewBackend` run, then both return the same terminal error text · a probe not plumbed;
  - C3 integration (`internal/pipeline/ca_trust_integration_test.go`): `collect` to the private-CA
    S3 httptest server from 2.1 with CA X in a config-dir Secret → exported · the CLI not passing
    the CA;
  - mutations, judged by exit status, with one no-op control that must survive: CLI `resolveCAPEM`
    bypasses the shared resolver (R2 or C1 fails); `probe.go` passes nil instead of the CA (C2 fails).
- [ ] 5.2 Fix whatever 5.1 shows; record "none" if all pass and the mutations are killed.

## 6. Documentation (SCT-6)

- [ ] 6.1 Docs, each claim checked against the code of groups 1-4:
  - `docs/REQUIREMENTS.md:90` FR-EXP-7: list Git, GitLab, Kafka, NATS, S3, GCS, Postgres, MongoDB;
  - `docs/crds/kollectsnapshotsink.md` (*Spec highlights*, `:15`) and
    `docs/crds/kollectdatabasesink.md` (`:17`): a `spec.tls` row and a short TLS trust section
    (append, keys `tls.crt`/`ca.crt`/`ca.pem`, the Postgres sslmode and `sslrootcert` rules, the
    MongoDB URI rules, no strict pinning);
  - `docs/operator-manual/upgrading.md`: a `### Sink CA trust (after v0.21.0)` section under
    *Values and behaviour changes* (`:121`) with the compatibility table from `design.md` and how to
    find affected sinks;
  - `docs/security/security-architecture.md:220-221` (Postgres/MongoDB "inherit transport policy
    from their connection strings") and `:239-243` ("Other backends ignore `spec.tls`");
  - `docs/guides/pipeline-cli.md:161` and the error table at `:481` (missing CA key, both sources);
  - `docs/operator-manual/troubleshooting.md:227` (the new terminal errors);
  - the CRD field comments `api/v1alpha1/kollectsink_types.go:341-348` (add `ca.pem`, append,
    errors), then regenerate per `AGENTS.md` *When adding a CRD field* (manifests, chart CRDs,
    glossary, schema golden) and finish with `task verify` green;
  - the v0.22.0 release-note line, in `pr-description.md`: "S3, GCS, Postgres and MongoDB sinks now
    trust `spec.tls.caBundle`/`caSecretRef` in addition to the system roots; a CA that cannot take
    effect (Postgres `sslmode` other than `require`/`verify-ca`/`verify-full`, a DSN `sslrootcert`,
    a MongoDB `tlsCAFile`, plaintext or `tlsInsecure`, a Secret without a CA key, both CA sources)
    now fails the sink instead of being ignored."
  - `task lint:markdown` reports `0 issues`; the docs guards under `hack/test/docs_*` pass.

## 7. Land

- [ ] 7.1 Integrator: `task check` on the tip; `task test-integration` (or CI `test-integration`
  with every new integration test in its `=== RUN` list and none skipped); branch review; PR
  closing SINK-CA-UNUSED-01's issue; after review and green CI, archive as the last commit.

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| SCT-1 | S1, S2, S3 (`go test -v ./internal/sink/s3/`) | rejections, guarded transport, no-CA unchanged | not-run | |
| SCT-1 | I1, I2, I3, G1 (`-tags integration`) | private CA trusted, Y refused, P trusted | not-run | |
| SCT-1 | mutation: pool built with `x509.NewCertPool()` | I3 fails | not-run | |
| SCT-2 | D1-D8 (`go test -v ./internal/sink/postgres/`) | conflicts and sslmodes rejected, no sentinel, fallbacks set | not-run | |
| SCT-2 | PI1-PI4 (`-tags integration`) | verify-full trusted, require verified, P trusted, recovery | not-run | |
| SCT-2 | mutation: skip the `sslrootcert` check; skip the `require` upgrade | D1 fails; PI2 fails | not-run | |
| SCT-3 | M1-M6 (`go test -v ./internal/sink/mongodb/`) | conflicts rejected, no DNS, no sentinel | not-run | |
| SCT-3 | MI1-MI3 (`-tags integration`) | private CA trusted, Y refused, P trusted | not-run | |
| SCT-3 | mutation: accept `tlsInsecure=true` | M4 fails | not-run | |
| SCT-4 | P1-P3, R1, D7, M5, S2 | terminal errors, no PEM echoed | not-run | |
| SCT-4 | mutation: resolver returns nil on a missing key | R1 fails | not-run | |
| SCT-5 | R2, C1, C2, C3 and the 5.1 mutations | same text on both paths, exit 2, no write | not-run | |
| SCT-5 | no-op mutation control | survives | not-run | |
| SCT-1..5 | `go test -race -count=3` on changed packages | pass | not-run | |
| SCT-6 | `task lint:markdown`, `hack/test/docs_*`, `task verify` | 0 issues, green | not-run | |
| SCT-6 | FR-EXP-7 read against `registry.go` | lists exactly the CA-applying types | not-run | |
| all | independent review (reviewer, revision, verdict) | recorded before merge | not-run | |
