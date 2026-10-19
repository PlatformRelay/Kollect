# Design

Required by `openspec/config.yaml` (credential change, and a compatibility change: a deleted or
emptied Secret stops exports on upgrade).

## Option chosen: key the pool on the referenced Secrets, read uncached

| Option | How a change is noticed | Cost | Decision |
| --- | --- | --- | --- |
| A | Each acquire reads the referenced Secrets through the informer cache and folds UID and `resourceVersion` into the pool key | No API calls | Rejected: eventual (a stale cache serves the old Secret), fails cross-namespace refs under `--watch-namespaces` (`multi_namespace_cache.go:239`, controller-runtime v0.24.1), deep-copies values on every acquire |
| B | As A, but a full uncached GET per acquire | 1-3 GETs per export, values transferred and copied every time | Rejected: same semantics as C with more value copies |
| C (chosen) | Each acquire does an uncached metadata-only GET per referenced Secret; a changed UID or `resourceVersion` triggers one full uncached GET per Secret, and the backend and its key come from those same objects | 1-3 small GETs per export, full GETs only on rebuild | Model-aligned decision 2026-10-19 (both legs C), brief `agent-context/decisions/2026-10-19-rotation-secret-read-path.md` |
| D | A Secret watch maps Secrets to sinks and evicts entries | A Secret-to-sink index, a new watch path | Not chosen: more moving parts for the same next-export guarantee; stays open for prompt recovery from terminal errors (proposal non-goals) |

Counterpoint recorded with the decision: the metadata GETs are steady API load (active
inventory-sink pairs × referenced Secrets, once per export), and an API-server outage fails
exports where a cache would have served stale values. Each read gets a 10 s timeout and a clear
error; a read metric is a follow-up (loop.md triage).

## Readers

`ExportEnvelopeRequest` and the cleanup request gain `SecretReader client.Reader`. Both inventory
reconcilers set it to `mgr.GetAPIReader()` (wired in `cmd/main.go` next to the existing client).
The pool uses `SecretReader` for every Secret read on the acquire path, metadata and full, and
for the rebuild's `BuildContextFromSpec`. When it is nil (unit tests that build requests by hand)
the pool uses `Client`; an envtest under a restricted cache (task 1.1) proves the production
wiring. The pool-disabled path and the connection probes keep their current reader.

## Referenced Secrets, one list

A new unexported function `referencedSecretRefs(spec, defaultNamespace) []secretUse` in
`internal/sink` returns, in a fixed order, every Secret reference `BuildContextFromSpec` reads for
the spec, each with its effective namespace (`ref.Namespace`, else `defaultNamespace`, else
`DefaultSecretNamespace`, as `ResolveSecret` does, `credentials.go:37-40`): `spec.secretRef`
(always read, `build_context.go:45`), then by type `postgres.databaseRef`, `bigquery.secretRef`,
`mongodb.databaseRef`, `kafka.secretRef`, `git.auth.secretRef`, `nats.secretRef`, and
`tls.caSecretRef` when `tls.caBundle` is unset. `BuildContextFromSpec` is rewritten to iterate
this list, so the build and the fingerprint cannot read different Secrets. A test walks
`KollectSinkSpec` by reflection, collects every field of type `*SecretReference`, and fails when
one is missing from the function's switch.

## Fingerprint and acquire

1. `specHash` as today. `secretMeta(ctx, reader, uses)` GETs each Secret as
   `metav1.PartialObjectMetadata` (kind Secret) and returns namespace, name, UID and
   `resourceVersion`; its return type carries no data.
2. Key = sha256(specJSON, then namespace, name, UID, `resourceVersion` per use). A pooled entry
   with the same key is a hit: take a hold and return it.
3. Miss: one full GET per use, through `SecretReader`, then the empty check, then build from those
   objects (`BuildContextFromSpec` is split so the rebuild hands it the fetched Secrets instead of
   reading again). The stored key is recomputed from the full objects' metadata, so key and values
   come from one snapshot even when the Secret changed between the two reads.
4. Store with a fence: `storePooledBackend` gets the key the caller saw at its miss (or "absent").
   It replaces the pooled entry only when the pooled entry still has that key; if another
   acquire stored a different entry in between, the caller's backend is not pooled: the caller
   uses it for its own export under an owning release (as the tombstone path does) and the newer
   entry stays. Two builders that end with the same key share one entry and the surplus build is
   closed at once.

Errors on the acquire path:

- Secret not found: the existing text `secret %q not found in namespace %q`, unclassified
  (transient, as today, `internal/sink/export.go:96-108`, `internal/errors/errors.go:86-106`).
- Secret found but `Data` is empty or every value in it is empty: `secret %q in namespace %q has
  no data`, unclassified. The same check runs in `ResolveSecret`, so the probes and the
  pool-disabled path give the same error.
- Other GET errors (timeout, forbidden, cancelled context): returned wrapped with the Secret's
  namespace and name, class as `ClassOf` derives it.

Only not-found, empty, and a failed rebuild after a key change retire the pooled entry. A
cancelled context or a transient read error from one caller does not retire a backend other
inventories share.

