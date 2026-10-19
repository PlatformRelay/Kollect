## Verdict

BLOCK — the plan leaves MongoDB verification and S3 plaintext bypasses, promises redaction without implementing it, and includes a Postgres CLI scenario that the current loader cannot execute.

## Findings

Paths to the spec set below are relative to `openspec/changes/sink-ca-trust/`. Dependency references name the cached module version. Review was read-only and offline; no tests were executed.

### F1 CRITICAL: MongoDB’s `sslInsecure` alias bypasses the planned rejection

- Where: `design.md:61-65`; `tasks.md:149`; `specs/sink-tls-trust/spec.md:104-109`; mongo-driver v1.17.10, `x/mongo/driver/connstring/connstring.go:619`.
- Evidence: The driver handles `case "sslinsecure", "tlsinsecure"` identically. The design only rejects `tlsInsecure=true`, then clones the driver’s TLS configuration.
- Why it matters: `mongodb://host/?tls=true&sslInsecure=true` can retain `InsecureSkipVerify=true`. Setting `RootCAs` does not restore verification. M4 does not cover this alias.
- Fix: Require rejection of both aliases, with case-insensitive option names and driver-consistent duplicate handling. Add a final invariant that the constructed TLS configuration cannot disable verification. Extend M4 and its mutation with `sslInsecure=true`, mixed-case names and duplicate options.

### F2 CRITICAL: S3 can select HTTP after the proposed endpoint validation

- Where: `design.md:69-73`; `tasks.md:81-100`; `internal/sink/s3/backend.go:124-148`; AWS config v1.33.6, `resolve.go:108-127`; S3 v1.114.0, `endpoints.go:215-230`.
- Evidence: `s3://` leaves `cfg.Endpoint` empty. `LoadDefaultConfig` loads endpoint configuration, and S3 resolves `AWS_ENDPOINT_URL`, `AWS_ENDPOINT_URL_S3` and service configuration. Kollect overrides that endpoint only when `cfg.Endpoint != ""`.
- Why it matters: A sink with CA X and `s3://bucket` can use an SDK-configured HTTP endpoint, bypassing the sink-spec scheme check. The retained `http.Client` also lacks an HTTPS-only redirect policy. Netguard checks destination addresses, not encryption.
- Fix: Extend SCT-1 and task 2.2 to reject an effective HTTP endpoint after SDK endpoint selection, and prevent every CA-bearing request—including redirects—from using HTTP. Add tests for both endpoint environment variables, shared configuration, and HTTPS-to-HTTP redirects. Keep netguard’s dialer and `Proxy=nil`.

### F3 CRITICAL: The Postgres CLI acceptance test has no supported production input

- Where: `tasks.md:168-172`; SCT-5’s CLI-fails-fast scenario; `internal/pipeline/loader.go:41`, `:122-135`; `internal/pipeline/wire.go:34-57`; `api/v1alpha1/kollectsnapshotsink_types.go:9-42`.
- Evidence: The loader stores only `KollectSnapshotSink` objects. Database sinks are skipped. Snapshot specs contain neither a Postgres block nor a MongoDB block.
- Why it matters: A Postgres manifest in a config directory cannot reach the planned CA/sslmode rejection. It fails earlier because no snapshot sink was loaded. Passing a synthetic `KollectSinkSpec` directly to pipeline internals would not prove the claimed CLI behavior or exit code.
- Fix: Make the scope decision explicit. Either add tasks for database-sink loading, selection, validation, documentation and command-level tests, or restrict actual CLI acceptance scenarios to S3/GCS and label Postgres/MongoDB tests as internal constructor parity. Test exit code 2 in `cmd/cli`, not solely through `ContextResult.Fatal`.

### F4 CRITICAL: MongoDB error paths remain capable of exposing sensitive input

