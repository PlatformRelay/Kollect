Now I'll verify the spec's factual claims against the actual code at HEAD.
Now verifying the deeper claims: the breaker-test migration premise, store shard-deletion claim, auth cache, and line-number citations.
## Verdict: CONCERNS

Verified per requirement against source at HEAD (92360d72), not against the spec's prose. Note: all tasks are unexecuted (`not-run`), so "holds" here means the requirement is well-defined and its factual premises reproduce at HEAD.

- **DR-1 holds** — probe 1.1's expected hit set matches reality exactly: prod-dead definition `internal/sink/export.go:95`, tests only in `export_test.go`/`circuit_breaker_test.go`, stale comment `internal/controller/kollectclusterinventory_controller.go:278`. The `export.go:267` cite is the exact `exportThroughBreaker` call. Breaker-test migration is faithful: `RunExportItems` delegates to `RunExportEnvelope` (export.go:132→267, same `ns/name` breaker key, `trip-at=5` at circuit_breaker.go:19) and both trip/open/reset errors keep the transient class via `classifyExportFailure` (export.go:306-312). The in-function comment at export.go:130-131 names `RunExportItems` but dies with the function, so task 8.3's zero-dangling-references gate still passes.
- **DR-2 holds** — `MergeRequestAPI` is definition-only, `internal/sink/gitlab/client.go:25-26`.
- **DR-3 holds** — constants only at `api/v1alpha1/constants.go:10-11`; repo-wide non-Go grep (docs, CRDs, charts, openapi, site) confirms task 2.3's expectation is satisfiable.
- **DR-4 / DR-4b holds** — `RemoveTarget` (store.go:194-203) deletes targets, never the shard; `RemoveCluster` (store.go:206-217) is the sole shard-deleter, so the monotonicity test guards an unreachable trap after deletion. `MarshalTargetExport` stays live (`internal/pipeline/wire.go:289`, `stdout.go:97`) — only the JSON wrapper goes. Cited comments at store.go:43/174/179 exist; `TestStoreSubscribeAndMarshal` at store_test.go:225 confirmed as the second adapt site.
- **DR-5 holds** — zero non-test `git.Export`/`ExportMemory` callers; `Backend.Export` uses `ExportWithBranch` (backend.go:60); integration tags confirmed on both named files; the wrapper's derivation to replicate is `CommitContextFromContext` else `CommitContextFromObjectPath` (export.go:38-44) — small and testable.
- **DR-6 holds** — aliases zero non-test refs; cross-package test users exist exactly as task 5.1 warns: 7 files in `internal/controller`, one in `internal/pipeline` (wire_test.go:34), all can import `internal/sink/cap`.
- **DR-7 holds** — "~14 test sites in 8 files" is exactly 14/8. Production bind via `RegisterTarget` synthetic object verified (`kollectclustertarget_controller.go:71` in `syncEngineTargets`); the reader scans `st.target.Name` (engine.go:426-427), which `RegisterTarget` populates, so 6.2's seeding keeps `NamespacesForClusterTarget` (4 prod sites: ct:254, ci:392,426,654) reachable.
- **DR-8 holds** — `auth.go:115,122,146` match task 7.1 exactly; `auth_cache_test.go`/`auth_test.go` drive the Authorizer, never the entry struct or get/set signatures, so the re-signature breaks no test compile.
- **DR-9 holds** — `task test/lint/coverage/spec:validate` all exist (Taskfile.yml:57,147,206,176); floor `COVERAGE_MIN: "90"` at Taskfile.yml:13.
- **DR-10 holds** — no surviving text refs found for any symbol outside the change dir itself.

## Findings
- [WARNING] The Assumptions' load-bearing count "RunExportEnvelope (6 production references)" does not reproduce by any counting — `proposal.md:91-92`
  Failure: grep of `*.go` non-test files gives 4 call sites (`export.go:132`, `cleanup.go:364`, `kollectinventory_controller.go:404`, `kollectclusterinventory_controller.go:331`) — and one of those four is the dead runner being deleted, i.e. 3 live; comments push it to 8, never 6. A verifier running the assumption's own probe gets a different number than the spec asserts, undermining the "loses no reachable coverage" evidence chain.
  Fix: correct to "3 live production call sites (the 4th is the deleted runner itself)".
  Confidence: 92
- [WARNING] The spec's sole external anchor — `data/kollect-xconsol-final/report.md`, cited as the source of the item list (proposal.md:5) and both exclusion justifications (proposal.md:74, 76) — does not exist in this repository (no `data/` directory; `git ls-files` has no xconsol entry).
  Failure: task 8.2 ("sweep exclusions... remain true") and the Non-goals' "report §7 item 4" are unverifiable by anyone working from this checkout; the reviewer cannot confirm the deletion list is complete or the exclusions are the report's.
  Fix: inline the §4 Sweep 2 item list and the two exclusion reasons into proposal.md, or record the report's real (external) location.
  Confidence: 90 (absence verified; the consequence is inference)
- [NOTE] Same test-only-exported-symbol class as the four deleted aliases — `ResetBreakersForTest` (`internal/sink/circuit_breaker.go:66`), `EnableBackendPoolForTest`/`ResetBackendPoolForTest` — is kept with no Non-goal entry; only the `*ForTest`-suffix convention distinguishes it.
  Fix: one Non-goals line saying why `*ForTest` hooks stay, so the next sweep doesn't re-litigate it.
  Confidence: 60

Not strengthened beyond spec anywhere: no task deletes a symbol whose premise I verified as live (checked `MarshalTargetExport`, `ExportWithBranch`, `NamespacesForClusterTarget`, the `*ForTest` hooks — all legitimately retained by the tasks as written).

## Could not check
- `data/kollect-xconsol-final/report.md` §4/§7 — outside this repo; could not verify the sweep's item list or exclusion rationale against its source.
- GitHub PR #451 (docs lane independence) — external, not fetched.
- Runtime behaviour after deletion (build, suites, coverage floor, tagged-file vet) — nothing executed; tasks are planning artifacts with all evidence columns `not-run`.
- External Go consumers of `api/v1alpha1` constants — unprovable in-repo; the proposal itself records this as an accepted limit (proposal.md:95-99).
