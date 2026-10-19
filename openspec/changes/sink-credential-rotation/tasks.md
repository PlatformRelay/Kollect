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

- [ ] 0.1 Verify the load-bearing assumptions in `proposal.md` and record each as verified
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

## 1. Rotation rebuilds the backend (SCR-1, SCR-4 key)

- [ ] 1.1 Tests (red on `origin/main`):
  - `TestAcquireBackend_rotatedSecretRebuilds` (table, one row per reference: `secretRef`,
    `git.auth.secretRef`, `kafka.secretRef`, `nats.secretRef`, `postgres.databaseRef`,
    `mongodb.databaseRef`, `bigquery.secretRef`, `tls.caSecretRef`; each row once with the
    reference's namespace omitted and once explicit): given a pooled backend built from value A /
    when the Secret's data changes to B and acquire runs / then the factory ran twice, the second
    `BuildContext` holds B, and after release the first backend's `Close` ran once · exposes a key
    that ignores Secrets (base: one build, A reused).
  - `TestAcquireBackend_readsSecretsUncached`: `Client` is a fake holding the stale Secret A,
    `SecretReader` a fake holding B / acquire builds from B · exposes reads through the cache.
  - `TestAcquireBackend_rotateBack`: A, B, A with an acquire after each / three builds, each with
    the current value · exposes a key on content that skips the return to A.
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
    the second export's rows carry `current_user = u2` (a column default), and after release
    `pg_stat_activity` shows no `u1` session · exposes a pool that keeps authenticating as `u1`
    on an idle connection (base: export succeeds as `u1`).
- [ ] 1.2 Implement `referencedSecretRefs`, `secretMeta` (metadata GET through `SecretReader`;
  not-found keeps today's text), the key, the rebuild from the full objects, the `SecretReader`
  field and its wiring in both inventory reconcilers and `cmd/main.go`, the `BuildContextFromSpec`
  split, and the info log (design: *Readers*, *Referenced Secrets, one list*, *Fingerprint and
  acquire* steps 1-3). The store fence (step 4) is G3; the empty check and retire rules are G2.
  Mutations, judged by exit status: drop the Secret part from the key (rotation tests fail); read
  through `Client` instead of `SecretReader` (uncached test fails); drop one case from
  `referencedSecretRefs` (reflection test fails); no-op control: reorder two independent
  statements in `acquireBackend` (all pass).

## 2. Deleted and emptied Secrets fail (SCR-2, SCR-4, SCR-5)

- [ ] 2.1 Tests (red on the G1 commit):
  - `TestAcquireBackend_deletedSecretFails` (rows: `secretRef`, `databaseRef`, `caSecretRef`,
    `git.auth.secretRef`): pooled backend, Secret deleted, acquire / error names Secret and
    namespace, `kollecterrors.ClassOf` is transient, the entry is gone and its `Close` ran once,
    the factory did not run · exposes a pooled entry left in place after a delete.
  - `TestAcquireBackend_emptiedSecretFails`: rows for `Data == nil`, `Data == {}` and all values
    empty / error `has no data` naming the Secret, transient, entry retired · exposes an empty
    Secret accepted (on G1 it rebuilds with empty values).
  - `TestAcquireBackend_restoredSecretRecovers`: delete, acquire fails, recreate, acquire / new
    build succeeds with the new value; no sink change · exposes a sticky failure.
  - `TestAcquireBackend_cancelledReadKeepsEntry`: the reader returns `context.Canceled` for one
    acquire / that acquire fails, the entry stays pooled, the next acquire is a hit · exposes
    retire on any error.
  - `TestResolveSecret_emptyIsError` (pool disabled, the probe path) · exposes a check that lives
    only in the pool.
  - `TestS3_emptiedSecretNoAmbientCredentials`: `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` set in
    the test environment, S3 sink Secret emptied, acquire / fails naming the Secret, factory not
    called · exposes the fall-through to the default chain.
  - `TestCleanup_streamSinkDeletedSecretRetained` (Kafka and NATS rows): Secret deleted,
    `RunCleanupExport` / `CleanupRetained`, nil error, no Secret read and no factory call ·
    exposes a deletion blocked by a Secret the cleanup never needs (on G1: acquire fails).
  - `TestCleanup_deletedGitAuthSecretFails`: Retain release for a Git sink whose auth Secret was
    deleted / `RunCleanupExport` returns the naming error, not success · exposes a release on the
    pooled backend.
  - `TestRotationErrors_noSentinel`: through rotate, empty and delete, every error this change
    adds and every line captured from the `sink` logger (a test logr sink) contains none of the
    sentinels.
  - envtest spec "deleted secret fails the export with a redacted status": delete the sink Secret /
    the inventory's export condition names the Secret, the recorded Events and status contain no
    sentinel, and after recreating the Secret the next retry exports with no sink change · exposes
    the controller path (condition text, requeue) and SCR-4 on status and Events.
  - CLI `TestResolveSinkSecretData_emptySecretFails` (`internal/pipeline`; rows: no `data` and no
    `stringData`; `data` keys all empty; `stringData` only, non-empty, *control*; a value
    `${env:VAR}` with `VAR` set, *control*): an empty Secret fails before `registry.NewBackend`
    with the operator's message text · exposes a missing CLI parity.
  - *control* `TestRunOneContext_secretManifestChangeNextRun`: a second run with a changed manifest
    builds from the new value.
- [ ] 2.2 Implement the empty check in `ResolveSecret`, the rebuild path and
  `resolveSecretValues`; retire only on not-found, empty, or a failed rebuild after a key change;
  the stream retain before the acquire in `RunCleanupExport` (design: *Fingerprint and acquire*
  errors, *Cleanup before acquire*). Mutations: skip the retire on not-found (deleted test fails);
  accept an empty map (emptied test fails); retire on every error (cancelled test fails); no-op
  control as in 1.2.

## 3. Retired backends drain (SCR-3)

- [ ] 3.1 Tests (red on the G2 commit, where a key change closes the old backend at once,
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
  - `TestAcquireBackend_concurrentAcquiresAfterRotation`: 16 goroutines acquire after one Secret
    change, with a barrier between the metadata read and the store / all hold one new backend;
    each surplus build and B1 closed exactly once; run with `-race -count=10` · exposes double
    close or a leaked surplus build.
  - `TestAcquireBackend_failedRebuildRetiresOld`: Secret changes to a value the fake factory
    rejects / acquire fails, B1 retired and closed after its holders release, a later acquire with
    a valid value builds B2 and never returns B1.
  - `TestRelease_idempotent`: calling a release twice decrements once.
  - Existing BEP-1 tests (`internal/sink/backend_pool_delete_hook_test.go`) stay green unchanged;
    any change to them is a *Test changes* row with a reason.
- [ ] 3.2 Implement holders, `retireLocked`, the release and the store fence (design: *Pool
  entries with holders*, *Fingerprint and acquire* step 4). Mutations: close at retire regardless
  of holders (defer tests fail); drop the fence (slow-builder test fails); omit the `sync.Once`
  (idempotent test fails); no-op control as in 1.2. `go test -race -count=10 ./internal/sink/`.

## 4. Docs (SCR-6)

- [ ] 4.1 Update `docs/crds/kollectsnapshotsink.md` and `kollectdatabasesink.md` (rotation
  paragraph; remove "A backend that already built keeps its CA until the sink spec changes" at
  `kollectsnapshotsink.md:53-54` and `kollectdatabasesink.md:70-71`), `kollecteventsink.md` (a
  rotation paragraph), `docs/security/security-architecture.md` (rotation sentence at the Secrets
  section, `:214-216`), `docs/operator-manual/troubleshooting.md` (rows for a deleted or emptied
  Secret; the Git auth row's "rotate secretRef" now takes effect on the next export, `:204`), and
  `docs/operator-manual/upgrading.md` (new section "Sink Secret rotation (after v0.21.0)" with the
  design compatibility table, "entirely empty" stated for the S3/GCS fallback, the Retain and
  `force-cleanup` note, and the CA-section sentence at `:167-168` removed). Checks:
  `rg -U 'keeps its\s+CA' docs` finds nothing; `task lint:markdown` 0 issues; `task docs:verify`
  where it runs (see AGENTS.md gotcha).

## 5. Integration

- [ ] 5.1 Integrator: `task check` on the branch head; CI `test-integration` names
  `TestPostgresExport_passwordRotation` and `TestS3Export_caRotation` as run and passed;
  independent branch review recorded (reviewer, revision, verdict); then archive the change as the
  last commit and record both revisions.

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| SCR-1 | `TestAcquireBackend_rotatedSecretRebuilds` (8 references × 2 namespace forms), `_rotateBack` | rebuild with the current value per reference kind | not-run | |
| SCR-1 | `TestAcquireBackend_readsSecretsUncached`, `_keyFromBuildSnapshot` | reads bypass the cache; key and values from one read | not-run | |
| SCR-1 | *control* `TestAcquireBackend_unchangedSecretsReuse`, `_unrelatedSecretReuse` | one build, same instance | not-run | |
| SCR-1 | `TestReferencedSecretRefs_coversEverySecretReferenceField` | every `*SecretReference` field covered | not-run | |
| SCR-1 | envtest specs "secret rotation takes effect on the next export", "cross-namespace secret under a restricted cache" | next export sees the new value; no cache error | not-run | |
| SCR-1 | integration `TestS3Export_caRotation` | handshake fails on CA X, succeeds after rotation to CA Y | not-run | |
| SCR-1 | integration `TestPostgresExport_passwordRotation` | rows as `u2`, no `u1` session left | not-run | needs Docker (CI `test-integration`) |
| SCR-1 | mutations: Secret part dropped from the key; reads through `Client` | rotation / uncached tests fail | not-run | |
| SCR-2 | `TestAcquireBackend_deletedSecretFails`, `_emptiedSecretFails`, `_restoredSecretRecovers`, `_cancelledReadKeepsEntry` | named, transient error; entry retired only on not-found or empty; recovery without sink change | not-run | |
| SCR-2 | `TestS3_emptiedSecretNoAmbientCredentials` | fails, factory not called | not-run | |
| SCR-2 | `TestCleanup_streamSinkDeletedSecretRetained`, `TestCleanup_deletedGitAuthSecretFails` | stream retained without a read; Git release fails naming the Secret | not-run | |
| SCR-2 | envtest spec "deleted secret fails the export with a redacted status" | condition names the Secret; recovers after recreate | not-run | |
| SCR-2 | mutations: no retire on not-found; retire on every error | deleted / cancelled tests fail | not-run | |
| SCR-3 | `TestAcquireBackend_rotationDuringExportDefersClose`, `_specChange…`, `_deleteEviction…`, `_ttlSkipsHeldEntry` | close after the holder's release | not-run | |
| SCR-3 | `TestAcquireBackend_slowBuilderDoesNotReplaceNewer` | newer entry stays pooled | not-run | |
| SCR-3 | `TestAcquireBackend_concurrentAcquiresAfterRotation` `-race -count=10` | each backend closed exactly once | not-run | |
| SCR-3 | `TestAcquireBackend_failedRebuildRetiresOld`, `TestRelease_idempotent` | no stale fallback; single decrement | not-run | |
| SCR-3 | BEP-1 tests unchanged | pass | not-run | |
| SCR-3 | mutations: close at retire regardless of holders; no fence | defer / slow-builder tests fail | not-run | |
| SCR-4 | `TestRotationErrors_noSentinel`, envtest status/Event assertions | no sentinel in errors this change adds, logs, status, Events | not-run | |
| SCR-4 | `TestSecretMeta_metadataOnly` | key from metadata only; a hit reads no values | not-run | |
| SCR-5 | `TestResolveSinkSecretData_emptySecretFails` (4 rows) | CLI fails before build, operator's text; `stringData` and env rows pass | not-run | |
| SCR-5 | *control* `TestRunOneContext_secretManifestChangeNextRun` | second run uses the new value | not-run | |
| SCR-6 | `rg -U 'keeps its\s+CA' docs`; `task lint:markdown` | no match; 0 issues | not-run | |
| all | `task check` on the branch head; CI on the PR head | green | not-run | |
| all | independent branch review | APPROVE recorded with revision | not-run | |
