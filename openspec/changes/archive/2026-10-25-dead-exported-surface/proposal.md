# Proposal

## Why

The final consolidated review (Kollect's kollect-xconsol effort, Sweep 2 of its §4; the
authoritative report lives outside this repo at
`data/kollect-xconsol-final/report.md` — its Sweep 2 list is quoted verbatim at the end of
this file) lists an exported surface that production never calls: the old sink export runner, a
stub GitLab interface, two unused condition constants, two superseded store methods, two
superseded git entry points, four test-only capability aliases, a superseded engine binding
method, and a cache entry field that is never read. Every item was re-verified at this HEAD
(probe commands and zero-caller counts in the tasks file). Keeping them shipped advertises API
that does nothing: a maintainer reading `RunExportItems` or `git.Export` reasonably believes it
is the production path, which it is not.

## What Changes

- Delete production-dead exported symbols, each verified by deletion compiling clean
  (`go build ./...` is the per-symbol proof that no caller was missed):
  - `RunExportItems` and `ExportItemsRequest` (`internal/sink/export.go`) plus their dead-path
    tests, and the stale comment in `kollectclusterinventory_controller.go` that still cites
    `RunExportItems`.
  - `gitlab.MergeRequestAPI` (`internal/sink/gitlab/client.go`) — zero references, tests
    included.
  - `ConditionConnected` and `ConditionCredentialsVerified` (`api/v1alpha1/constants.go`).
  - `Store.RemoveCluster` and `Store.MarshalTargetJSON` (`internal/collect/store.go`) plus
    their tests.
  - `git.Export` and `git.ExportMemory` (`internal/sink/git/export.go`) — tests move to the
    live `ExportWithBranch` pipeline (a test-local helper replaces the wrapper) so coverage of
    the real path is unchanged.
  - The four capability aliases `SnapshotStoreCapabilities`,
    `ObjectStoreSnapshotCapabilities`, `StreamEmitterCapabilities`, `RelationalStoreCapabilities`
    (`internal/sink/export.go`) — tests use `cap.*` directly.
  - `Engine.BindClusterTargetNamespaces` (`internal/collect/engine.go`) — superseded: production
    binds cluster targets via `RegisterTarget` with synthetic objects
    (`kollectclustertarget_controller.go` `syncEngineTargets`); tests migrate to that shape.
- Drop the dead `user` field from the inventory auth cache: it caches an authorization
  decision, not identity (`internal/inventory/auth_cache.go`, `auth.go` `_ = user`).
- Record the sweep's exclusions and their reasons (see Non-goals).

No observable behaviour changes: every deleted symbol is unreachable from production entry
points, so there is no behavioural red-first test to write. Per-task evidence is the recorded
zero-caller probe, a clean compile with the symbol gone, and the package suites; files behind
a build tag are compiled with that tag on as part of the proof.

## The production dispatch that stays (what the deleted surface is not)

Live envelope dispatch runs through `RunExportEnvelope` with three production call sites —
`internal/sink/cleanup.go`, `internal/controller/kollectinventory_controller.go` and
`internal/controller/kollectclusterinventory_controller.go` — and keeps its own test battery
(`TestRunExportEnvelope_*`) plus the controller suites. `RunExportItems` was the older
items-level runner: it delegated to `RunExportEnvelope` and no production entry point calls it.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None.

## Impact

- Code: `internal/sink/export.go`, `internal/sink/export_test.go`,
  `internal/sink/gitlab/client.go`, `internal/sink/git/export.go` and its test files,
  `api/v1alpha1/constants.go`, `internal/collect/store.go`, `internal/collect/engine.go` and
  their test files, `internal/inventory/auth.go`/`auth_cache.go`, one stale comment in
  `internal/controller/kollectclusterinventory_controller.go`.
- No CRD, chart, docs or workflow changes. Shipped generated artifacts are untouched
  (constants live only in Go code).
- Coverage: dead production code and its dead-path tests leave together; `task coverage`
  (internal/ floor 90%) is measured on the final tree.

## Dependencies

