# Proposal

## Why

The final consolidated review (`data/kollect-xconsol-final/report.md` §7) and pass B
(`data/kollect-xconsol-b/report.md` C-23/C-24, C-06 residue, C-16 eviction, C-09) surfaced five
product decisions the captain has now answered. Four are implementable at HEAD `3ee21266` (re-verified
below); one (Helm `mode`) has no defined wiring target and is parked for a decision (design.md D7).

What the docs promise and the code does not do today:

1. `docs/ANNOTATIONS-LABELS.md:100-101` advertises `kollect.dev/collectedGeneration` and
   `kollect.dev/requestedAt`; zero reads under `api/ internal/ cmd/` (grep at `3ee21266`). A GitOps
   user coding against the table gets nothing.
2. Namespaced `KollectTarget` exposes `status.collectedCount`/`collectedCountUpdatedAt` backed by the
   COLLECTED printer columns; `KollectClusterTarget` has no such fields — its count exists only as
   prose in the Ready message (`kollectclustertarget_controller.go:309-316`).
3. The backend pool's own comment (`internal/sink/backend_pool.go:33-35`) admits
   `EvictBackendPool`/`EvictBackendPoolByUID` have no production caller: a deleted sink holds a
   pooled backend up to `backendPoolTTL` (48 h) with its connections and credentials.
4. The git sink ships two complete engines. The CRD doc's claim that `cli` is required for
   SSH/KEX edge cases is stale: the go-git KEX pin is in-repo code (`ssh_auth.go`), and the
   shipped x/crypto v0.57.0 already implements the modern algorithms the pin omits. The pipeline
   image ships no git binary (`Dockerfile.pipeline:33-35`), so the CLI engine cannot be the single
   engine; go-git must be.

## What Changes

- `kollect.dev/requestedAt` is implemented: on `KollectInventory` and `KollectClusterInventory`,
  changing the annotation forces the next export past the per-sink debounce (one export per change).
- `kollect.dev/collectedGeneration` is implemented: an exported source-object copy (profile export
  mode `Resource`) records the source object's `metadata.generation` in its metadata annotations.
- `KollectClusterTarget` gains `status.collectedCount` and `status.collectedCountUpdatedAt`,
  mirroring `KollectTarget` (fields, semantics, printer columns, and the count-changed write
  escape hatch of PERF-FIX-05).
- Deleting a family sink (KollectSnapshotSink/KollectDatabaseSink/KollectEventSink) evicts its
  pooled backend immediately (evict-on-delete); the TTL stays as the backstop.
- The git engine converges to one engine, go-git: `spec.git.engine` admits only `go-git`
  (CRD enum, admission validation, backend config all reject `cli`); `file://` remotes and
  `git ls-remote` connection probes keep the CLI machinery they already share; the go-git SSH
  key-exchange list gains `mlkem768x25519-sha256` and `diffie-hellman-group16-sha512`, both already
  implemented by x/crypto v0.57.0.
- **Pending decision (not implemented here):** Helm value `mode` — see design.md D7. Options
  (a) delete value + schema entry, (b) fold `tenantMode` into `mode`, (c) keep the value and
  narrow the schema enum to `["single"]`. Stopped for the captain per the brief ("if wiring is
  unsafe, say so and stop").

## Capabilities

### New Capabilities

- `export-annotations`: the two documented annotation keys become behaviour (ERA-1..ERA-3).
- `target-status`: cluster-target collectedCount parity (TSP-1, TSP-2).
- `backend-pool`: pooled-backend lifecycle, incl. evict-on-delete (BEP-1..BEP-3).
- `git-engine`: one git engine, its auth modes and KEX surface (GTE-1..GTE-4).

### Modified Capabilities

None.

## Impact

- Entry points: both inventory reconcilers' export debounce
  (`internal/controller/per_sink_export.go`, `kollectinventory_controller.go:356,487`,
  `kollectclusterinventory_controller.go:305`); the Resource-mode embed
  (`internal/collect/engine.go:957`, `internal/collect/prune.go:40`); the cluster-target status
  write (`kollectclustertarget_controller.go:303-327`); the family-sink controller watches
  (`internal/controller/family_sink_controller.go:66-70`); the backend pool
  (`internal/sink/backend_pool.go`); git sink admission/build
  (`internal/validation/git.go:88-96`, `internal/sink/git/config.go:164-176`,
  `api/v1alpha1/kollectsink_types.go:130-134,179-183`, `internal/sink/git/ssh_auth.go:23-33`).
- Generated artifacts (`config/crd/bases`, `charts/kollect/crds`, `config/rbac/role.yaml`,
  deepcopy) regenerate via `make generate manifests`; `task verify` must stay green.
- Docs: `docs/ANNOTATIONS-LABELS.md`, `docs/crds/kollecttarget.md`,
  `docs/crds/kollectclustertarget.md`, `docs/crds/kollectsnapshotsink.md`,
  `docs/operator-manual/upgrading.md` (engine removal note), ADR-0803 (engine convergence).
  `task helm-docs` unaffected (no chart values change — `mode` is pending).
- CRD field addition (collectedCount) is additive; `engine: cli` removal is a schema rejection of
  an opt-in value, pre-v0.x, documented in ADR-0803 and the upgrade note.

## Non-goals

- Helm value `mode`: pending captain decision (D7), not implemented in this change.
- The Sweep 1 docs-truth train (chart links, ADR sweep, doc rot) — a separate worker/PR.
- Dead-surface deletion beyond what the engine convergence itself removes (Sweep 2 owns the rest).
- Deprecating or changing `file://` remote support, which the shared CLI machinery keeps serving.
- Any change to the debounce cadence other than the `requestedAt` bypass.

## Assumptions

- The go-git KEX pin is in-repo and extendable: `defaultSSHKeyExchangeAlgorithms`
  (`internal/sink/git/ssh_auth.go:23-33`) is passed to `ssh.Config{KeyExchanges}`. Source: read at
  HEAD. x/crypto v0.57.0 implements `mlkem768x25519-sha256` (`ssh/kex.go:409`,
  `KeyExchangeMLKEM768X25519`) and `diffie-hellman-group16-sha512` (`ssh/kex.go:448`). Source:
  module cache read.
- x/crypto v0.57.0 does not implement `sntrup761x25519-sha512@openssh.com` (only testdata hits);
  a server restricted to it alone would need the CLI. Accepted residual: such a transitional
  algorithm is deprecated by OpenSSH 10 and no in-repo evidence says a user runs one.
- The debounce record is process-local (`perSinkCoalesceTracker`), so a `requestedAt` change
  takes effect on the next reconcile of the same manager process; after a manager restart every
  inventory exports once anyway (cold tracker). Source: read of `per_sink_export.go`.
- Sink deletion events reach the controller as a `DeleteEvent` whose object is the last known
  state (UID present). Source: controller-runtime v0.24.1 `eventhandler.go:105-146`.
- Family sinks carry no finalizer of their own today; deletion is therefore a single delete
  event, not a finalizer-drain sequence. Source: `family_sink_controller.go` has no
  `DeletionTimestamp` branch; `guardReconcile` only recovers panics.
