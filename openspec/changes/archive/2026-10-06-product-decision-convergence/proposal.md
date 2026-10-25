# Proposal

## Why

The final consolidated review (§7 of the firstmate knowledge artifact
`data/kollect-xconsol-final/report.md`, union with `data/kollect-xconsol-b/report.md`; both live in
the supervisor's knowledge base, not this repository — the captain's decision implementing them was
relayed with this task) surfaced five product decisions. Four are implementable at HEAD `3ee21266`
(re-verified below); one (Helm `mode`) has no defined wiring target and is parked for a decision
(design.md D7).

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

## Source decisions (captain's answer, verbatim substance)

The five product decisions this change answers, as stated in the final consolidated review §7
(and pass B's "Recommended next actions" #5), with the resolution relayed with the task brief:

| # | Decision (review §7) | Resolution |
| --- | --- | --- |
| 1 | Un-document or implement `kollect.dev/requestedAt` and `kollect.dev/collectedGeneration` | **Implement both** (D1, D2) |
| 2 | Delete or wire the Helm `mode` value | **Wire it** — "either wire the templates to honour it or, if wiring is unsafe, say so and stop for a decision" → verification found the wiring target undefined; stopped for the captain → captain chose **delete** (option (a), D7) |
| 3 | Cluster `collectedCount` parity gap: decide parity vs documented difference | **Parity** — add `status.collectedCount` (+ updatedAt) to `KollectClusterTarget` (D3) |
| 4 | Backend-pool eviction: evict-on-delete, or document the 48 h TTL as the contract | **Evict-on-delete** (D4) |
| 5 | Git engine future: verify which auth modes genuinely need the CLI, then deprecate or hoist-and-keep | **Converge to ONE engine**: verify first; if the evidence is ambiguous, stop for a decision → verification found no auth mode needs the CLI engine; converge to go-git (D5) |

## What Changes

- `kollect.dev/requestedAt` is implemented: on `KollectInventory` and `KollectClusterInventory`,
  changing the annotation forces the next export past the per-sink debounce (one export per change).
- `kollect.dev/collectedGeneration` is implemented: an exported source-object copy (profile export
  mode `Resource`) records the source object's `metadata.generation` in its metadata annotations.
- `KollectClusterTarget` gains `status.collectedCount` and `status.collectedCountUpdatedAt`,
  mirroring `KollectTarget` (fields, semantics, printer columns, and the count-changed write
  escape hatch of PERF-FIX-05).
- Deleting a family sink (KollectSnapshotSink/KollectDatabaseSink/KollectEventSink) evicts its
  pooled backend immediately (evict-on-delete, keyed by the sink object UID); a backend whose
  build was in flight at eviction time is discarded rather than re-pooled, and the TTL stays as
  the backstop.
- The git engine converges to one engine, go-git: `spec.git.engine` admits only `go-git`
  (CRD enum, admission validation, backend config all reject `cli`); `file://` remotes and
  `git ls-remote` connection probes keep the CLI machinery they already share; the go-git SSH
  key-exchange list gains `mlkem768x25519-sha256` and `diffie-hellman-group16-sha512`, both already
  implemented by x/crypto v0.57.0.
- **Helm value `mode` deleted (captain's decision, option (a), design.md D7):** the value was a
  dead hub/spoke transport-era knob with no template consumer and an undefined wiring target;
  the docs already said single-cluster only. Value, schema entry, README sample blocks and the
  helm-values row are gone.

## Capabilities

### New Capabilities

- `export-annotations`: the two documented annotation keys become behaviour (ERA-1, ERA-2; the
  stamp-survival and no-metadata cases are scenarios of ERA-2).
- `target-status`: cluster-target collectedCount parity (TSP-1; the count-changed write escape
  hatch and the steady-timestamp rule are its scenarios).
- `backend-pool`: pooled-backend lifecycle, incl. evict-on-delete (BEP-1, BEP-2; the TTL
  backstop is BEP-2).
- `git-engine`: one git engine, its surface and KEX offer (GTE-1..GTE-3).

### Modified Capabilities

None.

## Impact

- Entry points: both inventory reconcilers' export debounce
  (`internal/controller/per_sink_export.go`, `kollectinventory_controller.go:356,487`,
  `kollectclusterinventory_controller.go:305`); the Resource-mode embed
  (`internal/collect/engine.go:957`, `internal/collect/prune.go:40`); the cluster-target status
  write (`kollectclustertarget_controller.go:303-341`); the family-sink controller watches
  (`internal/controller/family_sink_controller.go:66-70`); the backend pool
  (`internal/sink/backend_pool.go`); git sink admission/build
  (`internal/validation/git.go:88-96`, `internal/sink/git/config.go:164-176`,
  `api/v1alpha1/kollectsink_types.go:130-134,179-183`, `internal/sink/git/ssh_auth.go:27`).
- Generated artifacts (`config/crd/bases`, `charts/kollect/crds`, `config/rbac/role.yaml`,
  deepcopy) regenerate via `make generate manifests`; `task verify` must stay green.
- Docs: `docs/ANNOTATIONS-LABELS.md`, `docs/crds/kollecttarget.md`,
  `docs/crds/kollectclustertarget.md`, `docs/crds/kollectsnapshotsink.md`, the other `engine:
  cli` sites (`charts/kollect/README.md.gotmpl`, `docs/operator-manual/index.md`,
  `docs/development/coding-standards.md`, `docs/security/security-architecture.md`,
  `Dockerfile`, `Dockerfile.pipeline` comments), `docs/operator-manual/upgrading.md` (engine
  removal note), ADR-0803 (engine convergence). `task helm-docs` regeneration follows the
  gotmpl edit.
- CRD field addition (collectedCount) is additive; `engine: cli` removal is a schema rejection of
  an opt-in value, pre-v0.x, documented in ADR-0803 and the upgrade note.

## Non-goals

- Folding `tenantMode` into another knob or otherwise restructuring the chart's RBAC surface
  (the captain's decision (a) keeps `tenantMode` as is).
- The Sweep 1 docs-truth train (chart links, ADR sweep, doc rot) — a separate worker/PR.
- Dead-surface deletion beyond what the engine convergence itself removes (Sweep 2 owns the rest).
- Deprecating or changing `file://` remote support, which the shared CLI machinery keeps serving.
- Any change to the debounce cadence other than the `requestedAt` bypass.

## Assumptions

- The go-git KEX pin is in-repo and extendable: `defaultSSHKeyExchangeAlgorithms`
  (`internal/sink/git/ssh_auth.go:27-36`) is passed to `ssh.Config{KeyExchanges}`. Source: read at
  HEAD. x/crypto v0.57.0 implements `mlkem768x25519-sha256` (`ssh/kex.go:409`,
  `KeyExchangeMLKEM768X25519`) and `diffie-hellman-group16-sha512` (`ssh/kex.go:448`). Source:
  module cache read.
- x/crypto v0.57.0 does not implement `sntrup761x25519-sha512@openssh.com` (only testdata hits);
  a server restricted to it alone would need the CLI. Accepted residual: such a transitional
  algorithm is deprecated by OpenSSH 10 and no in-repo evidence says a user runs one.
- The debounce record is process-local (`perSinkCoalesceTracker`), so a `requestedAt` change
  takes effect on the next reconcile of the same manager process; after a manager restart every
  inventory exports once anyway (cold tracker). Source: read of `per_sink_export.go`.
- Sink deletion events reach the controller as a delete event carrying the last observed state;
  controller-runtime v0.24.1 wraps an uncached final state in a `cache.DeletedFinalStateUnknown`
  tombstone whose name/namespace/UID remain readable. Source:
  `event.DeleteEvent` / tombstone handling in controller-runtime v0.24.1 (`pkg/event/events.go`,
  `pkg/handler/eventhandler.go:105-146`). The eviction hook is defensive about the unknown-state
  shape: it evicts by UID, and falls back to namespace/name only when the UID is empty.
- Family sinks carry no finalizer of their own today; deletion is therefore a single delete
  event, not a finalizer-drain sequence. Source: `family_sink_controller.go` has no
  `DeletionTimestamp` branch; `guardReconcile` only recovers panics.
- Eviction and an in-flight export can interleave (delete event lands while an export holds or
  is still building the pooled backend): the entry is evicted and the in-flight build's
  re-store is discarded by a delete-tombstone; the in-flight export itself may then fail
  against a closed backend, which is acceptable — the sink no longer exists, the reconcile
  records a transient failure, and no new acquire can rebuild for that sink (production
  acquires load the sink object first). Design D4.
