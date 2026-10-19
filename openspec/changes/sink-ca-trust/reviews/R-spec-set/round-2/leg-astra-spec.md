## Verdict

CONCERNS — The trust policy is implementable and consistent with the ruling, but AWS endpoint validation, one CLI test, and several conflicting requirements need correction before executing the tasks.

## Findings

### R2-1 WARNING: The loaded AWS config does not contain the final S3 endpoint

- Where: `openspec/changes/sink-ca-trust/design.md:93-96`; `tasks.md:98-100`; cached `github.com/aws/aws-sdk-go-v2/config@v1.33.6/resolve.go:108-128`; cached `github.com/aws/aws-sdk-go-v2/service/s3@v1.114.0/endpoints.go:215-230`.
- Evidence: The design says `newClient` “reads the effective base endpoint from the loaded config.” However, S3 subsequently calls `ResolveServiceBaseEndpoint(..., "S3", cfg.ConfigSources)` and assigns the result to its own `Options.BaseEndpoint`.
- Why it matters: Checking `awsCfg.BaseEndpoint` misses a service-specific endpoint supplied by `AWS_ENDPOINT_URL_S3` or a shared-config services section. Following that construction recipe can leave an HTTP endpoint unchecked. S1b should catch the environment case, so this is an implementation-round blocker rather than an undetected shipping path if the prescribed tests run.
- Fix: Replace the design sentence with: “Construct the S3 client with the guarded HTTP client and explicit endpoint override, then inspect the resulting S3 client options’ `BaseEndpoint` after service-specific endpoint resolution; reject HTTP before returning the client.” Extend S1b with a shared-config service endpoint and global/service-specific precedence cases. Preserve `Proxy = nil` and the guarded dialer.

### R2-2 WARNING: C1 expects an error that the CLI command does not return

- Where: `openspec/changes/sink-ca-trust/tasks.md:204-208`; `cmd/cli/collect.go:144-154`; `internal/pipeline/wire.go:494-506`.
- Evidence: C1 requires the command’s error to wrap the operator’s shared error, checked with `errors.Is`. Backend construction failures are stored in `ContextResult.Fatal`; `runCollectPipeline` prints them and returns `mapContextResultsToExit(results), nil`.
- Why it matters: A correctly failing collection returns exit code 2 but no command error carrying that chain. C1 cannot pass as described without changing unrelated CLI error-return semantics. A process boundary also cannot preserve Go error identity.
- Fix: Split C1 into two assertions: a pipeline-level test checks `errors.Is(result.Fatal, sentinel)` against the operator construction path; the command-level test checks exit code 2, the diagnostic, and no export. Explicitly require that the test reaches backend construction, rather than succeeding because kubeconfig or collection failed earlier.

### R2-3 WARNING: MongoDB’s normative rule rejects explicit secure settings that the design accepts

- Where: `openspec/changes/sink-ca-trust/specs/sink-tls-trust/spec.md:132-134`; `design.md:82-85`; `tasks.md:176-177`.
- Evidence: SCT-3 rejects a URI with `tlsInsecure` or `sslInsecure` “set.” The design rejects these options only when set to anything but `false`. The tests cover only `true`.
- Why it matters: `tlsInsecure=false` explicitly preserves verification. The implementation could follow the design and violate the normative spec, or follow the spec and unnecessarily reject a secure URI. The current tests distinguish neither outcome.
- Fix: Change SCT-3 to reject either option when its effective value is `true`, and reject malformed values with a static validation error. Add accepted cases for both aliases set to `false`, plus repeated/mixed-case options whose effective value follows the driver’s parsing rules.

### R2-4 WARNING: The service-file CA backstop cannot diagnose unreadable CA files

- Where: `openspec/changes/sink-ca-trust/design.md:49-58`; `proposal.md:54-56`; `tasks.md:52-53`; cached `github.com/jackc/pgx/v5@v5.11.0/pgconn/config.go:445-451`, `910-937`.
- Evidence: The design says a service-file `sslrootcert` “is caught by the backstop in step 2.” pgx merges service settings before TLS construction and reads the CA file before returning a parsed config.
- Why it matters: A missing or malformed service-selected CA file makes `ParseConfig` fail before that backstop exists. The result remains safely rejected and statically redacted, but it cannot produce the promised clear CA-conflict diagnosis. Task 0.1 mentions service files without specifying the resulting implementation or regression cases.
- Fix: Explicitly include service-selected `sslrootcert` in the pre-parse conflict check, using pgx-compatible settings precedence. Add tests for valid, missing and malformed service-selected CA files, requiring a terminal, static conflict error. Update SCT-2’s source list accordingly.

