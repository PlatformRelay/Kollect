# Stage B round 1: holistic run and per-commit gate

Branch `feat/ci-failure-reporting` @ `856be263c`, base `origin/main` @ `81637dcde` (merge-base).
`origin/main` had not moved at review time (2026-10-16); `git merge-tree --write-tree origin/main HEAD`
is clean.

## `task check` (umask 022, fresh worktree after `make operator-sdk`, mise kubectl 1.37.0 on PATH)

Exit 0, 1091 s wall, "all runnable gates passed".

| Gate | Result | Elapsed at end |
| --- | --- | --- |
| verify | ok | 9 s |
| format:check | ok | 31 s |
| lint:shell | ok | 44 s |
| lint:markdown | ok | 47 s |
| lint | ok | 69 s |
| vulncheck | ok | 77 s |
| test | ok | 169 s |
| scrub | ok | 169 s |
| spec:validate | ok | 172 s |
| audit-rbac | ok | 177 s |
| build | ok | 182 s |
| helm | ok | 184 s |
| go-mod | ok | 186 s |
| gitleaks, gitleaks-detect | ok | 188 s |
| workflow-security, workflow-security-audit (zizmor offline, high) | ok | 190 s |
| hack/test guard sweep (95 guard runs) | ok | 1091 s |

Excluded by design (printed by `hack/check.sh`): test-integration, kind-smoke, docker-build,
pipeline-cli-smoke, dependency-review, `integration_no_docker_test.sh`.

### Gap in the sweep (pre-existing, not from this branch)

The sweep skipped the `--self-test` mode of three guards, among them this change's new test:
`ci_failure_report_test.sh` (83 KB), `ci_workflow_security_test.sh` (68 KB) and
`consistency_toolchain_pins_test.sh` (127 KB). Cause: `hack/check.sh:93` decides whether a guard has
a self-test with `grep -v '^[[:space:]]*#' "${guard}" | grep -q -- '--self-test'` under
`set -o pipefail`. `grep -q` exits on the first match, the upstream `grep -v` takes SIGPIPE on a file
larger than the pipe buffer, the pipeline fails, and the guard reads as having no self-test. The
same trap is a recorded lesson of change 4 (G4). `hack/check.sh` is unchanged on this branch
(last touched on main in 839edc914), so this is a main-line defect, not a regression here.

The two self-tests this branch touches were run by hand to close the gap:

| Command | Exit | Wall | Cases |
| --- | --- | --- | --- |
| `bash hack/test/ci_failure_report_test.sh --self-test` | 0 | 166 s | 89 `ok - self-test` lines |
| `bash hack/test/ci_workflow_security_test.sh --self-test` | 0 | 89 s | 49 `ok - self-test` lines |

`consistency_toolchain_pins_test.sh --self-test` is untouched by the branch and runs in CI's
`tool-pins-selftest` job; it was not rerun here.

## Wall-time estimates

From CI run 38931256941 on main (last green `ci.yaml` run): `lint` 12 min 06 s,
`tool-pins-selftest` 20 min 37 s. The branch adds to `lint` the reporter test plain (about 1 min
locally, 46-67 s) and its `--self-test` (166 s locally). Estimate for `lint` on the branch: about
16-17 min, allowing a slower runner. `tool-pins-selftest` is unchanged at about 20-21 min and stays
the slowest required job, consistent with the 5.1 decision to keep the self-test in `lint`.

## Per-commit gate

Each non-docs commit, checked out in a detached worktree
`/home/koni/Projects/PlatformRelay/worktrees/kollect/b-cfr-<sha>`, plain mode only, umask 022.
Every worktree was removed afterwards and `git worktree prune` run. Docs-only commits
(98ed12ff4, c7222bb14, 43a6b4525, 75a764bc4, b7fcce05a, 9227aa250, 7bbb3e98b, eae02482f,
244278b4e, 856be263c) were skipped.

| Commit | `ci_failure_report_test.sh` | `ci_workflow_security_test.sh` |
| --- | --- | --- |
| 2a0ab5bb5 test(ci): probe fixtures | absent (not yet written) | 0 (11 s) |
| 5fd0bd180 ci: issue body builder | 0 (1 s, body cases only) | 0 (14 s) |
| e2ba1871d ci: reporter and decision tests | 0 (65 s) | 0 (20 s) |
| 212ebd1d8 test(ci): payload guards | 0 (46 s) | 0 (10 s) |
| bca053f4d ci: workflow and wiring checks | 0 (50 s) | 0 (14 s) |
| fe268e389 test(ci): self-test mode | 0 (66 s) | 0 (11 s) |

Each run ended with its "All ... checks passed." line.
