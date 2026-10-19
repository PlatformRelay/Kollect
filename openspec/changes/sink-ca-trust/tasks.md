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
  binary as a subprocess (`os.Args[0] -test.v -test.run '^<ChildTest>$'`) with
  `SSL_CERT_FILE=<P.pem>` and `SSL_CERT_DIR=<an empty t.TempDir()>` (an empty value does not clear
  Go's default directories), so the rest of the package keeps the real pool; the parent asserts the
  child's exit status and its `=== RUN <ChildTest>` and `--- PASS` lines. Probe this in 0.1.
- Integration evidence: `task test-integration` runs `hack/tools/integrationevidence`, which hides
  per-test output; run the named tests separately with `go test -tags integration -v -run` or keep
  the raw stream with `-json-out=<file>` (`hack/tools/integrationevidence/main.go:30`), and quote the
  `=== RUN` lines. Without Docker the container-backed cases skip; that is neither red nor green and
  leaves the row `blocked` until a Docker run (CI `test-integration`) records it.
- Error-text assertions check for sentinel values put into the DSN/URI (host `sentinel-host`, user
  `sentinel-user`, password `sentinel-pass`, database `sentinel-db`) and SHALL find none of them.
- Each group records `go test -race -count=3` on the packages it changed.

## 0. Probe

- [x] 0.1 Verify the load-bearing assumptions in `proposal.md` and record each as verified (command,
  versions, date) in `evidence/T0.1.md`; a failed one stops the loop as a spec finding:
  - pgx v5.11.0: the `verify-ca` callback reads `RootCAs` from the same `*tls.Config` that
    `ConnConfig.TLSConfig` / `Fallbacks[i].TLSConfig` point to after `pgxpool.ParseConfig`
    (`pgconn/config.go:957-990`), and whether fallbacks share or copy that pointer; which parsed
    shape each sslmode yields (TLSConfig nil, fallbacks with nil TLSConfig, `InsecureSkipVerify` with
    or without `VerifyPeerCertificate`); that `PGSSLMODE` / `PGSSLROOTCERT` reach the parse;
  - mongo-driver v1.17.10: a parse of the URI's TLS keys that does no DNS lookup for
    `mongodb+srv` (the TXT and SRV lookups at `x/mongo/driver/connstring/connstring.go:994` and
    `:1016` must not run in validation); otherwise SCT-3's check parses the query string itself;
  - pgx v5.11.0: the home-directory default `~/.postgresql/root.crt` (`pgconn/defaults.go:58`),
    `PGSERVICEFILE` entries, and what an unreadable `PGSSLROOTCERT` does to `ParseConfig`;
  - mongo-driver v1.17.10: how a repeated query key and mixed-case keys are handled
    (`connstring.go:348`), and which options besides `tls`/`ssl` turn TLS on (`:596-640`);
  - Go 1.27.2 (`go.mod:3`) on Linux: `SSL_CERT_FILE` plus an empty `SSL_CERT_DIR` directory in a
    child process replace `x509.SystemCertPool()`;
  - Postgres image `postgres:16-alpine` accepts a test key mounted into the container (key owner
    and mode rules) and MongoDB's image accepts `--tlsMode requireTLS --tlsCertificateKeyFile`;
    record the command that worked. — closed 2026-10-19, evidence: evidence/G1.md (T0.1 section)

## 1. Shared pool and CA resolution (SCT-4, SCT-5)

- [x] 1.1 Tests, red against stubs of `sinktls.AppendedPool(pem []byte) (*x509.CertPool, error)` and
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
    and `caSecretRef` → the both-sources terminal error, also when the referenced Secret does not
    exist (the check runs before any fetch) · the silent fallback at `build_context.go:244` and the
    silent `caBundle` precedence at `:225` (both red today), a both-set spec failing on not-found;
  - R2 extend `TestCLICredentials_matchOperatorResolver`
    (`internal/pipeline/sink_credentials_test.go:312`): for the cases whose error comes from the
    shared functions, assert with `errors.Is` that both paths wrap the same shared error (today
    only presence is compared, `:384`); the not-found cases (`"both, default absent"`,
    `"gitAuth absent"`, `"caSecret absent"`) keep the presence check, their texts differ by design;
    rename `"inline bundle wins"` (`:356`) to `"both sources rejected"` and expect the both-sources
    error on both paths; change `"caSecret no key"` (`:354`) to expect the missing-key error on both
    paths; add `"caSecret ca.pem"` · operator and CLI drifting apart (record the changes as *Test
    changes* in `loop.md`). — closed 2026-10-19, evidence: evidence/G1.md
- [x] 1.2 Implement `AppendedPool` and the shared resolver; `resolveCAPEM` in
  `internal/sink/build_context.go:215-245` and `internal/pipeline/wire.go:143-171` call it. Fix the
  `sinktls.FromSpec` comment that claims `caSecretRef` precedence (`sinktls.go:24-26`). — closed
  2026-10-19, evidence: evidence/G1.md

## 2. S3 and GCS (SCT-1, SCT-4)

- [x] 2.1 Tests, red against stubs that accept `caPEM` in `s3.NewBackend`, `s3.TestConnection`,
  `gcs.NewBackend`, `gcs.TestConnection` (registry and probe pass `ctx.CAPEM`):
  - unit S1 given endpoint `http://h/b` and CA X, when `s3.NewBackend` runs, then a terminal error
    naming `spec.tls` and `http`, and the counting transport saw no request · CA ignored over HTTP;
    S1b the same for endpoint `s3://b` with `t.Setenv("AWS_ENDPOINT_URL_S3", "http://h")`, with
    `AWS_ENDPOINT_URL`, with both set (the S3-specific one wins), and with a shared-config file
    (`AWS_CONFIG_FILE` in a temp dir) whose `services` section sets an `http` S3 endpoint · an
    SDK-selected plaintext endpoint bypassing the spec check, a check on `aws.Config` that misses the
    S3-specific source;
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
  - integration I4: the X-signed server answers `PutObject` with a 307 to an `http://` httptest
    server; CA X → the export fails and the plaintext server recorded no request · a redirect that
    downgrades to plain HTTP;
  - integration G1 (`internal/sink/gcs/ca_trust_integration_test.go`): I1 and I2 through
    `gcs.NewBackend` / `gcs.TestConnection` · the GCS wrapper not passing the CA. — closed
  2026-10-19, evidence: evidence/G2.md
- [x] 2.2 Implement (`design.md`, *S3 and GCS*): `s3/config.go` rejects `http` with a CA,
  `s3/backend.go:124-149` sets the pool on the cloned netguard transport, `gcs/backend.go:26-40` and
  `gcs/connection.go:13-21` pass the CA, `registry.go:120-126` and `probe.go:151`, `:163` pass it.
  — closed 2026-10-19, evidence: evidence/G2.md

## 3. Postgres (SCT-2, SCT-4)

- [x] 3.1 Tests, red against stubs that accept `caPEM` in `postgres.NewBackend` and
  `postgres.TestConnection`:
  - unit D1 given keyword DSN `host=sentinel-host user=sentinel-user password=sentinel-pass
    dbname=sentinel-db sslmode=verify-full sslrootcert=/x` and CA X, when the config is built, then a
    terminal error naming `sslrootcert` and `spec.tls`, containing no sentinel · silent precedence
    and a DSN leak; D2 the same with URL form `postgres://...?sslrootcert=/x`;
  - unit D3 explicit-weak rejection, table, CA X, `HOME` set to an empty `t.TempDir()`: DSN
    `sslmode=disable`, `allow`, `prefer` → terminal error naming the mode, the source `DSN` and
    `verify-full`, `verify-ca`, `require`, before any dial; `require`, `verify-ca`, `verify-full` →
    accepted · an explicitly weak mode silently ignoring the CA, or upgraded instead of rejected;
  - unit D3b unset→verify-full: given CA X, a keyword DSN and a URL DSN without sslmode (and one with
    `sslnegotiation=direct`), `PGSSLMODE` unset, when the pool config is built, then the main config
    and every fallback have `ServerName` set to their host, `InsecureSkipVerify` false and no
    plaintext fallback · the default left at pgx's `prefer` (CA never checked, red today) or
    downgraded to a chain-only check;
  - unit D4 CA X and a root certificate from another source, each rejected naming its source:
    `t.Setenv("PGSSLROOTCERT", f)` with `f` a valid PEM file, a missing path and a malformed file;
    `HOME` = a temp dir holding `.postgresql/root.crt`; a `PGSERVICEFILE` whose selected entry sets
    `sslrootcert` to a valid, a missing and a malformed file (selected by `service=` in the DSN and
    by `PGSERVICE`) · env, home or service defaults bypassing the conflict rule, a malformed file
    reported as the static `invalid connection string`;
  - unit D5 `t.Setenv("PGSSLMODE", "prefer")` and a DSN without sslmode → rejected as D3 with
    source `PGSSLMODE`; `PGSSLMODE=verify-ca` and a DSN without sslmode → accepted as `verify-ca`,
    not upgraded · env defaults bypassing the rules, or an explicit env mode overridden;
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
  - integration PI2: server signed by Y, CA X, `sslmode=require` and `verify-ca` → verification
    error each · `require` left unverified (red today: it connects), a broken chain callback;
  - integration PI5: server certificate signed by X for `other.invalid` (not the dialled host), CA X:
    `require` and `verify-ca` succeed, `verify-full` fails on the hostname · chain-only modes checking
    the hostname, or `verify-full` not checking it;
  - integration PI3: subprocess with `SSL_CERT_FILE` = P, server signed by P, CA X, `verify-full`
    → success · replace instead of append;
  - integration PI6: the PI5 server (SAN `other.invalid`), CA X, DSN without sslmode → fails on the
    hostname; the PI1 server (matching SAN), DSN without sslmode → succeeds · the unset→verify-full
    upgrade missing on the real connection path;
  - integration PI4 (`internal/sink/ca_trust_recovery_integration_test.go`, `package sink`, so it
    can call the unexported `acquireBackend`, `internal/sink/backend_pool.go:94`): with a fake
    client and the PI1 server, a DSN Secret that sets `sslrootcert` fails; after the Secret is
    corrected, the next acquisition builds and exports, sink spec unchanged · a cached failure.
    — closed 2026-10-19, evidence: evidence/G3.md
- [x] 3.2 Implement (`design.md`, *Postgres*): `postgres/pool.go:16-24` and `:69-76`,
  `postgres/config.go`, `backend.go:53`, `connection.go:24`, `registry.go:132-139`, `probe.go:91`.
  — closed 2026-10-19, evidence: evidence/G3.md

## 4. MongoDB (SCT-3, SCT-4)

- [x] 4.1 Tests, red against stubs that accept `caPEM` in `mongodb.NewBackend` and
  `mongodb.TestConnection`:
  - unit M1 given URI `mongodb://sentinel-user:sentinel-pass@sentinel-host/?tls=true&tlsCAFile=/x`
    and CA X, then a terminal error naming `tlsCAFile` and `spec.tls`, no sentinel · silent
    precedence and a URI leak; M2 the same with `sslCertificateAuthorityFile`, with `TLSCAFILE`
    (letter case) and with the key repeated;
  - unit M3 table, CA X: `mongodb://h/` (no tls), `mongodb://h/?tls=false`,
    `mongodb+srv://sentinel-host.invalid/?tls=false` → terminal error naming TLS off; and the srv
    case does no DNS lookup (it returns before the SRV resolve; asserted with a resolver that fails
    the test if called, or by timing against an unroutable resolver as 0.1 decides) · CA unused on
    plaintext, validation that dials;
  - unit M4 `tlsInsecure=true`, `sslInsecure=true`, `TLSINSECURE=true` → terminal error;
    `tlsInsecure=false` and `sslInsecure=false` with `tls=true` → accepted; a malformed value
    (`tlsInsecure=maybe`) → terminal error · verification off next to a CA through an alias, or a
    secure explicit setting rejected;
  - unit M7 backstop: with the key check mutated to skip `tlsCAFile` (a test seam, or the mutation
    row), a URI with `tlsCAFile=<valid file>` is still rejected after `ApplyURI` because the
    driver-built `TLSConfig.RootCAs` is set · a spelling the key check misses passing silently;
  - unit M8 the key check accepts `mongodb+srv://h.invalid/` without `tls` and `mongodb://h/?ssl=true`
    and does no DNS lookup · SRV default TLS misread as plaintext;
  - unit M5 garbage PEM → terminal error before any connect;
  - unit M6 given `tls=true&tlsCertificateKeyFile=<test file>` and CA X, when client options are
    built, then `TLSConfig.RootCAs` is the appended pool and the client certificate is kept; given no
    CA, options equal today's (no-op control) · URI TLS settings dropped;
  - integration MI1 (`internal/sink/mongodb/ca_trust_integration_test.go`): given a MongoDB
    container with `--tlsMode requireTLS` and a certificate signed by X, when a sink with CA X and
    `tls=true` exports and probes, then both succeed · CA not applied (red today);
  - integration MI2: server signed by Y → verification error; MI3: subprocess with
    `SSL_CERT_FILE` = P, server signed by P → success · replace instead of append. — closed
    2026-10-19, evidence: evidence/G4.md
- [x] 4.2 Implement (`design.md`, *MongoDB*): `mongodb/config.go`, `backend.go:97`,
  `connection.go:34`, `registry.go:150-157`, `probe.go:115`. — closed 2026-10-19, evidence:
  evidence/G4.md

## 5. Operator and CLI parity (SCT-5)

These guard tests may pass when written, because groups 1-4 route both paths through the same
resolver and constructors; their red is a mutation instead (5.1 names it).

- [x] 5.1 Tests:
  - C1a (`internal/pipeline`, unit) given a hand-filled `collect.Store` (as
    `internal/pipeline/git_integration_test.go:36-50` does) and an `s3` `KollectSnapshotSink` spec
    with garbage CA PEM, and one with a CA and an `http://` endpoint, when `cliBuildContext` and
    `withCLIBackend` run, then the returned error satisfies `errors.Is` against the shared error the
    operator's `BuildContextFromSpec` + `NewBackend` returns for the same spec, and the export
    callback never ran · a CLI path with its own validation (the CLI loads no database sinks,
    `internal/pipeline/loader.go:122-135`);
  - C1b (`internal/pipeline`, L2 envtest like `TestPipelineCLI_collectAndWrite`,
    `internal/pipeline/integration_test.go:36-46`, which runs in `task test`) given a config
    directory with the garbage-CA `s3` sink and a target that collects, when the collect pipeline
    runs, then the context result is fatal after collection, the exit mapping gives 2
    (`cmd/cli/exitcode.go:17`), and the sink saw no write; the test asserts collection succeeded so
    the exit is not from an earlier failure · exit 2 for the wrong reason;
  - C2 given a `postgres` sink with CA X and a DSN without sslmode, and a `mongodb` sink with CA X
    and `tlsInsecure=true`, when `RunConnectionTest` (`internal/sink/probe.go:37`) and
    `Registry.NewBackend` run, then each pair returns the same terminal error · a probe not plumbed;
  - C3 integration (`internal/pipeline/ca_trust_integration_test.go`, no envtest, following
    `git_integration_test.go:36-50`): a hand-filled store, `cliBuildContext` with CA X from a
    config-dir Secret, `withCLIBackend` and `ExportTargets` to the private-CA S3 httptest server from
    2.1 → exported · the CLI not passing the CA;
  - mutations, judged by exit status, with one no-op control that must survive: CLI `resolveCAPEM`
    bypasses the shared resolver (R2 or C1 fails); `probe.go` passes nil instead of the CA (C2 fails).
  — closed 2026-10-19, evidence: evidence/G5.md
- [x] 5.2 Fix whatever 5.1 shows; record "none" if all pass and the mutations are killed. — closed
  2026-10-19 (none), evidence: evidence/G5.md

## 6. Documentation (SCT-6)

- [x] 6.1 Docs, each claim checked against the code of groups 1-4:
  - `docs/REQUIREMENTS.md:90` FR-EXP-7: list Git, GitLab, Kafka, NATS, S3, GCS, Postgres, MongoDB;
  - `docs/crds/kollectsnapshotsink.md` (*Spec highlights*, `:15`) and
    `docs/crds/kollectdatabasesink.md` (`:17`): a `spec.tls` row and a short TLS trust section
    (append, keys `tls.crt`/`ca.crt`/`ca.pem`, the Postgres sslmode and `sslrootcert` rules, the
    MongoDB URI rules, no strict pinning);
  - `docs/operator-manual/upgrading.md`: a `### Sink CA trust (after v0.21.0)` section under
    *Values and behaviour changes* (`:121`) with the compatibility table from `design.md` (previous
    behaviour per backend, sslmode stated as the effective mode), the all-sink-type scope of the
    missing-key and both-sources errors, how to find affected sinks, and that after fixing a Secret
    the sink must be touched (for example annotated) to retry;
  - `docs/security/security-architecture.md:220-221` (Postgres/MongoDB "inherit transport policy
    from their connection strings") and `:239-243` ("Other backends ignore `spec.tls`");
  - `docs/guides/pipeline-cli.md:161` and the error table at `:481` (missing CA key, both sources);
  - `docs/operator-manual/troubleshooting.md:227` (the new terminal errors and the retry note);
  - the CRD field comments `api/v1alpha1/kollectsink_types.go:341-348` (add `ca.pem` and the
    errors; append semantics stated only for the types that have them, since Kafka, NATS and the
    Git probe keep their pool), then regenerate per `AGENTS.md` *When adding a CRD field* (manifests, chart CRDs,
    glossary, schema golden) and finish with `task verify` green;
  - the v0.22.0 release-note line, in `pr-description.md`: "S3, GCS, Postgres and MongoDB sinks now
    trust `spec.tls.caBundle`/`caSecretRef` in addition to the system roots; on those types a CA
    that cannot take effect (an `http` endpoint, Postgres `sslmode` other than
    `require`/`verify-ca`/`verify-full` set explicitly or another root certificate source, a
    MongoDB `tlsCAFile`,
    plaintext or `tlsInsecure`/`sslInsecure`) now fails the sink instead of being ignored, and on
    every sink type, Kafka and NATS included, a `caSecretRef` Secret without a CA key or a spec
    with both `caBundle` and `caSecretRef` now fails the sink. CA-configured Postgres sinks without
    an sslmode now verify the server, including the hostname: a DSN host that does not match the
    certificate SAN now fails; set `sslmode=verify-ca` explicitly to verify the chain only."
  - `task lint:markdown` reports `0 issues`; the docs guards under `hack/test/docs_*` pass. — closed
  2026-10-19, evidence: evidence/G6.md

## 7. Land

- [ ] 7.1 Integrator: `task check` on the tip; `task test-integration` (or CI `test-integration`
  with every new integration test in its `=== RUN` list and none skipped); branch review; PR
  closing SINK-CA-UNUSED-01's issue; after review and green CI, archive as the last commit.

## 8. Stage-B fixes

Findings of the branch review (`reviews/B-branch/round-1/verification.md`) and the items the task
loop deferred to B. One commit each; behaviour fixes start from a failing test.

- [x] 8.1 H1: `task test-integration` fails on three skipped subprocess children; run each child
  inside its parent test (PI3, MI3, I3). — closed 2026-10-19: `task test-integration` expected 65,
  executed 65, skipped 0, PASS; a CA-only pool still fails all three parents.
- [x] 8.2 A1: a refused redirect's error carries the `Location` URL; return a static terminal
  `ErrCAInsecureRedirect` from the guarded S3/GCS client. — closed 2026-10-19: I4
  (`TestCATrustIntegration_plainRedirectRefused`) red on the probe error naming
  `http://127.0.0.1:…/sentinel-redirect/b`, green after; removing the wrapper fails it again.
- [x] 8.3 A2: the CLI resolves credential Secrets before the CA rules; resolve the CA first, as the
  operator does. — closed 2026-10-19: `TestCAParity_caRulesPrecedeCredentialLookup` red on the CLI
  lookup error in all three cases, green after; reverting either reorder fails it again.
- [x] 8.4 A3/O1: the MongoDB CA path returns the driver's URI error (host) unchanged; return a
  static, target-free error. — closed 2026-10-19: `TestMongoCATrust_driverURIErrorIsTargetFree`
  red on `invalid host … sentinel-host:1:2` and `lookup sentinel-host.invalid` (build and probe),
  green after; class unchanged (not terminal).
- [x] 8.5 D2: pin the effective Postgres sslmode in the DSN handed to pgx, so `require` and
  `prefer`+`sslnegotiation=direct` cannot be confused. — closed 2026-10-19:
  `TestCADSN_pinsEffectiveSSLMode` red on the `PGSSLMODE` and service-entry cases (caDSN's output
  parsed on its own fell back to plaintext), green after; Postgres unit and PI1-PI6 green. It is a
  property test (the output states its own mode); no DSN that splits the reader and pgx is known,
  so none is in the table.
- [x] 8.6 D3/O2: delete the unused third CA resolver `internal/controller/secret_ca.go`. — closed
  2026-10-19: no caller outside its own test (`grep resolveCAPEM internal/controller`); build and
  vet clean.
- [x] 8.7 D4: `git.ValidateTLSSpec` returns the shared terminal both-sources error. — closed
  2026-10-19: `TestRegistry_NewBackend_gitFactoryConfigError` red on `tls: set either caBundle or
  caSecretRef, not both` (git and GitLab), green after; the check and sentinel now live in
  `sinktls`, and `sink.CheckCASources`/`sink.ErrCABothSources` delegate to them.
- [x] 8.8 D7: `Registry.NewBackend` returns a nil `Backend` when the factory fails. — closed
  2026-10-19: `TestRegistry_NewBackend_failedBuildReturnsNilInterface` red on five typed nils (s3,
  gcs, postgres, mongodb, local), green after.
- [x] 8.9 Fix review (both legs): 8.2 left the redirect limit, an unparsable `Location` and a
  failed followed hop naming the server-sent target; the guarded client returns the static
  `ErrRedirectFailed` for those. — closed 2026-10-19:
  `TestCATrustIntegration_redirectFailuresWithholdTarget` red in all three cases, green after;
  dropping either detection branch fails it again.
- [x] 8.10 Fix review notes: the 8.5 comment and design note claimed `applyCA` judges a missed DSN
  mode (it cannot tell `require` from `prefer`+`direct`; the comment now says why such a mode is
  never weaker), and the `ResolveSinkSecretData` doc still put `spec.secretRef` first. Wording
  only. — closed 2026-10-19.
- [x] 8.11 Branch round 2: a redirect loop on the request's own URL slipped past 8.9's URL test,
  a redirected hop's context error lost its identity, and the unparsable-`Location` text match was
  guarded only in the integration tier; the redirect limit is now matched by identity, a context
  error is kept (`errors.Is`) without its text, and a unit test covers every case. — closed
  2026-10-19: `TestRedactedRedirectClient_withholdsTarget` red on the loop and the timeout, green
  after; mutants (limit match, context error, `Location` match) each killed in the unit tier.

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| SCT-1 | S1, S1b, S2, S3 (`go test -v ./internal/sink/s3/`) | rejections incl. SDK endpoints, guarded transport, no-CA unchanged | pass | `evidence/G2.md` rows 1-4 |
| SCT-1 | I1, I2, I3, I4, G1 (`-tags integration`) | private CA trusted, Y refused, P trusted, downgrade redirect refused | pass | `evidence/G2.md` rows 5-9 |
| SCT-1 | mutation: pool built with `x509.NewCertPool()` | I3 fails | pass | `evidence/G2.md` mutant 1 |
| SCT-2 | D1-D8, D3b (`go test -v ./internal/sink/postgres/`) | conflicts and explicit weak modes rejected, unset → verify-full, no sentinel, fallbacks set | pass | `evidence/G3.md` rows 1-8, 20-23 |
| SCT-2 | PI6 (`-tags integration`) | unset sslmode verifies the hostname | pass | `evidence/G3.md` row 13 |
| SCT-2 | mutation: leave unset sslmode at pgx's default | D3b and PI6 fail | pass | `evidence/G3.md` mutants 1, 2 |
| SCT-2 | PI1-PI5 (`-tags integration`) | verify-full trusted, require/verify-ca verified, hostname rule per mode, P trusted, recovery | pass | `evidence/G3.md` rows 9-12, 14 |
| SCT-2 | mutation: skip the `sslrootcert` check; skip the `require` upgrade | D1 fails; PI2 fails | pass | `evidence/G3.md` mutants 3, 4 |
| SCT-3 | M1-M8 (`go test -v ./internal/sink/mongodb/`) | conflicts and aliases rejected, backstop holds, SRV accepted, no DNS, no sentinel | not-run | |
| SCT-3 | MI1-MI3 (`-tags integration`) | private CA trusted, Y refused, P trusted | not-run | |
| SCT-3 | mutation: accept `sslInsecure=true`; drop the post-`ApplyURI` backstop | M4 fails; M7 fails | not-run | |
| SCT-4 | P1-P3, R1, D7, M5, S2 | terminal errors, no PEM echoed | not-run | |
| SCT-4 | mutation: resolver returns nil on a missing key | R1 fails | pass | `evidence/G1.md` mutant 1 |
| SCT-5 | R2, C1a, C1b, C2, C3 and the 5.1 mutations | same shared error on both paths, exit 2 after collection, no write | pass | `evidence/G5.md` rows 1-6 |
| SCT-5 | no-op mutation control | survives | pass | `evidence/G5.md` mutant 0 |
| SCT-1..5 | `go test -race -count=3` on changed packages | pass | not-run | |
| SCT-6 | `task lint:markdown`, `hack/test/docs_*`, `task verify` | 0 issues, green | pass | `evidence/G6.md` rows 12-13 |
| SCT-6 | FR-EXP-7 read against `registry.go` | lists exactly the CA-applying types | pass | `evidence/G6.md` row 1 |
| all | independent review (reviewer, revision, verdict) | recorded before merge | not-run | |
