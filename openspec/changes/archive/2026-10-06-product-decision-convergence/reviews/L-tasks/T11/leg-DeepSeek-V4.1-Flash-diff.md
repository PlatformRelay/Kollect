## Verdict: CLEAN

The classification chain and the tombstone sweep are correct; I found no path where the terminal class is lost or a transient error is wrongly terminal. Findings below are comment/artifact accuracy, not code defects.

## Findings
- [NOTE] The "identical class" comment overstates the `ClassOf`/`ClassifyAPI` equivalence — `internal/sink/export.go:174-179`, `internal/sink/cleanup.go:188-192`
  Failure: a reader trusts "NotFound/Invalid/BadRequest → terminal" from the wrapped chain, but `ResolveSecret` rewrites an API NotFound secret into a plain `fmt.Errorf` (`internal/sink/credentials.go:44-46`), so a missing secret is transient under both old and new code. The conclusion (dropping `ClassifyAPI` is safe) still holds; the cited reason is wrong, and a backend that deliberately wraps an API NotFound in `Transient` would now stay transient rather than being upgraded. No such path exists today.
  Fix: reword to "an already-classified class is preserved; unclassified errors keep `ClassifyAPI`'s mapping through `ClassOf`'s apierrors branch (note `ResolveSecret` rewrites NotFound)".
  Confidence: 85
- [NOTE] `git.NewBackend` classifies the K-14 `--allow-insecure-sinks` refusal as Terminal though it depends on a process flag, not the spec — `internal/sink/git/backend.go:21-34`, `internal/sink/sinktls/sinktls.go:32-34`
  Failure: a git sink with `tls.insecureSkipVerify: true` and the flag off now stops requeueing on the export path (`internal/controller/kollectinventory_controller.go:262-264`, `RequeueAfter=0`, no periodic retry) until a manager restart (full resync) or spec edit. Recoverable, but the doc comment "Every ConfigFromSpec fault is a spec fault" is inaccurate for this one case.
  Fix: exclude `validation.ErrInsecureSinksNotAllowed` from the Terminal wrap, or correct the comment.
  Confidence: 70 (behaviour verified; whether it is undesirable is a judgement)
- [NOTE] Cleanup terminal aggregation is any-terminal-wins, unlike export — `internal/controller/sink_cleanup.go:169`, `internal/controller/inventory_finalizer.go:62`
  Failure: `errors.Join` + `IsTerminal` means one persisted `engine: cli` sink now makes an inventory deletion report `reasonCleanupTerminal` even when another sink failed transiently; the export path deliberately uses an all-terminal rule (`internal/controller/per_sink_export.go:186-201`). Mitigated by `terminalCleanupRequeue` (5 min, `inventory_finalizer.go:30`) and the K-30 escape hatch, so no data is stranded.
  Fix: none required; note the asymmetry or reuse the all-terminal rule.
  Confidence: 75
- [NOTE] The evidence file's Verification matrix contradicts its own Sensors table — `openspec/changes/.../evidence/T11.md:72-92` vs `:100-118`
  Failure: rows 5 and 9-19 are marked `pending` while the Sensors table records the same commands exiting 0 (lint, spec:validate, full suite); Status is still `FRAMED` and Verdict/Independent review are unfilled. The artifact reads as unverified despite the recorded green run.
  Fix: stamp the matrix Result cells and Status/Verdict from the recorded exits.
  Confidence: 95

What I checked: the full chain `git.NewBackend` → `registry.newGitBackend` → `acquireBackend` → `RunExportEnvelope`/`RunCleanupExport` → `aggregateExportErrs` → `setSinkReachableFromExport`/`ExportErrorReason`/finalizer `IsTerminal`; `pruneExpiredTombstonesLocked` callers, the mutex/`timeNow` seam and the TTL boundary (`>`); doc truth against `Dockerfile:34-36`, `Dockerfile.pipeline:32-37`, `docs/operator-manual/upgrading.md:241-268`; `.gitignore:96-97`; the four new tests' assertions, parallel/global-state safety and imports; `.go-arch-lint.yml` (shared `internal/errors` is a common component).

## Could not check
- `task test-integration` — Docker unavailable; CI-owned (matches the evidence).
- I did not re-run `go test`/`build`/`lint`; I read the code, imports, arch config and trusted the evidence's recorded exits.
- The B round-1 register (`reviews/B-branch/round-1/register.md`) — treated as an earlier review, out of bounds.
- The other fanout legs' transcripts (`reviews/L-tasks/T11/leg-*.err`) — not read, per independent-review rules.
- Red-first/mutant claims were not reproduced (would require mutation).
