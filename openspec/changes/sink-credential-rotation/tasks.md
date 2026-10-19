# Tasks

Red first. In each group the test task comes before the implementation and records its red in
`evidence/G<N>.md`: the named test runs (`=== RUN <name>`) and fails on the target assertion, not on
a compile error, a missing Docker daemon or a skip. "Red on base" means red on the revision the
group starts from: `origin/main` for G1, the G1 commit for G2, the G2 commit for G3. A stub with
the signature from `design.md` and a `not implemented` body is in scope so the tests compile. Each
group closes in one commit (test and implementation), so the tree is green at every commit and
*Known red* stays empty unless a group is split. A case marked *control* is expected green on its
base and is recorded as a control, not a red.

Each case below reads: given / when / then · the failure it exposes.

Test rules for every group:

- Unit tests in `internal/sink` use the controller-runtime fake client and a recording fake
  registry (a factory that records the `BuildContext` it was given and returns a fake backend
  with a `Close` counter and an `Export` that can block on a channel). They call
  `EnableBackendPoolForTest` and restore the previous state in `t.Cleanup`. Sequential tests
  release every hold before asserting a `Close` count.
- The envtest cases are Ginkgo specs in `internal/controller` and follow
  `family_sink_delete_watch_envtest_test.go`, which turns the pool on for its spec (the suite turns
  it off, `internal/controller/suite_test.go:31`); they run the inventory controller against a
  real API server with a recording registry and the reconciler wired as in `cmd/main.go`
  (`SecretReader: mgr.GetAPIReader()`).
- Tests that dial loopback (httptest TLS) or a container carry `//go:build integration` as their
  first line, because netguard refuses loopback outside that tag
  (`internal/sink/netguard/dialer.go:242-248`), and live under `INTEGRATION_PACKAGES`
  (`Taskfile.yml`). Container-backed ones handle a missing Docker daemon like
  `internal/sink/postgres/export_integration_test.go:34-45`; without Docker they skip, which is
  neither red nor green and leaves the row `blocked` until CI `test-integration` records it. Run
  them with `go test -tags integration -v -run '<names>' <pkg>` and quote the `=== RUN` lines.
- Certificates are generated at test time with `internal/sink/sinktls/sinktlstest`; nothing with a
  private key is committed.
- Secret values in every test are the sentinels `sentinel-pass`, `sentinel-token`, `sentinel-key`
  (or values containing them), so SCR-4 assertions can search for them.
- Each group records `go test -race -count=3` on the packages it changed.

## 0. Probe

- [x] 0.1 Verify the load-bearing assumptions in `proposal.md` and record each as verified
  (command, versions, date) in `evidence/T0.1.md`; a failed one stops the loop as a spec finding:
  - `mgr.GetAPIReader().Get` of a `metav1.PartialObjectMetadata` with kind Secret works in envtest
    with the operator's RBAC verbs, returns UID and `resourceVersion` and no data; the fake client
    supports the same call;
  - envtest: a no-op Secret update keeps `resourceVersion`; a data change and a label change each
    change it; delete-and-recreate changes the UID;
  - the coalesce skip ends when the export interval is due
    (`internal/controller/kollectinventory_controller.go:360-368`), so an unchanged inventory
    still exports once per interval;
  - BigQuery v1.85.0 `Client.Close` and an operation in flight (cite file:line);
  - Git: the mirror directory is keyed by clone URL and branch (`internal/sink/git/mirror.go:149`)
    and exports are serialized by `exportLocks` (`internal/sink/git/export_lock.go:10-25`), and
    auth is read from the backend per call, so a rebuilt Git backend pushes with the new
    credentials;
  - an S3 `secretRef` Secret with no access key builds a client on the SDK default chain today
    (`internal/sink/s3/config.go:65-78`, `backend.go:159-163`);
  - a missing Secret's acquire error is classified transient on the export path and the inventory
    requeues (`internal/sink/export.go:96-108`, `internal/errors/errors.go:86-106`);
  - which sink types `RunCleanupExport` retains without contacting the sink
    (`internal/sink/cleanup.go:210-214`), and a type-level test for them that needs no backend.
  — closed 2026-10-19, evidence: evidence/T0.1.md (finding 3 resolved by decision
  2026-10-19 `rotation-zero-interval`, option B: SCR-7, tasks 2.3 and 2.4)

