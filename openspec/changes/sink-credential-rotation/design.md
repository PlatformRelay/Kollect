# Design

Required by `openspec/config.yaml` (credential change, and a compatibility change: a deleted or
emptied Secret stops exports on upgrade).

## Option chosen: fingerprint the referenced Secrets on acquire

| Option | How a change is noticed | Cost | Rejected because |
| --- | --- | --- | --- |
| A (chosen) | Each acquire reads the referenced Secrets' metadata from the informer cache and folds UID and `resourceVersion` into the pool key | N cache reads per export; no new watch, mapping or RBAC | |
| B | A Secret watch maps a Secret to the sinks that reference it and evicts their entries (optionally enqueues their inventories) | A Secret-to-sink index across three family kinds and the legacy kind, a reconnect per Secret update, a new watch path to test | More moving parts for the same "next export" guarantee; eager eviction alone does not stop a stale export unless acquire also checks |
| C | Hash the resolved Secret data instead of metadata | Same reads as A, plus hashing values | A digest of a low-entropy password in a long-lived map is avoidable; `resourceVersion` changes whenever the data does |

A is the smallest change that makes the next export use the current Secret. B stays open as a
later addition for prompt recovery from terminal errors (proposal non-goals); A does not block it.

## Referenced Secrets, one list

A new unexported function `referencedSecretRefs(spec) []secretUse` in `internal/sink` returns, in
a fixed order, every Secret reference `BuildContextFromSpec` reads for the spec, with the default
namespace applied: `spec.secretRef` (always read, `build_context.go:45`), then by type
`postgres.databaseRef`, `bigquery.secretRef`, `mongodb.databaseRef`, `kafka.secretRef`,
`git.auth.secretRef`, `nats.secretRef`, and `tls.caSecretRef` when `tls.caBundle` is unset.
`BuildContextFromSpec` is rewritten to iterate this list, so the build and the fingerprint cannot
read different Secrets. A test walks `KollectSinkSpec` by reflection, collects every field of type
`*SecretReference`, and fails when one is missing from the function's switch (a new reference
field without rotation coverage fails CI).

## Fingerprint

`acquireBackend` computes `poolFingerprint = sha256(specJSON || for each secretUse: namespace,
name, uid, resourceVersion)`. The secret part comes from `secretMeta(ctx, c, uses)`, which `Get`s
each Secret through the caller's client, copies the four metadata strings, and drops the object;
the function returns only metadata, so its signature is the guarantee that no value reaches the
key. Errors:

- Secret not found: the existing text `secret %q not found in namespace %q`, unclassified
  (transient, as today, `internal/sink/export.go:96-108`).
- Secret found but every value in `Data` is empty, or `Data` is empty: `secret %q in namespace %q
  has no data`, unclassified. The same check runs in `ResolveSecret`, so a build that bypasses the
  pool (`DisableBackendPoolForTest`, probes) gives the same error.
- Other `Get` errors: returned wrapped, as the build returns them today.

A metadata-only change (a label) changes `resourceVersion` and so rebuilds the backend once. That
is accepted: it is rare and harmless. A no-op update keeps `resourceVersion` (probe 0.1).

On any error, the acquire retires the key's pooled entry before returning, so a failed acquire
never leaves a backend built from a Secret that is gone.

## Pool entries with holders

`pooledEntry` gains `holders int` and `retired bool`. Under `globalBackendPool.mu`:

- A hit increments `holders` and returns a release that decrements it; when it reaches zero on a
  retired entry, the release closes the backend (after unlocking). A release is idempotent
  (`sync.Once`).
- `retireLocked(key)` removes the entry from `entries`, sets `retired`, and returns the backend to
  close now only when `holders == 0`; otherwise the last release closes it. Every path that drops
  an entry uses it: fingerprint mismatch, acquire error, delete eviction (BEP-1, the tombstone
  logic is unchanged), TTL prune (an entry with holders is never pruned; `lastUsed` keeps it
  fresh anyway), and the explicit test evictions. `ResetBackendPoolForTest` closes at once (test
  only).
- `storePooledBackend` returns the stored entry's backend with `holders` already incremented. When
  two acquires race to build, the loser's backend is closed at once (it has no holders) and the
  loser gets the winner's entry, as today, plus a hold.
- The tombstone path keeps its owning release (`backend_pool.go:159-163`).

The pool never calls Close while holding the mutex (`backend_pool.go:128-132` rule kept).

Callers already `defer release()` (`export.go:111`, `cleanup.go:197`); no caller changes beyond
that. `DisableBackendPoolForTest` keeps building and closing per call.

## Secret data lifetime

Values are read only on a rebuild, by `BuildContextFromSpec`, and passed to the constructor; the
pool does not keep the `BuildContext`. Backends keep what they need to authenticate (a DSN, a
token, static AWS keys) for their own lifetime, which now ends at the next Secret change instead
of at a spec change. The fingerprint holds digests of metadata only.

## Logging

A rebuild caused by a Secret change logs once at info on the `sink` logger: sink namespace and
name, the reason `referenced secret changed`, and the namespace/name of each Secret whose UID or
`resourceVersion` changed. Never a value, never a `resourceVersion` diff of anything else. Close
errors keep `closeBackendLogged` (`internal/sink/backend_pool_log.go`) with a new reason string.

## CLI

`kollect-pipeline` resolves Secrets from the config directory on every run and builds a fresh
backend per context (`internal/pipeline/wire.go:515-535`); no pool, so rotation needs nothing.
`resolveSecretValues` (`wire.go:177-208`) gains the same empty check after `stringData` and
`${env:VAR}` substitution, with the same message as the operator (parity at the message text; the
CLI keeps its own "not found in config directory" text for a missing Secret, CSC-3).

## Compatibility

| Before (v0.21.x) | After (v0.22.0) |
| --- | --- |
| Rotated credential or CA Secret: old value used until spec change, sink delete, restart or 48 h idle | New value used from the next export |
| Deleted referenced Secret: exports keep working on the cached backend | Next export fails (retryable), naming the Secret |
| Emptied referenced Secret: same as above; S3/GCS fall back to the pod's ambient AWS credentials on the next build | Next export fails (retryable), naming the Secret |
| Git sink, credentials rotated away, inventory deleted with Retain: release used cached credentials | Release fails and the inventory stays `Terminating` until the Secret is fixed or `kollect.dev/force-cleanup: "true"` is set |
| Spec change while another export used the backend: backend closed under it | Closed after that export releases it |

Rollback to v0.21.x restores the old behaviour; nothing is persisted.
