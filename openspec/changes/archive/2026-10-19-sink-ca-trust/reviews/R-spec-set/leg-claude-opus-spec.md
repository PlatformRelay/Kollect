## Verdict: CONCERNS

## Findings
- [WARNING] The both-set check is specified to run after the Secret is fetched, so R2's renamed case can never see the both-set error. — `openspec/changes/sink-ca-trust/design.md:16-20`, `tasks.md:69`, `internal/pipeline/sink_credentials_test.go:356-358`
  Failure: the design has both callers fetch the Secret "their own way" before calling the shared resolver. The existing case being renamed to "both sources rejected" uses `CASecretRef: ref("absent")`. Both paths therefore fail on not-found first, with different text: the operator returns a k8s NotFound and the CLI returns "not found in config directory" (`wire.go:137`). R2's text comparison fails, or the both-set rule is never exercised.
  Fix: reject `caBundle`+`caSecretRef` in the shared function before any fetch (split it into a spec check plus key selection), and state that order in design.md.
  Confidence: 85
- [WARNING] R2's "compare error text, not only presence" turns red the not-found cases the change doesn't touch. — `tasks.md:67-69`, `internal/pipeline/sink_credentials_test.go:346-347,355,384`
  Failure: "both, default absent", "gitAuth absent" and "caSecret absent" fail on both paths today with deliberately different text. A blanket text check breaks them, or pushes the implementer to rewrite the not-found messages, which is out of scope.
  Fix: limit the text comparison to the errors from the shared CA resolver (missing key, both set), as SCT-5's scenario table already does (`spec.md:182-185`).
  Confidence: 80
- [WARNING] The MongoDB conflict check has no backstop after parsing, unlike Postgres. — `design.md:61-65`, `spec.md:104-110`
  Failure: task 0.1 allows a hand-rolled query parse (`tasks.md:43-45`). A spelling the hand parser misses (`TLSCAFILE=/x`, since driver keys are case-insensitive, or a repeated key) gets past it. `ApplyURI` then loads the file and kollect overwrites `RootCAs`, so the URI's CA is silently replaced instead of rejected. That breaks the ruling's "rejected with a clear message".
  Fix: after `ApplyURI`, also reject when the driver-built `TLSConfig.RootCAs != nil` or `InsecureSkipVerify` is set, mirroring Postgres step 1 (`design.md:53-54`). Add a mixed-case case to M1.
  Confidence: 70
- [WARNING] SCT-2 names only the DSN and `PGSSLROOTCERT` as sources of `sslrootcert`, not pgx's home-directory default. — `spec.md:59-60`, `design.md:52-54`
  Failure: from memory, pgx's `defaultSettings` adds `~/.postgresql/root.crt` when that file exists (I could not read the source; see below). A CLI user with that file, a DSN without `sslrootcert` and a sink CA would then be rejected by the design's `RootCAs != nil` backstop, with a message pointing at a parameter they never set.
  Fix: add a check to 0.1 for the home-directory default. Have SCT-2 and the error text name all three sources, and add a D4 variant with `HOME` pointing at a temp dir.
  Confidence: 60
- [NOTE] The design puts conflict validation in two different places. — `design.md:33-34` vs `design.md:38-43`
  Failure: the Signatures section says validation is in `ConfigFromSpec`, while the Postgres section classifies sslmode after `pgxpool.ParseConfig` (`pool.go:17`). For S3, `ConfigFromSpec` (`s3/config.go:27`) doesn't receive the CA, yet the http rejection is placed there (`design.md:71-72`).
  Fix: say that `ConfigFromSpec` gains `caPEM` in all four packages, or put the checks in `newGuardedPool`/`newClient`, which probe and export both reach.
  Confidence: 75
- [NOTE] SCT-5 asks for "same error text", but the CLI adds prefixes the operator doesn't. — `spec.md:176-177`, `internal/pipeline/wire.go:496,506`
  Fix: say "the same resolver/build error, wrapped" and have C1 compare with `errors.Is`/contains.
  Confidence: 70
- [NOTE] The proposal says "No CRD schema change", but task 6.1 edits CRD field comments and regenerates the schema golden. — `proposal.md:84`, `tasks.md:197-199`
  Fix: change the proposal to "no CRD schema change except descriptions".
  Confidence: 90
- [NOTE] Task 0.1 says "Go 1.26", but `go.mod:3` is `go 1.27.2`. Also, R2 says "add" `"caSecret no key"`, but that case already exists at `sink_credentials_test.go:354`; it should say "change its expectation". — `tasks.md:46,70`
  Confidence: 95

Requirement status (spec vs. tasks/design and today's code):
- **SCT-1:** delivered by 2.1/2.2, which cover http rejection, the netguard/proxy invariants, append, and probe parity (`probe.go:151,163`, `registry.go:120-126`).
- **SCT-2:** delivered, apart from the home-directory gap above. pgx semantics match libpq's require-with-root rule.
- **SCT-3:** partial: the backstop above is missing.
- **SCT-4:** delivered. "Failure then recovery" already holds, because failed builds are never pooled (`backend_pool.go:145-153`, `:183`).
- **SCT-5:** partial: the ordering and text-scope issues above.
- **SCT-6:** delivered by 6.1.

Fit with the ruling: append, CA wins and conflicts rejected, sslmode kept, never-off are all honoured. Rejecting unset/`prefer` goes further than the ruling literally says, but it is consistent with "never ignored" and is release-noted.

File:line claims I verified: `build_context.go:38,215-245,225,244`, `wire.go:143-171,257,494,523`, `registry.go:120-157`, `probe.go:91,115,151,163`, `s3/backend.go:124-149,127`, `postgres/pool.go:16-24,69-76`, `mongodb/backend.go:97`, `mongodb/connection.go:34`, `gcs/backend.go:26-40`, `gcs/connection.go:13-21`, `sinktls.go:24-26,44`, `kollectsink_types.go:342`, `git/tls.go:23-25`, `netguard/dialer.go:242-249`, `integration_policy.go:1-13`, `backend_pool.go:94,112,145,263`, controller `:91` and `:49`, `REQUIREMENTS.md:90`, `security-architecture.md:221,243`, `exitcode.go:17`, and the versions in `go.mod` (pgx v5.11.0, mongo-driver v1.17.10, go-git v5.19.3). All hold.

## Could not check
- The pgx, mongo-driver and go-git sources (module cache, outside the repo): `configTLS` lines, `defaultSettings` home-directory `root.crt`, whether `tlsAllowInvalidCertificates` is absent, SRV resolve in connstring.
- `agent-context/decisions.md` (the ruling text itself). I relied on the proposal's account of it.
- No tests run. I didn't read `openspec/changes/sink-ca-trust/reviews/` (other reviewers), `loop.md` or `task-prompt.md`.