## 1. Rotation rebuilds the backend (SCR-1, SCR-4 key)

- [x] 1.1 Tests (red on `origin/main`):
  - `TestAcquireBackend_rotatedSecretRebuilds` (table, one row per reference: `secretRef`,
    `git.auth.secretRef`, `kafka.secretRef`, `nats.secretRef`, `postgres.databaseRef`,
    `mongodb.databaseRef`, `bigquery.secretRef`, `tls.caSecretRef`; each row once with the
    reference's namespace omitted and once explicit): given a pooled backend built from value A /
    when the Secret's data changes to B and acquire runs / then the factory ran twice, the second
    `BuildContext` holds B, and after release the first backend's `Close` ran once · exposes a key
    that ignores Secrets (base: one build, A reused).
  - `TestAcquireBackend_readsSecretsUncached` (rows: export; Retain cleanup; Delete cleanup
    through `pruneRelationalExport`): `Client` is a fake holding the stale Secret A,
    `SecretReader` a fake holding B / the backend is built from B · exposes reads through the cache
    and a nested request that drops the reader.
  - `TestInventoryReconcilers_defaultSecretReader` (both reconcilers): `SetupWithManager` with no
    reader set / the field holds the manager's API reader, and `cleanupSinkExports` receives it ·
    exposes production wiring that falls back to the cached client.
  - `TestAcquireBackend_rotateBack`: A, B, A with an acquire after each / three builds, each with
    the current value · exposes reuse of a backend built for an earlier version (base: one build).
  - `TestAcquireBackend_sameDataNewVersionRebuilds`: an update that rewrites the same data with a
    changed label / one rebuild · pins the metadata-identity rule (a content hash would not
    rebuild).
  - *control* `TestAcquireBackend_unchangedSecretsReuse`, `TestAcquireBackend_unrelatedSecretReuse`:
    two acquires, nothing referenced changed (or an unreferenced Secret changed) / one build, same
    instance · exposes an over-eager key once the change lands.
  - `TestAcquireBackend_keyFromBuildSnapshot`: the reader returns version 5 for the metadata GET
    and version 6 for the full GET / the stored key matches version 6, and the next acquire (still
    6) is a hit · exposes a key and values from different reads.
  - `TestReferencedSecretRefs_coversEverySecretReferenceField`: reflection over `KollectSinkSpec`
    finds every `*SecretReference` field; for each, a spec of the type that reads it lists that
    Secret with its effective namespace · exposes a reference field the key misses.
  - `TestSecretMeta_metadataOnly`: the metadata function's return type carries no data; two
    Secrets with equal metadata and different data give equal keys; a changed UID or
    `resourceVersion` changes the key · exposes a key derived from values.
  - envtest spec "secret rotation takes effect on the next export": an inventory bound to a
    snapshot sink whose recording backend saw `sentinel-token-1` / the Secret is updated to
    `sentinel-token-2` and the next export runs / the export's `BuildContext` holds the new value ·
    exposes the production path (reconciler wiring) not reaching the new value.
  - envtest spec "cross-namespace secret under a restricted cache": a manager whose cache is
    restricted to the inventory's namespace (`operator.CacheOptionsForWatchNamespaces`), the
    namespace of the Secret allowlisted / the export builds from that Secret · exposes the cached
    reader (base: "unknown namespace for the cache").
  - integration `TestS3Export_caRotation` (`internal/sink`, httptest TLS server): server
    certificate from CA Y, Secret `ca` first holds CA X, then CA Y / the first export fails TLS
    verification, the export after the update succeeds · exposes a rebuilt backend that keeps the
    old roots.
  - integration `TestPostgresExport_passwordRotation` (testcontainers Postgres): export as role
    `u1`; switch the `databaseRef` Secret to role `u2`, `ALTER ROLE u1 NOLOGIN`; export again /
    the second export carries one item with a new source UID, whose row (an insert, not the
    `ON CONFLICT` update path, `internal/sink/postgres/bulk_export.go:67-69`) has the column
    default `current_user = u2`; a separate observer connection (superuser) sees no `u1` session
    from the test's backends after release · exposes a pool that keeps authenticating as `u1` on an
    idle connection (`NOLOGIN` does not end open sessions; base: export succeeds as `u1`).
  — closed 2026-10-19, evidence: evidence/G1.md
- [x] 1.2 Implement `referencedSecretRefs`, `secretMeta` (metadata GET through `SecretReader`;
  not-found keeps today's text), the key, the rebuild from the full objects, the `SecretReader`
  field and its wiring in both inventory reconcilers and `cmd/main.go`, the `BuildContextFromSpec`
  split, and the info log (design: *Readers*, *Referenced Secrets, one list*, *Fingerprint and
  acquire* steps 1-3). The store fence (step 4) is G3; the empty check and retire rules are G2.
  Mutations, judged by exit status: drop the Secret part from the key (rotation tests fail); read
  through `Client` instead of `SecretReader` (uncached test fails); drop one case from
  `referencedSecretRefs` (reflection test fails); no-op control: reorder two independent
  statements in `acquireBackend` (all pass). — closed 2026-10-19, evidence: evidence/G1.md

## 2. Deleted and emptied Secrets fail (SCR-2, SCR-4, SCR-5, SCR-7)

Group G2 is 2.1 to 2.4, closed in one commit; 2.3 and 2.4 come from probe 0.1 finding 3
(model-aligned decision 2026-10-19, `rotation-zero-interval`, option B).

- [x] 2.1 Tests (red on the G1 commit):
  - `TestSecretMeta_notFoundIsTransient`: the metadata GET of a missing Secret (fake client and
    the envtest API reader both return an API NotFound) / `secretMeta` returns the plain
    `secret %q not found in namespace %q` error, `kollecterrors.ClassOf` is transient and
    `apierrors.IsNotFound` is false on it · exposes a raw NotFound, which `ClassOf` classes
    terminal (`internal/errors/errors.go:99-101`, probe 0.1 finding 7), so a deleted Secret is
    never retried (red on G1 while `secretMeta` returns the API error as is).
  - `TestAcquireBackend_deletedSecretFails` (rows: `secretRef`, `databaseRef`, `caSecretRef`,
    `git.auth.secretRef`): pooled backend, Secret deleted, acquire / error names Secret and
    namespace, `kollecterrors.ClassOf` is transient, the entry is gone and its `Close` ran once,
    the factory did not run · exposes a pooled entry left in place after a delete.
  - `TestAcquireBackend_emptiedSecretFails`: rows for `Data == nil`, `Data == {}` and all values
    empty / error `has no data` naming the Secret, transient, entry retired · exposes an empty
    Secret accepted (on G1 it rebuilds with empty values).
  - *control* `TestAcquireBackend_restoredSecretRecovers`: delete, acquire fails, recreate,
    acquire / new build succeeds with the new value; no sink change (green on G1 already).
  - *control* `TestAcquireBackend_cancelledReadKeepsEntry`: the metadata read returns
    `context.Canceled` for one acquire / that acquire fails, the entry stays pooled, the next
    acquire is a hit (green on G1, which retires nothing; the 2.2 mutation "retire on every
    error" must turn it red).
  - `TestAcquireBackend_observedChangeRetiresOnFailedRead`: the metadata read shows a new
    `resourceVersion`, the full read returns a timeout / the acquire fails and the old entry is
    retired · exposes a stale entry kept after an observed change.
  - `TestResolveSecret_emptyIsError` (pool disabled, the probe path) · exposes a check that lives
    only in the pool.
  - `TestS3_emptiedSecretNoAmbientCredentials`: `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` set in
    the test environment, S3 sink Secret emptied, acquire / fails naming the Secret, factory not
    called · exposes the fall-through to the default chain.
  - `TestCleanup_streamSinkDeletedSecretRetained` (Kafka and NATS rows): Secret deleted,
    `RunCleanupExport` / `CleanupRetained`, nil error, no Secret read and no factory call ·
    exposes a deletion blocked by a Secret the cleanup never needs (on G1: acquire fails).
  - *control* `TestCleanup_deletedGitAuthSecretFails`: Retain release for a Git sink whose auth
    Secret was deleted / `RunCleanupExport` returns the naming error, not success (green on G1).
  - `TestStreamSinkTypes_matchRetainingBackends`: every registered type whose backend is neither
    `SupportsDelete` nor an `ExportCleaner` is in `streamSinkTypes`, and no other type is ·
    exposes drift between the set and `cleanup.go:210-214`.
  - `TestRotationErrors_noSentinel`: through rotate, empty and delete, with a sentinel also in a
    Secret annotation, every error this change adds, every line captured from the `sink` logger
    (a test logr sink) and the pool's entries contain none of the sentinels.
  - envtest spec "deleted secret fails the export with a redacted status": delete the sink Secret /
    the inventory's export condition names the Secret, the recorded Events and status contain no
    sentinel, and after recreating the Secret the next retry exports with no sink change · exposes
    the controller path (condition text, requeue) and SCR-4 on status and Events.
  - CLI `TestResolveSinkSecretData_emptySecretFails` (`internal/pipeline`; rows: no `data` and no
    `stringData`; `data` keys all empty; `stringData` only, non-empty, *control*; a value
    `${env:VAR}` with `VAR` set, *control*; a Secret without a namespace): an empty Secret fails
    before `registry.NewBackend` with the operator's message text (without the namespace clause
    when the manifest has none) · exposes a missing CLI parity.
  - *control* `TestRunOneContext_secretManifestChangeNextRun`: a second run with a changed manifest
    builds from the new value. — closed 2026-10-19, evidence: evidence/G2.md
- [x] 2.2 Implement the empty check in `ResolveSecret`, the rebuild path and
  `resolveSecretValues`; retire only on not-found, empty, or a failed rebuild after a key change;
  the stream retain before the acquire in `RunCleanupExport` (design: *Fingerprint and acquire*
  errors, *Cleanup before acquire*). Mutations: skip the retire on not-found (deleted test fails);
  accept an empty map (emptied test fails); retire on every error (cancelled test fails); return
  the metadata GET's NotFound as is (`TestSecretMeta_notFoundIsTransient` and the deleted test's
  class assertion fail); no-op control as in 1.2. `secretMeta` maps NotFound to the plain text
  (design *Errors on the acquire path*). — closed 2026-10-19, evidence: evidence/G2.md
- [x] 2.3 Tests for SCR-7 (red on the G1 commit), unit tests in `internal/controller` next to
  `TestKollectInventoryReconciler_setInventoryDegraded` (`kollectinventory_status_test.go:18`)
  and `TestKollectClusterInventoryReconciler_setDegraded`
  (`kollectclusterinventory_helpers_test.go:273`), fake client with the status subresource:
  - `TestSetInventoryDegraded_zeroIntervalRequeues`: given a `KollectInventory` with
    `exportMinInterval: 0s` / when `setInventoryDegraded` runs / then `RequeueAfter ==
    validation.ZeroIntervalWatchdog` · exposes the `RequeueAfter: 0` that drops the retry
    (`kollectinventory_controller.go:652`). *Control* rows: `30s` and unset (CRD default 30 s)
    keep `RequeueAfter == 30s`; `5m` keeps `5m`.
  - `TestClusterSetDegraded_zeroIntervalRequeues`: the same rows for
    `KollectClusterInventoryReconciler.setDegraded`
    (`kollectclusterinventory_controller.go:713`) · exposes the cluster half.
  - *control* `TestApplyInventoryExportOutcome_zeroIntervalTerminalNoRequeue`: a `0s` inventory
    whose only sink export fails with `kollecterrors.Terminal` (recording registry whose factory
    returns it) / the result has `RequeueAfter == 0` and a nil error (green on G1; the 2.4 mutation
    "watchdog for terminal too" must turn it red).
  - envtest spec "zero-interval inventory retries a deleted secret": a `0s` inventory bound to one
    sink whose Secret is missing / the export fails with the not-found error and the reconcile
    result requeues after the watchdog; after the Secret is created the next reconcile exports
    with no change to the inventory or the sink · exposes the stuck failure end to end.
  - *control* envtest spec "unchanged zero-interval inventory is not re-exported": a `0s`
    inventory exports once successfully; reconcile it twice more with an unchanged payload, spec
    and `kollect.dev/requestedAt` / the recording backend's `Export` count stays 1 (green on G1;
    the 2.4 mutation "drop the interval-0 skip" must turn it red). — closed 2026-10-19, evidence: evidence/G2.md
