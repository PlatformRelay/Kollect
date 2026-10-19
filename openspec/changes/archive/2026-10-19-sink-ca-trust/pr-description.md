# PR description — sink-ca-trust

- **Source:** `feat/sink-ca-trust`
- **Target:** `main`
- **Title:** `:lock: fix(sink)!: honour tls.caSecretRef for S3, GCS, Postgres and MongoDB`
- **Opens as:** **ready** — the operator authorised a ready PR and a rebase merge once every check
  on the head is green.

Paste everything below the line.

---

## TL;DR

- **What:** S3, GCS, Postgres and MongoDB sinks now trust `spec.tls.caBundle`/`caSecretRef`,
  appended to the system roots, and reject every DSN, URI or endpoint on which that CA could not
  take effect.
- **Why:** backlog SINK-CA-UNUSED-01: the CA was resolved for every sink type and then ignored by
  these four, so a private-CA server failed verification, or a Postgres sink on the default
  `sslmode=prefer` connected without verifying anything.
- **Risk:** behaviour change on upgrade: a CA-configured sink whose CA, DSN or URI conflicts stops
  exporting with a terminal error, and a CA-configured Postgres sink without an `sslmode` now
  verifies the hostname.
- **Reviewer action:** check the CA rules in the four sink packages and the redirect wrapper;
  approving accepts the v0.22.0 release note below and two model-aligned decisions still pending
  operator validation.

A sink that names a CA now uses it. Where the configuration means the CA would never be checked,
the sink fails at backend build with a static, value-free error instead of exporting while the CA
is silently ignored. The operator (export and connection probe) and the CLI resolve the CA through
one shared function and build the same client.

## Most important things

- **Append, never pin.** Every type builds its root pool as system roots plus the CA
  (`sinktls.AppendedPool`), matching Git/GitLab through go-git. A CA next to a publicly signed
  endpoint keeps working; a CA does not restrict trust to a private PKI. Pinning would be a separate
  opt-in field.
- **Per sink:**
  - **S3 and GCS** (GCS is the S3 client): the HTTPS transport stays on netguard with the appended
    pool, for export and probe. The `http` check runs on the *effective* endpoint, after the AWS
    SDK's own sources (`AWS_ENDPOINT_URL`, `AWS_ENDPOINT_URL_S3`, shared config), so an `s3://`
    spec cannot reach a plain-HTTP endpoint with a CA configured (`ErrCAPlainHTTP`).
  - **Postgres**: the pool goes into every TLS config pgx builds from the DSN, main and fallbacks.
    `verify-full` and `verify-ca` keep their meaning; `require` gets the chain check libpq gives it
    with a root certificate. **Unset `sslmode`** (DSN, selected service-file entry and `PGSSLMODE`)
    **becomes `verify-full`**, which also checks the hostname: a DSN host that does not match the
    certificate SAN now fails, and an explicit `sslmode=verify-ca` is the escape hatch. Explicit
    `disable`, `allow` or `prefer` is rejected (`ErrCAWeakSSLMode`). The effective mode is pinned in
    the DSN handed to pgx so `require` and `prefer`+`sslnegotiation=direct` cannot be confused.
  - **MongoDB**: the pool goes into the driver's TLS config. TLS off (`mongodb://` without
    `tls`/`ssl`, or `tls=false`) is `ErrCATLSOff`; `tlsInsecure`, `sslInsecure` or
    `tlsAllowInvalid*` is `ErrCAInsecure`. Option names match case-insensitively as in the driver,
    no SRV/TXT lookup runs before the check, and a backstop after `ApplyURI` rejects a driver-built
    config that already carries `RootCAs` or `InsecureSkipVerify`.
- **DSN/URI CA conflicts are rejected, not merged.** Postgres: `sslrootcert` in the DSN, in a
  service-file entry the DSN selects, `PGSSLROOTCERT`, or pgx's default `~/.postgresql/root.crt`,
  detected before pgx reads the file (`ErrCARootConflict`). MongoDB: `tlsCAFile` /
  `sslCertificateAuthorityFile` (`ErrCAFileConflict`). The sink's CA wins by being the only source
  allowed.
- **Redirect hardening (S3/GCS with a CA).** An HTTPS-to-HTTP redirect is refused with the static
  `ErrCAInsecureRedirect`. net/http puts the server-sent `Location` into a `*url.Error` on every
  redirect failure (limit, unparsable `Location`, failed hop, refusal), so the guarded client
  returns the static `ErrRedirectFailed` for those, matching the redirect limit by identity and
  keeping a context error's identity (`errors.Is`) without its text. No redirect target reaches an
  error or a status.
