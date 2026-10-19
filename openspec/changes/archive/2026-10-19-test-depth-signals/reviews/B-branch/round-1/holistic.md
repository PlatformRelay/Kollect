# Stage B round 1: holistic run and per-commit gate

Branch `feat/test-depth-signals` @ `548d0d973`, base `origin/main` @ `b05651d20`. After
`git fetch origin` on 2026-10-18, `origin/main` had not moved (merge base = `origin/main`), and
`git merge-tree --write-tree origin/main HEAD` is clean.

Host setup: `umask 022`; `make operator-sdk` (fresh worktree); a real kubectl 1.37.0 first on
`PATH` for the kustomize guards (demo_04).

## Holistic run

| Gate | Command | Result | Wall time |
|---|---|---|---|
| Full local gate | `task check` | exit 0, "all runnable gates passed"; every guard in every mode, the three new ones (`fuzz_retry_test.sh`, `test_shuffle_test.sh`, `readme_quickstart_test.sh`) plain and `--self-test` included; `vulncheck` ok | 1941 s (32 min) |
| Shuffled nightly, once | `task test:shuffle` | exit 0, seed 964626013 printed first; 45 packages at `-count=3`, Ginkgo packages `internal/controller` and `internal/webhook/v1alpha1` three times at `-count=1`; 51 `ok` lines = 45 + 2 x 3, and `go list ./...` lists 47 packages | 211 s |

## Lint job wall-time estimate

The three new guards run in the required `lint` job, both modes (`ci.yaml`, after CFR-7).
Measured alone on this host:

| Guard | plain | `--self-test` |
|---|---|---|
| `fuzz_retry_test.sh` | 1 s | 58 s |
| `test_shuffle_test.sh` | 2 s | 68 s |
| `readme_quickstart_test.sh` | 0 s | 3 s |

About 132 s locally. The last green `main` CI run (39145000481) took 10 min 42 s for `lint` and
19 min 44 s for `tool-pins-selftest`. With runner overhead (call it 2-3.5 min), `lint` comes to
roughly 13-14 min, still well below `tool-pins-selftest`, so the PR's critical path does not move.

## Per-commit gate

Each code commit was checked out into a detached worktree
`/home/koni/Projects/PlatformRelay/worktrees/kollect/b-tds-<sha>`, the guards that exist at that
commit were run in plain mode, and the worktree was removed. Openspec-only commits were skipped.

| Commit | fuzz_retry | test_shuffle | readme_quickstart | ci_workflow_security | `go test -count=3` |
|---|---|---|---|---|---|
| `9ce186270` feat(ci): fuzz runner | ok | n/a | n/a | ok | n/a |
| `6d9f8436e` ci(fuzz): module download | ok | n/a | n/a | ok | n/a |
| `a52189aa9` test(collect) | ok | n/a | n/a | ok | `./internal/collect/` ok (see note) |
| `297952dee` test(metrics) | ok | n/a | n/a | ok | `./internal/metrics/` ok |
| `c6589f3e2` feat(ci): shuffle nightly | ok | ok | n/a | ok | n/a |
| `d0c73d644` test(ci): README guard | ok | ok | ok | ok | n/a |

Note: the first `go test -count=3 ./internal/collect/` run at `a52189aa9` failed in
`TestEngine_ScaleEnvtest500` with `fork/exec /usr/local/kubebuilder/bin/etcd: no such file`. That
is an environment failure: a fresh worktree has no `bin/k8s`, so the envtest-backed test found no
assets. A rerun with `KUBEBUILDER_ASSETS` set to the branch worktree's
`bin/k8s/1.36.2-linux-amd64` passed, and so did
`go test -count=3 -shuffle=on -run TestRecordLabeledMetricSeries_CapsCardinalityDeterministically`.
Every commit's tree is green on the guards it carries.
