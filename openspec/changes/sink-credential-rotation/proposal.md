# Proposal

## Why

The operator caches one built backend per sink in a process-wide pool and keys the cache on the
sink spec alone (backlog SECRET-ROTATION-01, release inventory 2026-10-19 items 12-13):

- `acquireBackend` (`internal/sink/backend_pool.go:94-166`) reuses a pooled entry when
  `specFingerprint(spec)` matches (`:136`, hash at `:263-272`). The fingerprint is the JSON of the
  spec; no Secret the spec references takes part in it.
- Credential and CA Secrets are read only when a backend is built: `BuildContextFromSpec`
  (`internal/sink/build_context.go:28-62`) runs at `backend_pool.go:145`, after a cache miss.
- Every hit refreshes `lastUsed` (`:137`), so the idle TTL (`backendPoolTTL`, 48 h, `:43`) never
  ages out an active sink.

So a rotated password, token, access key or CA keeps the old value in an active sink until the
sink spec changes, the sink is deleted (evict-on-delete, PR #455, BEP-1), the pod restarts, or the
sink sits idle for 48 h. The reverse is worse: deleting or emptying a credential Secret does not
stop exports, because the cached client keeps authenticating with what it read at build. The CA
Secret of the merged `sink-tls-trust` capability (`tls.caSecretRef`, resolved at
`build_context.go:215-243`) has the same gap; its docs say so
(`docs/crds/kollectsnapshotsink.md:53`, `docs/crds/kollectdatabasesink.md:70`,
`docs/operator-manual/upgrading.md:167`).

Two further defects sit on the same path:

- When the spec hash changes, `storePooledBackend` closes the superseded backend at once
  (`backend_pool.go:208-214`, closed by the caller at `:156-158`), although a concurrent export
  that acquired it earlier may still be using it: pooled acquires return a no-op release
  (`:141`, `:165`). A rotation-driven rebuild would make that race routine.
- An emptied Secret is not an error. `ResolveSecret` (`internal/sink/credentials.go:27-69`)
  returns whatever data it finds, and an S3 or GCS sink whose Secret has no access key falls
  through to the AWS SDK's default credential chain (`internal/sink/s3/config.go:65-78`,
  `internal/sink/s3/backend.go:159-163`), that is the pod's ambient identity.

## What Changes

- SCR-1: the pool key covers every Secret the build reads. On each acquire the pool reads the
  referenced Secrets' metadata (namespace, name, UID, `resourceVersion`) through the manager's
  cached client and folds it into the fingerprint. A changed Secret (credentials `secretRef`,
  `git.auth.secretRef`, `kafka.secretRef`, `nats.secretRef`, `postgres.databaseRef`,
  `mongodb.databaseRef`, `bigquery.secretRef`, `tls.caSecretRef`) retires the pooled backend, and
  the export that noticed builds a new one from the current values. One function enumerates the
  references, and both the build and the fingerprint use it.
- SCR-2: a deleted referenced Secret, or one whose data holds no non-empty value, fails the
  acquire with an error that names the Secret and its namespace, and retires the pooled backend,
  so the export fails instead of running on cached credentials. The error is retryable (as a
  missing Secret is today), so restoring the Secret recovers without touching the sink. The
  empty-Secret rule lives in the shared resolver and applies to the CLI too.
- SCR-3: retiring a pooled backend (rotation, spec change, delete eviction, TTL) never closes it
  under an export in flight. Pooled entries count their holders; a retired entry is removed from
  the pool at once (no new acquire gets it) and closed exactly once, when its last holder
  releases. A rebuild that fails after a Secret change still retires the old entry: no fallback to
  stale credentials.
- SCR-4: no credential value reaches a log line, Event, status condition or returned error on
  these paths; the fingerprint and pool hold no Secret data.
- SCR-5: operator and CLI behave the same where it applies (the empty-Secret rule); the CLI is
  otherwise unaffected, because `kollect-pipeline` builds one backend per context per run from the
  config directory and closes it (`internal/pipeline/wire.go:515-535`) and never uses the pool.
- SCR-6: docs and a v0.22.0 upgrade note: rotation takes effect on the next export, a deleted or
  emptied Secret now stops exports, S3/GCS no longer fall back to ambient credentials when their
  Secret is emptied, and a Git sink whose credentials were rotated away now keeps a Retain release
  in `Terminating` until the Secret is fixed (`kollect.dev/force-cleanup` stays the escape hatch).

## Capabilities

### New Capabilities

