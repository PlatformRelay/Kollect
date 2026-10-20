## Verdict: CONCERNS

## Findings
Requirement status (verified against the code; nothing is implemented yet, the branch holds only the spec set): SCR-1 to SCR-6 are all **absent** in code, as expected for a spec-only change. The base-code defects the proposal describes are real: `backend_pool.go:136-141` (cache hit on the spec hash alone, no-op release), `:145` (Secrets read only on a miss), `:208-214` with `:156-158` (immediate close on a spec-hash change), `credentials.go:51-68` (no empty check), `s3/config.go:65-79` with `backend.go:159-163` (ambient chain). The eight reference fields in SCR-1 match every `*SecretReference` in `KollectSinkSpec` (`kollectsink_types.go:36,201,262,278,308,324,358`; `sink_common_types.go:169`). GCS goes through `s3.NewBackend` (`gcs/backend.go:35`). The RBAC claim holds (`role.yaml:7-16`). Every scenario maps to a task row, except as noted below.

- [WARNING] SCR-2 also blocks inventory deletion for Kafka/NATS and database sinks. The compatibility table names only Git Retain. — `internal/sink/cleanup.go:184-214`, `design.md:102`
  Failure: during a namespace teardown the Secret can be deleted before the inventory finalizer runs. `cleanupSinkExports` (`sink_cleanup.go:145`) then calls `RunCleanupExport`. For a stream sink this acquires a backend only to return `CleanupRetained` at `:214`. Under SCR-2 the acquire fails, the inventory stays `Terminating`, and the namespace stays stuck until someone sets `force-cleanup`. K-29 (`sink_cleanup.go:119-126`) covers a vanished sink CR but not a vanished Secret.
  Fix: decide `CleanupRetained` before the acquire for types that cannot retract (no `ExportCleaner`, no `SupportsDelete`). Otherwise add those rows to the compatibility table and the SCR-6 docs, plus a scenario.
  Confidence: 70
- [NOTE] Retiring on "any error" (`design.md:47`) includes a cancelled reconcile context or a transient cache error from one caller. — `design.md:42-48`
  Failure: one inventory's cancelled `ctx` retires and closes the backend that other inventories share, which forces reconnects. This goes beyond what SCR-2 asks (missing or empty Secret only).
  Fix: retire only on not-found, empty, or a failed rebuild. Return other `Get` errors without retiring.
  Confidence: 55
- [NOTE] The fingerprint reads metadata in one `Get` and the build reads the data in another. A slow builder from before the rotation can also overwrite a newer entry. — `design.md:31-35`, `backend_pool.go:208-214`
  Failure: G1 reads rv5 and builds slowly. The Secret goes to rv6. G2 stores B(rv6). G1 then stores B(rv5), which replaces and retires B(rv6). The next acquire rebuilds again. The SCR-3 scenario "all receive the same new backend" (`spec.md:115-118`) can flake under `-count=10`.
  Fix: build from the same Secret object the metadata came from. In `storePooledBackend`, do not replace an entry whose key differs from the one the caller saw at its miss.
  Confidence: 50
- [NOTE] SCR-2's no-ambient-fallback guarantee covers only a fully empty Secret. — `spec.md:57-58`, `s3/config.go:65-79`
  Failure: if an operator removes `accessKeyID` and `secretAccessKey` but leaves `region`, the S3/GCS client silently falls back to the pod identity. This is stated as a non-goal (`proposal.md:108`), but the upgrade note reads as a general guarantee.
  Fix: in the SCR-6 note, say "entirely empty" explicitly. Optionally, have S3 reject a non-empty Secret that has neither key.
  Confidence: 60
- [NOTE] The scenario "Rotated CA is trusted by the next export … verifies the server against CA Y" (`spec.md:23-27`) has no test that checks a TLS handshake. It is only partly verified. — `tasks.md:57-62`
  Failure: the `tls.caSecretRef` row checks only that `BuildContext.CAPEM` holds B. A backend that keeps its old `RootCAs` from elsewhere would pass.
  Fix: add one `httptest` TLS server row (S3) signed by CA Y, or reword the scenario to the BuildContext level.
  Confidence: 65
- [NOTE] The SCR-6 check is line-based. The phrase is already wrapped (`kollectsnapshotsink.md:53-54`), so a partial rewrap could leave a stale sentence that `rg` does not match. — `tasks.md:162`, `spec.md:180-183`
  Fix: use `rg -U 'keeps its CA\s+until'`, or search for "keeps its CA".
  Confidence: 60
- [NOTE] Citation drift. — various
  - The Secrets RBAC marker is at `family_sink_controller.go:24`, not `:23`.
  - `defer release()` is at `export.go:109`, not `:111`.
  - It is at `cleanup.go:198`, not `:197`.
  - Task 0.1 says the Git mirror is "keyed by sink", but `mirrorDirFor(cloneURL, branch)` is at `git/mirror.go:149`. Concurrent B1/B2 Git exports are serialized by the process-wide `exportLocks` (`git/export_lock.go:10-25`), so draining adds no mirror race.
  - Task 4.1 asks to remove the CA sentence from `kollecteventsink.md`, which does not contain it. It appears only at `kollectsnapshotsink.md:53`, `kollectdatabasesink.md:70` and `upgrading.md:167`.
  Fix: correct the line numbers and wording.
  Confidence: 90

Not covered by any requirement:
- The design's TTL exemption for held entries (`design.md:60-61`).
- The info log on a rebuild (`design.md:82-85`): SCR-4 constrains its content, but no scenario asserts that it exists.
- The reflection guard (`design.md:25-27`), which matches the scenario at `spec.md:29-32`.

## Could not check
- I ran no tests or probes (read-only). The assumptions in `proposal.md:117-132` (informer-served `Get`, `resourceVersion` semantics, the Close behaviour of the client libraries) were not verified.
- I did not read the BEP-1 tests (`backend_pool_delete_hook_test.go`), `retractionPrecheck`, `ClassOf` (`internal/errors/errors.go:86`), or `cliBuildContext` and `resolveConfigSecret`, beyond the lines quoted above.
- I did not read the untracked `reviews/` directory or the other review legs, by design.
