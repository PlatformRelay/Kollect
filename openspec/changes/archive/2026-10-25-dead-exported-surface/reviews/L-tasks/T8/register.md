## Unified verdict: CONCERNS   (legs ok: 1/1)
| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | WARNING | `VerifySet` call-site tally overstated as "10"; actual is 6 test call sites in `manifest_test.go` + 1 consumer test | `evidence/T8.md:22,167` | DeepSeek-V4.1-Flash | diff | 88 |

## Disagreements
- None (single leg).

## Nobody could check
- Base-tree lint green at `3ee21266` (not re-run in a detached base worktree).
- Coverage 91.3% parity across trees and `task test` 47-package green (not re-run; ~9 min each).
- Docker/envtest premise and the T8 fanout leg failure itself (orchestrator-side, outside the record's claims).