None: independent of the ten in-flight tooling changes and of the docs lane
(https://github.com/PlatformRelay/Kollect/pull/451).

## Non-goals

- `EvictBackendPool` / `EvictBackendPoolByUID` stay: deleting them pre-empts the backend-pool
  eviction product decision (report §7 item 4) and ~30 test housekeeping call sites depend on
  them. The product-decision lane owns that surface.
- `MergeRequestConfig.AutoMerge` stays: the dead internal field is backed by the CRD field
  `autoMerge` (`api/v1alpha1/kollectsink_types.go:253-255`, documented "not yet implemented"),
  so removal is an API-contract decision for the captain, not dead-surface hygiene.
- `layout.VerifySet` stays: deliberate on-disk verification API (ADR-0405).
- No deduplication or structural extraction (S1/S2 and the P2 dedup trains are separate work).

## Assumptions

- Zero-caller probes are correct at this HEAD. Each task re-runs its own probe (full output
  recorded, never truncated) and the compile after deletion is the backstop: `go vet ./...`
  compiles test files too, so a missed non-test caller fails the build and a missed test
  caller fails vet. Tagged files are caught by the tag-on vet each task records.
- Deleting the dead runner's tests loses no reachable coverage, with two handled exceptions:
  (a) the two circuit-breaker tests drive the live `exportThroughBreaker` (production
  `RunExportEnvelope`) through the dead runner and are migrated to the live path, keeping their
  trip/reset assertions (task 1.3); (b) task 1.3 records, per deleted `TestRunExportItems_*`
  test, which live test or suite covers the same behaviour, or that the behaviour is
  runner-specific (the runner itself). The remaining coverage argument: the live dispatch is
  covered by the `TestRunExportEnvelope_*` battery, the pool/classifier/close unit tests and
  the controller suites.
- The coverage floor holds because unreachable code and its tests leave together; the final
  tree is measured (`task coverage`), not assumed.
- Deleting `ConditionConnected`/`ConditionCredentialsVerified` from the importable
  `api/v1alpha1` package assumes no external consumer: the module is pre-1.0, the constants
  have zero in-repo references (code, tests, docs, CRDs), and they were never wired to any
  status write, so no consumer could have observed them in a shipped object. Recorded as an
  accepted limit; the compile cannot prove the external half. The deletion commit's body names
  the removed exported symbols so the commit-derived changelog records the API change.

## Source excerpt (kollect-xconsol-final §4, Sweep 2)

> `RunExportItems`/`ExportItemsRequest` (definition in `internal/sink/export.go` + a **stale
> comment** still citing it at `kollectclusterinventory_controller.go:289`);
> `EvictBackendPool`/`EvictBackendPoolByUID` (own comment admits no caller — … decide
> evict-on-delete vs documenting the TTL); `BindClusterTargetNamespaces`; `Store.RemoveCluster`
> / `MarshalTargetJSON` (A-only, verified definition-only); `gitlab.MergeRequestAPI` (zero
> callers outside own file/tests); `MergeRequestConfig.AutoMerge` — parsed and stored but never
> read behaviourally (A-only, verified); `ConditionConnected`/`ConditionCredentialsVerified`
> (`constants.go:10-11`, unused); dead cached `user` in the auth cache (`inventory/auth.go:122`
> `_ = user` — the cache caches authorization, not identity); `s3.NewBackendWithClient`'s
> comment falsely claims gcs uses it (gcs constructs `s3.NewBackend` directly);
> `git.Export`/`ExportMemory` = production-dead/test-live — note the correction both passes
> carry: glm's "not even tests" was false (test callers exist); and `probe.go` has 9
> `testXxxConnection` wrappers (glm said ten). Keep `layout.VerifySet` (ADR-0405, deliberate).
> Fix the two misleading comments in the same sweep.

Re-measured at this HEAD, three excerpt items changed verdict and are recorded as non-goals:
`EvictBackendPool*` (product decision, §7) and `MergeRequestConfig.AutoMerge` (CRD-backed API
surface, see Non-goals); the `s3.NewBackendWithClient` comment is accurate as written at this
HEAD ("test helpers (gcs) and unit tests use this" — the gcs test files do use it, production
gcs constructs `s3.NewBackend`), and the `probe.go` wrappers are the live
`connectionTesters` dispatch table, not dead code.
