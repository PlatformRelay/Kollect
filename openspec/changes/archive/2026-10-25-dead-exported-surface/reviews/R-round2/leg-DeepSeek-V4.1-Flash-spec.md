## Verdict: CONCERNS

Spec-lens, round 2. Requirements re-verified against source at `92360d72` (not against the prose): every zero-caller claim holds except where noted — `RunExportItems`/`ExportItemsRequest` (def+tests+comment), `MergeRequestAPI` (def only), both conditions (def only), `RemoveCluster`/`MarshalTargetJSON` (test-only; `MarshalTargetJSON` a one-line wrapper, `store.go:250-252`), `git.Export`/`ExportMemory` (test-only), four cap aliases (test-only), `BindClusterTargetNamespaces` (test-only, 14 sites/8 files), `_ = user` (`auth.go:122`). The plan defect is an unlisted orphan plus one false coverage assumption.

## Findings
- [WARNING] Deleting `RunExportItems` orphans `sinkNamespaceForExport`, so `task lint` cannot go green — `internal/sink/export.go:302` (sole caller `:136`)
  Failure: task 1.2 removes the only caller; the unexported wrapper is then unreferenced. `unused` is explicitly enabled (`.golangci.yaml:32`), `make lint` fails, and `task lint` is a stated final gate (tasks.md:55) — the change ships red.
  Fix: extend task 1.2 to delete `sinkNamespaceForExport` (`export.go:302-304`); `SinkNamespaceForResolved` (`resolver.go:104`) stays, still used by controllers.
  Confidence: 85
- [WARNING] The assumption "every other deleted test exercises only the unreachable `RunExportItems` path" is contradicted by delegation — `internal/sink/export_test.go:120`, `:169`, `:212`, `:263`, `:306`
  Failure: `RunExportItems` delegates to the live `RunExportEnvelope` (`export.go:132`), so these tests assert live behaviour — YAML projection (`export_test.go:159-166`), capability skip (`:207-209`), backend pooling (`:306`), close-error tolerance (`:263`). Task 1.3 migrates only the two breaker tests, so live-path coverage is dropped while the live function stays; the 90% floor need not notice.
  Fix: extend task 1.3 to migrate the live-path assertions of the delegation tests to `RunExportEnvelope`, or record the file:line of an equivalent existing test for each.
  Confidence: 70
- [NOTE] Task 8.3's "per-symbol greps return no hits outside review records" is unachievable literally — `CHANGELOG.md`, `openspec/changes/archive/2026-10-05-inventory-export-identity/*`
  Failure: `RunExportItems`/`ExportItemsRequest` survive in CHANGELOG and the archived change; a broad grep fails the gate for purely historical text.
  Fix: scope 8.3 to `--include='*.go'` (as task 1.1 already does) and state CHANGELOG/archive are historical and not edited.
  Confidence: 90
- [NOTE] Removing exported `ConditionConnected`/`ConditionCredentialsVerified` is a breaking API change to module-public `api/v1alpha1` with no recorded release note — `api/v1alpha1/constants.go:10-11`
  Failure: external importers break on upgrade; the proposal discloses it as an "accepted limit" (proposal:95-99) but no task records it in CHANGELOG or an API-compat note.
  Fix: add a CHANGELOG "removed" entry to task 8.2's deliverables.
  Confidence: 80
- [NOTE] "6 production references" to `RunExportEnvelope` (proposal:92) overstates the call sites — `cleanup.go:364`, `export.go:132`, `kollectinventory_controller.go:404`, `kollectclusterinventory_controller.go:331`
  Failure: only 4 non-test call sites exist; the number is a load-bearing part of the "production dispatches through `RunExportEnvelope`" argument and is unverified as written.
  Fix: correct to 4 (or define what the sixth reference is).
  Confidence: 75

## Could not check
- External consumers of `api/v1alpha1` — outside the repo; the compile cannot prove the external half (as the proposal concedes).
- Integration-tagged suites (`//go:build integration`) were read, not executed; `go vet -tags integration` and `task test` not run.
- `task coverage` not run, so the "floor 90 holds on the final tree" claim is unverified here.
- Other review legs (`reviews/R*`) deliberately not read, per the out-of-bounds rule.
