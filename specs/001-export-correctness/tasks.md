# Tasks: Export correctness (B1)

**Input**: `specs/001-export-correctness/spec.md`, `plan.md`

## Phase 1 — K-01 loud spill failure

- [x] T001 Add `ErrSpillRequired` sentinel to `internal/sink/export.go`.
- [x] T002 Return `kollecterrors.Terminal(ErrSpillRequired)` from the
      `!shouldExportForSpill` branch instead of `return nil`.
- [x] T003 Add `sinkExportFailureReason` to `internal/controller/per_sink_export.go`.
- [x] T004 Map spill error to `SpillRequired` in `applyInventoryExportOutcome`
      (namespaced) and the cluster analogue; per-sink status uses the helper.
- [x] T005 Update `TestRunExportEnvelope_oversizedNonObjectStoreFailsLoudly`
      (was `..._skipsOversizedNonObjectStore`, which pinned the bug).
- [x] T006 Add `TestRunExportEnvelope_oversizedObjectStoreExports` (object-store exempt).

## Phase 2 — K-02 family-scoped multipart

- [x] T007 Namespaced `exportToSinks`: `ceiling = 0` for non-snapshot bindings.
- [x] T008 Cluster `exportClusterToSinks`: same.
- [x] T009 Verify existing git multipart tests still pass (torn-set, partitioning).

## Phase 3 — K-03 docs truth

- [x] T010 `docs/REQUIREMENTS.md` NFR-PERF-5 + §6 resolved question.
- [x] T011 `docs/adr/0103-etcd-limit.md` item 7, diagram, consequences.
- [x] T012 `docs/adr/0405-export-data-contract.md` bounded-size clause.
- [x] T013 `docs/crds/index.md` layout/inline-cap section.
- [x] T014 `docs/ANNOTATIONS-LABELS.md`, `docs/operator-manual/troubleshooting.md`,
      `docs/operator-manual/load-test-runbook.md` wording.

## Phase 4 — Test lock (end-to-end size band)

- [x] T015 `TestExportSizeBand_gitOnlyOversizeDegrades` — 1.2 MiB git-only →
      `Degraded`/`SpillRequired`, no sink write.
- [x] T016 `TestExportSizeBand_gitPostgresMixedNotSilentlyGreen` — mixed binding,
      no sink green.
- [x] T017 `TestExportSizeBand_postgresCeilingKeepsAllRows` — postgres 500 KiB
      ceiling, ~700 KiB snapshot → one complete payload, all rows survive.

## Phase 5 — K-31 register gap

- [x] T018 Record that the register contains no K-31 entry (ID only in the batch
      table; K-27/K-31/K-43 absent while the report claims 82 findings). The
      export-surface fix covers both call sites; the gap is reported, not guessed.

## Phase 6 — Gates and review

- [x] T019 `go test ./internal/sink/ ./internal/export/ ./internal/controller/` green.
- [x] T020 `bin/golangci-lint run` on changed packages: 0 issues.
- [x] T021 Adversarial self-review of the diff; fix findings.
- [x] T022 close-task each finding with evidence.
- [x] T023 Push `fm/kollect-b1-export`, open ready PR via `gh-axi`.
- [x] T024 (review M1) `ExportCeilingExceeded` Warning event for soft per-binding
      ceiling on database/event sinks + docs.
- [x] T025 (review M2) `ExportErrorReason` returns `spill_required` for
      `ErrSpillRequired`; unit-tested.
