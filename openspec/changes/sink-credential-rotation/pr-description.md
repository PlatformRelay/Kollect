# PR description — sink-credential-rotation

- **Source:** `feat/sink-credential-rotation`
- **Target:** `main`
- **Title:** `:sparkles: feat(sink): apply rotated Secrets at the next export`
- **Opens as:** **ready** — the operator authorised a ready PR and a rebase merge once every check
  on the head is green.

Paste everything below the line.

---

## TL;DR

- **What:** a pooled sink backend now follows the Secrets its sink references: a rotated
  credential, database or CA Secret takes effect at the next export, and a deleted or emptied one
  fails the export instead of running on cached credentials.
- **Why:** backlog SECRET-ROTATION-01 (launch blocker): the backend pool was keyed on the sink spec
  alone, so a rotated Secret kept its old value until a spec change, a sink delete, a restart or
  48 h idle, and deleting a Secret did not stop exports.
- **Risk:** behaviour change on upgrade: a deleted or entirely empty referenced Secret now stops
  exports (retryable), S3/GCS no longer fall back to ambient credentials when their Secret is
  emptied, and a Git Retain release needs a valid Secret.
- **Reviewer action:** read the pool (`backend_pool.go`) and the `0s` watchdog; approving accepts
  the v0.22.0 release note below and the model-aligned decisions listed.

No GitHub issue tracks SECRET-ROTATION-01 (`gh issue list --search rotation` returns none), so
this PR closes no issue.

## What rotation now does

On every acquire the pool reads the metadata (namespace, name, UID, `resourceVersion`) of every
Secret the sink references, uncached, through the manager's API reader, and folds it into the pool
key (SCR-1). A changed key retires the pooled backend; the export that noticed builds a new one
from the current values, and key and values come from the same read. One function
(`internal/sink/secret_refs.go`) lists the references for both the key and the build: credentials
`secretRef`, `git.auth.secretRef`, `kafka.secretRef`, `nats.secretRef`, `postgres.databaseRef`,
`mongodb.databaseRef`, `bigquery.secretRef`, `tls.caSecretRef`.

A retired backend is never closed under an export in flight (SCR-3): entries count holders, leave
the pool at once and are closed exactly once, when the last holder releases. A slower builder
never replaces a newer entry (store fence), and a rebuild that fails after a Secret change does
not fall back to the old backend. No controller watches Secrets: a Secret change alone triggers
nothing; the new value is used at the sink's next export.

## User-visible changes

- **`exportMinInterval: 0s` semantics.** `0s` stays "export only on material change": a Secret
  change alone never exports such a binding, and an unchanged, exported `0s` inventory is not
  re-exported by any timer. New (SCR-7): when every sink export of a `0s` inventory fails
  transiently, the reconciler requeues after `ZeroIntervalWatchdog` (30 s) until it succeeds;
  before, it was never retried on a timer. Terminal failures still do not requeue. A controller
  restart, or more than 48 h on a degraded path that returns before the export, re-sends an
  unchanged `0s` payload once (documented).
- **One retryable, redacted error for deleted or emptied Secrets** (SCR-2, SCR-4, SCR-5):
  `secret "<name>" not found in namespace "<namespace>"` or `… has no data`, on status, Events and
  logs, never a value. Restoring the Secret recovers without touching the sink. `kollect-pipeline`
  prints the same "has no data" text for an empty Secret manifest before it builds a backend.
- **S3/GCS: no ambient fallback when the Secret is empty.** An entirely empty `secretRef` Secret
  fails instead of falling through to the pod's ambient AWS credentials. A Secret that keeps other
  keys but drops both access keys still falls back, as before.
- **An empty CA Secret is retryable.** A `caSecretRef` Secret emptied entirely gives the retryable
  "has no data" error; a Secret with other keys but no CA key keeps the terminal error.
- **Stream cleanup needs no Secret.** Kafka and NATS sinks decide `Retained` before the acquire, so
  a deleted Secret never blocks their inventory deletion. Git, GitLab, database sinks and S3/GCS
  cleanup that retracts objects still need the Secret (or `kollect.dev/force-cleanup: "true"`).
- **Cross-namespace `secretRef` under `--watch-namespaces`** now works on the export path (uncached
  reads); the connection probe still reads through the cache.

**No RBAC change.** The manager already has `get/list/watch` on Secrets
(`config/rbac/role.yaml`); the uncached metadata GET uses `get`. No CRD or dependency change.

**Upgrade note:** `docs/operator-manual/upgrading.md`, *Sink Secret rotation (after v0.21.0)*,
with the before/after table. Release note for the v0.22.0 GitHub Release: rotation takes effect
at the next export; a deleted or entirely empty referenced Secret now fails the export (retryable)
and stops S3/GCS from falling back to ambient credentials; a Git sink whose credentials were
rotated away keeps a Retain release in `Terminating` until the Secret is fixed or
`kollect.dev/force-cleanup` is set; `0s` inventories retry a transient total failure every 30 s.

