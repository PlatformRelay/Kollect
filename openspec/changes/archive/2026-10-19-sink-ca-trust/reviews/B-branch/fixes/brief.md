# Review brief — sink-ca-trust stage-B fixes (security)

You are reviewing a set of fix commits, independently of whoever wrote them. Read-only: do not
edit, commit or push. Repository: the current directory, branch `feat/sink-ca-trust`. Diff under
review: `git diff 09541cb7c..HEAD` (eight commits, `git log --oneline 09541cb7c..HEAD`). Judge only
this diff; the rest of the branch was reviewed before.

Context: the branch makes S3, GCS, Postgres and MongoDB sinks trust a configured CA
(`spec.tls.caBundle`/`caSecretRef`) in addition to the system roots, and rejects configurations
where the CA would be ignored or verification weakened. Spec:
`openspec/changes/sink-ca-trust/specs/sink-tls-trust/spec.md` (SCT-1 … SCT-6). Each fix is a line
of `openspec/changes/sink-ca-trust/tasks.md` section *8. Stage-B fixes*; the findings they answer
are in `openspec/changes/sink-ca-trust/reviews/B-branch/round-1/verification.md` (A1, A2, A3/O1,
H1, D2, D3, D4, D7).

The fixes:

- 8.1 PI3/MI3/I3 integration tests run their subprocess child branch inside the parent test.
- 8.2 `internal/sink/s3/backend.go`: `redactedRedirectClient` replaces a refused redirect's
  `*url.Error` (whose text holds the `Location`) with the static terminal `ErrCAInsecureRedirect`.
- 8.3 `internal/pipeline/wire.go`: the CLI resolves the CA (shared check, Secret, key) before any
  credential or database Secret, as the operator's `BuildContextFromSpec` does.
- 8.4 `internal/sink/mongodb/ca_trust.go`: a URI the driver refuses after the CA checks gives a
  static error, not the driver's text (which names the host).
- 8.5 `internal/sink/postgres/ca_trust.go`: `caDSN` pins the effective sslmode it validated
  (`withSSLMode`) in front of the DSN keys, so pgx uses it or a later DSN mode.
- 8.6 delete `internal/controller/secret_ca.go` (unused resolver).
- 8.7 the both-sources check and sentinel move to `internal/sink/sinktls`; `git.ValidateTLSSpec`
  uses them.
- 8.8 `Registry.NewBackend` returns a nil interface when a factory fails.

Library sources are in the Go module cache (`go list -m -f '{{.Dir}}' <module>`): pgx v5.11.0
(`pgconn/config.go`, `pgconn/parse_url.go`), mongo-driver v1.17.10, aws-sdk-go-v2 and smithy-go;
Go's `net/http/client.go` is in `$(go env GOROOT)/src`.

Judge, with evidence (`path:line`, library `path:line`, or a command you ran and its output):

1. Does each fix close its finding completely? Look for the same defect on a path the fix did not
   touch (other S3/GCS operations or clients, other CLI entry points, other MongoDB error paths).
2. Does any fix weaken verification, change behaviour without a CA, or introduce a new leak? For
   8.5: can pinning the mode ever make pgx use a weaker mode than before, or break a DSN that
   worked (URL forms, `ssl=true`, multi-host, service files, a trailing backslash)? For 8.2: does
   returning `nil, err` from `Do` change SDK retry or response handling in a way that matters?
3. Is every new test able to fail if its fix were reverted, and are the tests honest about what
   they prove?
4. Anything in the repo's `AGENTS.md` rules the diff breaks (error handling, secrets, tests).

Rank findings CRITICAL / WARNING / NOTE, each with location, scenario, evidence and the smallest
fix. End with one line `VERDICT: BLOCK | REQUEST_CHANGES | APPROVE`. You may run `go build` and
`go test` (read-only sandbox: if writes fail, reason from the source and say so).
