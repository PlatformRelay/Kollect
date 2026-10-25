# Stage B round 1: holistic run

Branch `feat/mutation-testing-signal` @ `fe7d8e31e`, base `origin/main` @ `3c01608ff`. After
`git fetch origin` on 2026-10-19, `origin/main` had not moved (still `3c01608ff`, an ancestor of
HEAD) and `git merge-tree --write-tree origin/main HEAD` was clean (exit 0, no conflict).

Host setup: `umask 022`; `make operator-sdk` (fresh worktree); real kubectl 1.37.0
(`~/.local/share/mise/installs/kubectl/1.37.0`) first on `PATH`. `task check` was backgrounded
and waited on by PID through a completion marker written after it exited. Nothing in the tree was
edited while it ran.

## Gates

| Gate | Command | Result | Wall time |
|---|---|---|---|
| Full local gate | `task check` | exit 201, one gate red: `guard hack/test/consistency_toolchain_pins_test.sh --self-test` (the nested all-pins bump control, no `FAIL` line in its output); every other gate ok (`verify`, `format:check`, `lint:shell`, `lint:markdown`, `lint`, `vulncheck`, `test`, `scrub`, `spec:validate`, `audit-rbac`, `build`, `helm`, `go-mod`, `gitleaks`, `gitleaks-detect`, `workflow-security`, `workflow-security-audit`, every guard incl. `mutation_signal_test.sh` plain and `--self-test`); six gates excluded by design (Docker, kind, PR context) | 2157 s (36 min) |
| Rerun of the red gate, branch | `bash hack/test/consistency_toolchain_pins_test.sh --self-test` | exit 0, `All toolchain-pin consistency self-tests passed.` | 487 s |
| Same gate, base | same, in a detached worktree at `origin/main` | exit 0 | 477 s |
| Real chain, plain | `task mutation` with `GITHUB_OUTPUT` and `GITHUB_STEP_SUMMARY` pointed at temp files | exit 0; `reports=<dir>` written; the four reports kept; summary table with three rows | 15 s |
| Real chain, forced | `MUTATION_FORCE_FAILURE=true task mutation` | exit 201, `::error::mutation: forced failure: MUTATION_FORCE_FAILURE is true ...`, no gremlins call | under 1 s |

Classification of the `task check` red: ENVIRONMENT (believed). It passed in isolation on the
branch and on the base, and no product file differs between the red and the green runs. The
shared `/tmp` on this host was close to its quota during the session (other sessions' files; at
inode use 67% before the reruns), and the nested bump control writes a full tree copy there; the
exact cause of the nested failure was not pinned down, because its output carried no `FAIL` line.

Local hazard found on the way (NOTE, verification.md B6): after `task check` built
`bin/golangci-lint` and `make operator-sdk` filled `bin/` (575 MB), a direct gremlins run copied
the whole module, `bin/` included, into one `/tmp/gremlins-*` work directory per worker (16),
hit the `/tmp` quota (EDQUOT), panicked (`executor.go:165`, "error, this is temporary") and left
about 8 GB behind (a panic skips its cleanup). The runner fails closed on that (exit 2, "gremlins
failed"). After the leftovers were removed `/tmp` went from 13 GB used to 5.4 GB. CI is not
affected: its checkout is fresh and `bin/` holds only the gremlins binary.

## Real `task mutation`: repeated runs (finding, see verification.md B1)

Seven runs of `task mutation` on this host (16 CPUs, before `bin/` grew, before `task check`),
all exit 0:

| Run | aggregate (K, L, NC, TO, NV) | redact | pathvalidate |
|---|---|---|---|
| 1 | 4, 1, 0, 0, 0 | 12, 1, 1, 0, 0 | 10, 2, 0, 0, 0 |
| 2 | 0, 0, 0, 5, 0 | 12, 1, 1, 0, 0 | 10, 2, 0, 0, 0 |
| 3 | 0, 0, 0, 5, 0 | 12, 1, 1, 0, 0 | 10, 2, 0, 0, 0 |
| 4 | 4, 1, 0, 0, 0 | 12, 1, 1, 0, 0 | 10, 2, 0, 0, 0 |
| 5 | 4, 1, 0, 0, 0 | 12, 1, 1, 0, 0 | 10, 2, 0, 0, 0 |
| 6 | 2, 0, 0, 3, 0 | 12, 1, 1, 0, 0 | 8, 2, 0, 2, 0 |
| 7 | 2, 0, 0, 3, 0 | 12, 1, 1, 0, 0 | 10, 2, 0, 0, 0 |

Runs 2 and 3 measured nothing for `internal/aggregate` (every mutant TIMED OUT) and were green.
The coverage runs took 0.85-1.4 s for aggregate, so the per-mutant deadline (coverage time times
the default coefficient 3, started before the mutant's build) is about 3 s, which a mutant build
competing with 15 other workers can exceed.

Coefficient probe, in a clean detached worktree at HEAD (so `bin/` is small), the runner's exact
argv with and without `--timeout-coefficient=10`, six runs each:

| Coefficient | Runs with any TIMED OUT | Worst run |
|---|---|---|
| default (3) | 4 of 6 | pathvalidate K 4, L 2, TO 6 |
| 10 | 0 of 6 | none; every run K/L as the recorded reports |

## Lint wall-time estimate

| What | Seconds | Note |
|---|---|---|
| new guard, plain | 11 | HEAD `fe7d8e31e` |
| new guard, `--self-test` | 197 | 126 passing self-test lines, 0 FAIL (G2 measured 153 s at fewer mutants) |

So the required `lint` job grows by about three and a half minutes, nearly all of it the guard's
self-test. CFC-2's share is unchanged from G2 (about +5 s).