**Decisions this PR rests on** (maintainer's decision log, model-aligned, pending operator
validation): uncached metadata GET per acquire, full read only on a rebuild (2026-10-19, option C);
`rotation-zero-interval`, option B (2026-10-19): keep `0s` as material-change-only and add the
30 s retry for transient total failures.

## Risk

- **What can break:** sinks that ran on a deleted or entirely empty Secret, S3/GCS sinks relying on
  an empty Secret for ambient credentials, Git Retain releases after credentials were rotated away.
- **Load:** one uncached metadata GET per referenced Secret per export (no values unless the key
  changed). A metric for these reads is a recorded follow-up.
- **Accepted residuals:** a failed re-read after a stale first read retires the newer entry, which
  costs one rebuild, never stale use (pinned by
  `TestAcquireBackend_failedReReadAfterStaleReadDrainsNewer`); the one-off `0s` re-send above.
- **Rollback:** revert, or roll back to v0.21.x; nothing is persisted.

## Files to look at first

- `internal/sink/backend_pool.go` — key, holders, `retireLocked`, store fence, `observeAndLookup`.
- `internal/sink/secret_refs.go`, `credentials.go`, `build_context.go` — reference list, metadata
  read, empty rule, the build from one snapshot.
- `internal/sink/cleanup.go` — stream retain before the acquire.
- `internal/controller/per_sink_export.go`, `kollectinventory_controller.go`,
  `kollectclusterinventory_controller.go` — the `0s` watchdog and skip-as-activity prune.
- Docs: `docs/operator-manual/upgrading.md`, `troubleshooting.md`, `docs/crds/*sink.md`,
  `docs/security/security-architecture.md`.

**Review this commit-by-commit.** The feature lands as rebuild-on-rotation, fail-on-deleted/empty
plus SCR-7, drain-and-fence, docs, then one commit per branch-review fix.

## Testing protocol

- [x] Final gates on `00604732b` (`umask 022`, envtest assets): unit and envtest for
  `internal/sink`, `internal/pipeline`, `internal/controller`, `cmd` (envtest 59 of 59 specs);
  race (`-count=10 ./internal/sink/`, `-count=3 ./internal/pipeline/`, controller under `-race`);
  `task test-integration` 68 expected, 68 run and passed, 0 skipped (including
  `TestPostgresExport_passwordRotation` and `TestS3Export_caRotation`); `task lint` 0 issues;
  `go vet -tags=integration ./...`; `go mod tidy -diff && go mod verify`; `task verify`;
  `task spec:validate`; `task scrub`; gitleaks.
- [x] Mutation checks per group (evidence G1–G4 and tasks 6.x): every load-bearing test killed its
  mutant, no-op controls survived.
- [x] Branch review: two rounds of two independent review legs (adversarial and spec lenses).
  Round 1 filed two CRITICALs, verified and downgraded to WARNING (neither let a stale credential
  reach a later export), both fixed red-first; round 2 on the fixes: no CRITICAL.
- [ ] Every check on the PR head green: CI (lint, unit, `test-integration`, `verify`), Preflight,
  Docs, dependency-review, SonarCloud.
- [ ] Before tagging v0.22.0, on a lab cluster: rotate a Postgres sink's password Secret and see
  the next export authenticate with the new value; delete the Secret and see the inventory status
  name it with the retryable error; restore it and see the export recover without touching the
  sink.

---

## Optional reading — the long version

*Nothing above depends on this.*

- **Spec set:** `openspec/changes/sink-credential-rotation/` (archived after merge): proposal,
  design, `specs/sink-secret-rotation/spec.md` (SCR-1 to SCR-7), tasks with the verification table,
  per-group evidence and review records.
- **Why uncached metadata, not the cache or a watch:** cached reads are eventual and fail
  cross-namespace refs under `--watch-namespaces`; a Secret watch needs a Secret-to-sink index and
  a new watch path for the same next-export guarantee (design *Option chosen*, options A and D).
  Counterpoint: the metadata GETs are steady API load, and an API-server outage fails exports
  where a cache would have served stale values (10 s timeout per read).
- **Stage-B fixes:** a missing or empty Secret seen by the full read now retires and fences like the
  metadata path (6.2); a `0s` skip keeps the coalescing state alive (6.3); cluster `0s` terminal
  no-requeue twin (6.4); the stale-first-read residual pinned (6.5, 6.7); wording (6.6, 6.8); the
  `go mod tidy` fix for a test import (6.1).
- **Out of scope:** Secret watches, recovering terminal errors on a Secret fix (touch the sink),
  per-sink "required key missing" checks, narrowing the informer's Secret cache, runtime
  cross-namespace enforcement, database sinks in the CLI, redacting pre-existing third-party client
  errors, the legacy `KollectSink` kind.
