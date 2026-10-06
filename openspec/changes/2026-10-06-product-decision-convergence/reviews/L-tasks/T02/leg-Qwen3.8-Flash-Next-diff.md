## Verdict: CONCERNS

## Findings
- [WARNING] The commit ships API types whose committed CRD manifests are now stale, and the repo's CI `verify` job runs `task verify` on every PR touching `api/` — the loop.md "pin to T07" is a local convention CI does not read — `hack/verify.sh:22` (`make generate manifests` + diff of `config/crd/bases`, `charts/kollect/crds`), `.github/workflows/ci.yaml:216-230`
  Failure: any PR opened at this branch state (before T07 lands) gets a red `verify` check from CRD drift, independent of the intentional test reds; a cluster upgraded from this SHA prunes `status.collectedCount`/`collectedCountUpdatedAt` (absent from the shipped CRDs) — harmless only because nothing reads them yet. The evidence's "not-run: loop.md pins it to T07/T09" row (T02.md:72) records the local pin but not the CI exposure, and the Harness-gaps section doesn't either.
  Fix: run `make manifests` here (it is one `+`-only diff, same argument the evidence uses for regenerating deepcopy), or record the CI-verify red explicitly in T02.md Harness gaps so it isn't a surprise at PR-open.
  Confidence: 85 — verify.sh and ci.yaml read; not verified whether the GitHub ruleset lists `verify` as a required context (would only raise severity, not change the fact).
- [NOTE] Red failure messages print `%v` on `*int64`/`*metav1.Time` — once T07 makes the assertion reachable with a non-nil wrong value (say the bug persists 16), the diagnostic prints a hex pointer, not the number — `internal/controller/kollectclustertarget_collected_count_test.go:132,172,208` and the Degraded test
  Fix: dereference in the message (`%d` with a nil-guard, or a `countStr(stored.Status.CollectedCount)` helper).
  Confidence: 90 (cosmetic; affects only post-T07 failure legibility).
- [NOTE] Scope drift in bookkeeping: T02.md:6 lists "tasks.md tick" among files in scope, but the commit carries none — `tasks.md:20` is still `- [ ]` and T02.md itself is still "Status: FRAMED / Verdict: (pending)" at the final tree — `openspec/changes/2026-10-06-product-decision-convergence/tasks.md:20`
  Failure: a reader of tasks.md sees T02 open while the tests it promises exist on the branch; the verification matrix and tasks.md disagree.
  Fix: tick T02 (and set evidence Status) in this commit or the close commit, and state which.
  Confidence: 70 — the loop may intentionally tick only at CLOSE (T01's row in loop.md shows a close-time pattern); if so, drop the "tasks.md tick" claim from the evidence's files-in-scope line instead.

## What I checked (diff + context reading)
- Field parity with the namespaced anchor: pointers, json names, semantics, `+optional`, printcolumn markers vs `api/v1alpha1/kollecttarget_types.go:86-116` — identical shape; marker block placement matches the existing pattern controller-gen already accepts in this file.
- `zz_generated.deepcopy.go` insertion matches struct field order, nil-guarded, correct `metav1.Time` copy; the outer `DeepCopy()` template calls `DeepCopyInto` so it needs no change.
- The red is exactly the missing persistence: read `setReady`/`setDegraded`/`persistFilterStatusIfSkipped` (`kollectclustertarget_controller.go:261-330`) — setReady derives the count for the message and never stores it; setDegraded never touches the fields (row-6 green-by-construction claim verified at source).
- Byte-identical-condition fixtures build the message through the production `clusterTargetReadyMessage` via the test helper — cannot drift from what setReady writes.
- `collect.NewEngine(nil, nil, ...)` is safe for these tests (ItemCount reads only the store; no Start, no RefreshNamespaces on the driven seam); no duplicate test symbols across the `controller` package; `WithStatusSubresource` fakes preserve seeded status so the Degraded guard is genuinely seeded, not vacuous by loss.
- Commit message, task text (tasks.md:20-25), spec delta scenarios (specs/target-status/spec.md), and evidence matrix rows 1-8 are mutually consistent; no allow-list, baseline or exclusion is widened anywhere.

## Could not check
- Ran nothing (read-only review): the 4 reds + green guards, `task lint`, `make generate` regen-clean, and the `go vet/build` claims are taken from reading + the evidence, not executed.
- `task verify` itself (would create scratch dirs/regenerate) — asserted red by reading verify.sh against the missing manifests, not by running it.
- GitHub branch-protection ruleset (which CI contexts are required) — outside repo content.
- The 6 ERA-1 known reds, integration tier (no Docker), rendered `kubectl get` columns (manifests stale until T07), and anything outside this repo (sibling worktree, prior review legs).
