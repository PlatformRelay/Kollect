## Verdict: CONCERNS

## Findings
- [WARNING] The 8.5 pin only half-closes D2. The code comment says `applyCA` judges the case it misses, but that is the case `applyCA` cannot judge. — `internal/sink/postgres/ca_trust.go:96-100`, `:161-175`
  Failure: the pin goes in *front* of the DSN's keys, and pgx is last-wins (`connStringSettings` doc, `:217-219`). So if the reader misses a DSN `sslmode=prefer` that pgx sees, pgx still uses `prefer`. With `sslnegotiation=direct`, that parses to the same shape as `require`, which the comment at `:96-98` itself says `applyCA` cannot tell apart. The pin only helps in the other direction: the reader sees a mode that pgx does not.
  Fix: for the URL form, append the pin as the *last* query parameter. Last-wins then makes it authoritative, and there is no trailing-backslash risk in a URL. For the keyword form, keep the front pin but reword the comment and design note to name the remaining reader/pgx-divergence case instead of claiming the backstop covers it.
  Confidence: 60

- [WARNING] The 8.2 redaction covers only a *refused* redirect. Errors on a redirect that *was* followed still carry the server-sent `Location`. — `internal/sink/s3/backend.go:211-219`, `:225-226`
  Failure: say the CA-trusted endpoint sends a 307 to `https://other-host/...`. If that hop then fails (unknown CA, netguard refusing the dial, timeout), or the redirect limit fires (`"stopped after 10 redirects"`), net/http returns a `*url.Error` whose URL is the redirect target. `Do` passes it through unchanged, so the probe, export and status text name a URL the server chose. This is the same leak class as A1. GCS is affected too, because `gcs.NewBackend` wraps `s3.NewBackend` (`internal/sink/gcs/backend.go:35`).
  Fix: in `redactedRedirectClient.Do`, when `errors.As(err, &ue)` and `ue.URL != req.URL.String()`, set `ue.URL` to the original request URL, or return a static error. Then extend I4 with an https redirect to an untrusted host.
  Confidence: 70

- [NOTE] The new pin test fails on revert only because it clears `PGSSLMODE`/`PGSERVICE` before pgx parses. In production pgx would read the same environment and get the same mode. It shows the output states its own mode. It does not show the D2 confusion (reader and pgx splitting a DSN differently), and no case covers that. — `internal/sink/postgres/ca_trust_test.go:539-590`
  Fix: add a case where the DSN's text makes reader and pgx disagree, or describe the test in `tasks.md` 8.5 as a property test.
  Confidence: 75

- [NOTE] `errURIRefused` has no class, so a permanently invalid URI (bad port) is retried as transient forever, and the error gives no hint which part is wrong. The comment documents this trade-off. — `internal/sink/mongodb/ca_trust.go:80-84,98`
  Fix: name the failing component class without the host, e.g. "port" vs "SRV lookup", and mark the parse case terminal.
  Confidence: 50

- [NOTE] The `ResolveSinkSecretData` doc still says the function "follows the operator's order: spec.secretRef first". The CA is now resolved first. — `internal/pipeline/wire.go:72-73`
  Fix: update that doc sentence.
  Confidence: 90

## What I checked and found sound
- **8.3 (CLI CA ordering):** the CLI and the operator now share the order; the operator's version is `BuildContextFromSpec` (`internal/sink/build_context.go:38`). `--output`/stdout overrides drop `spec.tls` (`wire.go:41-47`), so CLI runs that override the sink cannot newly fail. The parity test fails if either reorder is reverted. Only the Postgres case exercises the reorder in `cliBuildContext`.
- **8.7 (shared both-sources error):** `sink.ErrCABothSources` is the same value as the `sinktls` sentinel, so `errors.Is` holds. `git/config.go:63` and `gitlab/config.go:37` return it unwrapped. No test or doc depended on the old message text.
- **8.8 (nil Backend):** the typed nil is real for every direct-return factory (`registry.go:126-162`), and no caller builds these backends outside the registry.
- **8.1 (child tests):** the child is anchored with `^name$` (`sinktlstest.go:133`). The child branch returns before `t.Parallel`, and the parent requires both the RUN and PASS lines.
- **8.6 (deleted resolver):** it has no remaining callers.
- **8.4 (MongoDB URI error):** the no-CA path is unchanged (`ca_trust.go:66-68`).

## Could not check
- **Library sources:** pgx v5.11.0 `pgconn/config.go`/`parse_url.go` (whether `prefer`+`direct` really parses identically to `require`, `ssl=false` handling), mongo-driver 1.17.10 error texts, aws-sdk-go-v2/smithy retry classification of the replaced error, and Go's `net/http/client.go` redirect error URL. All are outside the repository. The redirect-URL claim above comes from my reading of Go's `Client.do` from memory, not from the source.
- **Tests and builds:** I ran no tests or builds. The red/green claims in `tasks.md` 8.1–8.8 are unverified.
- **Other reviewers:** I deliberately did not read the other legs' outputs in `reviews/B-branch/fixes/`.
