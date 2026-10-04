# Tasks

Blocked until the operator picks an option in ADR-0422. The tasks below implement option A.

## 1. Behavioural tests first (IEI-1..IEI-10)

- [ ] 1.1 Add `TestRunExportEnvelope_ClusterKindAndClusterNamespaceIsolated` next to the identity-less regression `TestRunExportEnvelope_SharedControllerIdentityPreservesBothTrees` (`internal/sink/layout_export_multipart_prune_test.go:267`), which stays as IEI-9 evidence: disjoint trees of `KollectClusterInventory platform` and `KollectInventory cluster/platform`, each request carrying its identity, through the real layout and a local Git remote; an empty cluster snapshot removes only the cluster inventory's files. Watch it fail on "cluster inventory file still present" (today: suppressed prune)
- [ ] 1.2 Add owner tests: distinct owners for the two kinds (IEI-1); golden owner strings and record paths for namespaces other than `cluster` (IEI-4); record decodes with the strict reader (IEI-10)
- [ ] 1.3 Add migration tests with a pre-planted legacy record: ambiguous record retired, unlisted files kept (IEI-5); claims ignored for the same name, enforced for other names; no-record repository adopts only current paths
- [ ] 1.4 Add transition tests: A→B→A for the cluster kind (IEI-6); recreate with the same identity; kind swap does not inherit (IEI-7); interrupted and complete multipart sets over a legacy record (IEI-8)
- [ ] 1.5 Add controller tests that both reconcilers set the identity on `ExportEnvelopeRequest` (IEI-1); keep the identity-less suppression test (IEI-9)
- [ ] 1.6 Extend `FuzzOwnedPrune` (PR #412, `internal/sink/git/owned_prune_fuzz_test.go`): owner class L carries `KollectInventory cluster/platform` and loses its `legacy` flag; a fifth class K is `KollectClusterInventory platform` with the same object path; a migration opcode plants a legacy record (see Verification). Update the model and seeds; watch the K seeds fail before the fix

## 2. Implementation (option A)

- [ ] 2.1 Add the identity to `ExportEnvelopeRequest` (`internal/sink/export.go:72-86`) and pass it to `resolveSnapshotExport` (`layout_export.go:109`)
- [ ] 2.2 Set it in `kollectinventory_controller.go:404-415` and `kollectclusterinventory_controller.go:320-331`
- [ ] 2.3 Owner encoding per design D2 in `layout_export.go:199-207`; keep the `SuppressPrune` guard (`:209-217`) only for identity-less requests
- [ ] 2.4 Release owners per design D4: forward through `ExportFilesOptions` (`git/export.go:83-86`) in both backends (`git/backend.go:73-75`, `gitlab/backend.go:109-111`); honour in `validateOwnedPrunePaths` and `prepareOwnedPrune`/`apply` (`git/prune_owned.go:168-246`)
- [ ] 2.5 Defect controls (mutations, judged by exit status, plus a no-op control): owner without kind; release owner adopted instead of retired; legacy paths added to the deletion set; release on a non-final part; suppression kept for identity requests; namespaced owner moved to the new form

## 3. Docs and review

- [ ] 3.1 ADR-0419 "Exact file ownership" paragraph and ADR-0421 "Shared export identity" cross-reference updated; ADR-0422 moves to Current with the chosen option
- [ ] 3.2 Release-note text (by hand, git-cliff uses titles only): cluster inventories prune again; shared-path rejection; manual cleanup of files left by the retired legacy record
- [ ] 3.3 CI green on the PR head
- [ ] 3.4 Independent adversarial review
- [ ] 3.5 Archive the change as the last commit of the PR

## Verification

Planned checks. Every row stays `not-run` until it has executed with evidence (revision, command,
`=== RUN` lines, tool versions).

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| IEI-1 | `TestPruneOwner_clusterKindDiffersFromClusterNamespace`; controller tests `TestKollectInventory_exportCarriesIdentity`, `TestKollectClusterInventory_exportCarriesIdentity` | owners and record paths differ; neither equals the legacy triple; both requests carry kind/namespace/name | not-run | — |
| IEI-2 | `TestRunExportEnvelope_ClusterKindAndClusterNamespaceIsolated` (task 1.1) with Git and with GitLab; `FuzzOwnedPrune` with classes K and L | no export of one deletes a file recorded for the other; shared path rejected with no repo change | not-run | — |
| IEI-2 | mutation: owner built without kind | `TestRunExportEnvelope_ClusterKindAndClusterNamespaceIsolated` and a K/L fuzz seed fail | not-run | — |
| IEI-3 | `TestOwnedPrune_clusterInventoryPrunesOwnFiles` (shrink, empty snapshot) through `RunExportEnvelope` to a local Git remote | deleted exactly previous-minus-current; empty snapshot clears the record | not-run | — |
| IEI-3 | mutation: keep `SuppressPrune` for identity requests | the test above fails | not-run | — |
| IEI-4 | `TestPruneOwner_otherNamespacesUnchanged` (golden strings and SHA-256 record paths); existing owned-prune suite unchanged | byte-identical; no second record written | not-run | — |
| IEI-4 | mutation: namespaced owners moved to the new form | golden test fails | not-run | — |
| IEI-5 | `TestOwnedPrune_ambiguousLegacyRecordRetired`, `_legacyClaimSameNameAccepted`, `_legacyClaimOtherNameRejected`, `_noRecordAdoptsCurrentOnly` | legacy record removed in the same commit; unlisted files kept and never deleted later; other names rejected | not-run | — |
| IEI-5 | `FuzzOwnedPrune` migration opcode (opcode 5: plant a legacy record for an owner's legacy triple over unclaimed paths, writing those files as an earlier build would); model: K/L never delete planted paths, retire the record on their next complete export, A/B/C adopt their own triple's record | no FORBIDDEN failure over the seed corpus and the fuzz run | not-run | — |
| IEI-5 | mutations: release owner adopted into the new record; legacy paths added to the deletion set | migration tests and a migration fuzz seed fail | not-run | — |
| IEI-6 | `FuzzOwnedPrune` replay opcode seeds for K and L; `TestExportFingerprint` A→B→A for the cluster kind | second A restores the files | not-run | — |
| IEI-7 | `TestOwnedPrune_recreateSameIdentityContinuesRecord`, `TestOwnedPrune_kindSwapDoesNotInherit` | only the predecessor's recorded stale paths deleted; other kind deletes nothing of the old record | not-run | — |
| IEI-8 | `TestOwnedPrune_multipartReleaseOnFinalPartOnly`; fuzz multipart and retry-final opcodes with K over a planted record | interrupted set changes no record and deletes nothing; final part prunes the union once | not-run | — |
| IEI-8 | mutation: release on a non-final part | the multipart test fails | not-run | — |
| IEI-9 | existing `TestRunExportEnvelope_SharedControllerIdentityPreservesBothTrees` (`layout_export_multipart_prune_test.go:267`) and `TestResolveSnapshotExportSharedControllerIdentitySuppressesPrune` (`layout_export_test.go:404`), unchanged | identity-less requests on `inventory/cluster/platform.json` delete nothing | not-run | — |
| IEI-10 | `TestPruneRecord_formatUnchanged` | strict decode passes; record path is SHA-256 of the owner | not-run | — |
| all | no-op mutation control | survives (all tests pass) | not-run | — |
| all | `go test -race -count=N ./internal/sink/... ./internal/controller/...` (N recorded; `internal/sink` has the known SINK-BREAKER-RACE-01 race) | pass, or only the known pre-existing race | not-run | — |
| all | `go test -run=^$ -fuzz=FuzzOwnedPrune -fuzztime=<T> ./internal/sink/git/` (T recorded) and the CI fuzz matrix entry | no failure; any found input kept as a seed | not-run | — |
| all | pinned golangci-lint; `task spec:validate`; `task lint:markdown` | pass | not-run | — |
| all | CI on the PR head | required checks green | not-run | — |
| — | independent adversarial review | APPROVE | not-run | — |
