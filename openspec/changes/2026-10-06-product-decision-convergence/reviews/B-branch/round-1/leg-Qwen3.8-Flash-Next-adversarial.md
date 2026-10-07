## Verdict: CONCERNS

## Findings

- [WARNING] The upgrade note promises `engine: cli` construction "fails terminally"; the code classifies it Transient — `docs/operator-manual/upgrading.md:252` vs `internal/sink/git/config.go:170`
  Failure: a persisted sink with `engine: cli` hits `ConfigFromSpec` → plain `fmt.Errorf` (`internal/sink/git/backend.go:26`) → `ClassifyAPI` default branch returns Transient (`internal/errors/errors.go:136`), so every export requeues and retries the deterministic rejection forever (metrics/event churn), while the branch's own upgrade doc — the GTE-3 truth-up deliverable — states it fails terminally. The prior-round T09 HIGH was the same wording class and was fixed; this one survived.
  Fix: return `kollecterrors.Terminal(...)` from the engine rejection in `applyGitSpec` (or amend the doc sentence to match the transient reality).
  Confidence: 70 (read the full error path; did not run a live sink to observe requeue behaviour)

- [WARNING] `.github/release-notes-install.md` still advertises the CLI engine after convergence — `.github/release-notes-install.md:7` (and `:29`)
  Failure: a user reading the release notes sees the operator image ships `git`/`openssh-client` "for `spec.git.engine: cli`" and writes an `engine: cli` sink → admission/build rejection loop post-upgrade. The GTE-3 verification row claims "all engine: cli sites truthed up"; this site (plus the pipeline line phrasing cli as a still-nameable value) makes that claim false. The Dockerfile comments it mirrors were truthed; this one was missed.
  Fix: two-line edit — name the shared file://CLI/ls-remote machinery instead of `spec.git.engine: cli`, as `Dockerfile:34-36` already does.
  Confidence: 90

- [NOTE] `Pool.Close` runs inline on the informer's event-delivery goroutine — `internal/controller/family_sink_controller.go:89-93` + `internal/sink/backend_pool.go:305-311`
  Failure: controller-runtime dispatches watch events synchronously to handler listeners; a backend whose `Close` blocks against a black-holed endpoint stalls every subsequent sink event for that kind (the code acknowledges this hazard for the pool mutex, not for the informer). Survives the defence-attack: all in-repo `Close` implementations are short and bounded today.
  Fix: if latency ever shows up, hand `closeBackendLogged` to a one-shot goroutine per eviction; the exactly-once invariant is already lock-guarded.
  Confidence: 60

- [NOTE] Delete-tombstones age only when someone next acquires — `internal/sink/backend_pool.go:241-245`
  Failure: a controller that observes deletes then goes permanently export-less never calls `pruneStaleEntriesLocked`, so tombstones live for the process lifetime. Bounded and small (one short key per delete); no correctness impact — the "no rebuild for a dead UID" guarantee only gets stronger by ageing later.
  Fix: none required; optionally prune tombstones on a periodic tick if the pool ever grows an idle path.
  Confidence: 80

- [NOTE] TSP-1 leaves an open owner-decision request: the cluster count refreshes per reconcile while the namespaced path keeps its own cadence — `openspec/changes/2026-10-06-product-decision-convergence/loop.md:120`
  Failure: "same semantics as KollectTarget" holds for fields/scenarios, but a fleet operator comparing `kubectl get` output across scopes sees different update timing. Honestly recorded, not hidden — flagging so the decision does not die in loop.md.
  Fix: document the difference in `docs/crds/kollectclustertarget.md` or align, per the recorded request.
  Confidence: 85

What the adversarial pass actively tried and could not break (evidence of the attempt, not assertion): tombstone/evict interleavings (entry-delete + tombstone-set atomic under `mu` — `internal/sink/backend_pool.go:300-313`; store-after-delete discarded; double-Close unreachable; ns/name tombstone cannot block a recreated sink because all production acquirers pass a live-object UID — `internal/sink/export.go:138`, `internal/sink/cleanup.go:184`); controller-runtime v0.24.1 `DeletedFinalStateUnknown` unwrap (verified in the actual module cache, `pkg/internal/source/event_handler.go:132-146`, and nil-`Funcs` no-ops); the KEX offer strings (`mlkem768x25519-sha256` / `diffie-hellman-group16-sha512` verified against x/crypto v0.57.0 `ssh/common.go:54,62` and `kex.go` map); the Resource-mode stamp cannot mutate the informer cache (`DeepCopy` at `internal/collect/prune.go:46`) and cannot appear in Attributes mode (`ResourceExportEnabled` gate at `internal/collect/engine.go:952`); the `requestedAt` third axis pins per binding only on success and preview is read-only (`per_sink_export.go:73-96`, `kollectinventory_controller.go:494`); the 565 deleted CLI-mirror regression locks' preconditions are genuinely unreachable post-convergence (file:// gets a fresh temp workdir — `internal/sink/git/mirror.go:210-217`); the generated CRD enum, chart copy, printer columns and golden openapi all carry go-git-only (read, and pinned by `test/schema/engine_enum_test.go` / `printer_columns_test.go` — real sensors on the committed manifests, not hand-waving). No allow-list, baseline or lint exclusion was widened (single new `nolint` is a G304 test-fixture read).

## Could not check
- Ran no tests or gates (read-only review): the Verification table's executed evidence at rev `81bea0b0`, `task verify`/`lint`/`spec:validate`, the `-race` runs, and the 91.3% coverage-at-B claim are read, not reproduced.
- The claimed base `3ee21266` repro of the `TestRecordLabeledMetricSeries_CapsCardinalityDeterministically` non-idempotency (would need a worktree).
- Integration tier (`task test-integration`, Docker) — not-run by the branch itself either; CI owns.
- Live envtest behaviour of the delete-watch spec (needs KUBEBUILDER_ASSETS); I verified its wiring logic by reading only.
- Not every doc row in the sweep diff (only the engine/annotation sites above); did not read the per-task `reviews/L-tasks/*` registers beyond their loop-state summaries (prior reviews out of bounds).
