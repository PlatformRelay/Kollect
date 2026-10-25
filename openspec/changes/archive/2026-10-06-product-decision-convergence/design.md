# Design

## D1 — `requestedAt` rides the existing debounce state machine, it does not add a second one

The per-sink debounce (`perSinkCoalesceTracker`) already invalidates on two axes: generation and
checksum. A third axis (`requestedAt`) is the smallest change that gives GitOps users a manual
trigger without inventing a new state store, a new reconciler or a queue: the annotation value
threads through `shouldSkip`/`record` like the checksum does, and a change forces exactly one
export (the record then pins the new value, so steady state resumes). Alternatives rejected:

- a one-shot flag the controller clears on the object: needs a write-back and a mutation the
  user did not ask for, and interacts with conflict/retry;
- bypassing the tracker with a dedicated force path: a second debounce decision to keep in sync
  on three call sites (including the preview path), i.e. the exact drift class the review
  catalogued.

Semantics of "change": absence→present, present→absent, and any value→different value all
invalidate; the value itself is not parsed (any non-empty string is a key). RFC3339 remains the
documented convention so the annotation stays human-auditable, but the trigger does not depend
on the format. Kinds: the two reconciled work kinds with an export debounce —
`KollectInventory`, `KollectClusterInventory`. `KollectTarget` is excluded: its reconcile has no
export debounce, so an annotation there would promise something nothing honours.

Preview honesty: `previewAllSinksDebounced` (namespaced-only, third debounce copy) receives the
same value, so a preview after a `requestedAt` change does not report a debounce the export will
not do.

## D2 — `collectedGeneration` is stamped after prune and scrub, only when a metadata map survives

`PruneResource` is the single choke point for the embedded copy (engine.go:957). Stamping after
prune/scrub means profile-level pruning of `metadata.annotations` cannot erase the stamp, and a
scrub rule cannot redact it (the value is an integer string, never a credential). When the
profile's include section leaves no `metadata` map (e.g. `StatusOnly`), there is no honest place
for a metadata annotation — the copy is exported without one rather than inventing a metadata
block the profile explicitly excluded. The stamp is written unconditionally when metadata exists
(generation `0` included) so "no stamp" and "generation 0" never collapse. Attributes mode stays
byte-identical: the stamp exists only on the embedded object copy.

