## Verdict: CONCERNS

## Findings
- [WARNING] No test covers the two `applyCA` backstops (a plaintext config gives `ErrCAWeakSSLMode`; `RootCAs` already loaded by pgx gives `ErrCARootConflict`), and the mutation table has no mutant that removes either one. These two checks are what the design, step 2, relies on for any DSN that `caDSN` misreads — `internal/sink/postgres/ca_trust.go:322`, `:327`
  Failure: say the pre-parse reader and pgx disagree. Two examples: a URL with a fragment, `postgres://h/db?sslmode=disable#x`, where the reader sees `disable#x`; or a home directory that pgx resolves differently. Then only these two checks stop a plaintext connection or a pgx-loaded root. If someone deletes either check, every recorded test still passes.
  Fix: add unit cases that reach the backstops. For example, call `applyCA` on a `pgxpool.ParseConfig("… sslmode=prefer")` result and expect `ErrCAWeakSSLMode`, and on one parsed with `sslrootcert=<file>` and expect `ErrCARootConflict`. Add a mutant for each.
  Confidence: 85
- [WARNING] Sinks with no CA now get the TLS suffix too, and its text names a CA they do not have. That contradicts matrix row 16, "No-CA behaviour unchanged" — `internal/sink/postgres/pool.go:119`, `:127`
  Failure: a no-CA `verify-full` sink hits a hostname mismatch, or a CA set by `sslrootcert` in its DSN rejects the server. The error then says "verification failed against the system roots and the spec.tls CA", but no spec.tls exists. That points the operator at the wrong setting. Row 16's sensor only compares TLS config shape, so it cannot see this change.
  Fix: either only add the suffix when the backend was built with a CA (pass a flag or keep it on `Backend`), or give no-CA sinks wording that does not mention spec.tls. Then correct row 16, or narrow its claim.
  Confidence: 80
- [NOTE] The hand-written DSN reader has no direct unit test for the edge cases the brief lists — `internal/sink/postgres/ca_trust.go:181`, `:293`, `:143`
  Failure: none of these has a test: an empty `sslmode=` (the comment says it is rejected), `ssl=true` alone or before or after `sslmode`, repeated keys, quoted or escaped values, `?` in the URL user-info, a `#fragment`. The comment on `withVerifyFull` claims a trailing backslash cannot swallow the added pair; nothing tests that either. `unescape` also drops every backslash, while pgx only unescapes `\\` and `\'`, so `sslmode=verify\-full` passes the check and then fails in pgx's parse. That fails closed but differs from pgx. A fragment URL makes `withVerifyFull` append `?sslmode=…` after the `#`, and the user then gets the misleading "would not use TLS" error.
  Fix: a table test of `caDSN` inputs against the expected outcome (rejected, upgraded or left as written), with the fragment case included.
  Confidence: 70
- [NOTE] The home directory is resolved with `os.UserHomeDir` (`$HOME`). If pgx v5.11's `defaultSettings` takes it from `user.Current().HomeDir`, the two can point at different directories — `internal/sink/postgres/ca_trust.go:100`, `:119`
  Failure: when the home in `/etc/passwd` differs from `$HOME`, the reader misses `root.crt` there; the backstop then rejects it with a generic message. If it misses a service file there, the added `verify-full` overrides the service's `require` or `verify-ca`, and a SAN-mismatched host fails with a hostname error.
  Fix: check pgx's `defaults.go` and use the same home source, or note the backstop in a comment.
  Confidence: 35
- [NOTE] Checked and found sound. Every error path is static apart from `%q` of a value from a fixed set (no DSN, host, user, password or database). `RootCAs` is set on the parsed configs, which pgx's `verify-ca` closure captures by pointer, so the CA is not lost on a copy. `require` gets a chain check with the Server Auth key usage. The G123 suppression holds as long as pgx sets no `ClientSessionCache`. Fallbacks are covered. The probe and registry pass `CAPEM`. The no-CA config path is unchanged (`pool.go`, the `len(caPEM)==0` branch). Settings precedence (DSN, then service file, then environment) matches my reading of pgx. No private key or PEM fixture is committed: the test helper generates the key at run time and is behind the `integration` build tag. The D1–D8, D3b and PI1–PI6 tests exist and assert what the task names.

## Could not check
- pgx v5.11.0 source in the module cache, which is outside the repository: `parse_url.go` (how the URL query is split, `?` in user-info, fragment handling, `+` decoding), the `ssl=true` alias, and the home-directory source in `defaults.go`. Points 3 and 4 rest on my memory of pgx, not on reading v5.11.
- I ran no tests and no mutants: I am limited to read-only git, so the mutation table and the integration (container) results are taken from `evidence/G3.md` unverified.
- `internal/sink/postgres/ca_trust_integration_test.go`, plus the changes to `connection_test.go`, `pool_test.go`, `export_integration_test.go` and `backend_export_redact_test.go`, were not read line by line.
- The pending sensors in matrix rows 17 (scrub, gitleaks) and 19 (race), and the full task-tier suite.
