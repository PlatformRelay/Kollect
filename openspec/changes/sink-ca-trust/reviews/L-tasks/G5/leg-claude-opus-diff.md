## Verdict: CLEAN

## Findings
- [NOTE] The C1b doc comment says the CLI "exits 2 without building a backend", but the test asserts that the s3 factory *is* called once and fails. It is the export, not the backend build, that never happens. — `cmd/cli/ca_trust_parity_test.go:42`
  Failure: someone reading the comment could "fix" the `s3Factories != 1` assertion at :177 to 0. That would turn the after-collection proof into a check that passes when the code is wrong.
  Fix: reword to "the backend build fails after collection, so no export runs".
  Confidence: 85
- [NOTE] 5.1 asks C1a to start from a hand-filled `collect.Store`. The test only stubs the export callback (`exportRan`), and this deviation is missing from G5.md *Deviations*. The callback flag does still prove that no export happens, because `withCLIBackend` (`internal/pipeline/wire.go:518-528`) returns before `export`. — `internal/pipeline/ca_trust_parity_test.go:608`
  Failure: no behavioural gap; the evidence just does not match the task text.
  Fix: add one line to the *Deviations* section of `evidence/G5.md`.
  Confidence: 70
- [NOTE] C2 passes one hand-built `BuildContext` to both probe and export. It therefore cannot catch a controller that builds the probe's context differently. Today both controllers use `sink.BuildContextFromSpec` (`internal/controller/family_sink_connection.go:49`, `kollectconnectiontest_controller.go:100`), and C1a exercises that path for S3/GCS. The database-sink CA resolution through `BuildContextFromSpec` is left to G4's tests. — `internal/sink/ca_trust_parity_test.go:719`
  Failure: a future controller-level change that drops `CAPEM` for the database probe would pass C2.
  Fix: optional. Derive `build` from `BuildContextFromSpec` with a fake client, as C1a does.
  Confidence: 55
- [NOTE] C1b is the only end-to-end guard for "CLI exits 2 after collection", and it silently `t.Skip`s when envtest assets are missing. — `cmd/cli/ca_trust_parity_test.go:51-53`
  Failure: a CI runner without assets reports green, and C1b never runs.
  Fix: none for this group (`TestPipelineCLI_collectAndWrite` uses the same pattern). Check in the FINAL run that it prints `=== RUN`, not `SKIP`.
  Confidence: 60

Checked, and holding:
- **C1b's prefix check rules out the wrong reasons for exit 2.** Requiring `build sink backend: ` (`wire.go:501`) excludes both other fatal paths: "resolve sink credentials" (`wire.go:491`) and "collect for context" (`wire.go:459`). The `--output` control run proves that collection succeeds.
- **A stale result cannot pass a case.** `results` is not reset between subtests, but a leftover control result has `Fatal == nil`, so the `errors.Is` check would fail.
- **Errors are compared by identity throughout.** Every comparison uses `errors.Is` against the shared sentinels, plus `IsTerminal`; none compares error text.
- **Mutation 1 (CLI `resolveCAPEM` bypasses the shared resolver) is covered.** It is killed by R2 in `sink_credentials_test.go`, which 5.1 permits ("R2 or C1 fails").
- **C3 is in the integration gate.** `internal/pipeline` is in `INTEGRATION_PACKAGES` (`Taskfile.yml:20`).
- **No secrets or fixtures are committed.** Certificates come from `sinktlstest.NewCA`, and there is no PEM or private key in the diff. `AKIDTEST`/`test-secret` are dummy placeholders.
- **The swapped package var and `t.Setenv` are parallel-safe.** None of the new tests calls `t.Parallel`.
- **Both recorded deviations are justified.** C1b belongs in `cmd/cli` because `exitCodeForContext` is in package `main`. The C2 sslmode case was replaced because of the recorded unset→verify-full decision.

## Could not check
- I ran no tests or mutants (read-only leg). Matrix results come from the recorded `evidence/G5.md` only.
- The mutation harness `/tmp/g5/mutate.py` and `/tmp/g5/orig/` are outside the repository, so I could not confirm the mutant diffs or that each one was killed by an assertion rather than a build failure.
- `agent-context/decisions.md` (the 2026-10-19 unset→verify-full decision behind C2's deviation): I did not read it, so I took the decision as stated.
- `internal/sink/probe.go`, `registry.go`, `build_context.go`, `ca_source.go`, and the s3/gcs/postgres/mongodb constructors: I did not read their bodies. That every case returns a terminal error, rather than one wrapped in a transient class, rests on the recorded test passes.
