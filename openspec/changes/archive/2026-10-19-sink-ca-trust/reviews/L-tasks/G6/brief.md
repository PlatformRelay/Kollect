# Review brief — sink-ca-trust G6 (task 6.1: documentation)

You are reviewing one task, independently of whoever wrote it. Read-only: do not edit, commit or
push. Repository: the current directory (branch `feat/sink-ca-trust`).

Inputs:

1. TASK — `openspec/changes/sink-ca-trust/tasks.md`, section *6. Documentation* (6.1). Spec:
   `openspec/changes/sink-ca-trust/specs/sink-tls-trust/spec.md` requirement SCT-6 (and SCT-1 to
   SCT-5 for the rules the docs state). Design: `openspec/changes/sink-ca-trust/design.md`
   (*Compatibility* table, *Postgres*, *MongoDB*, *S3 and GCS*). The release-note line must cover:
   append semantics; Postgres unset → `verify-full` plus the hostname check, with explicit
   `verify-ca` as the escape hatch; explicit-weak sslmode rejected; DSN/URI CA-file conflicts
   rejected; the missing-CA-key and both-sources errors for every sink type; "touch the sink after
   fixing a Secret".
2. MATRIX and SENSORS — `openspec/changes/sink-ca-trust/evidence/G6.md`.
3. DIFF — staged changes against `9584aa3c3`: `git diff --cached 9584aa3c3`. Docs, the `TLSSpec`
   field comments in `api/v1alpha1/kollectsink_types.go`, regenerated CRDs and schema goldens,
   and `openspec/changes/sink-ca-trust/pr-description.md` (release note).
4. RULES — `AGENTS.md` (repo root, *When adding a CRD field* for regeneration) and
   `docs/development/spec-workflow.md`.

Judge, in order:

a. INTENT — does the diff cover every bullet of 6.1 and of the release-note list above?
b. TRUTH — check each claim against the code, not against the spec: `internal/sink/registry.go`,
   `internal/sink/probe.go`, `internal/sink/ca_source.go`, `internal/sink/sinktls/sinktls.go`,
   `internal/sink/git/tls.go`, `internal/sink/s3/{config,backend}.go`,
   `internal/sink/postgres/ca_trust.go`, `internal/sink/mongodb/ca_trust.go`,
   `internal/pipeline/wire.go`, `internal/controller/kollectinventory_controller.go` (sink
   watches), `internal/controller/family_sink_connection.go` (test-connection annotation). Flag
   any statement that is false, overclaims (for example append semantics for Kafka, NATS or the
   Git probe), or omits a rule a user needs to avoid a broken sink after upgrade. Check that the
   quoted error strings match the code exactly and that the "touch the sink" advice really leads to
   a rebuilt backend.
c. GENERATED — CRDs and goldens match the comment change only.
d. Matrix — is any row's class or result dishonest?

RETURN at most 15 lines, nothing else, starting with a `## Verdict` heading:

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why>
could_not_check:
  - <what you did not verify>
summary: <one line>
```

P0 (a doc that would lead users into an insecure or broken configuration, a vacuous gate
presented as proof) → BLOCK. P1 (a false or missing claim the task requires) → REQUEST_CHANGES.
P2 → note only.
