## Verdict: CONCERNS

## Findings

- [WARNING] The metrics "profile" label is bound to the unrelated `StaticRefTypeProfile` (ADR-0208 static-ref enum) and the new `LabelProfile` is dead — the stated fix for review finding 1 is not implemented — `internal/metrics/metrics.go:37-40`
  Failure: the diff replaced the literal `"profile"` with `StaticRefTypeProfile` in every vector/catalog (`metrics.go:82,283`, `aggregation.go:28`, `aggregation_labeled.go:101`, `metrics_catalog.go:36,200,208,216`). A future rename of `StaticRefTypeProfile` (a static-ref enum, not a label) silently renames the Prometheus label; `LabelProfile` is never read (grep: only its definition + comment). The comment at `metrics.go:37-39` ("a rename of either must not touch the other") is contradicted by the code.
  Fix: use `LabelProfile` in the label-name arrays (and the catalog), or delete `LabelProfile` and the comment if conflation is intended.
  Confidence: 95

- [NOTE] SA1019 conflict sites are a real requeue-policy change, not a mechanical rename — `internal/controller/finalizer.go:17-22`
  Failure: 11 conflict paths moved from `Requeue: true` (AddRateLimited backoff) to fixed `RequeueAfter: 1s`; on a persistently contended object the retry cadence changes from exponential to 1 Hz. Documented in the const comment, commit and review register, so LTB-2 ("fixed or justified") holds; flagged only so the B-stage owner sees the behaviour delta, not just a deprecation fix.
  Fix: none required; keep the monitoring note.
  Confidence: 90

- [NOTE] No fitness function can catch the dead exported constant; `task lint` was green with it — `.golangci.yaml` (enabled set)
  Failure: staticcheck's `unused`/U1000 ignores exported identifiers, so `LabelProfile` passes the linter and the metrics catalog test (which pins metric *names* only). The defect above is invisible to every recorded sensor.
  Fix (ratchet): a test asserting each `Label*` const appears in some label list, or enable `unused` for exported symbols; cheapest is wiring the const and asserting the label slice value.
  Confidence: 80

## Requirement mapping (spec `lint-toolchain/spec.md`, verified against code)
- LTB-1 **holds** — `Makefile:184` and `.custom-gcl.yml:6` both `v2.13.1`; the scenario's pin-equality check is a review-time grep (tasks.md:34), correctly deferred runtime guard to DTP-3 (`developer-toolchain-pins/specs/developer-toolchain/spec.md:42`, verified present).
- LTB-2 **partial** — no linter disabled, `.golangci.yaml` diff empty, both new `//nolint`s reasoned same-line; but one "fix" (goconst `profile`) is ineffective and introduced the conflation above.
- LTB-3 **holds** — before 57 / after 0 / 55 fixed / 2 justified / `bin/golangci-lint version` = pin, in `evidence/probe.md` §5/§7 and `evidence/1.3.md` §Findings disposition.
- LTB-4 **holds** — `.custom-gcl.yml:11` `version: v0.10.1`, recorded in `evidence/probe.md` §8.
- Not in any requirement: the 12 uniq-by-line-hidden goconst positions were also fixed (stronger than spec), and the metrics constant naming above.

## Could not check
- The v2.11.4 "before = 0" baseline: I could not run the old binary. It is consistent with a required-green main (base tree carries `Requeue: true`), but not independently reproduced; the probe itself marks attempt 1's ordering "unexplained".
- Whether the bundled goconst/staticcheck version delta explains all 57 findings (no old binary/config available locally).
- CI on the PR head and the `hack/test/*_test.sh` guard suite (not-run locally per `evidence/1.3.md`).
