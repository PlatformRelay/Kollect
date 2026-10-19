# Review brief — sink-ca-trust G4 (tasks 4.1, 4.2: MongoDB)

You are reviewing one task group, independently of whoever implemented it. Read-only: do not
edit, commit or push. Repository: the current directory (branch `feat/sink-ca-trust`).

Inputs:

1. TASK — `openspec/changes/sink-ca-trust/tasks.md`, section *4. MongoDB* (4.1, 4.2), plus the
   test rules at the top of that file. Spec:
   `openspec/changes/sink-ca-trust/specs/sink-tls-trust/spec.md` requirement SCT-3 (and SCT-4 for
   the garbage-PEM case). Design: `openspec/changes/sink-ca-trust/design.md` section *MongoDB*.
   Dispatch rules: `caSecretRef` wins (shared resolver, G1, not in this diff); a URI that also
   sets a CA file (`tlsCAFile` and aliases) is rejected; `tlsInsecure`, `sslInsecure`,
   `tlsAllowInvalidCertificates`, `tlsAllowInvalidHostnames` are matched case-insensitively and
   checked again after `ApplyURI`, with `tlsInsecure=false` accepted; SRV URIs handled without
   DNS; errors this change adds must not leak hosts (the driver's own host leaks are a known
   follow-up, out of scope).
2. MATRIX and SENSORS — `openspec/changes/sink-ca-trust/evidence/G4.md` (verification matrix,
   sensors run, iterations with the recorded red, mutation table). The full `task`-tier suite is
   pending: it runs once after review, not missing.
3. DIFF — the uncommitted working tree against `d5ead06c0`: `git diff d5ead06c0` (new files are
   intent-to-add, so they appear). Judge only this diff. Shared helpers it uses
   (`internal/sink/sinktls/sinktls.go` `AppendedPool`, `sinktlstest.NewCA`/`RunWithSystemRoots`)
   landed in G1 and are not under review. mongo-driver v1.17.10 source is in the Go module cache
   (`go list -m -f '{{.Dir}}' go.mongodb.org/mongo-driver`); the URI reader in
   `internal/sink/mongodb/ca_trust.go` mirrors `x/mongo/driver/connstring/connstring.go`
   `parse`, `extractDatabaseFromURI`, `extractQueryArgsFromURI` and `addOptions`;
   `mongo/options/clientoptions.go` `ApplyURI` builds the TLS config.
4. RULES — `AGENTS.md` (repo root) and `docs/development/spec-workflow.md`. Nothing private ships;
   no private key or PEM fixture with a key is committed; tests generate certificates at run time.
   Real-TLS tests are `//go:build integration` (netguard refuses loopback otherwise).

Judge, in order:

a. INTENT — does the diff do what 4.1 and 4.2 ask, all of it, and nothing more? Check every case
   M1-M8, MI1-MI3 and each file 4.2 names.
b. MATRIX — for each row: is the sensor real, is the class honest, would it fail if the claim
   were false? If you can, mutation-verify in a scratch copy OUTSIDE the repository and judge by
   `go test` exit status; never edit the repository itself. Unit:
   `go test -run 'TestMongoCATrust' ./internal/sink/mongodb/`. Integration (Docker):
   `go test -tags integration -run 'TestMongoCATrust' ./internal/sink/mongodb/`.
c. RULES — anything in AGENTS.md the diff violates (error handling, secrets, test conventions).
d. Failure modes and security posture — can a CA-configured MongoDB sink connect without
   verifying against system roots plus the CA: a URI spelling the reader misreads versus the
   driver (escapes, `;` separators, `@` or `?` in user info or query, repeated keys, present but
   empty values, letter case of names or values, `tls` versus `ssl`, options that turn TLS on, a
   later `tls=false`), an SRV URI whose TXT record changes TLS, the clone losing or keeping the
   wrong settings, the backstop missing a shape, the CA reaching only one of probe and export;
   can trust widen beyond system roots plus the CA; can any error this change adds contain the
   URI, host or credentials; is no-CA behaviour unchanged; do the DNS-failing resolver swaps
   really prove no lookup (and are they race-safe with parallel tests); hallucinated APIs,
   tautological tests, weakened assertions, prose that claims more than the code does.

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
