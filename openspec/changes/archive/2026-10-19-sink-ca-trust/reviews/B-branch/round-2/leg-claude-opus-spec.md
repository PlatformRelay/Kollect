## Verdict: CLEAN

## Findings
- [NOTE] The redirect redaction also drops the error's cause, so a timeout or cancel on a followed hop loses its context-error identity — `internal/sink/s3/backend.go:226-227`
  Failure: the S3 endpoint returns 307 to another HTTPS host and the export's context deadline expires on that second hop. `redirectFailed` matches because `urlErr.URL` is the hop, not the request, and returns the bare `ErrRedirectFailed`. Then `errors.Is(err, context.DeadlineExceeded)` is false, and so is any `net.Error` timeout check above it. Nothing in the diff relies on that today, but classifying a timeout is now impossible on this path. No target leaks.
  Fix: return an error that wraps both `ErrRedirectFailed` and the original `urlErr.Err`'s context or timeout sentinel, without its text, or leave as is and record it.
  Confidence: 40
- [NOTE] One of the three redirect cases is detected by matching net/http's error text, and only an integration-tagged test guards it — `internal/sink/s3/backend.go:244`
  Failure: the unparsable-Location case is caught by `strings.HasPrefix(urlErr.Err.Error(), "failed to parse Location header")`. If a Go toolchain bump rewords that message, the quoted `Location` reaches the probe and status text again. Only I4b catches that (`internal/sink/s3/ca_trust_integration_test.go:241-289`), and it runs only under `task test-integration`.
  Fix: also treat `resp != nil && resp.StatusCode/100 == 3` as a redirect failure, or add a unit test with a fake transport that runs without the integration tag.
  Confidence: 45
- [NOTE] The `withSSLMode` comment says "a later sslmode in the DSN still wins", which holds for the URL form only if pgx v5.11's `parseURLQueryParams` keeps the last value; I could not read the pgx source to confirm — `internal/sink/postgres/ca_trust.go:164-168`
  Failure: none for security either way. If pgx keeps the first value, the pinned mode always wins, which is the mode the reader already validated, so it is equal or stricter. Only the comment would be wrong.
  Fix: none needed. If pgx keeps the first value, reword the comment.
  Confidence: 25

## Round-1 fixes: all closed
- **8.1 (H1):** each child now runs inside its parent test and the separate skip-only child tests are deleted (`s3/ca_trust_integration_test.go:128-150`, postgres PI3, mongo MI3).
- **8.2 / 8.9 (A1, FX1):** the CA'd client wraps every redirect failure. A refused downgrade gives the terminal `ErrCAInsecureRedirect`; the redirect limit (URL is the next target), an unparsable Location (text match) and a failed hop (URL is the hop) give `ErrRedirectFailed` (`s3/backend.go:144-147, 218-245`). This matches net/http's `url.Error` URL rules: it is the target after a `CheckRedirect` error and the hop after a send error. GCS uses the same `newClient` (`gcs/backend.go:35`, `gcs/connection.go:22`), and so do the probe and export (`s3/connection.go:32`, `s3/backend.go:51`).
- **8.3 (A2):** the CLI checks the CA rules before the credential Secret (`internal/pipeline/wire.go:87`) and before the database Secret (`wire.go:255-260`). `cmd/cli/collect.go:119-121` returns `ExitFatalError`, which is SCT-5's exit 2. The parity test covers a missing credential Secret and a missing database Secret.
- **8.4 (A3/O1):** `errURIRefused` carries no target. It is non-terminal, as SCT-3 allows (`mongodb/ca_trust.go:80-83, 325`).
- **8.5 / 8.10 (D2, FX2):** the mode `caDSN` validated is now pinned in front of the DSN's own keys (`postgres/ca_trust.go:103`). I could not build a downgrade: under verify-full, `applyCA` still rejects a hostless config (`:376`), and a plaintext config or fallback is rejected (`:366`). The FX2 comment is now accurate (`:96-102`). FX5 is fixed (`wire.go:85-87`).
- **8.6:** `internal/controller/secret_ca.go` is deleted. No other code still uses `ca-bundle.crt` or `caSecretKeys`.
- **8.7:** `sinktls.CheckCASources` is the single check, used by `sink.CheckCASources` (`internal/sink/ca_source.go:28`) and by git/GitLab (`git/tls.go:299`). Both return the same terminal sentinel.
- **8.8:** `Registry.NewBackend` now returns a nil interface when a factory fails (`registry.go:94-100`), with a test.

## SCT compliance (spec lens)
- **SCT-1 holds:** system roots plus the CA, the netguard client, the `http` endpoint check after the SDK resolves the endpoint (`s3/backend.go:175-181`), and redirects to non-https refused. Without a CA the bare client is unchanged; the spec says redirect errors there are not required to be redacted.
- **SCT-2 holds:** sslmode precedence is DSN, then service-file entry, then `PGSSLMODE` (`ca_trust.go:72-79`); the root-cert conflict is checked against all four sources (`:108-130`); errors do not echo the DSN.
- **SCT-3 holds:** the driver-error leak is now closed.
- **SCT-4 and SCT-5 hold:** the both-sources check runs before any Secret is fetched on both paths, with the same sentinel.
- **SCT-6:** the security architecture paragraph (`docs/security/security-architecture.md:225-234`) matches the code. The release-note line is still deferred to hand-off (round-1 O3).
- **Stronger than the spec, in the safe direction:** git/GitLab both-sources is now terminal (it was a plain error before), and the pinned sslmode can only be equal or stricter than what pgx would read on its own.

## Could not check
- The pgx v5.11, mongo-driver and net/http sources in the module cache are outside the repo. I relied on my knowledge of net/http's `Client.do` and did not confirm pgx's URL duplicate-key rule.
- I ran no tests (I am read-only). The I4/I4b redirect tests, the PI3/MI3 child runs and the mutant kills are taken from `reviews/B-branch/fixes/verification.md`, not re-run.
- I did not read the `reviews/B-branch/round-2/` directory, the other legs, or the full `tasks.md` section 8 wording, so I could not compare the tasks' claims with the code word for word.
