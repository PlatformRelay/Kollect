# Stage B round 2 (last) — verification

Range `4f6c15cd4..f253ae8f8` (the stage-B fixes, tasks.md section 6). Fresh legs: reviewer-a,
adversarial lens (`brief-reviewer-a.md`, verdict **CONCERNS**); reviewer-b, spec lens
(`brief-reviewer-b.md`, verdict **CLEAN** with notes). No CRITICAL. Each claim below was checked
against the code on 2026-10-20. This was the last round: verified behaviour defects are fixed
with mutants and no further review; everything else is fixed as wording or rejected with a
reason.

## Round-1 findings

| Id | reviewer-a | reviewer-b | Verified |
|---|---|---|---|
| H1 | closed | closed | yes: `go mod tidy -diff` exit 0 (final gates) |
| A1 | closed | closed | yes: tests 6.2 red on the pre-fix code, mutants killed |
| A2 | not closed (C1 below) | closed | closed for the watchdog requeue (SCR-7's sentence); C1 is a different path |
| B1 | closed | kept as accepted residual | yes |
| B3 | closed | closed | yes |
| B4 | closed | closed | yes |

## New findings

| Id | Leg | Sev | Claim | Verified? | Disposition |
|---|---|---|---|---|---|
| C1 | reviewer-a | WARNING | a `0s` inventory that stays on a degraded early-return path (sink `ConnectionVerified=False`, suspended) for more than 48 h does not consult the tracker, so its state ages out and the reconcile after recovery exports the unchanged payload once | yes, by reading (`kollectinventory_controller.go:145,158` return before `exportToSinks`, so `shouldSkip` never runs) | not a behaviour defect: one export after a recovery that followed two days without any export attempt, like the one after a controller restart (the tracker is in memory, and that restart case predates the branch). SCR-7's sentence (no re-export "by this requeue or any other timer") holds; its scenario's "at any later time" and the docs' "never re-sent" over-promised. Fixed as wording: SCR-7 and design *Zero export interval* name both cases, and the scenario reads "on any later watchdog requeue"; the troubleshooting page says a restart or more than 48 h degraded re-sends once (`df1993fa3`) |
| C2 | both | NOTE | the B1 test accepted both outcomes (`next == b6` branch), so dropping the failed re-read retire left it green | yes (mutant "no failed re-read retire" survived the old test by reading; killed by the new one) | fixed: the test asserts the entry is retired, drains, and is not handed to the next acquire (`b7c415293`); mutants "no failed re-read retire" (`:47`) and "retire ignores holders" (`:50`) killed, control survives |
| C3 | reviewer-b | NOTE | SCR-2's new clause says the earlier read "observed a change", but in the B1 ordering it saw only a mismatch with a newer entry | yes | fixed: "a mismatch with the pooled entry that an earlier successful read of the same acquire saw still retires that entry" (`df1993fa3`) |
| C4 | reviewer-b | NOTE | the full-read retire can hit an entry built from a Secret recreated after the not-found read | yes | no action: the design's "an entry another acquire stored meanwhile is retired" covers it; one drain and rebuild, never stale use |
| C5 | reviewer-b | NOTE | a controller restart re-exports each unchanged `0s` binding once; a degraded `0s` inventory for over 48 h loses its state | yes | same as C1: documented (`df1993fa3`) |

## Result

No CRITICAL in round 2; no verified behaviour defect. Fixes after the last round: one test
assertion (C2) and wording (C1, C3, C5), each recorded above; no further review.

## Final gates

On the final code tree, `00604732b` (2026-10-20, `umask 022`, envtest assets from the main
checkout), every exit 0:

| Gate | Command | Result |
|---|---|---|
| unit + envtest | `go test -count=1 ./internal/sink/... ./internal/pipeline/... ./internal/controller/... ./cmd/...` | 23 packages `ok`, no `FAIL` |
| race | `go test -race -count=10 ./internal/sink/`; `-count=3 ./internal/pipeline/`; `-count=1 ./internal/controller/` | no `DATA RACE`; envtest `Ran 59 of 59 Specs`, 0 failed, under `-race` (separate `-v` run) |
| integration | `task test-integration` | expected 68, executed 68, passed 68, skipped 0 |
| lint | `task lint` | `0 issues.`, go-arch-lint OK |
| vet | `go vet -tags=integration ./...` | clean |
| module graph | `go mod tidy -diff && go mod verify` | clean (H1 closed) |
| codegen | `task verify` | clean |
| OpenSpec | `task spec:validate` | 16 passed, 0 failed |
| scrub, secrets | `task scrub`; `gitleaks detect --log-opts ae98ef106..HEAD` | `scrub: ok`; no leaks |