A metadata-only change (a label) changes `resourceVersion` and rebuilds once; accepted as rare and
harmless. A no-op update keeps `resourceVersion` (probe 0.1).

## Pool entries with holders

`pooledEntry` gains `holders int` and `retired bool`. Under `globalBackendPool.mu`:

- A hit increments `holders` and returns a release that decrements it; when it reaches zero on a
  retired entry, the release closes the backend (after unlocking). A release is idempotent
  (`sync.Once`).
- `retireLocked(key)` removes the entry from `entries`, sets `retired`, and returns the backend to
  close now only when `holders == 0`; otherwise the last release closes it. Every path that drops
  an entry uses it: key mismatch on acquire, not-found or empty Secret, delete eviction (BEP-1),
  TTL prune, and the explicit test evictions. TTL prune skips an entry with holders (its
  `lastUsed` is fresh anyway). `ResetBackendPoolForTest` closes at once (test only).
- The tombstone path is unchanged: after a delete eviction an acquire may still build a backend,
  but it is never pooled for the deleted sink; the caller owns it and its release closes it
  (`backend_pool.go:159-163`). BEP-1's "Close has run" holds once the holders that acquired before
  the delete have released.

The pool never calls Close while holding the mutex (`backend_pool.go:128-132` rule kept). Callers
already `defer release()` (`internal/sink/export.go:109`, `internal/sink/cleanup.go:198`).
`DisableBackendPoolForTest` keeps building and closing per call.

Client libraries differ in what Close does to an operation in flight: pgx v5.11.0 `Pool.Close`
waits for acquired connections (`pgxpool/pool.go:468-470`), mongo-driver v1.17.10 `Disconnect`
waits unless its context ends (`mongo/client.go:299-306`; kollect passes `context.Background()`,
`internal/sink/mongodb/backend.go:148-152`), kafka-go v0.4.51 `Writer.Close` flushes and waits
(`writer.go:551-554`), and the NATS backend latches closed so the next call fails
(`internal/sink/nats/backend.go:90-100`). Holder draining protects the whole backend operation,
including calls an export makes after another goroutine retired the entry, whatever each client
does.

## Cleanup before acquire for sinks that cannot retract

`RunCleanupExport` (`internal/sink/cleanup.go:184-214`) acquires a backend and only then returns
`CleanupRetained` for a backend that neither supports delete nor implements `ExportCleaner`
(Kafka, NATS). With SCR-2 that acquire would fail on a deleted Secret and keep the inventory
`Terminating` for a sink that is never contacted. The retain decision for stream types moves
before the acquire, decided by sink type (the registry's stream types), so their cleanup needs no
Secret. Git, GitLab, S3, GCS and the database sinks still acquire (they retract or release).

## Secret data lifetime

On a hit, no Secret value is transferred or copied (metadata-only GET). On a rebuild, values are
read once, passed to the constructor and dropped; the pool keeps no `BuildContext`. Backends keep
what they need to authenticate (a DSN, a token, static AWS keys) for their own lifetime, which now
ends at the next Secret change. The key holds a digest of metadata only. The informer cache's own
copy of Secrets (the probes and other controllers read through it) is unchanged.

## Logging

A rebuild caused by a Secret change logs once at info on the `sink` logger: sink namespace and
name, the reason `referenced secret changed`, and the namespace/name of each Secret whose UID or
`resourceVersion` changed. Never a value. Close errors keep `closeBackendLogged`
(`internal/sink/backend_pool_log.go`) with a new reason string.

## CLI

`kollect-pipeline` resolves Secrets from the config directory on every run and builds a fresh
backend per context (`internal/pipeline/wire.go:515-538`); no pool, so rotation needs nothing.
`resolveSecretValues` (`wire.go:177-208`) gains the same empty check after `stringData` and
`${env:VAR}` substitution, with the same message as the operator (parity at the message text; the
CLI keeps its own "not found in config directory" text for a missing Secret, CSC-3).

## Compatibility

| Before (v0.21.x) | After (v0.22.0) |
| --- | --- |
| Rotated credential or CA Secret: old value used until spec change, sink delete, restart or 48 h idle | New value used from the next export |
| Deleted referenced Secret: exports keep working on the cached backend | Next export fails (retryable), naming the Secret |
| Entirely empty referenced Secret: same as above; S3/GCS fall back to the pod's ambient AWS credentials on the next build | Next export fails (retryable), naming the Secret. A Secret that has other keys but no access key still falls back, as before |
| Cross-namespace `secretRef` under `--watch-namespaces`: build fails ("unknown namespace for the cache") | Export path reads through the API reader and works; the connection probe still reads through the cache |
| Git sink, credentials rotated away, inventory deleted with Retain: release used cached credentials | Release fails and the inventory stays `Terminating` until the Secret is fixed or `kollect.dev/force-cleanup: "true"` is set |
| Kafka/NATS sink with a deleted Secret, inventory deleted | Unchanged: retained without contacting the sink |
| Spec change while another export used the backend: backend closed under it | Closed after that export releases it |

Rollback to v0.21.x restores the old behaviour; nothing is persisted.
