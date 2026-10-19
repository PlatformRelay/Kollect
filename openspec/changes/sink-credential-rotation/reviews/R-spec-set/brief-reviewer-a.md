# Cold review brief: spec set of OpenSpec change `sink-credential-rotation` (kollect)

You are an independent reviewer with read-only access to this repository (cwd is the repo root,
branch `feat/sink-credential-rotation`). You have no network. Read the files; do not guess. Change
nothing. Do not name yourself, your model or its vendor in the report. Never copy a secret value.
Go module sources are in the module cache (`go env GOMODCACHE`, typically `~/go/pkg/mod`):
`github.com/jackc/pgx/v5@v5.11.0`, `go.mongodb.org/mongo-driver@v1.17.10`,
`github.com/segmentio/kafka-go@v0.4.51`, `github.com/nats-io/nats.go@v1.54.0`,
`sigs.k8s.io/controller-runtime` (version in `go.mod`).

## Context

kollect is a Go Kubernetes operator that exports inventories to sinks. It caches one built backend
per sink in a process-wide pool (`internal/sink/backend_pool.go`) keyed on the sink spec only, so
credential and CA Secrets are read once at build: a rotated Secret is ignored until a spec change,
sink delete, restart or 48 h idle, and a deleted or emptied Secret does not stop exports. A
previous change added evict-on-delete (BEP-1, in the unarchived change
`openspec/changes/2026-10-06-product-decision-convergence/specs/backend-pool/spec.md`). The merged
`openspec/specs/sink-tls-trust/spec.md` made the CA Secret (`tls.caSecretRef`) reach every sink, so
CA rotation is in scope too. The CLI `kollect-pipeline` reads Secrets from a config directory per
run (`internal/pipeline/wire.go`). No RBAC change is wanted: the manager already has
`get/list/watch` on Secrets.

The change under review is `openspec/changes/sink-credential-rotation/`: `proposal.md`,
`design.md`, `specs/sink-secret-rotation/spec.md`, `tasks.md`. Repo rules: `AGENTS.md`,
`docs/development/spec-workflow.md`, `openspec/config.yaml` (red-first TDD; a test task records a
red that fails on the target assertion; test and implementation land in one commit; evidence
names the test that ran).

## Lens: spec

For each requirement (SCR-1 to SCR-6): does the design deliver it, and would executing `tasks.md`
as written prove it? Verify against the code, never against a document claiming it.

Look specifically for:

1. Ways a rotated, deleted or emptied Secret could still be used after the change: every path
   that acquires or builds a backend (export, cleanup, connection probes, CLI, the
   pool-disabled path), every Secret reference field in `api/v1alpha1` that the build reads
   (compare `internal/sink/build_context.go` with design's list), and the legacy `KollectSink`.
2. Whether keying on UID + `resourceVersion` read through the manager's cached client is sound:
   cache staleness, a `watchNamespaces`-restricted cache with an allowlisted cross-namespace
   `secretRef`, what `Get` through a cached client costs and copies, and whether a per-acquire
   read changes behaviour for any sink.
3. The holder-count drain design: deadlocks, double close, leaks, the tombstone path, TTL prune,
   the race between two builders, and whether existing callers (`internal/sink/export.go`,
   `internal/sink/cleanup.go`) always release. Whether BEP-1 still holds.
4. The new "empty Secret" error: which sinks or users legitimately reference a Secret that could be
   empty (or rely on `stringData` only), the error class (transient vs terminal) and its effect on
   requeue in `internal/controller/kollectinventory_controller.go`, and the S3 ambient-credential
   fall-through claim (`internal/sink/s3/config.go`, `backend.go`).
5. Redaction: could any new error, log line, Event or status text carry a Secret value?
6. Whether each test case can fail for the reason it names, whether the planned reds are real on
   the base (not compile errors or skips), the envtest approach (the suite disables the pool), the
   Postgres integration test's assertions (`pg_stat_activity`, `NOLOGIN` on a role with open
   sessions), and Docker-gated behaviour.
7. Missing or misordered tasks, docs that will be wrong after the change, and anything needing an
   operator decision (for example: should a Secret watch trigger exports or recovery; should an
   empty Secret be terminal).
8. Stale `file:line` claims: re-check every `path:line` cited in the four files against the
   current tree and report each that is wrong.

## Output, and nothing else (at most 60 lines, plain markdown)

## Verdict: BLOCK | CONCERNS | CLEAN
## Findings
- [CRITICAL|WARNING|NOTE] <one-line claim> — `path:line`
  Failure: <concrete input/state → wrong outcome>
  Fix: <smallest change that removes it>
  Confidence: <0-100>
## Could not check
- <what you did not read or run>