- [x] 2.4 Implement SCR-7: `setInventoryDegraded` and the cluster `setDegraded` return
  `RequeueAfter: validation.RequeueAfterForZeroInterval(r.exportDebounce(inv))`; the terminal
  branches that zero it (`kollectinventory_controller.go:262-264`, cluster `:224-226`) stay
  (design *Zero export interval*). Mutations, judged by exit status: keep `exportDebounce` (the
  zero-interval tests fail); requeue terminal failures at the watchdog too (terminal control
  fails); make `shouldSkip` return false for interval 0 (the not-re-exported control fails);
  no-op control: rename a local variable in `setDegraded` (all pass). `go test -race -count=3
  ./internal/controller/`. — closed 2026-10-19, evidence: evidence/G2.md

## 3. Retired backends drain (SCR-3)

- [x] 3.1 Tests (red on the G2 commit, where a key change closes the old backend at once,
  `backend_pool.go:156-158` and `:208-214`):
  - `TestAcquireBackend_rotationDuringExportDefersClose`: E1 acquires B1 and blocks in `Export`;
    Secret changes; E2 acquires / E2 gets B2, B1's `Close` count is 0 until E1's release, then 1;
    E1's `Export` returns nil · exposes the immediate close under an export.
  - `TestAcquireBackend_specChangeDuringExportDefersClose`: same with a spec change.
  - `TestAcquireBackend_deleteEvictionDuringExportDefersClose`: `EvictBackendPoolForSink` while E1
    holds B1 / entry gone, tombstone set, the next acquire for that UID builds an unpooled backend
    under an owning release, B1 closed after E1 releases.
  - `TestAcquireBackend_ttlSkipsHeldEntry`: advance `timeNow` past the TTL while E1 holds B1 and
    another acquire runs / B1 not closed until E1 releases.
  - `TestAcquireBackend_slowBuilderDoesNotReplaceNewer`: G1 misses at version 5 and blocks in the
    factory; the Secret moves to 6; G2 builds and stores B6; G1's build completes / B6 stays
    pooled, G1's backend is returned under an owning release and closed after it, the next acquire
    gets B6 · exposes the last-writer-wins store (`backend_pool.go:208-214`).
  - `TestAcquireBackend_slowBuilderAfterRetireNotPooled`: G1 misses and blocks; the Secret is
    deleted and an acquire retires the entry; G1 completes / G1's backend is not pooled (owning
    release), the pool holds no entry for the sink · exposes a fence that treats an absent entry
    as permission.
  - `TestAcquireBackend_sameKeyBuildersShare`: two builders miss at the same version and both
    complete / one entry, both hold it, the surplus is closed once · exposes a fence checked
    before the same-key share.
  - `TestAcquireBackend_concurrentAcquiresAfterRotation`: 16 goroutines acquire after one Secret
    change, with a barrier between the metadata read and the store / all hold one new backend;
    each surplus build and B1 closed exactly once; run with `-race -count=10` · exposes double
    close or a leaked surplus build.
  - `TestAcquireBackend_failedRebuildRetiresOld`: Secret changes to a value the fake factory
    rejects / acquire fails, B1 retired and closed after its holders release, a later acquire with
    a valid value builds B2 and never returns B1.
  - `TestRelease_idempotent`: calling a release twice decrements once.
  - Existing BEP-1 tests (`internal/sink/backend_pool_delete_hook_test.go`) stay green unchanged;
    any change to them is a *Test changes* row with a reason. — closed 2026-10-20, evidence: evidence/G3.md
