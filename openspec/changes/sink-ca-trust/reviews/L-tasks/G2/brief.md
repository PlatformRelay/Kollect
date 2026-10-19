# Review brief — sink-ca-trust G2 (tasks 2.1, 2.2: S3 and GCS)

You are reviewing one task group, independently of whoever implemented it. Read-only: do not
edit, commit or push. Repository: the current directory (branch `feat/sink-ca-trust`).

Inputs:

1. TASK — `openspec/changes/sink-ca-trust/tasks.md`, section *2. S3 and GCS* (2.1, 2.2), plus the
   test rules at the top of that file. Spec:
   `openspec/changes/sink-ca-trust/specs/sink-tls-trust/spec.md` requirement SCT-1 (and SCT-4 for
   the garbage-PEM case). Design: `openspec/changes/sink-ca-trust/design.md` sections
   *Signatures* and *S3 and GCS*.
2. MATRIX and SENSORS — `openspec/changes/sink-ca-trust/evidence/G2.md` (verification matrix,
   sensors run, iterations with the recorded red, mutation table). The full `task`-tier suite is
   pending: it runs once after review, not missing.
3. DIFF — the uncommitted working tree against `0cdece478`: `git diff 0cdece478` (new files are
   intent-to-add, so they appear). Judge only this diff. Shared helpers it uses
   (`internal/sink/sinktls/sinktls.go` `AppendedPool`, `internal/sink/sinktls/sinktlstest`) landed
   in G1 and are not under review.
4. RULES — `AGENTS.md` (repo root) and `docs/development/spec-workflow.md`. Nothing private ships;
   no private key or PEM fixture with a key is committed; tests generate certificates at run time.
   Real-TLS tests are `//go:build integration` (netguard refuses loopback otherwise).

Judge, in order:

a. INTENT — does the diff do what 2.1 and 2.2 ask, all of it, and nothing more? Check every case
   S1, S1b (each env/config source), S2, S3, I1-I4 and G1, and each file 2.2 names.
b. MATRIX — for each row: is the sensor real, is the class honest, would it fail if the claim
   were false? Mutation-verify the new tests in a scratch copy OUTSIDE the repository (for
   example `cp -r . "$TMPDIR/g2-copy"` then edit there), judge by `go test` exit status, never
   edit the repository itself. Integration tests: `go test -tags integration -run '<name>' <pkg>`
   (httptest only, no Docker needed).
c. RULES — anything in AGENTS.md the diff violates (error handling, secrets, test conventions).
d. Failure modes and security posture — can a CA-configured S3/GCS client reach an `http`
   endpoint by any route (spec endpoint, SDK env or shared config, redirect, mixed-case scheme,
   an endpoint the SDK resolves later than `client.Options().BaseEndpoint`, per-operation
   endpoint resolution); does the CA client still use netguard's dialer with no proxy; can trust
   widen beyond system roots plus the CA; can an error echo the PEM or credentials; is no-CA
   behaviour unchanged; hallucinated APIs, tautological tests, weakened assertions, prose that
   claims more than the code does.

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
