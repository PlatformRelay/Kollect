# Cold review brief: spec set of OpenSpec change `sink-ca-trust` (kollect)

You are an independent reviewer with read-only access to this repository (cwd is the repo root,
branch `feat/sink-ca-trust`). You have no network. Read the files; do not guess. Change nothing.
Go module sources are in the module cache (`go env GOMODCACHE`, typically `~/go/pkg/mod`):
`github.com/jackc/pgx/v5@v5.11.0`, `go.mongodb.org/mongo-driver@v1.17.10`,
`github.com/go-git/go-git/v5@v5.19.3`, `github.com/aws/aws-sdk-go-v2/config@v1.33.6`.

## Context

kollect is a Go Kubernetes operator that exports inventories to sinks. A sink's
`spec.tls.caBundle` / `spec.tls.caSecretRef` is resolved for every sink type but ignored by the S3,
GCS, Postgres and MongoDB clients. This change makes those four apply it. The operator ruled:

- append the CA to the system roots (consistent with Git/GitLab, which pass it to go-git as
  `CABundle`);
- for Postgres and MongoDB, the sink CA wins, and a DSN/URI that also sets a CA file
  (`sslrootcert`, `tlsCAFile`, ...) is rejected at validation with a clear message;
- the DSN's sslmode and verify settings still apply;
- verification is never turned off.

Strict pinning (trust only the CA) is a non-goal. The behaviour reaches users through the family
sink CRDs, the connection probe (`KollectConnectionTest`) and the CLI `kollect-pipeline collect`.
Repo conventions: red-first TDD (a test task records a red failing on the target assertion; test
and implementation land in one commit); real TLS tests must be `//go:build integration` because
the netguard dialer refuses loopback otherwise; evidence names the test that ran (`=== RUN`).

## Target question

Would executing `tasks.md` as written deliver the spec (`specs/sink-tls-trust/spec.md`), and is
the spec itself right for the operator's ruling?

Look specifically for:

1. Ways the client could still connect without verifying against the CA, or with verification off,
   for each of the four types: every code path that builds a client (export, probe, CLI), pgx
   sslmode handling (`prefer` fallbacks, `require`, multi-host fallbacks, `sslnegotiation=direct`,
   env defaults `PGSSLMODE`/`PGSSLROOTCERT`), MongoDB URI options (`tls`, `ssl`, `tlsInsecure`,
   `mongodb+srv` defaults), S3 endpoint schemes and the AWS SDK's own HTTP client/config loading.
2. Whether the Postgres rule set (reject unset/`disable`/`allow`/`prefer`, accept `require` as a
   chain check, reject `sslrootcert`) matches the ruling "DSN sslmode and verify settings still
   apply", and whether it can be implemented on pgx v5.11.0 as `design.md` claims
   (`pgconn/config.go` `configTLS`, how `ParseConfig` builds `Fallbacks`).
3. Whether the shared-resolver changes (missing CA key → error, both sources → error, for every
   sink type) are in scope, correctly scoped, and correctly release-noted; Kafka/NATS/Git impact.
4. Error redaction: Postgres errors are deliberately static (`internal/sink/postgres/pool.go`);
   would any new error leak the DSN, URI, host or credentials?
5. Whether each test case can actually fail for the reason it names, and whether the planned reds
   are real (not compile errors or skips); the `SSL_CERT_FILE` subprocess approach for "still
   trusts public CAs"; Docker-gated tests and what happens without Docker.
6. SSRF guard: does any planned client construction bypass `internal/sink/netguard` (dialer,
   `Proxy = nil`)?
7. Integration tagging and `INTEGRATION_PACKAGES` in `Taskfile.yml`; the integration evidence tool
   (`hack/tools/integrationevidence`) and whether new tests are counted.
8. Missing or misordered tasks, docs that will be wrong after the change, and anything needing an
   operator decision.
9. Stale `file:line` claims: re-check every `path:line` cited in proposal.md, design.md, spec.md
   and tasks.md against the current tree and report each that is wrong.

## Files

- `openspec/changes/sink-ca-trust/proposal.md`, `design.md`, `specs/sink-tls-trust/spec.md`,
  `tasks.md`; `loop.md` (context only)
- `internal/sink/registry.go`, `build_context.go`, `probe.go`, `backend_pool.go`,
  `internal/sink/sinktls/`, `internal/sink/netguard/`, `internal/sink/s3/`, `internal/sink/gcs/`,
  `internal/sink/postgres/`, `internal/sink/mongodb/`, `internal/sink/git/tls.go`,
  `internal/sink/git/connection.go`, `internal/sink/git/config.go`
- `internal/pipeline/wire.go`, `internal/pipeline/sink_credentials_test.go`, `cmd/cli/`
- `internal/controller/secret_ca.go`, `internal/controller/kollectconnectiontest_controller.go`,
  `internal/controller/family_sink_connection.go`
- `api/v1alpha1/kollectsink_types.go` (`TLSSpec`), `internal/validation/`
- `internal/integrationtest/`, `Taskfile.yml`, existing `*_integration_test.go` in the sink packages
- `docs/REQUIREMENTS.md`, `docs/security/security-architecture.md`,
  `docs/operator-manual/upgrading.md`, `docs/crds/kollectsnapshotsink.md`,
  `docs/crds/kollectdatabasesink.md`, `docs/guides/pipeline-cli.md`
- `openspec/config.yaml`, `docs/development/spec-workflow.md`

## Output format

Markdown. Start with `## Verdict` and one of APPROVE / CONCERNS / BLOCK and one sentence.
Then `## Findings`, each as:

```text
### <ID> <CRITICAL|WARNING|NOTE>: <one-line title>
- Where: <file:line> (and any other file:line)
- Evidence: what the file says (quote briefly)
- Why it matters: how executing tasks.md as written would miss the spec, or fail open
- Fix: the exact edit to the spec set
```

CRITICAL = executing the tasks as written would ship a client that can connect unverified or with
an ignored CA while the spec says otherwise, leak a credential, or a task that cannot be executed.
WARNING = a real gap or stale claim that will cost a task round. NOTE = minor. Finish with
`## Stale file:line claims` as a table: claim, cited, actual.
