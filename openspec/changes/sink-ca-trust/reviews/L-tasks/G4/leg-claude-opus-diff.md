## Verdict: CLEAN

## Findings
- [NOTE] The URI key rules sit in `clientOptions`, but the design puts them in `ConfigFromSpec(…, caPEM)`. Task 4.2 names `mongodb/config.go`, and the diff leaves that file unchanged. — `internal/sink/mongodb/ca_trust.go:65`
  Failure: none I can see. `ConfigFromSpec` has no callers outside the package, and both build (`backend.go:97`) and probe (`connection.go:33`) go through `clientOptions`, so they reject the same input with the same error. What's missing is that `evidence/G4.md` doesn't mention the change from the design.
  Fix: add one line to `evidence/G4.md` Notes, or to the design, recording where the check lives.
  Confidence: 85
- [NOTE] The MI2 comment says "nothing is written", but the test never checks the collection. — `internal/sink/mongodb/ca_trust_integration_test.go:82`
  Failure: in this case the build fails, so `Export` never runs and nothing can be written. The comment still claims more than the test checks.
  Fix: drop the clause, or count the documents in the collection.
  Confidence: 70
- [NOTE] `ErrCAURIInvalid` is only exercised through `readURITLS`. No test runs it through build or probe, or checks it for leaked sentinels. — `internal/sink/mongodb/ca_trust.go:275`
  Failure: no leak today, because every reason passed in is a fixed string. A later change that interpolates the URI into a reason would pass every test.
  Fix: add one `sentinelURI+"?tls=true&a=%zz"` case to M1M2 using `requireCAError(…, ErrCAURIInvalid, "spec.tls")`.
  Confidence: 60
- [NOTE] M8 calls only `checkCAURI`, which has no DNS code. `failOnDNS` therefore can't fail there, and the M8 row's "no DNS lookup" is true by construction. M3 is the real DNS proof: rejected SRV URIs go through build and probe, and Iteration 1 records the recorded red at `ca_trust_test.go:51`. — `internal/sink/mongodb/ca_trust_test.go:352`
  Failure: none. The evidence makes M8 sound stronger than it is.
  Fix: none needed. Optionally reword the M8 row to "key check accepts".
  Confidence: 75

What I checked, and found sound:
- **Wiring:** `registry.go:156` and `probe.go:115` pass `CAPEM`. This settles matrix row 14 (independent-review): it holds.
- **No-CA path:** it is byte-for-byte today's `ApplyURI().SetDialer(netguard)` (`ca_trust.go:66-67`).
- **Order of checks:** key check, then PEM, then `ApplyURI`+`Validate`, then the backstop (nil config, `RootCAs`, `InsecureSkipVerify`), then clone with `RootCAs` set. Trust can't widen past system roots plus the CA, and a missed spelling that weakens TLS reaches the backstop.
- **Reader shapes:** a TLS-on option followed by `tls=false`, or `tls=false` followed by one, resolves last-wins. If the reader and driver ever disagree, the nil-config backstop catches it. An `@` in the query falls into a reject branch or the shape the driver also parses. `tlsAllowInvalid*` is rejected unless the value is exactly `false`, which errs toward rejecting.
- **Error text:** every error this change adds names only the option and `spec.tls`. The `mongodb connect: %w` wrap around `Validate` passes the driver's own text through, as `mongo.Connect` did before (out of scope).
- **Race safety of `failOnDNS`:** the tests that call it (M3, M4, M8) are serial. Top-level `t.Parallel` tests resume only after the serial ones finish, and `-race -count=3` passing is recorded.
- **Old call sites:** they only gained a `nil` argument; no assertion was weakened.
- **No key committed:** the integration helper generates certificates and keys at run time.

## Could not check
- The mongo-driver v1.17.10 source in the Go module cache is outside the repository and out of bounds. Two claims rest on it:
  - The claim about what turns TLS on (`tlsOnKeys`). Unchecked: whether `tlsCertificateKeyFilePassword` turns TLS on in the driver. It isn't in the reader's list or the comparison table, so at worst it causes a false rejection.
  - That `dns.DefaultResolver` is a pointer. Iteration 1's recorded red at `ca_trust_test.go:51` is indirect evidence that it is.
  - The driver's case and alias rules for `tlsAllowInvalid*`.
- I did not run any tests, mutants, lint, `task scrub` or gitleaks; I relied on the results recorded in `evidence/G4.md`. Row 16 (scrub/gitleaks) is still pending.
- SRV URIs aren't in the reader-vs-driver comparison table. I judged the SRV path by reading the code and the backstop only.
