## Verdict: CONCERNS

## Findings
- [WARNING] T9 claims "T8 matrix rows 2 and 24 flip to pass" but `evidence/T8.md` is not touched by the diff — the blocked record still reads itself as blocked — `openspec/changes/dead-exported-surface/evidence/T9.md:21`
  Failure: a reader of the change's own block record at `299323bf` finds `evidence/T8.md:7` "Status: BLOCKED", `evidence/T8.md:15` row 2 "**FAIL**", `evidence/T8.md:37` row 24 "pass except the golangci row (red, decision request)". The resolution exists only in T9.md and the tasks.md DR-9 row; the primary gate is re-verified green by me (below), so the substance is right but the register is self-contradictory — the artifact of this task *is* the register.
  Fix: append to T8.md rows 2/24 and Status a dated pointer "resolved by T9, green on final tree — evidence/T9.md", or reword T9.md:21 to "superseded by this record" and keep T8.md frozen deliberately.
  Confidence: 85 (fact checked in the committed tree; severity is my call)
- [NOTE] T9.md contradicts its own scope claim: line-11 "three files: two test files + tasks.md" and line-4 "touches exactly the two named test files ... nothing else" vs the actual 4-file diff (the record itself) — `openspec/changes/dead-exported-surface/evidence/T9.md:19`
  Failure: `git diff --stat 7f4223c6..299323bf` = 4 files incl. evidence/T9.md; a checker running row 11's own `git diff --name-only` gets a count that fails the stated expectation.
  Fix: "plus this record" in line 11's parenthetical.
  Confidence: 90
- [NOTE] tasks.md 8.1 ("One full suite on the final tree", `task test` + `task coverage`) is ticked citing T8's runs, which executed at pre-task sha `f5736ec7`, not the final tree; T9 re-ran only the two affected package suites — `openspec/changes/dead-exported-surface/tasks.md:55`
  Failure: the tick's stated sensor (full suite/coverage on final tree) was not re-run post-edit. The reason it is safe — only test files changed, coverage measures non-test lines, other packages cannot import a `_test.go` — is sound but recorded nowhere in T9's matrix.
  Fix: one matrix row in T9.md stating that reasoning, or note "full suite at f5736ec7 + affected packages re-run" on the 8.1 tick.
  Confidence: 55

## What I checked (lens: diff)
- Both fixes correct and minimal: helper body `kollectclusterinventory_helpers_test.go:31-88` now contains no bare `ctx` (grep: only :40/:41 `cancel`), so the rename cannot have silently rebound the helper to the suite's `ctx` (`suite_test.go:39`) — the one failure mode build/vet/lint would *not* catch; it reads the helper's own `engineCtx` everywhere, preserving the deliberate T8 Finding-1 separation.
- `circuit_breaker_test.go:78` `:=`→`=`: assigns the same outer `err` (declared :55, already reused `=` at :84), checked immediately after each call — no behavioural change.
- Independently re-sensored on committed `299323bf`: full `bin/golangci-lint run` → "0 issues." (row 5); `task arch-lint` → "OK" (row 10); `go vet ./internal/sink/ ./internal/controller/` → exit 0 (row 7); `go test` both packages → ok (cached) (row 8).
- No allow-list/exclusion/coverage-baseline/`.golangci.yaml`/`nolint`/CI edit in the diff (row 11 independently verified; `govet.enable-all` unchanged — the shadow ratchet stays armed). The ratchet *did* exist and did fire at T8; had T9 gamed it, only the diff-scope check (row 11) would catch it, and it holds.
- Loop decision rows R2#7/R1#6: already satisfied pre-diff (`proposal.md:95` cites `go vet ./...` at `7f4223c6`; evidence framing already "per-task evidence") — not owed by this diff.

## Could not check
- Fresh (uncached) ~82s test run — the pass came from Go's test cache at this exact tree.
- Full 47-package `task test` and `task coverage` on the final tree (finding 3).
- T8's base-attribution claim (lint green at `3ee21266`) — not re-run at base.
- The untracked `reviews/L-tasks/T9/` register contents; local `bin/golangci-lint` version vs CI's pinned v2.11.4.
