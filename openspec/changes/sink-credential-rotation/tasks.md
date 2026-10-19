# Tasks

Red first. In each group the test task comes before the implementation and records its red in
`evidence/G<N>.md`: the named test runs (`=== RUN <name>`) and fails on the target assertion, not on
a compile error, a missing Docker daemon or a skip. A stub with the signature from `design.md` and a
`not implemented` body is in scope so the tests compile. Each group closes in one commit (test and
implementation), so the tree is green at every commit and *Known red* stays empty unless a group is
split.

Each case below reads: given / when / then · the failure it exposes.

Test rules for every group:

- Unit tests in `internal/sink` use the controller-runtime fake client and a recording fake
  registry (a factory that records the `BuildContext` it was given and returns a fake backend
  with a `Close` counter and an `Export` that can block on a channel). They call
  `EnableBackendPoolForTest` and restore the previous state in `t.Cleanup`.
- The envtest cases follow `internal/controller/family_sink_delete_watch_envtest_test.go`, which
  turns the pool on for its spec (the suite turns it off, `internal/controller/suite_test.go:31`),
  and run the inventory controller against a real API server with a recording registry, so the
  Secret is read through the manager's cached client as in production.
- Container-backed tests carry `//go:build integration` as their first line, live under
  `INTEGRATION_PACKAGES` (`Taskfile.yml`), and handle a missing Docker daemon like
  `internal/sink/postgres/export_integration_test.go:34-45`. Without Docker they skip; that is
  neither red nor green and leaves the row `blocked` until CI `test-integration` records it.
  Run them with `go test -tags integration -v -run '<names>' <pkg>` and quote the `=== RUN` lines.
- Secret values in every test are the sentinels `sentinel-pass`, `sentinel-token`, `sentinel-key`
  (or values containing them), so SCR-4 assertions can search for them.
- Each group records `go test -race -count=3` on the packages it changed.

## 0. Probe

- [ ] 0.1 Verify the load-bearing assumptions in `proposal.md` and record each as verified
  (command, versions, date) in `evidence/T0.1.md`; a failed one stops the loop as a spec finding:
  - both inventory reconcilers pass the manager's client to `RunExportEnvelope` and the cleanup
    path, and a `Get` of a Secret through it is served from an informer (envtest: count API
    requests, or check `mgr.GetCache().GetInformer` for `corev1.Secret` after one export);
  - envtest: a no-op Secret update keeps `resourceVersion`; a data change and a label change each
    change it; delete-and-recreate changes the UID;
  - the coalesce skip ends when the export interval is due
    (`internal/controller/kollectinventory_controller.go:360-368`), so an unchanged inventory
    still exports once per interval;
  - for pgx v5.11.0 `Pool.Close`, mongo-driver v1.17.10 `Client.Disconnect`, kafka-go v0.4.51
    `Writer.Close`, nats.go v1.54.0 `Conn.Close` and BigQuery v1.85.0 `Client.Close`: what an
    operation in flight sees when Close runs (read the source, cite file:line);
  - the Git mirror work directory is keyed by sink and survives a backend rebuild, and Git auth is
    read from the backend per call (`internal/sink/git`), so a rebuilt Git backend pushes with the
    new credentials;
  - an S3 `secretRef` Secret with no access key builds a client on the SDK default chain today
    (`internal/sink/s3/config.go:65-78`, `backend.go:159-163`);
  - a missing Secret's acquire error is classified transient on the export path and the
    inventory requeues (`internal/sink/export.go:96-108`, `internal/errors/errors.go:86`).

## 1. Rotation rebuilds the backend (SCR-1, SCR-4 fingerprint)

- [ ] 1.1 Tests (red on base):
  - `TestAcquireBackend_rotatedSecretRebuilds` (table, one row per reference: `secretRef`,
    `git.auth.secretRef`, `kafka.secretRef`, `nats.secretRef`, `postgres.databaseRef`,
    `mongodb.databaseRef`, `bigquery.secretRef`, `tls.caSecretRef`): given a pooled backend built
    from value A / when the Secret's data changes to B and acquire runs / then the factory ran
    twice, the second `BuildContext` holds B, and the first backend's `Close` ran once · exposes a
    key that ignores Secrets (base: one build, A reused).
  - `TestAcquireBackend_rotateBack`: A, B, A with an acquire after each / three builds, each with
    the current value · exposes a cache keyed on content that skips the return to A.
  - `TestAcquireBackend_unchangedSecretsReuse` and `TestAcquireBackend_unrelatedSecretReuse`:
    two acquires, nothing referenced changed (or an unreferenced Secret changed) / one build, same
    instance · the no-op control: must pass on base and after the change; exposes an
    over-eager key.
  - `TestReferencedSecretRefs_coversEverySecretReferenceField`: reflection over
    `KollectSinkSpec` finds every `*SecretReference` field; for each, a spec of the type that
    reads it lists that Secret · exposes a reference field the fingerprint misses.
  - `TestSecretFingerprint_metadataOnly`: the fingerprint function's input type carries no data;
    two inputs with equal metadata give equal digests, and a changed UID or `resourceVersion`
    changes it · exposes a key derived from values.
  - envtest spec (Ginkgo `It`, as in the delete-watch spec) "secret rotation takes effect on the next export": given an inventory bound to a
    snapshot sink whose recording backend saw `sentinel-token-1` / when the Secret is updated to
    `sentinel-token-2` and the next export runs / then the export's `BuildContext` holds the new
    value · exposes the production path (cached client, controller) not reaching the new key.
  - integration `TestPostgresExport_passwordRotation` (`internal/sink/postgres` or
    `internal/sink`, testcontainers Postgres): export as role `u1`; switch the `databaseRef`
    Secret to role `u2`, `ALTER ROLE u1 NOLOGIN`; export again / rows of the second export carry
    `current_user = u2` (a column default or `pg_stat_activity`), and after release
    `pg_stat_activity` shows no `u1` session from the test process · exposes a pool that keeps
    authenticating with the old role (base: export succeeds as `u1` on the idle connection).
