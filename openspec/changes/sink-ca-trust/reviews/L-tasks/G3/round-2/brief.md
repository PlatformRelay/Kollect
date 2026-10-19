# Review brief — sink-ca-trust G3 (tasks 3.1, 3.2: Postgres), round 2 (last)

You are reviewing one task group, independently of whoever implemented it. Read-only: do not
edit, commit or push. Repository: the current directory (branch `feat/sink-ca-trust`).

Inputs:

1. TASK — `openspec/changes/sink-ca-trust/tasks.md`, section *3. Postgres* (3.1, 3.2), plus the
   test rules at the top of that file. Spec:
   `openspec/changes/sink-ca-trust/specs/sink-tls-trust/spec.md` requirement SCT-2 (and SCT-4 for
   the garbage-PEM case). Design: `openspec/changes/sink-ca-trust/design.md` section *Postgres*.
   Orchestrator decision B (2026-10-19): sslmode unset in DSN and `PGSSLMODE` → add
   `sslmode=verify-full` before parsing; explicit `disable`/`allow`/`prefer` → terminal error naming
   mode and source; `require` → chain check.
2. MATRIX and SENSORS — `openspec/changes/sink-ca-trust/evidence/G3.md` (verification matrix,
   sensors run, iterations with the recorded red, mutation table). The full `task`-tier suite and
   the race run are pending: they run once after review, not missing.
3. DIFF — the uncommitted working tree against `d9023cca3`: `git diff d9023cca3` (new files are
   intent-to-add, so they appear). Judge only this diff. Shared helpers it uses
   (`internal/sink/sinktls/sinktls.go` `AppendedPool`, `sinktlstest.NewCA`/`RunWithSystemRoots`)
   landed in G1 and are not under review. pgx v5.11.0 source is in the Go module cache
   (`go list -m -f '{{.Dir}}' github.com/jackc/pgx/v5`); the DSN reader in `ca_trust.go` mirrors
   `pgconn/config.go` `parseKeywordValueSettings`, `parseEnvSettings`, `parseServiceSettings`
   and `pgconn/parse_url.go` `parseURLQueryParams`.
4. RULES — `AGENTS.md` (repo root) and `docs/development/spec-workflow.md`. Nothing private ships;
   no private key or PEM fixture with a key is committed; tests generate certificates at run time.
   Real-TLS tests are `//go:build integration` (netguard refuses loopback otherwise).

Round 1 (`reviews/L-tasks/G3/leg-*.md`) found: astra P0, a bracketed URL host holding `?`
misread by `queryIndex`; opus W1 backstops untested, W2 no-CA connect errors gained a suffix naming
`spec.tls`, N3 reader edge cases untested. The fixes and their tests are recorded in
`evidence/G3.md` (*Iterations* 5, *Fix-line mutants*, *Independent review*). Check that each fix
holds, and look again for any remaining way the reader and pgx disagree on sslmode, sslrootcert,
service or servicefile.

Judge, in order:

a. INTENT — does the diff do what 3.1 and 3.2 ask, all of it, and nothing more? Check every case
   D1-D8, D3b, PI1-PI6 and each file 3.2 names.
b. MATRIX — for each row: is the sensor real, is the class honest, would it fail if the claim
   were false? Mutation-verify the new tests in a scratch copy OUTSIDE the repository (for
   example `cp -r . "$TMPDIR/g3-copy"` then edit there), judge by `go test` exit status, never
   edit the repository itself. Unit: `go test -run '<name>' ./internal/sink/postgres/`.
   Integration (Docker): `go test -tags integration -run 'TestPostgresCATrust' ./internal/sink/postgres/ ./internal/sink/`.
c. RULES — anything in AGENTS.md the diff violates (error handling, secrets, test conventions).
d. Failure modes and security posture — can a CA-configured Postgres sink end up connecting
   without verifying against system roots plus the CA: a DSN spelling the key reader misreads
   versus pgx (quoting, escapes, URL user-info containing `?`, repeated keys, `ssl=true`, an
   empty `sslmode=`), the `verify-full` insertion being overridden or swallowed, a service-file or
   environment source missed, a fallback config left on system roots or plaintext, `require` left
   unverified, the CA set on a copy that pgx's callback never reads, session resumption bypassing
   the installed `VerifyPeerCertificate`; can trust widen beyond system roots plus the CA; can any
   new error contain the DSN, host, user, password or database; is no-CA behaviour unchanged;
   hallucinated APIs, tautological tests, weakened assertions, prose that claims more than the
   code does.

For each P0/P1 you are about to file, argue the opposite in one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

RETURN at most 15 lines, nothing else, starting with a `## Verdict` heading:

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you did not verify>
summary: <one line>
```

P0 (security, invariant broken, vacuous gate presented as proof) → BLOCK. P1 (correctness gap,
untested new behaviour, dishonest evidence class, missing half of the task) → REQUEST_CHANGES.
P2 → note only.
