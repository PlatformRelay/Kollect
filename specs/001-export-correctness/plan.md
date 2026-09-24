# Implementation Plan: Export correctness (B1)

**Branch**: `fm/kollect-b1-export` · **Spec**: `specs/001-export-correctness/spec.md`

## Summary

Close K-01, K-02, K-03 (and record the K-31 register gap) by:

1. Raising a terminal `ErrSpillRequired` at the sink boundary instead of the
   silent `return nil` when a non-object-store backend cannot take an oversize
   payload, and mapping it to `SpillRequired` in both inventory controllers.
2. Restricting multipart partitioning to snapshot-family bindings so relational
   and event sinks receive one complete payload.
3. Rewriting the "spill" documentation as an inline cap with a loud skip.

## Technical Context

- **Language**: Go (module `github.com/platformrelay/kollect`).
- **Key packages**: `internal/sink`, `internal/sink/cap`, `internal/export`,
  `internal/controller`, `api/v1alpha1`.
- **Test frameworks**: standard `go test`; envtest for controller integration;
  testify where already used.
- **Gates**: `make test` / `go test ./...`, `golangci-lint` (`.golangci.yaml`),
  coverage floor `hack/coverage.sh` (90% aggregate), go-arch-lint.

## Files Touched

| File | Change |
| --- | --- |
| `internal/sink/export.go` | Add `ErrSpillRequired`; return terminal error from the `!shouldExportForSpill` branch. |
| `internal/sink/export_test.go` | Unit test: non-object-store oversize → `ErrSpillRequired`, terminal; object-store → success. |
| `internal/controller/kollectinventory_controller.go` | Partition only snapshot bindings; map spill error to `SpillRequired` per-sink and aggregate. |
| `internal/controller/kollectclusterinventory_controller.go` | Same partition decision and error mapping. |
| `internal/controller/per_sink_export.go` | Shared reason helper for per-sink failures. |
| `internal/controller/*_test.go` | End-to-end size-band test (git-only, git+postgres, postgres-only). |
| `docs/REQUIREMENTS.md`, `docs/adr/0103-etcd-limit.md` | Inline-cap wording; remove spill-mechanism claims. |
| `specs/001-export-correctness/*` | Spec/plan/tasks record. |

## Design Decisions

### D1 — Sentinel error, not a string match

`ErrSpillRequired` is a package-level sentinel in `internal/sink`, wrapped in
`kollecterrors.Terminal`. Controllers detect it with `errors.Is`, so the mapping
survives error wrapping and aggregation. This mirrors how other typed errors are
classified in this codebase (`kollecterrors.ClassOf`, `IsTerminal`).

**Alternative rejected**: re-using the existing pre-export gate only. The gate is
skipped whenever a snapshot sink is bound (`!hasSnapshotSinkBinding`), which is
exactly the reproduced black hole. The gate stays as a cheap early-out, but the
sink boundary is now authoritative.

### D2 — Partition by family, not by sink type

The existing code already treats multipart as a snapshot-family concern:
`exportToSinks` only uses `PartitionsChecksum` for `SinkFamilySnapshot`. Git
multipart (part suffixes + union prune) is a shipped feature, so the correct
boundary is the family, not `IsObjectStoreSinkType` (s3/gcs only). Database and
event families pass `maxBytes = 0` to `PartitionEnvelopes`, producing one
markerless complete part.

**Consequence**: a database/event binding whose complete payload exceeds the
1 MiB inline cap now degrades with `SpillRequired` rather than being torn into
parts. This is intended and consistent with K-01 — there is no spill path for
these families.

### D3 — Reason mapping

Per-sink failures get reason `SpillRequired` when the error is `ErrSpillRequired`,
else the existing `ExportFailed`/`ExportTerminal` classification. The aggregate
total-failure path maps to `SpillRequired` so the inventory condition reads
`Degraded`/`SpillRequired`.

### D4 — K-31 register gap

The unify report's register (§4) contains no K-31 entry; the ID appears only in
the batch table (§6). Register IDs K-27, K-31, K-43 are all absent from §4 while
the report claims 82 findings (79 present + 3 gaps = 82), so K-31 was dropped in
editing. The batch's export fixes cover both the namespaced and cluster export
call sites, which is the plausible subject matter for a fourth export finding.
This is recorded for the captain; it is not guessed at in code.

## Constitution / Constraints Check

- No API/CRD change; no new dependency.
- Branch stays local; no push to main.
- No agent attribution in commits/PR.
- K-01/K-02/K-03 only; no edits to other batches' findings.

## Verification Strategy

1. Unit: `internal/sink` — spill error raised/classified; object-store exempt.
2. Unit: `internal/export` — partition unchanged for snapshot; single part for
   `maxBytes = 0`.
3. End-to-end controller test (envtest, existing harness):
   - git-only 1.2 MiB → `Degraded`/`SpillRequired`, sink not `Exported`.
   - git+postgres 1.2 MiB → git `SpillRequired`, inventory not `Ready=True Exported`.
   - postgres-only 500 KiB ceiling + ~700 KiB snapshot → all rows delivered.
4. `go test ./...`, `golangci-lint run`, coverage floor.
5. Adversarial self-review, then `close-task` per finding.

## Risks

- **Behaviour change**: database/event exports above the inline cap now fail
  instead of silently dropping. This is the point of K-01, but it is
  operator-visible; documented in the ADR/REQUIREMENTS rewrite.
- **Test harness**: the repo may lack envtest assets locally. If the end-to-end
  controller test cannot run locally, it must still be written to run in CI; note
  the local gap honestly in the close-task evidence.
