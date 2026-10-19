**One WARNING remains: FX1 is not fully closed.** I found no concrete certificate-trust bypass in the four sinks.

**WARNING — A redirect back to the original URL bypasses redaction**

Location: `internal/sink/s3/backend.go:243`.

Concrete input: a CA-trusted S3/GCS endpoint answers the probe with `307`, setting `Location` to the exact absolute URL requested, repeatedly.

Source-level execution:

1. At ten redirects, `refuseInsecureRedirect` returns its limit error (`backend.go:251`).
2. Go places the raw `Location` in `url.Error.URL` (`/home/koni/.local/share/mise/installs/go/1.27.2/src/net/http/client.go:731–738`).
3. Here `urlErr.URL == req.URL.String()`. The error also lacks the malformed-Location prefix, so `redirectFailed` returns false.
4. `Do` returns the original URL-bearing error (`backend.go:230`). Smithy preserves transport errors through `RequestSendError` (`/home/koni/go/pkg/mod/github.com/aws/smithy-go@v1.28.1/transport/http/client.go:92–102`).

The probe therefore exposes the server-sent URL/host instead of `ErrRedirectFailed`. This demonstrates incomplete redaction, **not exposure of a new credential**: the target equals the original request URL.

The regression test misses this because its loop redirects to a different path, `/sentinel-redirect` (`internal/sink/s3/ca_trust_integration_test.go:258`).

Smallest robust fix: track whether a redirect occurred in request-local state, preserving the existing redirect policy, and redact subsequent failures regardless of URL equality. Retain malformed-Location handling. Add an exact-original-URL loop regression. Avoid shared mutable tracking because the client handles concurrent requests.

**Round-1 closure assessment**

| Finding | Result and evidence |
|---|---|
| A1 | **Closed for rejected downgrades.** `errors.Is(err, ErrCAInsecureRedirect)` is checked before URL comparison and replaced with a static terminal error (`s3/backend.go:223`). The HTTP-target integration test passed. |
| FX1 | **Partially closed.** Malformed Location, a different-URL loop and an untrusted followed hop are covered by passing integration tests. The same-URL loop above defeats the detector. |
| A2 | **Closed for the reported ordering defect.** Both CA sources plus missing credentials, and missing CA keys plus missing credentials, now encounter CA resolution first (`pipeline/wire.go:85`, `:255`; `sink/build_context.go:38`). The parity regression at `pipeline/ca_trust_parity_test.go:170` passed. |
| A3/O1 | **Closed.** A malformed port and failed SRV lookup cannot escape through `opts.Validate()`: the cause is discarded for a static error (`mongodb/ca_trust.go:79–83`). Both concrete cases are tested at `mongodb/ca_trust_test.go:447`; the suite passed. |
| H1 | **Closed structurally.** The three children now execute inside their parent tests (`s3/ca_trust_integration_test.go:129`, `postgres/ca_trust_integration_test.go:156`, `mongodb/ca_trust_integration_test.go:109`). The subprocess helper requires successful exit and RUN/PASS markers (`sinktls/sinktlstest/sinktlstest.go:140`). S3 ran successfully; database integration execution remains unverified here. |
| D2 | **Closed for the identified concern; no splitting counterexample found.** Accepted modes are inserted into the DSN (`postgres/ca_trust.go:103`). Plaintext main/fallback configurations and hostname-check loss under expected verify-full are rejected (`:365–379`). Chain-only configurations receive certificate verification (`:383–388`). |
| D3/O2 | **Closed.** The unused controller resolver and its tests are deleted in `git diff 09541cb7c..HEAD`. |
| D4 | **Closed.** Git calls the shared check (`git/tls.go:24`); GitLab calls Git’s validation (`gitlab/config.go:37`); `sink.ErrCABothSources` aliases the shared sentinel (`ca_source.go:17`). |
| D7 | **Closed.** Every factory error returns an actual nil interface (`registry.go:94–97`). The regression at `registry_test.go:230` passed. |

O3’s release-note publication and D5/D6’s broader gate work remain deferred as recorded; this review does not establish their completion.

**Whole-branch trust and regression checks**

I checked repeated/escaped Postgres settings, `ssl=true`, environment/service precedence, multi-host fallbacks, direct negotiation, MongoDB option aliases and driver backstops, and the no-CA constructor paths.

No concrete acceptance of an unrelated CA emerged. In particular, pgx’s verify-ca callback reads the updated `RootCAs` at handshake (`pgx/v5@v5.11.0/pgconn/config.go:968–987`); MongoDB checks the driver configuration before cloning and replacing its roots (`mongodb/ca_trust.go:86–93`). The no-CA paths retain their prior client construction.

The Postgres pin test is useful property coverage, but does not demonstrate a real parser disagreement (`postgres/ca_trust_test.go:731`). I found no additional vacuous test by inspection; I did not mutation-test them.

Verification completed:

```text
go test ./internal/sink/s3 ./internal/sink/postgres ./internal/sink/mongodb \
  ./internal/sink/sinktls ./internal/pipeline ./internal/sink ./cmd/cli
All seven packages: ok

go test -tags=integration ./internal/sink/s3 ./internal/sink/gcs \
  -run TestCATrustIntegration -count=1
Both packages: ok
```

Limitations: no full `task check`, Docker-backed database integration run, or mutation testing. The same-URL counterexample is established from source, not a newly executed regression. No files were edited, committed or pushed.

VERDICT: REQUEST_CHANGES