- **Two rejections now apply to every sink type, Kafka and NATS included**, because the resolver is
  shared: a `caSecretRef` Secret with none of `tls.crt`, `ca.crt`, `ca.pem` (`ErrCAKeyMissing`, was
  a silent "no CA"), and `caBundle` together with `caSecretRef` (`ErrCABothSources`, was "`caBundle`
  wins"). Git/GitLab already rejected the latter and now return the same shared error.
- **Errors are terminal and do not requeue**, and no controller watches Secrets. After fixing a
  Secret, touch the sink (change any annotation); the upgrade guide gives the command.

**Release note — hand-add to the v0.22.0 GitHub Release** (git-cliff drops `BREAKING CHANGE:`
footers):

> S3, GCS, Postgres and MongoDB sinks now trust `spec.tls.caBundle`/`caSecretRef` in addition to the
> system roots. On those types, a CA that cannot take effect now fails the sink instead of being
> ignored: an `http` endpoint, a Postgres `sslmode` set explicitly to `disable`, `allow` or `prefer`,
> another root certificate source (`sslrootcert` in the DSN, a service file, `PGSSLROOTCERT` or
> `~/.postgresql/root.crt`), a MongoDB `tlsCAFile`, a plaintext URI, or `tlsInsecure`/`sslInsecure`.
> On every sink type, Kafka and NATS included, a `caSecretRef` Secret without a CA key (`tls.crt`,
> `ca.crt`, `ca.pem`) or a spec that sets both `caBundle` and `caSecretRef` now fails the sink.
> CA-configured Postgres sinks without an `sslmode` now use `sslmode=verify-full`, verifying the
> server including the hostname: a DSN host that does not match the certificate SAN now fails; set
> `sslmode=verify-ca` explicitly to verify the chain only. These errors are terminal: after fixing a
> Secret, touch the sink (change any annotation, for example
> `kubectl annotate <kind> <name> ca-fixed-at="$(date +%s)" --overwrite`) so it is retried. See the
> upgrade guide, *Sink CA trust (after v0.21.0)*.

**Decisions this PR rests on** (recorded in the maintainer's decision log with counterpoints):

1. **Honour the CA, operator ruling A (2026-10-04):** S3, GCS, Postgres and MongoDB must use the
   resolved CA, with a test per type; the DSN-versus-`caSecretRef` precedence must be stated, not
   left to library defaults.
2. **CA semantics, outcome A (2026-10-19, model-aligned, pending operator validation):** append the
   CA to the system roots for all four types; supersedes the 2026-10-04 wording "trusts that CA and
   only that CA". Counterpoint: append never pins.
3. **Postgres `sslmode` with a CA, option B (2026-10-19, model-aligned, pending operator
   validation):** unset → `verify-full`, explicit weak mode → error, `require` → chain check.
   Counterpoint: the hostname check can fail existing sinks whose DSN host does not match the SAN;
   `verify-ca` is the escape hatch.

The missing-key and both-sources rejections on every sink type, Kafka and NATS included, are the
spec's own choice (SCT-4, because the resolver is shared), not a logged decision; they are
release-noted.

## Risk

- **What can break:** CA-configured sinks whose configuration was quietly wrong start failing:
  Postgres on an unset `sslmode` with a mismatched hostname, any type with a `caSecretRef` Secret
  missing a CA key or both CA sources set, MongoDB/Postgres connection strings carrying their own CA
  or disabling verification, S3/GCS on `http`.
- **Blast radius:** only sinks that set `spec.tls`; sinks without a CA take the unchanged code path
  (no-CA controls in every package). A failing sink stops exporting and shows the terminal error on
  its status and connection test; other sinks are unaffected.
- **Rollback:** revert, or roll back to v0.21.x; nothing is persisted.
- **Residual risk accepted:** MongoDB driver errors that existed before (`mongodb connect/ping: %w`)
  can still carry hosts (follow-up, non-goal); Kafka, NATS and the Git probe keep a CA-only pool
  (inconsistency recorded as a follow-up); `kollect-pipeline` does not load database sinks, so
  Postgres/MongoDB parity is proven at the shared constructors only.

## Files to look at first

- `internal/sink/ca_source.go`, `internal/sink/sinktls/sinktls.go` — the shared CA rules and
  appended pool; `ErrCABothSources` now lives in `sinktls` and `internal/sink/git/tls.go` returns it.
- `internal/sink/build_context.go`, `internal/pipeline/wire.go` — operator and CLI credential/CA
  resolution; the CLI now checks the CA before credential Secrets, as the operator does.
- `internal/sink/postgres/ca_trust.go`, `pool.go` — DSN reader, sslmode upgrade and root-source
  conflicts; read it against pgx's parser, it mirrors it.
- `internal/sink/mongodb/ca_trust.go` — URI reader, alias handling and the post-`ApplyURI` backstop.
- `internal/sink/s3/backend.go`, `config.go` — effective-endpoint check and the redacted redirect
  client.
- `internal/controller/secret_ca.go` (deleted) — the unused third resolver, test-only, no callers.
- Docs: `docs/operator-manual/upgrading.md` (*Sink CA trust*), `docs/crds/kollectdatabasesink.md`,
  `docs/crds/kollectsnapshotsink.md`, `docs/security/security-architecture.md`,
  `docs/REQUIREMENTS.md` FR-EXP-7.

**Review this commit-by-commit, not as one flattened diff.** The feature lands per sink type
(shared rules, S3/GCS, Postgres, MongoDB, parity, docs), then one commit per branch-review fix; the
flattened diff hides which fix answers which finding.

## Testing protocol

- [x] `task check` green on the tip (three environment-only guards green with `kubectl` and a
  linux `operator-sdk` supplied); `task docs:verify` green locally.
- [x] `task test-integration`: every new TLS test in the `=== RUN` list, none skipped.
- [x] Branch review: two rounds of review legs, no CRITICAL open.
- [ ] Every check on the PR head green: CI (lint, unit, `test-integration` reporting all tests ran,
  `verify`), Docs, dependency-review (`jackc/pgservicefile` moves from indirect to direct),
  vulncheck, SonarCloud.
- [ ] Before tagging v0.22.0: on a lab cluster, a `KollectDatabaseSink` (postgres) with
  `caSecretRef` and no `sslmode`, pointed at a server signed by that CA with a matching SAN —
  `KollectConnectionTest` succeeds; change the DSN host to an IP not in the SAN — the test fails
  with a TLS verification error that names no host.
- [ ] Same lab: set `caBundle` and `caSecretRef` on a Kafka sink — status shows
  `spec.tls sets both caBundle and caSecretRef; set exactly one`.

---

## Optional reading — the long version

*Nothing above depends on this. Read it if you want the full story, the road not taken, or the
deployment fine print — skip it if the top half already told you what you needed.*

- **Spec set:** `openspec/changes/sink-ca-trust/` (archived in this PR's last commit): proposal,
  design, `specs/sink-tls-trust/spec.md` (SCT-1 to SCT-6), tasks with the verification table,
  per-group evidence and review records.
- **Why reject instead of merge DSN CAs:** merging two CA sources makes trust depend on pgx's or the
  driver's precedence, which the 2026-10-04 ruling said must be stated, not left to defaults.
  Rejecting keeps one answer per sink.
- **Why upgrade unset `sslmode` instead of rejecting it:** the first draft rejected it, which would
  have failed every existing CA-configured Postgres sink that relied on the default. `verify-full`
  is what a user who configured a CA most likely meant.
- **Readers that mirror libraries.** The Postgres DSN and MongoDB URI readers decide before the
  library parses, so they are tested on differential tables against `pgconn`/`connstring`
  (including inputs the library refuses), and each has a post-parse backstop.
- **Test shape.** Real TLS tests use generated certificates (no key in the repository) and run in
  the integration tier because netguard refuses loopback outside it. "Still trusts public roots"
  re-executes the test binary with `SSL_CERT_FILE` set to a stand-in root; the child runs inside its
  parent test so `task test-integration` counts no skip.
- **Out of scope:** strict pinning, Kafka/NATS/Git-probe pool alignment, CA rotation without
  restart (SECRET-ROTATION-01), database sinks in the CLI, early CA resolution in the CLI
  (CLI-CA-EARLY-01), Secret watches, redacting pre-existing MongoDB driver errors, mTLS from the
  spec.

BREAKING CHANGE: CA-configured Postgres sinks now verify the hostname: with `spec.tls` set and no
`sslmode`, the sink uses `sslmode=verify-full`, so a DSN host that does not match the server
certificate's SAN fails (set `sslmode=verify-ca` to verify the chain only). Conflicting DSN/URI CA
settings are rejected: a Postgres `sslrootcert`/`PGSSLROOTCERT`/service-file root or a MongoDB
`tlsCAFile` next to `spec.tls` now fails the sink with a terminal error.