- [x] 3.2 Implement holders, `retireLocked`, the release and the store fence (design: *Pool
  entries with holders*, *Fingerprint and acquire* step 4). Mutations: close at retire regardless
  of holders (defer tests fail); drop the fence (slow-builder test fails); omit the `sync.Once`
  (idempotent test fails); no-op control as in 1.2. `go test -race -count=10 ./internal/sink/`.
  — closed 2026-10-20, evidence: evidence/G3.md

## 4. Docs (SCR-6)

- [x] 4.1 Update `docs/crds/kollectsnapshotsink.md` and `kollectdatabasesink.md` (rotation
  paragraph; remove "A backend that already built keeps its CA until the sink spec changes" at
  `kollectsnapshotsink.md:53-54` and `kollectdatabasesink.md:70-71`), `kollecteventsink.md` (a
  rotation paragraph), `docs/security/security-architecture.md` (rotation sentence at the Secrets
  section, `:214-216`), `docs/operator-manual/troubleshooting.md` (rows for a deleted or emptied
  Secret, which recover on the next retry, within 30 s for a `0s` inventory; a sentence that
  `exportMinInterval: 0s` means "export only on material change", so a Secret change alone never
  triggers an export for such a binding; the Git auth `terminal` row at `:204` keeps its
  re-annotate step, because a terminal error does not requeue and no controller watches
  Secrets), and
  `docs/operator-manual/upgrading.md` (new section "Sink Secret rotation (after v0.21.0)" with the
  design compatibility table, "entirely empty" stated for the S3/GCS fallback, the Retain and
  `force-cleanup` note, and the CA-section sentence at `:167-168` removed). Checks:
  `rg -U 'keeps its\s+CA' docs` finds nothing; `task lint:markdown` 0 issues; `task docs:verify`
  where it runs (see AGENTS.md gotcha).
  — closed 2026-10-20, evidence: evidence/G4.md

