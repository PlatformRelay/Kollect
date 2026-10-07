# Tasks

## 1. Bump

- [x] 1.1 Probe (evidence: `evidence/probe.md`, raw output and exit codes for every command —
  do not rely on `task format:check`'s stderr-swallowed view): FIRST run `task lint` at the
  current pin (v2.11.4, `bin/golangci-lint` already built) and record its findings count as
  the before baseline; then in the working tree (uncommitted) set BOTH pins —
  `GOLANGCI_LINT_VERSION` (`Makefile`) and `version:` (`hack/tooling/.custom-gcl.yml`) — to
  v2.13.1, `rm -f bin/golangci-lint*` (make skips the stale file target — verified with
  `make -n`), `make golangci-lint` (custom build with plugins; a vanilla binary fails config
  validation on the custom logcheck linter — record that failure as probe evidence), then
  `task lint` and `task format:check`; record the findings count, any formatter drift, and
  `bin/golangci-lint version`; record the resolved `sigs.k8s.io/logtools` version the plugin
  build used; revert both pins when done — closed 2026-10-07, evidence: `evidence/1.1.md`
- [x] 1.2 Version-only commit: `Makefile` and `.custom-gcl.yml` together — the linter version
  in both sites, and the logcheck plugin pin (`version: latest` becomes the resolved version
  recorded by the probe) — closed 2026-10-07, evidence: `evidence/1.2.md`
- [x] 1.3 Fix or justify each new finding, one commit per group, no linter disabled; every new
  `//nolint` directive or exclusion carries a reason on the same or preceding line;
  `task lint` and `task format:check` clean; record in `evidence/1.3.md`: findings count
  after, number fixed, number justified, `bin/golangci-lint version` (must report the pin).
  A clean `task lint` at the pin structurally proves the custom build: a vanilla binary
  cannot pass this config (logcheck is unregistered), established by the 1.1 probe evidence

## 2. Land

- [x] 2.1 Archive the change as the last commit of the PR, after review and green CI; the
  review record names the reviewed and the archive revision, and the archive record carries
  the deferred tech-debt notes with their numbers: gomodguard → gomodguard_v2 migration
  (deprecation warning on every lint run since v2.12.0), the 9/11 untested conflict-requeue
  sites (pre-existing coverage; the conflict-requeue policy itself is disclosed at
  finalizer.go:16-24 and accepted in review), the Makefile stale-`$(GOLANGCI_LINT)` file
  target (make skips a version-variable change), `task format:check`'s stderr swallowing
  (Taskfile.yml:324), and the `--uniq-by-line=false` CI sensor proposal for goconst
  - Landing record: reviewed revision `2711bb99` (branch review round 2 head; CI 42/42 green
    on it, run 37558117734 — perf-report flaked once on the untouched `internal/sink` parent
    package and passed on rerun); the archive commit is the last commit of this PR and moves
    this directory to `openspec/changes/archive/2026-10-07-golangci-lint-bump/`.

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| LTB-1 | grep both files; `task lint` + `task format:check` real runs | equal, at least v2.13.1; both clean | pass (1.2 evidence + orchestrator re-run at c9e795bb: lint 0, format 0) | evidence/1.2.md, evidence/1.3.md |
| LTB-2 | diff review of `.golangci.yaml` and every `//nolint` in the diff | no disabled linter or blanket exclusion; every new nolint/exclusion has a reason | pass (empty `.golangci.yaml` diff; 3 reasoned nolints in the branch — 1 product + 2 test, the gitlab one being a Go test file; corrected at stage B round 2 — all reviewed) | evidence/1.3.md §7, reviews/L-tasks/1.3 |
| LTB-3 | findings count before and after in the review record | recorded, with the fixed/justified numbers and the executed binary's version | pass (57 → 0; 55 fixed, 2 justified; binary reports the pin) | evidence/probe.md §5, evidence/1.3.md §8 |
| LTB-4 | grep `.custom-gcl.yml` plugin block | a pinned version, not `latest` | pass (`version: v0.10.1`, machine-verified by `go version -m`) | evidence/1.2.md |
| all | independent review; CI on the PR head | APPROVE, green; both revisions recorded | pass — independent review recorded (per-task legs + 2 branch rounds, loop.md triage); CI on the PR head green 42/42 (run 37558117734, rerun green after the perf-report flake) | loop.md, PR https://github.com/PlatformRelay/Kollect/pull/453 |

## Deferred notes (tech-debt register, named per task 2.1)

| # | Note | Where it bites | Suggested owner |
| --- | --- | --- | --- |
| 1 | `gomodguard` is deprecated (since v2.12.0; replaced by `gomodguard_v2`) and still enabled — a deprecation warning prints on every lint run | `.golangci.yaml:19,69` | follow-up tooling change (change 5 or later) |
| 2 | 9 of the 11 conflict-requeue sites have no test pinning the requeue; a future regression silently drops retries (stuck deletes) | `internal/controller/{finalizer,target_finalizer,kollect*_controller}.go` | test-depth-signals (change 8) or a follow-up |
| 3 | `make golangci-lint` skips the stale `$(GOLANGCI_LINT)` file target when only the version variable changes | `Makefile:209-216` | change 5 (developer-toolchain-pins) |
| 4 | `task format:check` swallows the linter's stderr (`2>/dev/null` inside the substitution) — a crashed fmt binary passes green | `Taskfile.yml:324` | change 5 or follow-up |
| 5 | goconst's `uniq-by-line` (default true) hides findings on lines that already carry one — a "0 visible" gate is measured against a truncated set | golangci-lint goconst settings | change 5: run the check with `--uniq-by-line=false` |
