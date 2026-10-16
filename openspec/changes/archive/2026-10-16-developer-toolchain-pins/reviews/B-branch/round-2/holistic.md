# Stage B round 2: holistic run

Branch `feat/developer-toolchain-pins` at `481bde067`, base `966e17e10`, G6 range
`b1f5a4369..HEAD` (11 commits). `git fetch origin`: `origin/main` did not move during round 2
(`dfe032184` before and after the fetch; it is one commit past the base, as recorded in round 1).
`git merge-tree --write-tree origin/main HEAD`: clean (exit 0).

Setup: existing `bin/` (operator-sdk, golangci-lint, controller-gen, envtest assets), mise kubectl
1.37.0 on `PATH`, `umask 022`, backgrounded and waited on by PID. `/tmp` at 26% inodes, 15% space
before the run.

| Command | Exit | Duration | Notes |
|---|---|---|---|
| `task check` | 0 | 1224s | verify, lint, lint:shell, lint:markdown, arch-lint, format:check, test:run, helm-lint, helm-unittest, helm-docs:verify, audit:rbac, scrub, spec:validate, vulncheck; 1081 `ok` lines (48 Go package `ok`); 39 toolchain lines; 0 `FAIL`/`not ok` |

## Per-commit cheap gate (tree green at each commit)

Each commit checked out in a detached temp worktree `worktrees/kollect/b2-<sha>`, then removed and
pruned. Gate: `bash hack/test/consistency_toolchain_pins_test.sh` (plain) and
`bash hack/test/ci_workflow_security_test.sh` (plain).

| Commit | Subject | toolchain-pins | ci_workflow_security |
|---|---|---|---|
| `632246c28` | test(ci): prove a full Renovate bump keeps the toolchain guard green | 0 (1s) | 0 (12s) |
| `1eae68344` | ci: run the toolchain-pins self-test in tool-pins-selftest | 0 (2s) | 0 (14s) |
| `7bc714f61` | fix(hack): verify installer checksums with shasum on macOS | 0 (3s) | 0 (30s) |
| `ba1ab8e52` | fix(ci): reject a Taskfile pin variable that holds latest in CFC-2 | 0 (3s) | 0 (27s) |
| `5a3b5da78` | fix(ci): read mixed-case uses: refs as the same action | 0 (2s) | 0 (19s) |
| `b7aeee567` | build(deps): give golangci-lint its own Renovate group | 0 (3s) | 0 (17s) |
| `851c1aa76` | docs(spec): align developer-toolchain with the stage-B fixes | 0 (1s) | 0 (15s) |
| `ed9d6cd76` | fix(ci): require a full version for CFC-2 pins | 0 (1s) | 0 (17s) |
| `4869b4d40` | test(ci): read the govulncheck mutant values from the tree | 0 (2s) | 0 (15s) |
| `58c516203` | docs(spec): record task group 6 of developer-toolchain-pins | 0 (1s) | 0 (11s) |
| `481bde067` | docs(spec): record task group 6 of developer-toolchain-pins | 0 (2s) | 0 (11s) |

All 22 runs exit 0 and print their pass lines. Not covered by this cheap gate: each commit's
`--self-test` modes (several minutes each). Note: `58c516203` and `481bde067` share one subject.
