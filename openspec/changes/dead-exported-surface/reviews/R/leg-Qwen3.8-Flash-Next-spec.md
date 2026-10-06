Now verifying each claim against the code with the same probes the tasks prescribe.
Confirmed: circuit-breaker tests drive the reachable breaker via `RunExportItems`. Checking remaining claims in parallel.
## Verdict: CONCERNS

## Findings
- [WARNING] Task 1.2's deletion scope misses `circuit_breaker_test.go`, and its tests are not dead-path tests — `internal/sink/circuit_breaker_test.go:51,62,68,108,119,121,127`
  Failure: `TestRunExportItems_circuitBreakerTripsAfterRepeatedFailures` and `TestResetBreakersForTest_clearsOpenBreaker` drive `ExportItemsRequest`/`RunExportItems`. Task 1.1's probe expectation ("only export.go, export_test.go, the stale comment") is already falsified by this file. Task 1.2 ("delete the `TestRunExportItems_*` tests") doesn't cover the second test's name → package fails to compile (1.3 green unreachable as written); if instead deleted wholesale, the only tests of the shared breaker (`internal/sink/circuit_breaker.go:26`, reached from live `RunExportEnvelope` via export.go:267) vanish — proposal Assumption 2 ("deleted tests exercise only the unreachable path") is contradicted, and trip-at-5/reset coverage of live code is lost.
  Fix: task 1.2 must migrate both breaker tests to drive `RunExportEnvelope` (state `circuitBreakerTripAt` failures, assert open + `ResetBreakersForTest`) rather than delete them.
  Confidence: 92
- [WARNING] Task 3.2's adaptation list misses a live `MarshalTargetJSON` caller in `store_test.go` — `internal/collect/store_test.go:252`
  Failure: `TestStoreSubscribeAndMarshal` calls `s.MarshalTargetJSON("team-a","deploys")` and must survive (it also tests Subscribe + `MarshalNamespaceJSON`). Deleting the method (3.1) without touching this site → `internal/collect` test build fails at 3.4.
  Fix: add "drop the `MarshalTargetJSON` assertion at store_test.go:252-255" to task 3.2.
  Confidence: 90
- [NOTE] Alias self-tests pin live `cap.*` semantics that `cap`'s own suite does not fully re-pin — `internal/sink/export_test.go:769-770` vs `internal/sink/cap/capabilities_test.go:8-20`
  Failure: `TestCapabilityConstructors` never asserts `RelationalStore()` sets `SupportsDelete`; after 5.2 the only surviving pin is indirect via `postgres/backend.go:94` + `postgres/capabilities_test.go:20`. Acceptable, but the "self-tests are dead" framing is slightly false.
  Fix: one `RelationalStore()` equality line in `TestCapabilityConstructors` when the aliases go.
  Confidence: 80
- [NOTE] Test-site counts understate reality — `internal/sink/export.go:42-51` aliases have ~24 test references (incl. 4 self-tests), not "≈19"; `BindClusterTargetNamespaces` has 15 test refs, not ~14. Both "≈"-hedged; migration lists in 5.2/6.2 must come from the probe, not the counts.
  Confidence: 85

Verified holds (checked, not assumed): `MergeRequestAPI` zero refs (`gitlab/client.go:26`); constants only at `api/v1alpha1/constants.go:10-11`, no docs/CRD/chart hits; `RunExportItems` production-dead, comment at `kollectclusterinventory_controller.go:278`; `RemoveCluster`/`MarshalTargetJSON` test-only, store.go comments :43/:174/:179 exist; `git.Export`/`ExportMemory` have no production callers (`git/backend.go:60`, `gitlab/backend.go:70` use `ExportWithBranch`), integration call sites exist as claimed (`export_forgejo_integration_test.go:48,98,130`); `BindClusterTargetNamespaces` test-only, prod binds via `RegisterTarget` (`kollectclustertarget_controller.go:193,230`), 4 live `NamespacesForClusterTarget` call sites support task 6.3; auth `_ = user` at `auth.go:122`, entry field at `auth_cache.go:15-18`; `AutoMerge` CRD backing at `kollectsink_types.go:253-255`; coverage floor 90 at `Taskfile.yml:13`; `RunExportEnvelope` has exactly 6 prod refs outside export.go.

## Could not check
- `data/kollect-xconsol-final/report.md` (§4 Sweep 2, §7 item 4) — directory absent from the tree, so DR-1…DR-10 requirement IDs and the exclusions inventory could not be cross-checked to their source.
- No run of `task test`/`task coverage`/`task spec:validate` (plan mode, read-only); floor-hold claim is argued, not measured.
- Whether any external consumer imports this module's deleted exports (Go module external callers not probed; change assumes in-repo-only).