Doc alignment: `ANNOTATIONS-LABELS.md` row narrows to the Resource-mode copy ("Exported source
objects (metadata)") instead of implying write-back to live cluster objects, which the code never
does and ADR-0202's "collected rows or export metadata" wording left ambiguous.

## D3 — Cluster-target parity reuses the namespaced semantics, not a new field design

Field names, JSON names, pointer types, "timestamp moves only when the count changes", null
semantics and printer columns mirror `KollectTargetStatus` so a fleet operator reads one contract
twice, not two contracts. The write escape hatch (`countChanged && !written`) mirrors
PERF-FIX-05: `setTargetCondition` skips byte-identical writes, so a moving count with a frozen
condition must still persist. The cluster path already has one such hatch for the filter-status
fields (`persistFilterStatusIfSkipped`, `kollectclustertarget_controller.go:332-342`); the count
extends that single write (`filterChanged || countChanged`) rather than adding a second
Status().Update site. The count source is the same one the Ready message already uses
(`collectedCount(ct, matched)`), so the number and the prose cannot disagree. CRD/codegen ripple
(`config/crd/bases`, `charts/kollect/crds`, deepcopy, `hack/gen-glossary.py`) regenerates via
`make generate manifests` and the docs task.

## D4 — Evict-on-delete rides the family-sink controller's delete events, with a delete tombstone in the pool

`FamilySinkReconciler` already exists generically for the three kinds; each instantiation adds a
`Watches` on its own kind whose `DeleteFunc` calls a small hook. Eviction is by sink UID
(`EvictBackendPoolByUID`) — production pooling is always UID-keyed (`poolKeyForSink` prefers UID;
both production acquirers pass a resolved UID), so a namespace/name eviction would be dead code;
the namespace/name form is used only as the defensive fallback when a
`DeleteStateUnknown` tombstone carries no UID.

The interleaving hazard: a delete event can land while an export still holds the pooled backend,
or while an in-flight `acquireBackend` build is about to `storePooledBackend` — without a guard,
the build would re-pool a fresh backend for a deleted sink and the 48 h TTL would again be the
contract, re-creating the exact defect this decision removes. The pool therefore keeps a
delete-tombstone per evicted key (bounded, pruned with the same opportunistic cycle as entries);
`storePooledBackend` discards a backend whose key is tombstoned. A tombstoned UID can never be
reused (Kubernetes UIDs are unique), so discarding is correct; the in-flight export may fail
against the closed backend — acceptable, the sink no longer exists and no new acquire can
legitimately rebuild for it (production acquires load the sink object first, and are
UID-keyed anyway, so a stale namespace/name tombstone can never block a legitimate build).

No finalizer: adding one to drain connections would turn best-effort eviction into a deletion
dependency the product decision does not ask for. The TTL stays as the backstop for entries
whose sink never produced a delete event (pipeline-mode one-shots, restarts). Metrics: eviction
is silent on success — a counter would count ordinary deletes, not defects; the pool's existing
log line on Close names the reason. Two accepted residuals, deferred with reasons: (1) the
eviction's Close runs inline on the informer dispatch goroutine — the same shape as the
pre-existing TTL prune and the acquire-time swap, and the delete watch's listener is
non-blocking, so a slow Close delays that sink family's dispatch briefly but does not stall the
For() reconcile path; (2) an export holding a nats backend at eviction could re-dial a fresh
connection after the close — fixed by the closed-latch (tasks.md T13) rather than refcounting.

## D5 — The git engine converges to go-git; the CLI machinery survives where it is genuinely shared

Verified facts driving this: (1) the pipeline image ships no git binary, so the CLI engine
cannot serve pipeline mode — one engine means go-git; (2) the go-git KEX offer is an in-repo pin,
and x/crypto v0.57.0 already implements `mlkem768x25519-sha256` and
`diffie-hellman-group16-sha512`, so the CRD doc's "cli required for SSH/KEX edge cases" reduces
to a stale pin plus a transitional-algorithm residue (sntrup761-only servers) with no in-repo
evidence of users; (3) `file://` remotes and `ls-remote` probes call the CLI machinery for both
engines today, so `exec_git.go`/`cli_env.go`/`export_file.go` stay regardless — removing them was
never on the table; (4) ADR-0104 documents the CLI SSH path as the weaker host-key story
(no fail-closed guard), so making the CLI the only engine would regress the documented security
model. Convergence therefore removes the `engine: cli` opt-in (CRD enum, admission validation,
backend config), removes the exported `GitEngineCLI` API constant and the CRD field's `cli` enum
value (the marker becomes `+kubebuilder:validation:Enum=go-git`), deletes the engine branch
points and the internal `GitEngine` type and `Config.Engine` field, and extends the KEX pin.
Existing `engine: cli` sinks fail validation with an error naming `go-git`
(pre-v0.x, no breaking marker; upgrade note in `docs/operator-manual/upgrading.md`; ADR-0803,
which records why it skips 0802 — at spec time eight pipeline code comments cited 0802 as
the future pipeline-CLI contract; the docs-truth sweep has since removed them).

Doc sites that assert `engine: cli` works and must be truthed-up with the code: the CRD reference
row (`docs/crds/kollectsnapshotsink.md:22`), `charts/kollect/README.md.gotmpl:15-18` (+ chart
README regeneration), `docs/operator-manual/index.md:131-143` (engine table),
`docs/development/coding-standards.md:74-78` (MUST line), `docs/security/security-architecture.md`
(engine sections — the address-pin and host-key paragraphs now describe the shared probe/file
machinery, not a selectable engine), `Dockerfile:34-35` and `Dockerfile.pipeline:33-38`
comments, and ADR-0415's per-engine commit-ergonomics sentence.

Not converged here: the per-engine probe/delivery function pairs inside the surviving shared
machinery (Sweep 2's ~130-line hoist) — the engine branch points that made them drift are gone,
but touching them now would widen this change beyond its five decisions.

## D6 — Requirement-shaped tests through production paths, race-checked

Each capability gets its red test before its implementation, through the production path
(fake-client reconciles for the controllers, the real `PruneResource` call chain for the stamp,
the real pool + a Close spy for eviction, admission/build rejection for the engine). The changed
packages run under `-race` per the repo's stateful-change rule (`task coverage:race` subset:
`./internal/controller/... ./internal/sink/git/... ./internal/collect/... ./internal/sink/...`).
Integration-tier rows (`task test-integration`, Docker) are named and `not-run` with reason if
Docker is unavailable here; CI owns them on the PR.

## D7 — Helm `mode`: decided by the captain — delete the value

The wiring target was undefined, and any wiring either duplicates or contradicts `tenantMode`:

- (a) **Delete the value + schema entry** (+ docs rows). Matches the value's history: it was the
  hub/spoke transport-era knob (`single|hub|spoke`, commit `5ee4ef89`), whose pair (`transport`)
  and purpose were abandoned (ADR-0501 runs fleets as N single-mode operators). Smallest
  truthful change; removes the schema's lie.
- (b) **Fold `tenantMode` into `mode`** (`mode: single` ≡ `--tenant-mode` + namespaced RBAC;
  `mode: cluster` ≡ today's default). A real single-knob design, but it flips the documented
  default (`mode: single` today while the operator default is cluster-scoped) or adds a
  migration, and it rewrites the chart's public surface (schema, README template, unit tests,
  e2e overlays, `helm-values.md`).
- (c) **Keep the value, narrow the schema enum to `["single"]`.** The schema stops admitting
  `cluster` (the defect the review filed), the descriptive knob survives for future modes.

**Decision (captain, relayed 2026-10-07T07:19Z on the open needs-decision
`[key=helm-mode-wiring]`): option (a)** — the value is a dead hub/spoke transport knob with no
consumer and an undefined wiring target; the docs already say single-cluster only. Implemented
in T14: `charts/kollect/values.yaml` value + comment, `values.schema.json` entry,
`README.md.gotmpl` sample blocks, `docs/operator-manual/helm-values.md` row, chart README
regenerated (`task helm-docs`). `tenantMode` stays the RBAC/cluster-scope knob it already was.
