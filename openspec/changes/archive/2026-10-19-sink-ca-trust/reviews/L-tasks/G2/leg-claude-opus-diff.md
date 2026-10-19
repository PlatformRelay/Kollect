## Verdict: CLEAN

## Findings
- [NOTE] S1/S1b/S2 do not use the counting transport that task 2.1 asks for. They prove "no request sent" by checking that the error text lacks the substring `HeadBucket`. — `internal/sink/s3/ca_trust_test.go:45`
  Failure: if `HeadBucket` were ever called before the check and its error wrapped differently (without that word), the assertion would still pass. Today nothing can reach that state, because `ConfigFromSpec` (`config.go:54-63`) and the post-build check (`backend.go:166-174`) both run before any client call. `evidence/G2.md` "Harness gaps" records this deviation.
  Fix: none needed now. Optionally inject a counting `http.Client` through a test seam if the call order ever changes.
  Confidence: 60
- [NOTE] The redirect refusal is wrapped `Terminal` (`refuseInsecureRedirect`). No test checks that the class survives the SDK's `OperationError` / `url.Error` wrapping or the retryer. I4 only asserts `err != nil`. — `internal/sink/s3/backend.go:199-201`, `internal/sink/s3/ca_trust_integration_test.go:166-171`
  Failure: if the SDK retryer treats the `url.Error` as retryable, a downgrade redirect becomes a retried transient error rather than a terminal one. That stays safe (nothing is sent in plaintext) but contradicts the stated intent. The spec does not require the class.
  Fix: add `kollecterrors.IsTerminal(err)` to I4, or drop the `Terminal` wrap.
  Confidence: 45
- [NOTE] The S3 test's netguard check compares `reflect.Value.Pointer()` of method values. That identifies `(*Dialer).DialContext`, not the `DefaultDialer` receiver. — `internal/sink/s3/ca_trust_test.go:182`
  Failure: a mutant that builds `netguard.NewDialer(nil,nil).DialContext` passes the test, and that dialer ignores the process-wide `SetAllowPrivateSinks`. Mutant 7 (plain `DefaultTransport`) is still killed.
  Fix: also assert `hc.Transport` comes from `netguard.HTTPClient`, or accept this limit and note it in the matrix.
  Confidence: 55
- [NOTE] `requireHTTPS` says "the s3 endpoint does not parse" for gcs sinks too. That branch is effectively unreachable because `parseEndpoint` already built the URL from a successful `url.Parse`. — `internal/sink/s3/config.go:138`
  Fix: change the wording to "the endpoint does not parse".
  Confidence: 80

Checked and holding:
- **Fresh transport:** `netguard.HTTPClient` clones a new transport on each call, so setting `TLSClientConfig` does not change shared state (`netguard/dialer.go:39`). `Proxy` stays nil and `MinVersion` is TLS 1.2.
- **Mixed-case schemes:** `url.Parse` lowercases the scheme, so `HTTP://` is rejected.
- **`s3://` endpoints:** the spec check skips them (`cfg.Endpoint` is empty). The SDK-chosen endpoint is checked on `client.Options().BaseEndpoint` after `NewFromConfig`, which covers the env variables and the shared-config `services` section.
- **Redirect limit:** setting `CheckRedirect` drops net/http's default limit, and the 10-redirect cap is re-implemented.
- **Garbage CA:** it fails before the scheme check and the error does not echo the input.
- **No-CA path:** it returns the unmodified netguard client.
- **Call sites:** every caller of the changed signatures is updated (`registry.go:121,125`, `probe.go:151,163`, `gcs/backend.go:35`, `gcs/connection.go:22`). No other caller exists in `internal/`.
- **Env-var test isolation:** the `t.Setenv` test is not parallel, so it does not race with the parallel tests.
- **No committed key material:** the tests generate certificates at run time, and only the public CA PEM crosses the I3 child env.

## Could not check
- I ran no tests or mutants (Bash is limited to git). The pass and kill results are taken from `evidence/G2.md`.
- `sinktls.AppendedPool` and `sinktlstest` (out of scope per the brief). I did not confirm that `ErrNoCACertificate` is terminal; the S2 test asserts it.
- The AWS SDK internals: whether any per-operation resolver can produce `http` when `BaseEndpoint` is nil, and how the retryer classifies a `CheckRedirect` error.
- Row 12: `task scrub` / gitleaks on `caTrustCreds` (`"test-secret"`). It is still pending pre-commit.
- `design.md` *Signatures* / *S3 and GCS*. I did not read them; I judged intent against `tasks.md` 2.1/2.2 and spec SCT-1.
