## Verdict: CLEAN

## Findings

Spec delta `specs/lint-toolchain/spec.md`, each requirement checked against the tree and the binary, not the evidence prose:

- **LTB-1 (equal pins, ≥ v2.13.1) — HOLDS.** `Makefile:184` = `v2.13.1`, `hack/tooling/.custom-gcl.yml:6` = `v2.13.1`; `git log 3ee21266..HEAD -- Makefile hack/tooling/.custom-gcl.yml .golangci.yaml` shows only `5579b33f` touched either pin site, so the merged tree never held a mismatch. No CI guard exists for the "never differ" clause — the spec itself assigns the drift test to change 5 (DTP-3), so this is spec-conformant, not a gap in this change.
- **LTB-2 (no loosening; reasoned nolints) — HOLDS.** Range diff of `.golangci.yaml` is empty (verified myself). Exactly two *new* `//nolint` directives in the range — `internal/controller/reconcile_guard.go:36` (staticcheck) and `internal/sink/gitlab/client_test.go:135` (gosec G710) — both carry the reason on the same line (verified vs base `3ee21266`, where `reconcile_guard.go` had none and `client_test.go` had zero nolints; the test-file directive at `reconcile_guard_test.go:32` pre-existed the range). Fix commits (`b1d32712`, `55bc0d38`, `e2941062`) are all after the version-only commit, one per rule group, as the scenario requires.
- **LTTB-3 (observable bump) — HOLDS.** Version-only tree ran `task lint` to 57 findings with a starting binary (evidence/1.2.md row 6, exit 201 = runnable-not-clean); before/after (57 → 0), fixed (55 = 44 goconst + 11 SA1019) and justified (2 = guard staticcheck + gosec) recorded in evidence/1.3.md row 8 — arithmetic I re-derived independently from the nolint inventory and the 57-line probe list; v2.11.4 baseline of 0 in probe.md §1. I ran `go version -m bin/golangci-lint` myself: `mod v2.13.1`, matching the pin.
- **LTB-4 (plugin does not float) — HOLDS, stronger than probe evidence claims.** `.custom-gcl.yml:11` = `version: v0.10.1`, recorded in probe.md §8 where the "build used v0.10.1" sub-claim was only *believed*; my `go version -m` shows `dep sigs.k8s.io/logtools v0.10.1` in the actual binary, closing that epistemic gap.

Not mentioned by any requirement:

- [NOTE] Lint "fix" ships a runtime behaviour change at 11 production reconcile sites — `internal/controller/finalizer.go:21`
  Failure: `Requeue: true` (rate-limiter exponential backoff) → `RequeueAfter: 1s` (fixed, bypasses the limiter); a persistently contended object now polls at 1 Hz per object forever. Spec LTB-2 only demands "fixed or justified", which this is — but no requirement covers behaviour preservation, and only 2/11 sites are test-pinned. Mitigation is real (watch re-enqueue argument, shared constant, named in const comment and both review legs).
  Fix: none required; keep the monitoring note from the L-1.3 register.
  Confidence: 85
- [NOTE] The goconst fix introduced a coupling of the same class the branch later decoupled in metrics — `internal/collect/prune.go:125` reuses `namespaceField`/`configField` (defined `helmdecode.go:23-24` for helm-release aliases) to drop k8s envelope keys; a helm-driven change of the constant's value silently re-scopes envelope pruning. Qwen's L-1.3 leg flagged it; the register (WARNINGs-only merge policy) dropped it and `c9e795bb` decoupled `LabelProfile` but not this.
  Fix: one-line own constant in `prune.go` (or a comment), mirroring the `LabelProfile` treatment.
  Confidence: 70
- [NOTE] v2.13.1 emits a `gomodguard` deprecation warning every run; migration deferred to the archive record — observed in evidence/1.2.md, no requirement covers it. Harmless today.

## Could not check
- Did not run `task lint`, `task coverage` or `go test` myself (read-only run); relied on the recorded sensors plus the two L-1.3 legs' independent re-runs (0 findings, package tests green at HEAD).
- The pre-fix 57 count and the 12 uniq-by-line-hidden goconst positions (needs a base-tree checkout).
- CI green and the B-stage branch review — `reviews/B-branch/` is untracked, not in the commit range.
- `hack/test/*_test.sh` CI guard meta-tests (CI-job steps, not runnable read-only).
