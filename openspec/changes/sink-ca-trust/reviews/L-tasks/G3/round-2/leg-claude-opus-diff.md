## Verdict: CLEAN

No P0 or P1. The diff covers 3.1 and 3.2. I checked cases D1–D8, D3b and PI1–PI6 against the test code. All named call sites are wired: `registry.go:138`, `probe.go:91`, `backend.go:56`, `connection.go:25`. `postgres/config.go` is named in 3.2 but needed no change. The round-1 fixes hold:
- **astra P0:** `queryIndex` now follows pgx's bracket and path rules (`ca_trust.go:179-211`). An invalid sslmode is rejected without being echoed (`ca_trust.go:89-93`).
- **opus W1:** the backstops are now tested directly (`TestApplyCA_backstops`).
- **opus W2:** the TLS suffix is added only when a CA is configured: `withCAHint` (`pool.go:137`) is applied at the probe, `NewBackend` and `Export`.

The safety argument does not depend on the DSN reader being right. After `applyCA` (`ca_trust.go:337-375`), every config is TLS, its `RootCAs` is the system roots plus the CA, and it either fully verifies or carries a chain check. Any config pgx parsed as plaintext, with roots pgx loaded itself, or chain-only when verify-full was expected, is rejected as terminal. pgx's verify-ca callback reads `RootCAs` from the same pointer `applyCA` sets, before the pool clones it. Errors use static source labels; parse errors go through `redactedParseError`.

## Findings
- [NOTE] Row 20 claims more than its sensor proves — `ca_trust_test.go:607-637`
  Failure: the test checks the end-to-end `poolConfig` result. The rejected cases ("ssl=true then prefer", "repeated, prefer last", "fragment after the mode") would also pass if the reader misread the DSN and the `applyCA` backstop caught the result, because that path returns the same `ErrCAWeakSSLMode`. "Reader agrees with pgx" is proven only for the upgraded and kept cases.
  Fix: either reword row 20 to "poolConfig outcome", or assert on `caDSN`'s returned mode for each case.
  Confidence: 80
- [NOTE] A URL DSN with a fragment and no query gets its `?sslmode=verify-full` appended after the `#` — `ca_trust.go:157-167`
  Failure: `postgres://u@h/db#x` becomes `…#x?sslmode=verify-full`. pgx drops the fragment, falls back to `prefer` with a plaintext fallback, and the backstop rejects it with "a connection would not use TLS". This fails closed, but the error is misleading for a DSN that sets no sslmode at all, and no test covers the case.
  Fix: insert before the first `#` (or reject URL fragments outright), and add a case to the table test.
  Confidence: 55 (depends on pgx v5.11 dropping fragments, which I could not read)
- [NOTE] Residual hostname downgrade (not a trust widening) — `ca_trust.go:83-96`
  Failure: suppose the reader finds a valid chain-only sslmode that pgx does not see, and the DSN also has `sslnegotiation=direct`. pgx would then make `prefer` into `require`, and the sink ends up chain-checked instead of being upgraded to verify-full. Only the DSN author can write such a DSN, and they could set `require` openly. I could not build a concrete string against pgx's parser.
  Fix: none needed; optionally reject a config that is chain-only when no sslmode came from an explicit source.
  Confidence: 25
- [NOTE] `rootCertSource` is stricter than pgx when the DSN sets `sslrootcert=` empty — `ca_trust.go:103-105`
  Failure: pgx lets that empty DSN value override `PGSSLROOTCERT` and `~/.postgresql/root.crt`, but the reader skips it and rejects the config anyway. This fails closed and is consistent with the conflict rule, but it goes beyond what the spec asks.
  Fix: document it, or treat an explicit empty DSN value as "no root".
  Confidence: 60

## Could not check
- The pgx v5.11.0 source is in the module cache, outside the repository, so I did not read it. The evidence relies on it for two claims I could not confirm: `unescapeKeywordValue` drops every backslash, and `parseURLQueryParams` handles last-wins, the `ssl=true` alias, fragments and key trimming the way the reader assumes. I rated the reader against the backstops instead.
- I ran no tests and no scratch-copy mutants (my access is read-only git). Every pass or kill in the matrix comes from the recorded `evidence/G3.md`, not from me. Rows 17 and 19 (scrub/gitleaks, race) are still pending there.
- I did not re-read `sinktls.AppendedPool`, `sinktlstest.NewCA` or `RunWithSystemRoots` (landed in G1, out of scope), the `acquireBackend` caching path beyond what PI4 exercises, or `docs/development/spec-workflow.md`.
