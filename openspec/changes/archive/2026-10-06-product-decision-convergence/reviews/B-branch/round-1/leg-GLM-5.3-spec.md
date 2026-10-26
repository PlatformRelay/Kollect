I have the spec and the branch shape. Now reading the implementation diffs in full, starting with the core production code.
Core annotations and count parity read clean so far. Now the pool/eviction and the git-engine convergence — the two riskiest areas. Reading current files plus diffs.
Pool eviction and delete-watch wiring hold up under interleaving scrutiny. Now the git-engine convergence surface.
Engine convergence surface is consistent. Now verifying the remaining ripple: namespaced target refactor, PruneResource call sites (Attributes-mode leakage risk), CRD artifacts, and residual `engine: cli` references.
Potential ERA-2 problem: the only PruneResource call site assigns into `attrs[...]`. Reading the surrounding mode logic.
The stamp is gated behind `ResourceExportEnabled()`. Checking the include default and the Resource-mode predicate — this decides whether ERA-2's core scenario holds for the default profile.
Checking the Ready message and condition-writer to judge whether the TSP-1 escape hatch is reachable, plus the remaining doc sites for GTE-3.
One residual `engine: cli` claim found in `.github/release-notes-install.md`. Checking whether it's a live install template or frozen history, then the condition writer and Ready message for the escape-hatch reachability question.
Escape-hatch reachability resolved (nil→value with identical message is the live case). Now the debounce semantics, interval==0 meaning, and the red-test files.
Tests confirm the documented default-include divergence is pinned deliberately. Now the remaining verification batch: envtest gating, delete-watch tests, pool delete-hook tests, callers of acquireBackend (UID resolution), and ADR-0803.
Final checks: the cluster count source, and the tombstone/owning-release tests at the tail of the pool test file.
## Verdict: CONCERNS

Spec-lens verification of `3ee21266..HEAD` (30 commits) against the OpenSpec delta ERA-1/ERA-2, TSP-1, BEP-1/2, GTE-1/2/3, checked against code, not against the loop's evidence files. ERA-1, TSP-1, BEP-1, BEP-2, GTE-1 hold as specified; the two findings below are the residue.

## Findings

- [WARNING] `.github/release-notes-install.md:7,29` still advertises `spec.git.engine: cli` as an image-supported value — `.github/release-notes-install.md:7`
  Failure: the file is a live per-release template (`${IMAGE_REPO}:${TAG}` placeholders); the next release notes will say the operator image ships `git`/`openssh-client` "for `spec.git.engine: cli`", a value this branch deletes everywhere else — falsifying ADR-0803's own bullet "every in-repo site asserting `engine: cli` works is truthed up" (`docs/adr/0803-git-engine-convergence.md:45`). GTE-3's SHALL enumerates five site classes and holds for each; this site sits outside the enumeration, so the spec technically passes while the ADR claim does not.
  Fix: two-line edit of the template (line 7 → "for `file://` remotes and `git ls-remote` probes"; line 29 → drop the `git.engine: cli` mention).
  Confidence: 85

- [WARNING] ERA-2 scenario 1 does not hold for a default-include Resource-mode profile — `internal/collect/prune.go:96-98`
  Failure: spec scenario 1 says a profile with `export.mode: Resource` and generation 42 yields a stamped copy, unqualified; but the default include `SpecAndStatus` drops `metadata` entirely (`selectIncludeSections`, prune.go:137-139), so `stampCollectedGeneration` returns without stamping. The stamp only lands with `include: All`/`MetadataOnly`. The branch documents this divergence in three places (constants.go comment, ANNOTATIONS-LABELS row, dedicated test `TestPruneResource_defaultIncludeCarriesNoStamp`) but never folds it back into the spec delta's scenario 1 text, while the Verification table marks ERA-2 simply "pass".
  Fix: one sentence in `specs/export-annotations/spec.md` scenario 1 (stamp requires a surviving metadata map; default include yields none) — docs and code need no change.
  Confidence: 70

- [NOTE] Delete-tombstones age out only on the acquire cycle — `internal/sink/backend_pool.go:135,241-245`
  Failure: `pruneStaleEntriesLocked` runs only from `acquireBackend`; a controller that stops exporting entirely (no acquires) accumulates one tombstone string per sink delete indefinitely. Bounded in practice (tiny strings, only on deletes); no correctness impact since UIDs are never reused.
  Fix: none needed now; prune tombstones in the same hook that evicts if this ever matters.
  Confidence: 55

Requirement verdicts (file:line deciding): ERA-1 holds (`per_sink_export.go:82` third axis before the interval==0 early return; absence↔present both changes; both controllers export+record, preview reads-only, `kollectinventory_controller.go:292,474`; docs scoped to the two kinds). ERA-2 otherwise holds — stamp after scrub (`prune.go:73-79`), generation from source object, gen-0 stamps "0", Attributes byte-identical (sole call site gated by `ResourceExportEnabled()`, `engine.go:952-957`). TSP-1 holds — fields/printcolumns mirrored incl. the load-bearing Age column (`printer_columns_test.go:56-96`), one write site extended to `filterChanged || countChanged` (`kollectclustertarget_controller.go:338`), namespaced semantics refactored into the shared helper without change; note the escape hatch's reachable live case is nil→value with identical message (count is in the message, so a 17→18 move always writes through the condition). BEP-1/2 hold — UID-keyed eviction + ns/name fallback, tombstone discard with owning-release handoff (T08 r2), production acquirers pass resolved UIDs (`export.go:138`, `cleanup.go:370`), delete-only handler, TTL untouched. GTE-1 holds at all three reject points plus removed branch points/type/field; file:// and probe machinery retained.

Not mentioned by any requirement: removal of the CLI×persistent-mirror integration locks (`delete_mirror_regression_test.go`, −592 lines, sanctioned as unreachable by ADR-0803); `cfgNeedsCLISSH` removal narrows CLI-machinery SSH setup to `authType==ssh` (`cli_env.go:44`) — no reachable regression since ssh:// endpoints resolve to AuthTypeSSH; ADR-0104/0407/0415 truth-ups beyond GTE-3's five enumerated classes; a comment-only `.gitignore` addition with no pattern under it.

## Could not check

- No test or gate execution (read-only session): every green/red/`-race`/`task verify`/lint claim rests on reading the tests' assertions and the Verification table, not re-running. The envtest wiring spec for the delete watch and the integration tier are unexercised.
- x/crypto v0.57.0 actually implementing `mlkem768x25519-sha256`/`diffie-hellman-group16-sha512` (module cache is outside this repository; in-repo tests pin only the offer list's contents and order).
- controller-runtime v0.24.1 `DeletedFinalStateUnknown` unwrapping internals (module source out of bounds); the seam is defensive either way (UID empty → ns/name fallback).
- Whether a sntrup761-only SSH server exists for any user (accepted residual, no in-repo evidence either way).