- [ ] 1.2 Implement `referencedSecretRefs`, `secretMeta`, the fingerprint and the
  `BuildContextFromSpec` rewrite (design: *Referenced Secrets, one list*, *Fingerprint*), and the
  info log on a Secret-driven rebuild. Mutations, judged by exit status: drop the secret part from
  the fingerprint (the rotation tests fail); drop one case from `referencedSecretRefs` (the
  reflection test fails); no-op control: reorder two independent statements in `acquireBackend`
  (all pass).

## 2. Deleted and emptied Secrets fail (SCR-2, SCR-4, SCR-5)

- [ ] 2.1 Tests (red on base):
  - `TestAcquireBackend_deletedSecretFails` (rows: `secretRef`, `databaseRef`, `caSecretRef`,
    `git.auth.secretRef`): pooled backend, Secret deleted, acquire / error names Secret and
    namespace, `kollecterrors.ClassOf` is transient, the entry is gone and its `Close` ran once,
    the factory did not run · exposes an export on cached credentials (base: acquire succeeds).
  - `TestAcquireBackend_emptiedSecretFails`: rows for `Data == nil`, `Data == {}` and all values
    empty / error `has no data` naming the Secret, transient, entry retired · exposes an empty
    Secret accepted.
  - `TestAcquireBackend_restoredSecretRecovers`: delete, acquire fails, recreate, acquire / new
    build succeeds with the new value; no sink change · exposes a sticky failure.
  - `TestResolveSecret_emptyIsError` (pool disabled, the probe path) · exposes a check that lives
    only in the pool.
  - `TestS3_emptiedSecretNoAmbientCredentials`: `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` set
    in the test environment, S3 sink Secret emptied, acquire / fails naming the Secret, factory
    not called · exposes the fall-through to the default chain.
  - `TestCleanup_deletedGitAuthSecretFails`: Retain release for a Git sink whose auth Secret was
    deleted / `RunCleanup` returns the naming error, not `CleanupCleaned` with success · exposes a
    release on cached credentials.
  - `TestRotationErrors_noSentinel`: through rotate, empty, delete, every returned error and
    every line captured from the `sink` logger (a test logr sink) contains none of the sentinels.
  - envtest spec "deleted secret fails the export with a redacted status": delete the sink Secret / the
    inventory's export condition names the Secret, the recorded Events and status contain no
    sentinel, and after recreating the Secret the next retry exports with no sink change ·
    exposes the controller path (condition text, requeue) and SCR-4 on status and Events.
  - CLI `TestResolveSinkSecretData_emptySecretFails` and
    `TestRunOneContext_secretManifestChangeNextRun` (`internal/pipeline`): an empty Secret
    manifest fails before `registry.NewBackend` with the operator's message text; a second run
    with a changed manifest builds from the new value (expected green on base; recorded as the
    parity control, not a red).
- [ ] 2.2 Implement the empty check in `ResolveSecret` and `resolveSecretValues`, the not-found and
  empty errors in `secretMeta`, and retire-on-acquire-error (design: *Fingerprint*). Mutations:
  skip the retire on error (deleted-Secret test fails); accept an empty map (emptied test fails);
  no-op control as in 1.2.

## 3. Retired backends drain (SCR-3)

- [ ] 3.1 Tests (red on base):
  - `TestAcquireBackend_rotationDuringExportDefersClose`: E1 acquires B1 and blocks in `Export`;
    Secret changes; E2 acquires / E2 gets B2, B1's `Close` count is 0 until E1's release, then 1;
    E1's `Export` returns nil · exposes the immediate close under an export (base: `Close` runs
    on E2's acquire, `backend_pool.go:156-158`).
  - `TestAcquireBackend_specChangeDuringExportDefersClose`: same with a spec change.
  - `TestAcquireBackend_deleteEvictionDuringExportDefersClose`: `EvictBackendPoolForSink` while
    E1 holds B1 / entry gone, tombstone set, no rebuild on the next acquire for that UID, B1
    closed after E1 releases.
  - `TestAcquireBackend_concurrentAcquiresAfterRotation`: 16 goroutines acquire after one Secret
    change / all get one new backend; each surplus build and B1 closed exactly once; run with
    `-race -count=10` · exposes double close or a leaked surplus build.
  - `TestAcquireBackend_failedRebuildRetiresOld`: Secret changes to a value the fake factory
    rejects / acquire fails, B1 retired and closed, a later acquire with a valid value builds B2,
    never returns B1.
  - `TestRelease_idempotent`: calling a release twice decrements once.
  - Existing BEP-1 tests (`internal/sink/backend_pool_delete_hook_test.go`) stay green unchanged;
    any change to them is a *Test changes* row with a reason.
