# Proposal

## Why

The final consolidated review (`data/kollect-xconsol-final/report.md`, §4 Sweep 2) lists an
exported surface that production never calls: the old sink export runner, a stub GitLab
interface, two unused condition constants, two superseded store methods, two superseded git
entry points, four test-only capability aliases, a superseded engine binding method, and a
cache entry field that is never read. Every item was re-verified at this HEAD (probe commands
and zero-caller counts in the tasks file). Keeping them shipped advertises API that does
nothing: a maintainer reading `RunExportItems` or `git.Export` reasonably believes it is the
production path, which it is not.

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
points, so there is no behavioural red-first test to write. The per-task "red" is the recorded
zero-caller probe; the green is a clean compile with the symbol gone.

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

- Zero-caller probes are correct at this HEAD. Each task re-runs its own probe before deleting
  and the compile after deletion is the backstop: a missed caller fails `go build ./...`.
- Deleting the dead runner's tests loses no reachable coverage: production dispatches through
  `RunExportEnvelope` (6 production references) and the controller export loops, which keep
  their own suites. The deleted tests exercise only the unreachable `RunExportItems` path.
- The coverage floor holds because unreachable code and its tests leave together; the final
  tree is measured (`task coverage`), not assumed.