### R2-5 NOTE: Unqualified mode examples contradict the effective-mode rule

- Where: `openspec/changes/sink-ca-trust/specs/sink-tls-trust/spec.md:102-112`; `design.md:104`; `tasks.md:239-245`.
- Evidence: One scenario accepts an omitted sslmode with direct negotiation or `PGSSLMODE=verify-full`; the following scenario says a DSN “with no sslmode” fails. The compatibility table and release-note wording likewise omit the effective-mode qualification.
- Why it matters: The main requirement is clear, but copying these examples into documentation would incorrectly tell users that supported configurations fail.
- Fix: Qualify rejection examples with “effective sslmode, after environment/service defaults and direct-negotiation rewriting.” State explicitly that omitted sslmode is rejected only when it resolves to a rejected mode.

The Postgres construction itself is sound: pgx’s `verify-ca` callback closes over the TLS config whose `RootCAs` the design modifies, and `ParseConfig` retains those pointers in the main configuration and fallbacks. Rejecting every nil TLS configuration prevents plaintext fallback, including Unix-socket entries. Installing chain verification for `require` preserves its hostname distinction from `verify-full`. This reasonably reconciles “sslmode still applies” with “verification is never turned off”; it does not require another operator decision.

The shared-resolver expansion is explicitly scoped and release-noted. Kafka/NATS retain their existing trust-pool semantics, while missing keys and both sources become errors. The proposed static Postgres errors preserve its redaction boundary. I found no prescribed replacement of the Postgres/MongoDB guarded dialers; the S3 design expressly retains the guarded transport and proxy prohibition.

The integration package selection covers the proposed files. The evidence tool discovers integration-tagged top-level tests, and Docker skips cannot satisfy its required mode. The subprocess root-pool approach correctly specifies an empty directory for `SSL_CERT_DIR`; implementations must also ensure child-only tests do not introduce skips into the required integration run.

Read-only source review only; no tests or Docker probes were run.

### Round-one fix verification

The round-one record was read after the fresh source review.

| Record row | Does the fix hold? |
|---|---|
| A-F1 | Yes. Both insecure aliases, case-insensitive matching and a parsed-config backstop are specified. Explicit `false` remains inconsistent: R2-3. |
| A-F2 | Partly. Effective-endpoint rejection and redirects are specified and tested, but the construction recipe checks the wrong configuration layer: R2-1. |
| A-F3 | Partly. CLI scope now correctly excludes database sinks; C1 still has an invalid error-chain assertion: R2-2. |
| A-F4 | Yes, as a scope narrowing. Existing MongoDB driver error leakage remains explicitly outside this change. |
| A-F5 | Yes for the principal rule and D3/D5. Some examples and release wording remain unqualified: R2-5. |
| A-F6 / C-W4 | Partly. Environment and home-directory conflicts are checked before parsing; the service-file backstop claim fails for unreadable/malformed files: R2-4. |
| A-F7 / C-W1 / C-W2 / C-N2 | Yes. Both-source validation precedes fetching; shared-error identity and source-specific lookup errors are distinguished. |
| A-F8 | Yes. PI5 covers successful chain-only modes and hostname distinction; PI4 has the correct package; M8 covers SRV/default TLS. |
| A-F9 | Yes. Empty-directory isolation, anchored child execution and visible test evidence are specified. Runtime verification remains task 0.1. |
| A-F10 | Yes for all-type resolver scope and backend-specific previous behaviour. Effective-mode wording needs R2-5. |
| C-W3 | Yes. The MongoDB parsed-config backstop and mutation coverage are present. |
| C-N1 | Yes. Validation placement is described consistently between spec validation and shared client builders. |
| C-N3 | Yes. Field-description regeneration is explicitly acknowledged. |
| C-N4 | Yes. Go 1.27.2 and modification of the existing resolver cases are correctly stated. |
| A-stale | Yes. The corrected resolver, pool, pgx fallback and MongoDB lookup locations match the source. One associated ordering claim remains wrong, below. |

## Stale file:line claims

No stale numeric locations were found among the cited repository and pinned-module references. One claim attached to valid locations is inaccurate:

| Claim | Cited | Actual |
|---|---|---|
| MongoDB SRV default TLS is enabled “after the TXT and SRV lookups” (`proposal.md:148`) | `connstring.go:1000`, lookups at `:994` and `:1016` | TLS is enabled at `:1000` **after TXT lookup but before SRV lookup** at `:1016`. Change the wording; the line numbers are correct. |