- Where: `specs/sink-tls-trust/spec.md:109`; `design.md:65`; `tasks.md:141-158`; `internal/sink/mongodb/backend.go:97-105`; `internal/sink/mongodb/connection.go:34-41`.
- Evidence: MongoDB connect and ping errors are returned with raw `%w` wrapping. The cached URI parser quotes invalid hosts and option values; for example, `connstring.go:969` returns `invalid host %q`. `ApplyURI` retains parser errors at `mongo/options/clientoptions.go:426-429`.
- Why it matters: Rejecting selected TLS options with static messages does not sanitize malformed URIs, certificate-file errors, DNS failures or server-selection errors. Malformed connection strings can place credential bytes in a quoted host or option value. The plan promises no URI, host or credential exposure but supplies no MongoDB redaction implementation.
- Fix: Add a shared MongoDB error boundary for URI parsing, client-option validation, connection and subsequent connection failures. Return static, classified messages; never interpolate raw driver errors into user-facing text. Add malformed-input and runtime-error sentinel tests, including the wrong-CA probe. Preserve the existing Postgres static-redaction approach.

### F5 WARNING: Postgres’s policy needs an explicit interpretation and direct-negotiation coverage

- Where: `proposal.md:39-45`; `design.md:43-50`; `tasks.md:110-119`; `specs/sink-tls-trust/spec.md:53-78`; pgx v5.11.0, `pgconn/config.go:903-908`, `:1063-1071`.
- Evidence: pgx changes `prefer` to `require` when `sslnegotiation=direct`, yielding no plaintext fallback. The design classifies modes solely through nil TLS configurations. The scenario nevertheless rejects every DSN with unset sslmode or `prefer`.
- Why it matters: The written implementation accepts direct-negotiation inputs that the scenario rejects. Also, rejecting `allow`/`prefer` and strengthening `require` is a compatibility policy, not literal preservation of every DSN setting. It is a reasonable way to reconcile “never unverified” with the ruling, but that interpretation needs to be explicit.
- Fix: State whether permitted modes are determined before or after pgx’s direct-negotiation rewrite. Align proposal, scenarios, compatibility table and D3 accordingly. Add direct negotiation, `PGSSLMODE=verify-full` with omitted DSN mode, and Unix/mixed-host cases. Record the chosen interpretation of the operator ruling.

The central pgx mechanism is otherwise supported by the cached source: `verify-ca` captures the original TLS configuration and reads its `RootCAs`; `ParseConfig` stores those pointers in the primary configuration and fallbacks. Mutating those original configurations before pool creation is viable.

### F6 WARNING: Detecting `PGSSLROOTCERT` after parsing misses required conflict diagnostics

- Where: `design.md:52-54`; `tasks.md:113-115`; `specs/sink-tls-trust/spec.md:59-60`; pgx v5.11.0, `pgconn/config.go:910-937`.
- Evidence: The design detects an environmental CA by checking parsed `TLSConfig.RootCAs`. pgx reads and parses the root file before returning that configuration.
- Why it matters: An unreadable or malformed `PGSSLROOTCERT` fails parsing first. Existing static Postgres parse redaction then produces “invalid connection string,” rather than the specified terminal conflict naming `sslrootcert` and `spec.tls`. D4 does not distinguish valid and invalid files. pgx also obtains roots from home-directory defaults and service files.
- Fix: Define which effective root sources conflict and their precedence, including explicit overrides. Detect conflicts before root-file loading, without exposing parser text. Extend D4 with nonexistent and malformed files, overridden environment values, and the chosen home/service-file policy.

### F7 WARNING: Shared-resolution ordering and error-parity assertions disagree

- Where: `design.md:16-21`; `tasks.md:66-75`, `:170-174`; `internal/pipeline/sink_credentials_test.go:355-357`; `internal/pipeline/wire.go:137`, `:496`, `:506`.
- Evidence: The design calls the shared resolver “after fetching the Secret.” The renamed both-sources test references an absent Secret. R2 changes all existing error comparisons to exact text, including missing-Secret cases. CLI fatal errors also add contextual prefixes.
- Why it matters: Fetching first can return a missing-Secret error instead of the required both-sources error. Existing operator and CLI missing-Secret diagnostics intentionally differ. Whole CLI error strings also cannot equal bare constructor errors without additional changes.
- Fix: Validate mutually exclusive sources before fetching either Secret. Define parity at the shared CA/configuration error boundary, preserving outer context and source-specific lookup diagnostics, or explicitly task their normalization. Narrow R2’s exact-text assertions to the specified shared errors.

### F8 WARNING: Planned TLS tests leave important behavior unproved

