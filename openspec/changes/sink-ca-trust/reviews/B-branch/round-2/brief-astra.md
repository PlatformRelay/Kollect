# Review brief — sink-ca-trust, whole branch, round 2 (adversarial)

You are an adversarial reviewer, independent of whoever wrote this code. Read-only: do not edit,
commit or push. Repository: the current directory, branch `feat/sink-ca-trust`. Whole branch:
`git diff 317279084..HEAD`. Round-1 fixes: `git diff 09541cb7c..HEAD` (`git log --oneline
09541cb7c..HEAD`).

The branch makes S3, GCS, Postgres and MongoDB sinks trust a configured CA
(`spec.tls.caBundle`/`caSecretRef`) in addition to the system roots, and rejects configurations
where the CA would be ignored or verification weakened; the CA source rules (missing key, both
sources) are shared and terminal for every sink type, for the operator and the
`kollect-pipeline collect` CLI. Spec: `openspec/changes/sink-ca-trust/specs/sink-tls-trust/spec.md`;
design: `openspec/changes/sink-ca-trust/design.md`.

Round 1 found, and the branch now claims to fix (`openspec/changes/sink-ca-trust/tasks.md` section
*8. Stage-B fixes*; findings in `openspec/changes/sink-ca-trust/reviews/B-branch/round-1/verification.md`
and `reviews/B-branch/fixes/verification.md`):

- A1 + FX1: a redirect's server-sent target reached probe/export/status errors (S3/GCS). Now
  `redactedRedirectClient` in `internal/sink/s3/backend.go`.
- A2: the CLI read credential Secrets before the CA rules. Now `internal/pipeline/wire.go`.
- A3/O1: the MongoDB CA path returned the driver's URI error, which names the host. Now
  `internal/sink/mongodb/ca_trust.go`.
- H1: three integration tests skipped a child test (`task test-integration` red).
- D2: Postgres `caDSN` now pins the effective sslmode in the DSN pgx parses.
- D3, D4, D7: an unused resolver deleted; git/GitLab share the both-sources error; a nil
  `Backend` on factory error.

Library sources are in the Go module cache (`go list -m -f '{{.Dir}}' <module>`): pgx v5.11.0,
mongo-driver v1.17.10, aws-sdk-go-v2, smithy-go; Go's `net/http` is under `$(go env GOROOT)/src`.

Answer, each with evidence (`path:line`, library `path:line`, or a command and its output):

1. Is each round-1 finding closed? Try to defeat each fix with a concrete input.
2. Did any fix introduce something new: weaker verification, behaviour change without a CA, a new
   leak of a DSN, URI, host, credential or server-sent URL, a broken DSN or URI form, a parity
   break between operator and CLI, or a test that cannot fail?
3. Anything else in the whole branch that round 1 missed and that matters: can a CA-configured
   sink trust a server signed by neither the CA nor a system root, or be silently weakened?

Rank findings CRITICAL / WARNING / NOTE with location, scenario, evidence and smallest fix. End
with one line `VERDICT: BLOCK | REQUEST_CHANGES | APPROVE`. You may run `go build`/`go test`
(read-only sandbox: if writes fail, reason from the source and say so).
