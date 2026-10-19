**VERDICT: BLOCK.** I found one credential-disclosure path and a validation-order mismatch. I did not establish a certificate-verification bypass.

1. **CRITICAL — Rejected S3/GCS redirects can expose credentials in errors and status.**

   **Location:** `internal/sink/s3/backend.go:199`, with propagation at `backend.go:124` and `internal/sink/s3/connection.go:47`.

   **Scenario:** A CA-trusted endpoint returns a redirect with:
   ```text
   Location: http://private.example/object?X-Amz-Credential=EXAMPLE&X-Amz-Signature=SECRET
   ```

   The new callback correctly refuses the HTTP request. However, Go subsequently assigns the **raw Location** to `url.Error.URL`: `/home/koni/.local/share/mise/installs/go/1.27.2/src/net/http/client.go:731–738`. Smithy preserves the error (`github.com/aws/smithy-go@v1.28.1/transport/http/client.go:102,146`), and the sink wraps it unchanged.

   This also survives status redaction: `internal/controller/family_sink_connection.go:58,147` passes the message through `redact.Text`, but `internal/redact/redact.go:57–60` does not match `X-Amz-Credential` or `X-Amz-Signature`. Ordinary URL userinfo would be masked in status, but remains in the raw returned error.

   **Smallest fix:** Before returning SDK errors from export, probe and deletion paths, detect `errors.Is(err, ErrCAInsecureRedirect)` and return a static terminal error preserving that sentinel. Test the final returned error and persisted message, not just the callback.

2. **WARNING — CLI credential lookup can preempt the shared terminal CA error.**

   **Location:** `cmd/cli/collect.go:119`; `internal/pipeline/wire.go:247`.

   **Scenario:** An S3 snapshot sink sets both CA sources and references a missing credential Secret.

   The operator resolves CA first (`internal/sink/build_context.go:38`), returning terminal `ErrCABothSources` before credential lookup. The CLI resolves credentials first (`collect.go:119–121`), returning a source-specific lookup error without the CA sentinel or terminal classification. Similarly, `cliBuildContext` resolves database credentials before checking CA sources.

   This is more than different lookup-error wording: one path returns the required shared configuration error; the other never evaluates it. The design explicitly requires the source check “before any Secret is fetched” (`openspec/changes/sink-ca-trust/design.md:18`).

   **Smallest fix:** Run `CheckCASources` before CLI credential resolution; align CA resolution ordering with `BuildContextFromSpec`. Add a parity case combining invalid CA sources with missing credentials.

3. **NOTE — The new MongoDB validation path retains unsafe driver error text.**

   **Location:** `internal/sink/mongodb/ca_trust.go:79–81`.

   **Scenario:** With valid CA PEM:
   ```text
   mongodb://private-db.internal:1:2/?tls=true
   ```

   The TLS-option check passes. Driver host parsing returns a `net.AddrError` containing the target (`mongo-driver@v1.17.10/x/mongo/driver/connstring/connstring.go:844–849`). `ApplyURI` stores that error (`mongo/options/clientoptions.go:426–429`), and `Validate` returns it unchanged (`:307–309`). The new wrapper exposes it.

   This target-disclosure behavior existed through `mongo.Connect` before the branch; I am **not counting it as a newly introduced vulnerability**. Nevertheless, the new CA validation path does not provide the requested target-free errors.

   **Smallest fix:** Return a static parse/validation error at this boundary, as Postgres already does.

Answers to the five review questions:

- **1 — Unintended certificate trust:** No bypass established. Postgres applies roots to the primary configuration and every fallback, rejects plaintext configurations, and installs chain verification for `require` (`internal/sink/postgres/ca_trust.go:350–380`). MongoDB checks the driver-produced configuration before replacing roots (`internal/sink/mongodb/ca_trust.go:84–89,120–136`). S3/GCS use normal hostname verification with the appended pool (`internal/sink/s3/backend.go:193`).

- **2 — Silent weakening:** The examined Postgres reader handles repeated keys, quoting, escapes, bracketed hosts and `ssl=true` consistently with the pinned parser. Specifically, empty `service=` **does override `PGSERVICE`** (`ca_trust.go:55–62`; pgx `pgconn/config.go:449–455`), and its regression test passed (`ca_trust_test.go:255`). Explicit weak modes are rejected before direct-negotiation rewriting. MongoDB’s aliases and mixed-case options are checked, with a driver backstop; SRV TXT cannot supply TLS options (`mongo-driver` `x/mongo/driver/dns/dns.go:135–148`). S3 checks SDK-selected endpoints, and `Proxy` remains nil (`backend.go:169–174`; `netguard/dialer.go:44`). `SSL_CERT_*` can change what Go considers system roots; this is explicitly part of the branch’s pool contract and tests, rather than public-root pinning.

- **3 — Error leakage:** Yes: finding 1. Finding 3 identifies retained MongoDB exposure. Postgres’s added conflict messages use fixed source labels, and its parse/connect redaction avoids interpolating the DSN.

- **4 — Operator/CLI parity:** Ordinary CA-source cases share helpers and constructors, but finding 2 breaks error precedence. `git.ValidateTLSSpec` does not bypass the shared both-sources check on these production paths. The divergent `internal/controller/secret_ca.go` resolver has no production callers in the searched tree—only its tests—so it is not evidence of an active parity defect.

- **5 — Documentation:** Finding 1 contradicts this sentence at `docs/security/security-architecture.md:232`: **“These errors name the setting, never the DSN, URI or PEM.”** The release note’s core trust and sslmode changes match the examined implementation.

Validation: `go test` passed for `./internal/sink/{postgres,mongodb,s3,gcs,sinktls}`, `./internal/sink`, and `./internal/pipeline`.

Limits: Findings above are source-derived; I did not add reproducers, mutate tests, run Docker integration suites, or exercise live SRV/proxy infrastructure. No repository files were edited.

VERDICT: BLOCK
