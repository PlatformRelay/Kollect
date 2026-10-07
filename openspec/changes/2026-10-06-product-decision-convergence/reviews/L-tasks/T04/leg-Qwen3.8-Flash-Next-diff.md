## Verdict: CLEAN

## Findings
- [NOTE] The probe test re-implements the skip condition instead of reusing the package's `skipWithoutGit`, so the two file:// files can drift if the skip criteria ever change — `internal/sink/git/file_remote_convergence_test.go:67` vs `internal/sink/git/mirror_branch_test.go:17`
  Failure: a future change to `skipWithoutGit` (e.g. a git-version floor) leaves this test running on hosts the rest of the package skips.
  Fix: call `skipWithoutGit(t)` (note the subtest structure needs the call before `t.Run` or in each subtest).
  Confidence: 90
- [NOTE] The admission red pins `len(errs) == 1` exactly, so T09 must reject `cli` with exactly one field error; a T09 that emits a second related error (e.g. an engine+default combo warning as an error) turns this guard's red into a wrong-reason red — `internal/validation/engine_convergence_test.go:22`
  Failure: state after T09 with two field errors → test still fails though the spec claim ("rejects naming go-git") holds.
  Fix: acceptable as-is (one `field.NotSupported` is the only spec-shaped outcome); if loosening, assert `len(errs) >= 1` and the naming on the engine-path error.
  Confidence: 55
- [NOTE] The branch intentionally carries four failing tests until T09 (branch test/CI red by design); verified this is the change's sanctioned pattern, not an oversight — `openspec/changes/2026-10-06-product-decision-convergence/loop.md:60`, evidence "Known red" section. The 19 `Config{Engine: GitEngineCLI}` fixtures were correctly left untouched.
  Confidence: 85
- [NOTE] Order guard tolerates a duplicated existing name in the offer (map keeps last position); cannot demote the eight's relative order, only over-count one name's position — `internal/sink/git/kex_convergence_test.go:71-75`. Spec asks only for relative order, so this is within spec.
  Confidence: 40

Checked (context, not just diff): all four reds fail for their stated reason at HEAD — validation switch accepts `cli` (`internal/validation/git.go:88-96`), `applyGitSpec` maps `cli` (`internal/sink/git/config.go:171-178`), CRD enum is `[go-git cli]` in both copies (`api/v1alpha1/kollectsink_types.go:181` marker, `config/crd/bases/…:144-146`), KEX pin is exactly today's eight in the guard's order (`internal/sink/git/ssh_auth.go:27-36`, set on `ClientConfig` at `ssh_auth.go:60`). All five guards are green-by-construction: `ClassifyExportError` preserves the `git ls-remote failed:` substring for non-auth/non-transient errors (`internal/sink/git/errors.go:20-45`), file:// routes to `lsRemote` (`connection.go:35`). Every new test is engine-less (string literals only, no `GitEngineCLI`/`Config.Engine`), so all survive T09's removal of the internal `GitEngine` type and constants while the API `GitSpec.Engine` string field persists; no name collisions with existing helpers/fixtures; signatures of `ExportFilesWithBranch`, `DeleteExportWithBranch`, `withDefaults`, and all reused fixtures match; no production file touched; no secrets (insecure host-key callback only via the existing `InsecureSkipVerify` test flag).

## Could not check
- Did not execute `go test` (plan-mode read-only): red/green statuses verified by reading HEAD code against the evidence log, not by running the suite.
- Did not verify `charts/kollect/crds` enum block line-by-line (grep-counted only); the subtest itself asserts it.
- Did not verify CI behaviour of the branch carrying known-red tests beyond loop.md's "Known red" section.
