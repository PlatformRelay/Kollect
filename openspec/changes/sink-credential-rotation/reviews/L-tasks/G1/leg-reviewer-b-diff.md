## Verdict: CONCERNS

## Findings
- [WARNING] No test covers the cluster-inventory export path passing `SecretReader` — `internal/controller/kollectclusterinventory_controller.go:341`
  Failure: if `SecretReader: r.SecretReader` is deleted on that line, every test still passes. The reader then defaults to the cached `Client`, so a cluster-inventory export of a cross-namespace Secret under a restricted cache fails with "unknown namespace for the cache" again. The envtest specs only drive `KollectInventoryReconciler`, and `TestInventoryReconcilers_defaultSecretReader` only checks the cluster reconciler's default and its cleanup path. Matrix row 2 has an "export" row, but it calls `RunExportEnvelope` directly and never goes through a reconciler.
  Fix: add a cluster-inventory export row to the wiring test, with a stale `Client`, a fresh `SecretReader` and a recording registry.
  Confidence: 75
- [WARNING] Every acquire now GETs each referenced Secret uncached before the pool lookup, so a pool hit can now fail — `internal/sink/backend_pool.go:162-166`
  Failure: a namespace cascade deletes the sink's Secret before the inventory finalizer runs. Before this change, a pooled hit let `RunCleanupExport` (`cleanup.go:186`) finish without reading any Secret. Now `secretMeta` returns `secret "x" not found`, the error is transient, and the finalizer retries indefinitely. The same applies to exports during an API-server blip. The K-29 test (`kollectinventory_finalizer_test.go:299`) only covers a missing *sink*. G2's NotFound work keeps this error transient (evidence "Notes for later groups"), so G2 does not remove the risk.
  Fix: either record in `design.md` that this regression is accepted, or on the cleanup path fall back to an existing pooled entry when the metadata GET fails. Add a test for "Secret deleted before inventory finalizer".
  Confidence: 50
- [WARNING] The evidence says closed but its own sensor rows are still pending — `openspec/changes/sink-credential-rotation/evidence/G1.md:19`, `:46`, `:63-64`, `:166-172`
  Failure: `Status: CLOSED`, yet the race run of `internal/controller` with `-ginkgo.repeat=2` and the FINAL suite are marked `pending`. Row 19 points at those pending rows. *Independent review* and *Verdict* both say `Pending`. Someone reading the status line would think race-freedom of the controller wiring is proven, and it is not.
  Fix: set the status to IN-REVIEW, or record the exit codes of those two runs before closing.
  Confidence: 90
- [NOTE] The rebuild log line fires before `NewBackend` can fail — `internal/sink/backend_pool.go:209-213`
  Failure: rotated credentials that the factory rejects log "rebuilding sink backend" on every export, but nothing is rebuilt. The line is noisy and misleading. It contains no Secret values (only `namespace/name`).
  Fix: log after `reg.NewBackend` succeeds.
  Confidence: 60
- [NOTE] Two concurrent builds: the last one to store wins, even if it was built from an older snapshot — `internal/sink/backend_pool.go:270-279`
  Failure: build A (older resourceVersion) stores after build B (newer). B's backend is closed while a caller may still be using it, and the pool keeps the stale credentials until the next acquire rebuilds. The next acquire does fix it. This is G3's store-fence scope.
  Fix: G3, as planned.
  Confidence: 55
- [NOTE] The core logic is correct (checked against the old code):
  - **Same Secrets as before:** `referencedSecretRefs` (`secret_refs.go:46-86`) lists exactly the Secrets the removed `populate*`/`override*` helpers read. Namespace defaulting is the same. Kafka and NATS fall back to `spec.secretRef`, git auth applies only for type `git`, and the CA Secret only when there is no `caBundle`.
  - **Override order kept:** kafka, then git, then nats (`build_context.go:106-115`).
  - **Key from metadata only:** the key is built from namespace, name, UID and resourceVersion (`secret_refs.go:199-210`).
  - **One read for key and values:** the stored key and the values both come from the single full read (`versionsOf` + `buildContextFromSecrets`).
  - **All production paths wired:** both exports, both finalizers, the nested prune at `cleanup.go:374`, and the defaults in `SetupWithManager` (`kollectinventory_controller.go:681`, `kollectclusterinventory_controller.go:859`).
  - **RBAC:** `get` on secrets is already granted (`config/rbac/role.yaml:12-14`).
  - **Tests:** `keyFromBuildSnapshot` really tells the two reads apart (`skewReader`). The delete-prune row would catch a missing reader on the nested call, because the stale and fresh fake clients have different resourceVersions.

## Could not check
- I ran no tests and no mutants (this leg is read-only). The red, green and mutant exit codes are taken from `evidence/G1.md`.
- I did not check `design.md` or `specs/sink-secret-rotation/spec.md` to see whether the per-export uncached GET and the failure on a deleted Secret are explicitly accepted there.
- I did not trace error classification (`ExportErrorReason`, `ClassOf`) for the new `read secret …: %w` wrapping beyond confirming the `%w` chain is kept.
- `internal/sink/git` release internals and the `retractionPrecheck` paths that run before the acquire in cleanup.
