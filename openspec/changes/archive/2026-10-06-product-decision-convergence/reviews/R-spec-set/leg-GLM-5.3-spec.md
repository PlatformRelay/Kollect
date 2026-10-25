I'm reviewing this spec set. Let me read the change files directly (skipping the `reviews/` subdirectory — other legs' output is out of bounds for me).
The git-engine delta defines only GTE-1..GTE-3, yet the proposal promises GTE-1..GTE-4. Let me check the other three deltas and the source of the product decisions (report §7).
The report path cited by the proposal doesn't exist at `data/` in this checkout. Let me locate it and check the repo layout.
The tracker matches D1's cited structure (two axes: generation, checksum). Now the call sites and controllers.
Debounce call sites match the proposal (3 sites incl. preview). Now the cluster-target status write, family-sink controller, and pool.
## Verdict: CONCERNS

## Findings

- [CRITICAL] Proposal enumerates 11 requirement ids (ERA-1..ERA-3, TSP-1..TSP-2, BEP-1..BEP-3, GTE-1..GTE-4) but the four deltas define only 7; ERA-3, TSP-2, BEP-3 and GTE-4 exist nowhere — `proposal.md:52-55`
  Failure: archiving/validating against the proposal leaves four dangling ids; the git-engine capability line promises "its **auth modes**" as scope, yet no requirement pins auth-mode behaviour (ssh/token auth through `buildAuthMethod`) across the engine-removal in T09 — an auth-regression class no scenario would catch.
  Fix: either correct the four enumerations to the defined ids, or restore the missing requirements — the substantive one being GTE-4 (auth-mode parity under one-engine convergence).
  Confidence: 92

- [NOTE] T06's doc step "narrow the ANNOTATIONS-LABELS.md row to the Resource-mode copy" is already satisfied at HEAD; the row already reads "Exported source objects (metadata)" — `tasks.md:38`, `docs/ANNOTATIONS-LABELS.md:100`
  Failure: none — work already done in a prior commit; task as written is a no-op for docs.
  Fix: reword T06 to "verify row states Resource-mode semantics" or drop the doc clause.
  Confidence: 85

- [NOTE] The `requestedAt` doc row ("On: Reconciled Kollect CRs") stays over-broad — D1 explicitly excludes `KollectTarget` ("an annotation there would promise something nothing honours"), and unlike GTE-3 for git, no requirement or scenario pins the narrowed doc row; T05's "docs rows in ANNOTATIONS-LABELS.md (ERA-1)" is unpinned — `docs/ANNOTATIONS-LABELS.md:101`, `design.md` D1
  Failure: implementer updates only the Effect text, doc keeps promising a trigger on every reconciled Kollect CR.
  Fix: one sentence in T05: narrow the "On" column to the two inventory kinds.
  Confidence: 70

- [NOTE] Negative clauses lack test pins: BEP-1's "SHALL NOT re-export, retract, or reconcile" and "nothing logged as an error" are not asserted in T03 (which tests only entry-gone/Close/no-op/spec-update), and D1's preview-honesty (preview after a `requestedAt` change must not report a debounce the export will not do) has no scenario — `tasks.md:22-25`, `design.md` D1
  Failure: delete-watch implementation that also fires on update events or logs errors on missing keys would pass the planned tests.
  Fix: extend T03 with a no-re-export and no-error-log assertion; give ERA-1 a preview-path scenario.
  Confidence: 60

Spec-lens verification done against code at HEAD (all cited facts checked, not trusted): zero reads of `collectedGeneration`/`requestedAt` under `api/ internal/ cmd/` (rg exit 1) supports ERA-1/ERA-2; `perSinkCoalesceTracker` is exactly the two-axis structure D1 extends, with all three call sites at the cited lines (`kollectinventory_controller.go:356`, preview ~487, `kollectclusterinventory_controller.go:305`); namespaced fields + printer columns exist (`kollecttarget_types.go:96-116`) while the cluster target has none and counts only in prose (`kollectclustertarget_controller.go:309`, no `CollectedCount` in its types file); the no-caller pool comment and both eviction functions are real (`backend_pool.go:33-35,227,236`, both UID- and ns/name-keyed as BEP-1 needs, `FamilySinkReconciler.SetupWithManager` has only `For()` — hook point as D4 says); the KEX pin is the claimed 8-name in-repo list passed to `ssh.Config{KeyExchanges}` (`ssh_auth.go:23-33,59`); all three `engine: cli` accept points exist (`kollectsink_types.go` enum, `validation/git.go:88-96`, `config.go:170-177`) with branch points at `export.go:125`, `delete.go:107`, `cli_env.go:95`; the stale CRD-doc claim is at `docs/crds/kollectsnapshotsink.md:22`; the pipeline image truly ships no git binary (`Dockerfile.pipeline:33-35`); `PruneResource` sits at `prune.go:40`, called only from the Resource-mode branch (`engine.go:957`), so the Attributes-mode byte-identity scenario holds structurally; `make generate manifests` and the Taskfile gate targets exist. No design decision contradicts a code fact. Stronger-than-spec: none found.

## Could not check

- `data/kollect-xconsol-final/report.md` §7 and `data/kollect-xconsol-b/report.md` — the `data/` directory does not exist in this checkout; the five product decisions are verified only via the proposal's own restatement, not against the primary source.
- x/crypto v0.57.0 implementation of `mlkem768x25519-sha256`/`diffie-hellman-group16-sha512` (module cache is outside this repo; only the `go.sum` pin verified) and the controller-runtime DeleteEvent-UID assumption.
- Execution of `task spec:validate`, `task verify`, tests or builds (read-only review; no commands that write).
- The `reviews/` subdirectory of the change — other reviewers' output, out of bounds by brief.
