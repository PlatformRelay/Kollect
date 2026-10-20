## Verdict: CONCERNS

## Findings
- [WARNING] The cleanup paths are not wired to the uncached reader, so SCR-1's "export or cleanup… SHALL read from the API server" is not met for cleanup — `openspec/changes/sink-credential-rotation/design.md:22-27`
  Failure: the design wires `SecretReader` only in "both inventory reconcilers" and falls back to `Client` when it is nil. The cleanup request is built in a different place, `internal/controller/sink_cleanup.go:145-159`, which only receives `c`. The relational prune also builds its own export request at `internal/sink/cleanup.go:369-379` without the reader. So a Postgres or Mongo cleanup after a rotation reads the cached Secret. A cross-namespace `secretRef` under `--watch-namespaces` still fails cleanup with the unknown-namespace error. No 1.1 or 2.1 test covers cleanup reads, and the 1.2 mutation only covers the export path.
  Fix: list `sink_cleanup.go` and `cleanup.go:369` in the design and in proposal Impact, so `SecretReader` is passed through both. Add a cleanup row to `TestAcquireBackend_readsSecretsUncached`, or make a nil reader in production a programming error.
  Confidence: 80
- [WARNING] SCR-2 and SCR-3 contradict each other on whether a failed read after an observed change retires the backend — `specs/sink-secret-rotation/spec.md:73-74` vs `:129-130`, `design.md:70-72` vs `:85-87`
  Failure: say the metadata GET shows a new `resourceVersion` and the full GET then times out or is cancelled. SCR-2 says "SHALL NOT retire". SCR-3 says a failed rebuild after a Secret change "SHALL still be retired". The design's `retireLocked` list also says "key mismatch on acquire" retires. Two implementers can both claim compliance and still disagree, and no test settles it.
  Fix: state that a key mismatch observed by a successful metadata GET retires, whatever the rebuild does. Scope SCR-2's "SHALL NOT retire" to read failures where no change was observed. Add that row to `_cancelledReadKeepsEntry`.
  Confidence: 75
- [WARNING] The planned doc change for a terminal Git auth failure would be false, because terminal errors do not requeue — `tasks.md:200-201`, `docs/operator-manual/troubleshooting.md:204`
  Failure: the troubleshooting row covers Git auth `terminal` (401/403). A terminal export error sets `RequeueAfter = 0` (`internal/controller/kollectinventory_controller.go:262-264`), and nothing watches Secrets. After fixing the Secret, no "next export" runs until something touches the sink, which proposal non-goal `proposal.md:113-116` itself says.
  Fix: keep the touch or re-annotate step in that row. State "next export" only for retryable failures.
  Confidence: 80
- [NOTE] SCR-3 says the TTL path removes a backend from the pool at once; the design says TTL prune skips held entries — `spec.md:125-126` vs `design.md:87-88`
  Failure: the two texts disagree. The scenario at `spec.md:165-168` holds either way, but the requirement sentence does not match the design.
  Fix: either drop "the TTL" from SCR-3's at-once list or have TTL prune retire held entries.
  Confidence: 60
- [NOTE] Step 4 does not fix the order of the fence check and the same-key check — `design.md:53-58`
  Failure: if the fence runs first, concurrent builders that saw the old key all go unpooled. That breaks "they all receive the same new backend" (`spec.md:146`).
  Fix: say that a stored entry with the same key is checked first (share, close the surplus), then the fence.
  Confidence: 55
- [NOTE] The design relies on "the registry's stream types", which do not exist — `design.md:113`
  `internal/sink/registry.go:44-58` has no stream classification. A new set is needed and must stay in step with `cleanup.go:210-214`.
  Confidence: 85
- [NOTE] The fallback to `DefaultSecretNamespace` is attributed to the wrong code — `design.md:33-34`
  `credentials.go:37-40` falls back only to `defaultNamespace`. The `DefaultSecretNamespace` fallback is in `build_context.go:34-36`.
  Confidence: 90
- [NOTE] `referencedSecretRefs` must skip refs with an empty `Name`, which `ResolveSecret` treats as no-op (`credentials.go:33-35`); otherwise the metadata GET errors. Kafka and NATS also list `spec.secretRef` twice when their own ref is nil (`build_context.go:138-141`, `:189-192`). Neither case is stated — `design.md:31-40`
  Confidence: 60
- [NOTE] SCR-5 requires "the same message text" as the operator, but the operator's message includes a namespace, and a config-dir Secret may not have one (`internal/pipeline/wire.go:132-141`) — `spec.md:203-205`
  Fix: say what the CLI prints when the namespace is empty.
  Confidence: 50

Requirement status (none of this is implemented yet; judged as design against current code):
- **SCR-1:** partial, because the cleanup read path is missing (above).
- **SCR-2:** holds as designed, except for the contradiction above. The not-found error is a plain error and comes out transient (`errors.go:86-107`). The S3 fallback is real (`s3/config.go:65-78`).
- **SCR-3:** holds. The immediate close today is real (`backend_pool.go:156-158`, `:208-214`), and pooled hits return a no-op release (`:141`).
- **SCR-4:** holds. Status and Events carry `ExportErr.Error()` (`kollectinventory_controller.go:258-259`), which the envtest checks.
- **SCR-5:** holds. `${env:VAR}` empties are already rejected (`wire.go:195-199`).
- **SCR-6:** the three stale "keeps its CA" lines exist where cited.

Beyond the requirements:
- The design adds a 10 s read timeout that no requirement mentions (`design.md:17`).
- An emptied CA Secret changes from SelectCAKey's error to a transient "has no data". This is not in the compatibility table (`design.md:141-149`).

Coverage and citations checked:
- All 8 `*SecretReference` fields are covered (`kollectsink_types.go:36,201,262,278,308,324,358`, `sink_common_types.go:169`).
- The citations I opened are accurate: `backend_pool.go:43,128-166,183-215`, `build_context.go:28-62,215-243`, `export.go:89-109`, `cleanup.go:184-214`, `wire.go:177-205,515-538`, `kollectinventory_controller.go:262-264,360-368`, `suite_test.go:31`, `mongodb/backend.go:97-105,148-152`, `security-architecture.md:214-216`, `troubleshooting.md:204`.

## Could not check
- Citations into module sources outside the repository: `multi_namespace_cache.go:239`, `pgxpool/pool.go:468-470`, `mongo/client.go:299-306`, `writer.go:551-554`.
- `agent-context/decisions/2026-10-19-rotation-secret-read-path.md`, which is gitignored and outside the review scope.
- I did not open: `internal/sink/nats/backend.go:90-100`, `git/mirror.go:149`, `git/export_lock.go:10-25`, `s3/backend.go:159-163`, `resolver.go:35`, `config/rbac/role.yaml:6-16`, `family_sink_controller.go:24`, `postgres/export_integration_test.go:34-45`.
- Where netguard allows loopback under the integration tag: `netguard/dialer.go:242-248` shows only an unconditional refusal.
- Whether `family_sink_delete_watch_envtest_test.go` exists.
- I ran no tests or sensors, and none were recorded for this change.
