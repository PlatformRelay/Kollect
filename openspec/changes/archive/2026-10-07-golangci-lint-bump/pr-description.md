# golangci-lint v2.11.4 → v2.13.1 + logcheck plugin pin (golangci-lint-bump)

## TL;DR

- **What:** bumps golangci-lint to v2.13.1 in both pin sites (`Makefile`, `hack/tooling/.custom-gcl.yml`), pins the floating logcheck plugin at v0.10.1, and fixes or justifies the 57 findings the new linter reports. Spec: `openspec/changes/golangci-lint-bump/specs/lint-toolchain/spec.md` (LTB-1..LTB-4).
- **Why:** the operator baseline is v2.13.1; the next change (cross-file-consistency-gates) moves `go.mod` to a newer Go and a linter built with an older Go refuses to lint it — the linter must move first.
- **Risk:** low-medium. One real behaviour delta: 11 controller conflict-requeue sites move from the deprecated rate-limited backoff to a fixed 1 s requeue (rationale in the `conflictRequeueAfter` comment; panic path keeps the limiter). `.golangci.yaml` is untouched — no linter disabled, no exclusion added.
- **Reviewer action:** eyeball the requeue delta and the metric label-name rename (values unchanged); the records to trust are `evidence/probe.md` (57→0 with before/after counts) and `loop.md` (two review rounds, triage table).

## The most important things

- **Version-only commit `5579b33f`** touches exactly three version strings; the custom-build downgrade path (vanilla binary) is structurally excluded — a vanilla v2.13.1 fails this config at run start (`plugin "logcheck" not found`, probe.md §4).
- **57 findings → 0**: 55 fixed (44 goconst string hoists + 11 SA1019 deprecation migrations), 2 justified with reasoned `//nolint` (panic-path backoff; gosec G710 in a httptest fixture). A third reasoned nolint lives in the tightened guard test.
- **Review:** per-task diff legs (DeepSeek + Qwen3.8-Flash-Next, both families distinct from the implementer) and two branch rounds (9 legs round 1, 4 legs round 2) — all free internal models; findings and dispositions in `loop.md`.
- **Gates at HEAD:** `task lint` 0 findings · `task format:check` clean · arch-lint OK · full suite green · internal/ coverage 91.3% (floor 90) · markdown/shell/zizmor clean.

## Risk

- The fixed 1 s conflict requeue is a deliberate policy trade, disclosed and accepted by review; the per-site tests (2/11) are pre-existing coverage, recorded as deferred debt in the archive record.
- The goconst hoists created two alias hazards; both are fixed in-branch (`LabelProfile` now genuinely used at all 8 label-name sites; `dropEnvelopeIdentity` owns its own constants).

## Files to check first

1. `internal/controller/finalizer.go` (const comment + the 11 call sites)
2. `internal/metrics/metrics.go` and the label-name sites in `aggregation*.go`, `metrics_catalog.go`
3. `internal/collect/prune.go`, `internal/collect/helmdecode.go`
4. `Makefile`, `hack/tooling/.custom-gcl.yml` (the pins)

## Testing protocol

- From draft to ready: nothing to flip (opened ready for review).
- Local gates already green at HEAD: `task lint`, `task format:check`, `task arch-lint`, full `go test ./internal/...` with coverage (91.3%).
- If you want the downgrade proof: `rm -f bin/golangci-lint* && make golangci-lint && bin/golangci-lint version` reports `v2.13.1-custom-gcl-…`; with the pins reverted, a vanilla binary exits 3 on `plugin "logcheck" not found`.

---

### Deep dive (optional)

**Why the bump can't wait:** `go.mod` moves to a newer Go in cross-file-consistency-gates; golangci-lint refuses to lint a module whose `go` directive is newer than the Go that built it. Landing order: this change must land before change 4.

**Why the plugin pin (LTB-4):** `version: latest` floated the logcheck checker set between builds; the pin (v0.10.1) is machine-verified via `go version -m bin/golangci-lint`.

**Spec-set history:** the spec set itself went through two cold review rounds before implementation (9 CRITICALs fixed round 1 — e.g. the pin-mismatch scenario promised a runtime guard that is change 5's job; the probe was not executable as written; the format:check drift was uncovered — all fixed in the spec set first).

**Branch review:** round 1 (9/9 legs, BLOCK) found the `LabelProfile` decoupling cosmetic (real fix applied: the const was unused; the 8 label-name sites now use it), the `namespaceField` alias hazard (created by this branch's hoist; fixed), stale nolint counts, and a mis-stated "believed" in the probe record. Round 2 (4/4: 2 CLEAN, 2 CONCERNS) confirmed the closures; the remaining CONCERNS are the pre-existing conflict-requeue test gap (behaviour explicitly accepted by both critics), now bound into the archive record's contents by task 2.1.

**Deferred (named in the archive record):** gomodguard → gomodguard_v2 migration (deprecation warning since v2.12.0); the 9/11 untested requeue sites; the Makefile stale-target hazard; `task format:check` stderr swallowing; a `--uniq-by-line=false` CI sensor proposal.