- `sink-secret-rotation`: how a pooled sink backend follows changes to the Secrets its sink
  references, and how a retired backend is drained.

### Modified Capabilities

None. BEP-1 (evict-on-delete, unarchived change `2026-10-06-product-decision-convergence`) stays
true: a deleted sink's entry leaves the pool at once and no acquire rebuilds it; its Close now
waits for an export that already holds the backend, which BEP-1 allowed to fail.
`pipeline-sink-credentials` (CSC-3) already fails a missing Secret; SCR-5 adds the empty case.

## Impact

- Entry points: `KollectInventory` and `KollectClusterInventory` exports through
  `RunExportEnvelope` (`internal/sink/export.go:89`) and the Retain/Delete cleanup through
  `internal/sink/cleanup.go:184`, both on `acquireBackend`, for every family sink
  (`KollectSnapshotSink`, `KollectDatabaseSink`, `KollectEventSink`) and the legacy
  `KollectSink`. The connection probes (`internal/controller/family_sink_connection.go:49`,
  `internal/controller/kollectconnectiontest_controller.go:91`) build a fresh backend per run and
  are unchanged. `kollect-pipeline collect` gains only the empty-Secret error.
- Code: `internal/sink/backend_pool.go`, `build_context.go`, `credentials.go`, `export.go`,
  `cleanup.go` (release handling), `internal/pipeline/wire.go` (empty rule). No CRD, RBAC or
  dependency change: the manager already has `get/list/watch` on Secrets
  (`config/rbac/role.yaml:6-15`, marker `internal/controller/family_sink_controller.go:23`).
- Behaviour change, release-noted for v0.22.0 (see SCR-6).
- Tests: unit tests in `internal/sink` and `internal/pipeline`; envtest in `internal/controller`
  for the Secret update and delete path through the inventory controller; a Postgres integration
  test (testcontainers) for a real long-lived pool across a rotation.

## Non-goals

- Watching Secrets. No controller gains a Secret watch, and a Secret change does not by itself
  trigger an export, a probe or a reconcile. Rotation takes effect on the sink's next export
  (bounded by the inventory's export interval, at most `validation.MaxExportInterval`, 24 h), and
  an idle pooled backend keeps its old connections until then or until the TTL. Design records the
  alternative.
- Recovering a terminal error on a Secret fix. A terminal export error does not requeue
  (`internal/controller/kollectinventory_controller.go:262-264`); after fixing a Secret that
  caused one (for example a CA Secret with no CA key), the docs keep the "touch the sink" step.
  Missing and emptied Secrets are retryable, so they recover on their own.
- Per-sink "required key missing" checks (a Secret that has data but lacks the key a sink needs).
  Each sink keeps its own validation; only an entirely empty Secret is a new shared error.
- The informer cache's own copy of Secrets. The manager already caches Secrets in the watched
  namespaces (it reads them through the cached client); narrowing that cache is separate work.
- Cross-namespace `secretRef` enforcement at runtime (backlog SECRETREF-NS-RUNTIME-01).
- Database sinks in the CLI (backlog CLI-DB-SINK-REACH-01).
- Redacting third-party driver errors that exist today (MongoDB, see the `sink-ca-trust`
  follow-up). This change adds no error that embeds a driver message with a connection string.

## Assumptions

- The inventory reconcilers pass the manager's client to `RunExportEnvelope`, and that client
  serves `Get` on a Secret from an informer cache, so a per-acquire metadata read costs no API
  call. Believed from `cmd/main.go:190-203` (default `client.Options`, no `DisableFor`);
  probe 0.1 verifies it.
- The API server keeps `resourceVersion` unchanged on a no-op update and changes it on every write
  that changes the object, and a delete-and-recreate changes the UID. Kubernetes API conventions;
  probe 0.1 checks it against envtest.
- Exports run at least once per export interval: the coalesce skip ends when the interval is due
  (`internal/controller/kollectinventory_controller.go:360-368`). Believed; probe 0.1 verifies it.
- `pgxpool.Pool.Close` blocks until acquired connections are returned (pgx v5.11.0,
  `pgxpool/pool.go:468-470`, read 2026-10-19); the other long-lived clients (mongo-driver v1.17.10
  `Disconnect`, kafka-go v0.4.51 `Writer.Close`, nats.go v1.54.0 `Conn.Close`, BigQuery v1.85.0
  `Client.Close`) do not promise to wait for an operation in flight, which is why SCR-3 drains in
  the pool instead of relying on each client. Probe 0.1 records each.