## 5. Integration

- [ ] 5.1 Integrator: `task check` on the branch head; CI `test-integration` names
  `TestPostgresExport_passwordRotation` and `TestS3Export_caRotation` as run and passed;
  independent branch review recorded (reviewer, revision, verdict); then archive the change as the
  last commit and record both revisions.

## 6. Stage-B fixes

From the stage-B branch review, round 1 (`reviews/B-branch/round-1/verification.md`). One commit
each; red first where behaviour changes; mutants in a scratch tree outside the repository, judged
by the test command's exit status, with a no-op control that survives.

- [x] 6.1 H1 (holistic): the controller tests use the package's `boolPtr`, not `k8s.io/utils/ptr`,
  so `go mod tidy -diff` (Preflight) is clean. — `e2b3193e1`; `go mod tidy -diff` exit 0
- [x] 6.2 A1: a missing or empty Secret seen by the full read of a miss (or the CA-rules read behind
  it) retires the key, forced, so an entry stored meanwhile is retired and a build in flight is
  not pooled (SCR-2). Tests `TestAcquireBackend_fullReadNotFoundFencesBuildInFlight`,
  `_fullReadNotFoundRetiresEntryStoredMeanwhile`, `_fullReadCAGoneFencesBuildInFlight`
  (`internal/sink/backend_pool_full_read_test.go`), red on the pre-fix `backend_pool.go` at `:76`,
  `:112`, `:181`; mutants: no retire, not forced, CA-rules result ignored, all killed; control
  survives. — `353acdb78`
