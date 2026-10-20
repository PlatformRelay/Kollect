# Review brief — task 4.1 (docs, SCR-6), sink-credential-rotation

You are reviewing one task independently of whoever did it. Read-only: do not edit files. Repo root
is the current directory (branch `feat/sink-credential-rotation`, base commit `b6fca7d1e`).

Inputs:

1. TASK — `openspec/changes/sink-credential-rotation/tasks.md`, item 4.1 (section "## 4. Docs"),
   and requirement SCR-6 in `openspec/changes/sink-credential-rotation/specs/sink-secret-rotation/spec.md`.
   The behaviour the docs describe is SCR-1 to SCR-7 in the same spec and `design.md`
   (*Compatibility*, *Zero export interval*, *Cleanup before acquire*).
2. MATRIX — `openspec/changes/sink-credential-rotation/evidence/G4.md`.
3. DIFF — the staged, uncommitted change: `git diff --cached b6fca7d1e` (six doc pages plus the
   evidence file and this brief). Docs only; no Go code.
4. SENSORS — recorded in the evidence's *Sensors run* table (`rg -U 'keeps its\s+CA' docs`,
   `task lint:markdown`, the docs guards, `task docs:verify`). Trust their exit codes as reported.
5. RULES — every doc claim must hold for the code on the branch at `b6fca7d1e`; no secret values;
   evidence is Markdown only.

Judge, in order:

a. INTENT — does the diff cover every clause of 4.1: the rotation paragraph on the snapshot,
   database and event sink pages; the stale "keeps its CA" sentence removed everywhere; the
   security-architecture sentence; the troubleshooting rows for a deleted or emptied Secret
   (recovery on the next retry, within 30 s for `0s`), the `0s` "export only on material change"
   sentence, the Git auth `terminal` row keeping its re-annotate step; the upgrade section
   "Sink Secret rotation (after v0.21.0)" with the design compatibility table, "entirely empty"
   for the S3/GCS fallback, the Retain and `force-cleanup` note?
b. ACCURACY — check each doc claim against the code, not against the spec's prose: the referenced
   Secret fields per sink family (`internal/sink/secret_refs.go` `referencedSecretRefs`), the
   error texts (`internal/sink/credentials.go`, `secret_refs.go`), that an entirely empty CA Secret
   is retryable while one with data but no CA key stays terminal, the `0s` watchdog and terminal
   no-requeue (`internal/controller/kollectinventory_controller.go`,
   `kollectclusterinventory_controller.go`, `internal/validation/export_interval.go`), which
   cleanups read the Secret (`internal/sink/cleanup.go` `RunCleanupExport`), the Degraded reason
   on a total export failure, the CLI parity (`internal/pipeline/wire.go`), "no RBAC change"
   (`config/rbac/role.yaml`, `git diff origin/main..b6fca7d1e -- config charts`). Flag any
   sentence that claims more than the code does, or contradicts another page.
c. MATRIX — is any evidence class over-stated?
d. Failure modes — dropped clauses, broken anchors, prose that promises behaviour the code lacks.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.

RETURN ≤15 lines, nothing else, under a `## Verdict` heading:

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you could not verify>
summary: <one line>
```

P0 (security, a doc that would lead an operator to leak or misuse a credential) → BLOCK. P1 (a
false or missing claim the task requires) → REQUEST_CHANGES. P2 → note only, APPROVE allowed.