- Where: `tasks.md:116-133`, `:144-158`.
- Evidence: Postgres integration coverage tests successful `verify-full` and failed `require`, but no successful `require`, no `verify-ca` handshake, and no hostname-mismatch distinction. MongoDB tests rejected SRV input but not successful default-TLS SRV configuration. PI4 invokes unexported `sink.acquireBackend`.
- Why it matters: The tests can miss a broken chain-verification callback or mode semantics. A blanket handshake failure can satisfy PI2. PI4 cannot live in the Postgres package alongside PI1 and call that unexported function.
- Fix: Add successful and wrong-CA `verify-ca`/`require` cases, plus a trusted certificate with the wrong hostname that succeeds only under chain-only modes. Add accepted SRV-default and `ssl=true` cases, and document handling of URI options that implicitly enable TLS. Place PI4 explicitly in an integration-tagged `package sink` test.

### F9 WARNING: The subprocess and evidence instructions need executable details

- Where: `tasks.md:25-28`, `:43`, `:209-210`; `go.mod:3`; `hack/tools/integrationevidence/runner.go:45-71`.
- Evidence: `SSL_CERT_DIR=` means an empty environment value, not an empty directory; Go retains its default certificate directories in that case. Task 0.1 names Go 1.26, while this tree declares Go 1.27.2. The evidence runner suppresses per-test output, including successful `=== RUN` lines, from its normal console output.
- Why it matters: The subprocess does not establish the stated isolated pool, and the ordinary integration gate log cannot supply the named-run evidence the task demands.
- Fix: Set `SSL_CERT_DIR` to an actual empty temporary directory and probe the repository’s toolchain. Explicitly run the child with verbosity and an anchored test selector. Retain the raw integration JSON using `-json-out`, or run named tests separately with `-v`.

The package selection itself is correct: `INTEGRATION_PACKAGES` covers the proposed sink and pipeline files, and discovery counts their integration-tagged top-level tests. Required mode rejects skips and demands Docker; explore mode is incomplete evidence. Without Docker, container-dependent reds and greens must remain blocked until executed elsewhere.

### F10 WARNING: Compatibility documentation overgeneralizes the shared resolver change

- Where: `design.md:82`; `tasks.md:197-204`; `proposal.md:97-104`; `internal/sink/sinktls/sinktls.go:54-70`.
- Evidence: The compatibility table says missing CA keys previously meant “system roots (all types).” Kafka and NATS can instead remain plaintext when no TLS material is resolved. The proposed release-note sentence starts with four sink types without explicitly saying that resolver errors affect every type. The shared CRD comment task says to add “append,” although several consumers retain replacement pools.
- Why it matters: Users of Kafka/NATS may miss an upgrade-breaking change, and an unconditional shared-field description would contradict the explicitly retained behavior.
- Fix: Describe prior behavior by backend: system-root TLS, DSN/URI-derived behavior, or plaintext as applicable. Explicitly name the all-sink scope of missing-key and both-source errors in the release note. Qualify append semantics in shared field comments and document the unchanged Kafka/NATS/Git-probe distinction.

The resolver changes are explicitly in scope in this spec set. Their broader compatibility impact needs clearer documentation rather than removal.

## Stale file:line claims

| Claim | Cited | Actual |
|---|---|---|
| CLI resolver silently returns no CA | Proposal: `internal/pipeline/wire.go:170` | Return is at `:169`; `:170` closes the function. |
| CA is read at backend build | Proposal: `internal/sink/backend_pool.go:263` | `:263` starts `specFingerprint`. Resolution occurs at `:145`, through `internal/sink/build_context.go:38`; the uncached path is `backend_pool.go:112`. |
| MongoDB SRV resolution location | Tasks 0.1: mongo-driver `x/mongo/driver/connstring/connstring.go:995-1002` | TXT lookup is at `:994`; SRV lookup is at `:1016`. Both precede completion of ordinary SRV parsing and must be avoided during rejection-only validation. |
| pgx plaintext fallback construction | Proposal assumption cites `pgconn/config.go:883-993` | TLS slice construction is at `:1063-1071`; host expansion and assignment to main/fallback configurations are at `:542-607`. The cited range covers verification setup but not the claimed fallback construction. |

Other explicit and abbreviated line citations checked against the current tree support their associated locations.