- [ ] 3.2 Implement holders, `retireLocked` and the release (design: *Pool entries with
  holders*). Mutations: close at retire regardless of holders (3.1 defer tests fail); omit the
  `sync.Once` (idempotent test fails); no-op control as in 1.2. `go test -race -count=10
  ./internal/sink/`.

## 4. Docs (SCR-6)

- [ ] 4.1 Update `docs/crds/kollectsnapshotsink.md`, `kollectdatabasesink.md`,
  `kollecteventsink.md` (rotation paragraph; remove "keeps its CA until the sink spec changes"),
  `docs/security/security-architecture.md` (rotation sentence at the Secrets section),
  `docs/operator-manual/troubleshooting.md` (rows for a deleted or emptied Secret, and the Git auth
  row's "rotate secretRef" now takes effect on the next export), and
  `docs/operator-manual/upgrading.md` (new section "Sink Secret rotation (after v0.21.0)" with the
  design compatibility table, the Retain/`force-cleanup` note, and the CA-section sentence
  removed). Checks: `rg -n 'keeps its CA until the sink spec' docs` finds nothing;
  `task lint:markdown` 0 issues; `task docs:verify` where it runs (see AGENTS.md gotcha).

## 5. Integration

- [ ] 5.1 Integrator: `task check` on the branch head; CI `test-integration` names
  `TestPostgresExport_passwordRotation` as run and passed; independent branch review recorded
  (reviewer, revision, verdict); then archive the change as the last commit and record both
  revisions.

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| SCR-1 | `TestAcquireBackend_rotatedSecretRebuilds` (8 rows), `TestAcquireBackend_rotateBack` | rebuild with the current value per reference kind | not-run | |
| SCR-1 | `TestAcquireBackend_unchangedSecretsReuse`, `TestAcquireBackend_unrelatedSecretReuse` | one build, same instance (no-op control) | not-run | |
| SCR-1 | `TestReferencedSecretRefs_coversEverySecretReferenceField` | every `*SecretReference` field covered | not-run | |
| SCR-1 | envtest spec "secret rotation takes effect on the next export" | next export sees the new value | not-run | |
| SCR-1 | integration `TestPostgresExport_passwordRotation` | rows as `u2`, no `u1` session left | not-run | needs Docker (CI `test-integration`) |
| SCR-1 | mutation: secret part dropped from the fingerprint | rotation tests fail | not-run | |
| SCR-2 | `TestAcquireBackend_deletedSecretFails`, `_emptiedSecretFails`, `_restoredSecretRecovers` | named, transient error; entry retired; recovery without sink change | not-run | |
| SCR-2 | `TestS3_emptiedSecretNoAmbientCredentials` | fails, factory not called | not-run | |
| SCR-2 | `TestCleanup_deletedGitAuthSecretFails` | release fails naming the Secret | not-run | |
| SCR-2 | envtest spec "deleted secret fails the export with a redacted status" | condition names the Secret; recovers after recreate | not-run | |
| SCR-2 | mutation: no retire on acquire error | deleted-Secret test fails | not-run | |
| SCR-3 | `TestAcquireBackend_rotationDuringExportDefersClose`, `_specChange…`, `_deleteEviction…` | close after the holder's release | not-run | |
| SCR-3 | `TestAcquireBackend_concurrentAcquiresAfterRotation` `-race -count=10` | each backend closed exactly once | not-run | |
| SCR-3 | `TestAcquireBackend_failedRebuildRetiresOld`, `TestRelease_idempotent` | no stale fallback; single decrement | not-run | |
| SCR-3 | BEP-1 tests unchanged | pass | not-run | |
| SCR-3 | mutation: close at retire regardless of holders | defer tests fail | not-run | |
| SCR-4 | `TestRotationErrors_noSentinel`, envtest status/Event assertions | no sentinel anywhere | not-run | |
| SCR-4 | `TestSecretFingerprint_metadataOnly` | digest from metadata only | not-run | |
| SCR-5 | `TestResolveSinkSecretData_emptySecretFails` | CLI fails before build, operator's text | not-run | |
| SCR-5 | `TestRunOneContext_secretManifestChangeNextRun` | second run uses the new value (parity control) | not-run | |
| SCR-6 | `rg -n 'keeps its CA until the sink spec' docs`; `task lint:markdown` | no match; 0 issues | not-run | |
| all | `task check` on the branch head; CI on the PR head | green | not-run | |
| all | independent branch review | APPROVE recorded with revision | not-run | |
