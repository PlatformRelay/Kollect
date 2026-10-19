**WARNING — 8.2 leaves redirect-target disclosure paths open.**

Location: `internal/sink/s3/backend.go:213–229`.

The wrapper sanitizes only errors wrapping `ErrCAInsecureRedirect`. A trusted endpoint can still expose its `Location` through:

- A malformed redirect such as `http://target.invalid/%zz?token=sentinel`: Go rejects it before `CheckRedirect`, quoting the complete header.
- An HTTPS redirect chain reaching `maxRedirects`: the callback returns a different error, which Go wraps with the raw target URL.

Evidence: `$GOROOT/src/net/http/client.go:662–665` quotes malformed `Location`; lines `731–738` attach `Location` to callback errors. Both bypass the check at `backend.go:215`. Smithy preserves their text in `RequestSendError` (`github.com/aws/smithy-go@v1.28.1/transport/http/client.go:146`). S3 and GCS probes share this client.

This is an incomplete closure of A1, not a newly introduced leak. The ordinary HTTP downgrade is fixed.

**Smallest fix:** sanitize the remaining redirect-error paths on CA-configured clients, including malformed headers before Go parses them; give redirect-limit refusals a recognizable sentinel. Add malformed-header and HTTPS-loop tests asserting target markers never reach probe errors.

The remaining fixes look sound within this diff:

| Fix | Assessment and evidence |
|---|---|
| 8.1 | Child assertions remain intact and execute under the parent’s name. `sinktls/sinktlstest/sinktlstest.go:118` requires successful exit plus matching RUN/PASS output. Reverting restores the integration-gate skips. |
| 8.3 | Both CLI resolution boundaries now resolve CA first (`pipeline/wire.go:83`, `:247`). `cmd/cli/collect.go:119` uses the corrected credential resolver. The new parity cases detect either reorder being reverted. |
| 8.4 | Both MongoDB constructors use `clientOptions`; its validation failure is now static (`mongodb/ca_trust.go:80`). Other newly added CA-validation errors use fixed reasons. Existing connect/ping errors still expose driver details, but those wrappers are outside this diff and are not the URI-validation finding. |
| 8.5 | No concrete downgrade or accepted-DSN regression found. Prepending preserves later DSN precedence, including `ssl=true`; service/environment modes become explicit. Multi-host syntax remains untouched, and prefixing cannot be swallowed by a trailing backslash. Evidence: pgx v5.11.0 `pgconn/config.go:449–455`, `:768–858`, and `pgconn/parse_url.go:219` onward. |
| 8.6 | Safe deletion: `git grep` at the base commit found only the resolver and its own test callers. |
| 8.7 | Git/GitLab now return the shared terminal sentinel; the registry test checks identity and classification (`registry_test.go:207`). |
| 8.8 | Every factory error now returns a nil interface while preserving the error (`registry.go:94–100`). The five failure cases detect the prior typed-nil behavior. |

Returning `nil, err` in 8.2 does not introduce a response-handling failure: Go already closed the body, and Smithy substitutes an empty response (`transport/http/client.go:92–102`). Response status/header metadata is lost intentionally. AWS still treats the wrapped failure as retryable through `RequestSendError.ConnectionError()` and `aws/retry/retryable_error.go:116`; Kollect’s terminal classification does not stop those SDK retries. That retry behavior predates this fix.

The tests are generally honest. The Postgres test proves mode pinning survives removal of environment/service selection; it does **not** demonstrate a concrete parser-disagreement exploit. The S3 test explicitly acknowledges that PutObject’s 307 does not exercise `CheckRedirect`; its probe assertion detects the redaction regression. I did not perform revert/mutation runs.

Validation performed:

- Focused MongoDB, Postgres, registry and CLI regression tests passed.
- Full Postgres, MongoDB, S3 and sinktls unit suites passed.
- S3 system-roots and refused-redirect integration tests passed.
- Full Git suite failed two unrelated mirror-directory tests because temporary directories had mode `0775`. A focused rerun was blocked by `/tmp` being read-only.
- No additional AGENTS.md violation found. Full repository gates and Postgres/MongoDB integration suites were not rerun. No files were edited.

VERDICT: REQUEST_CHANGES
