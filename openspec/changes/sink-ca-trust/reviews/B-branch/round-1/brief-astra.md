# Review brief — sink-ca-trust, whole branch (stage B, adversarial)

You are an adversarial reviewer of a whole feature branch, independent of whoever wrote it.
Read-only: do not edit, commit or push. Repository: the current directory, branch
`feat/sink-ca-trust`. The range under review is `git diff 317279084..HEAD` (merge base with
`origin/main`); `git log --oneline 317279084..HEAD` lists the commits.

What the branch does: kollect is a Kubernetes inventory exporter with pluggable sinks. Every sink
spec has a `spec.tls` (`caBundle` or `caSecretRef`). Before this branch, S3, GCS, Postgres and
MongoDB ignored that CA. The branch makes them trust the system roots plus the configured CA, and
rejects configurations where the CA would be ignored or verification weakened. It also makes the
CA source rules (missing key, both sources set) shared and terminal for every sink type, for the
operator and the `kollect-pipeline collect` CLI alike.

Inputs:

- Spec: `openspec/changes/sink-ca-trust/specs/sink-tls-trust/spec.md` (SCT-1 … SCT-6).
  Design: `openspec/changes/sink-ca-trust/design.md`. Tasks and test rules:
  `openspec/changes/sink-ca-trust/tasks.md`. Per-group evidence: `openspec/changes/sink-ca-trust/evidence/G1.md` … `G6.md`.
- Product code: `internal/sink/sinktls/sinktls.go` (`AppendedPool`), `internal/sink/ca_source.go`,
  `internal/sink/build_context.go`, `internal/pipeline/wire.go` (CLI resolver),
  `internal/sink/s3/{config,backend,connection}.go`, `internal/sink/gcs/`,
  `internal/sink/postgres/{ca_trust,pool,backend,connection,errors}.go`,
  `internal/sink/mongodb/{ca_trust,backend,connection,errors}.go`, `internal/sink/{registry,probe}.go`.
- Docs: `docs/crds/kollectsnapshotsink.md`, `docs/crds/kollectdatabasesink.md`,
  `docs/operator-manual/upgrading.md`, `docs/operator-manual/troubleshooting.md`,
  `docs/security/security-architecture.md`, `docs/guides/pipeline-cli.md`, `docs/REQUIREMENTS.md`
  (FR-EXP-7), `api/v1alpha1/kollectsink_types.go` field comments, and the release-note line in
  `openspec/changes/sink-ca-trust/pr-description.md`.
- Library sources are in the Go module cache: pgx v5.11.0
  (`go list -m -f '{{.Dir}}' github.com/jackc/pgx/v5`, see `pgconn/config.go`,
  `pgconn/parse_url.go`, `pgconn/defaults.go`), mongo-driver v1.17.10
  (`x/mongo/driver/connstring/connstring.go`, `mongo/options/clientoptions.go`), aws-sdk-go-v2.
  The Postgres and MongoDB readers in `ca_trust.go` mirror those parsers on purpose; judge them
  against the library source, not against memory.

Answer these questions, each with evidence (`path:line`, library `path:line`, or a command you
ran and its output). A claim with no evidence is not a finding.

1. Can any sink (S3, GCS, Postgres, MongoDB) connect to a server whose certificate is signed
   neither by the configured CA nor by a public (system) root while a CA is set? Try concrete
   inputs: DSN and URI spellings, repeated keys, escapes, quoting, bracketed hosts, aliases,
   multi-host DSNs and their fallbacks, `mongodb+srv`.
2. Can verification be silently weakened while a CA is set: Postgres `sslmode`
   (DSN, `ssl=true`, service file, `PGSSLMODE`, `PGSERVICE`, `PGSERVICEFILE`, `sslnegotiation`,
   `PGSSLNEGOTIATION`), MongoDB URI options (`tlsInsecure`, `tlsAllowInvalid*`, `tls=false`,
   `tlsCAFile` and aliases), environment (`AWS_ENDPOINT_URL*`, `SSL_CERT_*`), redirects, proxies
   (`HTTPS_PROXY` and the transport `Proxy` field), hostname checks? Pay special attention to
   `caDSN` in `internal/sink/postgres/ca_trust.go`: the handling of a DSN `service=` key that is
   present but empty (it must win over `PGSERVICE`, as in pgx `ParseConfigWithOptions`), and any
   input where the reader and pgx disagree on the effective sslmode.
3. Can any error this branch adds (or changes) leak a DSN, URI, host, user or credential into an
   error string, a status condition or a log line?
4. Do the operator (export and connection probe) and the CLI (`kollect-pipeline collect`) behave
   identically for the same spec: same CA bytes, same errors (`errors.Is` on the shared
   sentinels), same terminal classification? Look for any path where one checks something the
   other does not, including `internal/sink/git/tls.go` `ValidateTLSSpec` and
   `internal/controller/secret_ca.go`.
5. Are the docs and the release note true against the code? Quote any sentence the code
   contradicts.

Rank each finding CRITICAL (a CA-configured sink can trust an unintended server, verification is
weakened silently, or a secret leaks), WARNING (wrong behaviour or a false doc claim with lesser
impact), NOTE. For each give: location, the concrete input or scenario, why it is wrong
(evidence), and the smallest fix. End with one line `VERDICT: BLOCK | REQUEST_CHANGES | APPROVE`.
You may run `go test` and `go build` (read-only sandbox: if writes fail, say so and reason from
the source instead).
