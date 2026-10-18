# Stage B round 2: holistic run

Branch `feat/test-depth-signals` @ `d1dd0930e` (G5 and its review fixes committed), base
`origin/main` @ `b05651d20`. After `git fetch origin` on 2026-10-18, `origin/main` had not moved
and is an ancestor of HEAD.

Host setup: `umask 022`; `bin/operator-sdk` present; real kubectl 1.37.0
(`~/.local/share/mise/installs/kubectl/1.37.0`) first on `PATH`. Run backgrounded and waited on by
PID (2303649) through a completion marker written after it exited. Nothing in the tree was edited
while it ran.

| Gate | Command | Result | Wall time |
|---|---|---|---|
| Full local gate | `task check` | exit 0, `task check: all runnable gates passed`; `vulncheck` ok; the three new guards ran plain and `--self-test` (`fuzz_retry_test.sh` plain line `(F0-F18)`; `--self-test ok` for `fuzz_retry`, `readme_quickstart`, `test_shuffle`); CFC-6 self-test rejects each new guard unwired; six gates excluded by design (`check: excluded ...`, the same list as round 1: kind cluster, PR context) | 2138 s (36 min) |

Round 1 took 1941 s at `548d0d973`; the 197 s difference is the G5 cases and mutants (fuzz
self-test 58 s alone in round 1, about 114 s now). `task test:shuffle` was not rerun: G5 changed
only the script's `make setup-envtest` failure path, which S9c covers with a fake `make`.
