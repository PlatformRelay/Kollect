## Verdict: CONCERNS

## Findings
- [WARNING] SCT-3: one new MongoDB error path can expose the host — `internal/sink/mongodb/ca_trust.go:80-81`
  Failure: a sink with a CA and `mongodb+srv://cluster.example.com/` gets past `checkCAURI`. `ApplyURI` then runs the SRV/TXT lookup, and if DNS fails, `opts.Validate()` returns the driver's `error parsing uri: lookup _mongodb._tcp.cluster.example.com …`. That error is wrapped and returned unchanged. SCT-3 says "the errors this requirement adds SHALL NOT contain the URI, host or credentials". The non-CA path leaks the same text through `mongo.Connect`, so the leak is older than this branch. This branch is what puts it in a CA-specific branch.
  Fix: replace the error with a fixed, target-free one (for example `ErrCAURIInvalid: the driver refused the URI`), as `redactedParseError` does for Postgres.
  Confidence: 55
- [NOTE] SCT-5: a third CA resolver with different rules is still in the tree — `internal/controller/secret_ca.go:19-60`
  Failure: its keys are `tls.crt, ca.crt, ca-bundle.crt` (no `ca.pem`), it does not call `CheckCASources`, and its errors are not terminal. Only `secret_ca_test.go` calls it, so nothing breaks today. Anyone who wires it into a controller path breaks "same key order, same error values" without noticing.
  Fix: delete it and its test, or make it call `sink.CheckCASources` and `sink.SelectCAKey`.
  Confidence: 90 (that it is dead and different); 20 (that it causes harm now)
- [NOTE] SCT-6 is partial: the v0.22.0 release-note line only exists in `openspec/changes/sink-ca-trust/pr-description.md` (`tasks.md:273`). No changelog or release-notes file in the tree carries it. git-cliff builds the notes from commit messages, so the line only appears if someone carries it over at tag time.
  Fix: put the line in a commit body that cliff picks up, or add it when tagging.
  Confidence: 70
- [NOTE] Stronger than spec, deliberately and in the safe direction. None of these breaks a URI or DSN the spec requires to be accepted:
  - Mongo refuses `tlsAllowInvalidCertificates`/`tlsAllowInvalidHostnames` with any value other than `false` (`ca_trust.go:199-204`).
  - Postgres refuses an empty `sslmode=` (`ca_trust.go:86`).
  - Postgres refuses `PGSSLROOTCERT` or `~/.postgresql/root.crt` even when the DSN has `sslrootcert=` empty (`ca_trust.go:102-120`).
  - An S3 endpoint that does not parse is reported as a plain-HTTP error (`s3/config.go:137-139`); the text is misleading but nothing leaks.
  Confidence: 85
- [NOTE] Not tested: accepting a CA'd `mongodb+srv://` through the whole `clientOptions` path. M8 only exercises `checkCAURI` (`ca_trust_test.go:356-371`). If the driver left `TLSConfig` nil for SRV, `checkDriverTLS` would reject a URI the spec says must be accepted. My reading of driver v1 is that SRV forces SSL on, so this should hold.
  Confidence: 25

Checked, with the code location that decides each:
- **SCT-1 holds.**
  - System roots plus the CA: `sinktls.go:90-102`.
  - netguard dialer and no proxy are kept, because the transport is cloned from netguard: `s3/backend.go:182-196`, `netguard/dialer.go:38-53`.
  - The `http` scheme is rejected both on the spec endpoint and on the endpoint the SDK resolves (`AWS_ENDPOINT_URL*`, shared config): `s3/config.go:54-63`, `s3/backend.go:166-177`.
  - Redirects to non-HTTPS URLs are refused: `backend.go:199-207`.
  - The probe goes through the same `newClient`: `s3/connection.go:27-35`.
  - Without a CA the client is netguard's, unchanged: `backend.go:184`.
- **SCT-2 holds.**
  - Root-certificate sources are checked in pgx's precedence order, whether or not the file is readable: `postgres/ca_trust.go:101-123`.
  - DSN, then service-file entry, then `PGSSLMODE`, with no sslmode anywhere becoming `verify-full`: `:72-96`.
  - After pgx parses, a backstop rejects a nil TLS config, a root pool pgx loaded, or a `verify-full` that skips the hostname; `require` gets a chain check: `:350-385`.
  - Errors are redacted, with a fixed hint for certificate failures: `pool.go:115-157`.
- **SCT-3 holds, apart from the finding above.** Option names match case-insensitively without DNS, and the driver-built config is checked again: `ca_trust.go:96-141`, `:148-219`.
- **SCT-4 holds.**
  - PEM with no certificate is a terminal error that does not echo the PEM: `sinktls.go:97-99`.
  - Both CA sources set fails before any Secret fetch, and a Secret with none of the keys fails: `ca_source.go:26-46`, `build_context.go:221`.
  - A failed build is not cached: `backend_pool.go:145-153`.
  - Touching the sink does trigger a reconcile, since no controller filters events (no `WithEventFilter` or predicates in `internal/`).
- **SCT-5 holds.**
  - The CLI and the operator resolve through the same shared functions: `pipeline/wire.go:142-165`, `build_context.go:215-243`.
  - The probe uses the same constructors with the CA passed: `probe.go:91-163`, `registry.go:121-156`.
  - The CLI exit code 2 path has a test: `cmd/cli/ca_trust_parity_test.go:154`.
- **SCT-6:** FR-EXP-7 lists exactly the eight sink types (`docs/REQUIREMENTS.md:90`). The field descriptions do not claim append semantics for Kafka, NATS or the probe (`api/v1alpha1/kollectsink_types.go:341-358`).
- **Not asked for by any requirement:** `go.mod` promotes `jackc/pgservicefile` from indirect to direct. It was already in the module graph.

## Could not check
- I ran no tests and found no recorded sensor output, so I can't confirm the integration suites passed: `*_ca_trust_integration_test.go`, `ca_trust_recovery_integration_test.go`.
- I didn't check the pinned pgx v5.11 source to confirm that its `verify-ca` callback reads `tlsConfig.RootCAs` when the handshake runs; I relied on the code comment and the integration tests.
- I didn't check mongo-driver v1's SRV default for `SSLSet` in `connstring`.
- I didn't fully read these docs pages: `docs/crds/*.md`, `security-architecture.md`, `pipeline-cli.md`, `troubleshooting.md`.
- I didn't read the review and evidence files under `openspec/changes/sink-ca-trust/reviews/` and `evidence/`; this review is independent of them.