- [x] 6.3 A2: a `0s` skip counts as activity for the coalescing-state prune, so the watchdog's
  reconciles of an unchanged, exported `0s` binding never age its state out and re-export it
  (SCR-7). Test `TestPerSinkCoalesceTracker_zeroIntervalSkipsKeepStateAlive`, red at `:165`
  (`not skipped at +60h0m0s`); mutants: no touch, prune on export time only, both killed;
  control survives. — `e42866e4c`
- [x] 6.4 B4 (G2 gap): cluster twin `TestClusterReconcile_zeroIntervalTerminalNoRequeue`; green
  (the code already zeroed it); mutants: zeroing removed, watchdog for terminal, both killed;
  control survives. — `404da3763`
- [x] 6.5 B1 (accepted residual): `TestAcquireBackend_failedReReadAfterStaleReadDrainsNewer` pins
  that a failed re-read after a stale first read only costs a build (no Close while held, the next
  acquire builds from the current value); green; killed by "retire ignores holders". —
  `349c22173`
- [x] 6.6 B1, B3 wording: SCR-2 (a failed read retires nothing on its own account), SCR-3 (the
  fence starts at the pool miss), design *Fingerprint and acquire* (full-read retire, stale-read
  residual); `task spec:validate` exit 0. — `c542c6f22`

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| SCR-1 | `TestAcquireBackend_rotatedSecretRebuilds` (8 references × 2 namespace forms), `_rotateBack` | rebuild with the current value per reference kind | pass | evidence/G1.md (2026-10-19) |
| SCR-1 | `TestAcquireBackend_readsSecretsUncached` (export, Retain, Delete prune), `_keyFromBuildSnapshot`, `_sameDataNewVersionRebuilds`, `TestInventoryReconcilers_defaultSecretReader` | reads bypass the cache on every path; key and values from one read; production wiring | pass | evidence/G1.md (2026-10-19) |
| SCR-1 | *control* `TestAcquireBackend_unchangedSecretsReuse`, `_unrelatedSecretReuse` | one build, same instance | pass | evidence/G1.md (2026-10-19) |
| SCR-1 | `TestReferencedSecretRefs_coversEverySecretReferenceField` | every `*SecretReference` field covered | pass | evidence/G1.md (2026-10-19) |
| SCR-1 | envtest specs "secret rotation takes effect on the next export", "cross-namespace secret under a restricted cache" | next export sees the new value; no cache error | pass | evidence/G1.md (2026-10-19) |
| SCR-1 | integration `TestS3Export_caRotation` | handshake fails on CA X, succeeds after rotation to CA Y | pass (local, Docker not needed) | evidence/G1.md (2026-10-19); CI `test-integration` at 5.1 |
| SCR-1 | integration `TestPostgresExport_passwordRotation` | rows as `u2`, no `u1` session left | pass (local Docker) | evidence/G1.md (2026-10-19); CI `test-integration` at 5.1 |
| SCR-1 | mutations: Secret part dropped from the key; reads through `Client` | rotation / uncached tests fail | pass | evidence/G1.md (2026-10-19) (M1, M2; also M3 reflection, M0 control) |
| SCR-2 | `TestAcquireBackend_deletedSecretFails`, `_emptiedSecretFails`, `_observedChangeRetiresOnFailedRead`; *controls* `_restoredSecretRecovers`, `_cancelledReadKeepsEntry` | named, transient error; retire on not-found, empty or observed change only; recovery without sink change | pass | evidence/G2.md (2026-10-19); also `_caGoneBehindFailedCredentialReadRetires`, `_emptiedCASecretIsRetryable` |
| SCR-2 | `TestS3_emptiedSecretNoAmbientCredentials` | fails, factory not called | pass | evidence/G2.md (2026-10-19) |
| SCR-2 | `TestCleanup_streamSinkDeletedSecretRetained`, `TestStreamSinkTypes_matchRetainingBackends`; *control* `TestCleanup_deletedGitAuthSecretFails` | stream retained without a read; set matches backends; Git release fails naming the Secret | pass | evidence/G2.md (2026-10-19) |
| SCR-2 | envtest spec "deleted secret fails the export with a redacted status" | condition names the Secret; recovers after recreate | pass | evidence/G2.md (2026-10-19; green on G1 too) |
| SCR-2 | mutations: no retire on not-found; retire on every error | deleted / cancelled tests fail | pass | evidence/G2.md (2026-10-19) (M1, M3; also M2, M5-M8, M13-M15, M0 control) |
| SCR-3 | `TestAcquireBackend_rotationDuringExportDefersClose`, `_specChange…`, `_deleteEviction…`, `_ttlSkipsHeldEntry` | close after the holder's release | pass | evidence/G3.md (2026-10-20); also `TestEvictBackendPool_heldEntryDrains`, `TestResetBackendPoolForTest_lateReleaseDoesNotCloseAgain` |
| SCR-3 | `TestAcquireBackend_slowBuilderDoesNotReplaceNewer`, `_slowBuilderAfterRetireNotPooled`, `_sameKeyBuildersShare` | newer or retired entry not overwritten; same-key builders share | pass | evidence/G3.md (2026-10-20); also `_agedGenerationDoesNotAdmitOldBuild` |
| SCR-3 | stale reads: `_staleReadDoesNotRetireNewer`, `_staleNotFoundDoesNotRetireNewer`, `_observedChangeRetiresEntryStoredDuringRead`, `_failedReReadKeepsObservedChange` | a read older than the entry does not retire it; an observed change still retires | pass | evidence/G3.md (2026-10-20; G2 deferral; the last test's fix reviewed at stage B round 1) |
| SCR-3 | `TestAcquireBackend_concurrentAcquiresAfterRotation` `-race -count=10` | each backend closed exactly once | pass | evidence/G3.md (2026-10-20; green on G2 too, killed by M5, M9) |
| SCR-3 | `TestAcquireBackend_failedRebuildRetiresOld`, `TestRelease_idempotent` | no stale fallback; single decrement | pass | evidence/G3.md (2026-10-20) |
| SCR-3 | BEP-1 tests unchanged | pass | pass | evidence/G3.md (2026-10-20) |
| SCR-3 | mutations: close at retire regardless of holders; no fence | defer / slow-builder tests fail | pass | evidence/G3.md (2026-10-20) (M1, M2; also M3-M16, M0 control) |
| SCR-4 | `TestRotationErrors_noSentinel`, envtest status/Event assertions | no sentinel in errors this change adds, logs, status, Events | pass | evidence/G2.md (2026-10-19) |
| SCR-4 | `TestSecretMeta_metadataOnly` | key from metadata only; a hit reads no values | pass (key and type; G1) | evidence/G1.md (2026-10-19) |
| SCR-5 | `TestResolveSinkSecretData_emptySecretFails` (4 rows) | CLI fails before build, operator's text; `stringData` and env rows pass | pass | evidence/G2.md (2026-10-19) (5 rows; plus `_emptySecretFailsBeforeTheBackend`) |
| SCR-5 | *control* `TestRunOneContext_secretManifestChangeNextRun` | second run uses the new value | pass | evidence/G2.md (2026-10-19) |
| SCR-2 | `TestSecretMeta_notFoundIsTransient` | metadata NotFound becomes the plain, transient not-found error | pass | evidence/G2.md (2026-10-19; green on G1 too) |
| SCR-2 | mutation: metadata NotFound returned as is | not-found and deleted tests fail | pass | evidence/G2.md (2026-10-19) (M4a, M4b) |
| SCR-7 | `TestSetInventoryDegraded_zeroIntervalRequeues`, `TestClusterSetDegraded_zeroIntervalRequeues` (controls: 30s, unset, 5m) | `0s` requeues at the watchdog; other intervals unchanged | pass | evidence/G2.md (2026-10-19) |
| SCR-7 | *control* `TestApplyInventoryExportOutcome_zeroIntervalTerminalNoRequeue` | terminal under `0s` is not requeued | pass | evidence/G2.md (2026-10-19); cluster twin `TestClusterReconcile_zeroIntervalTerminalNoRequeue`, tasks 6.4 |
| SCR-7 | envtest specs "zero-interval inventory retries a deleted secret", *control* "unchanged zero-interval inventory is not re-exported" | retried and recovers; no re-export of an unchanged `0s` inventory | pass | evidence/G2.md (2026-10-19) |
| SCR-7 | mutations: keep `exportDebounce`; watchdog for terminal; no interval-0 skip | zero-interval / terminal / not-re-exported tests fail | pass | evidence/G2.md (2026-10-19) (M9, M9e, M10, M11, M12) |
| SCR-2 | `TestAcquireBackend_fullReadNotFoundFencesBuildInFlight`, `_fullReadNotFoundRetiresEntryStoredMeanwhile`, `_fullReadCAGoneFencesBuildInFlight` | a gone Secret seen by the full read retires the key and fences builds in flight | pass | tasks 6.2 (2026-10-20) |
| SCR-3 | `TestAcquireBackend_failedReReadAfterStaleReadDrainsNewer` | a spurious retire drains and rebuilds from the current value | pass | tasks 6.5 (2026-10-20) |
| SCR-7 | `TestPerSinkCoalesceTracker_zeroIntervalSkipsKeepStateAlive` | an unchanged `0s` binding is still skipped after two state TTLs of watchdog reconciles | pass | tasks 6.3 (2026-10-20) |
| SCR-6 | `rg -U 'keeps its\s+CA' docs`; `task lint:markdown` | no match; 0 issues | pass | evidence/G4.md (2026-10-20; also docs guards and `task docs:verify`) |
| all | `task check` on the branch head; CI on the PR head | green | not-run | |
| all | independent branch review | APPROVE recorded with revision | not-run | |
