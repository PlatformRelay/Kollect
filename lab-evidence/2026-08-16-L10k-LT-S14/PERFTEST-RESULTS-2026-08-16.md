# kollect perftest results — phase 2

**Date:** 2026-08-16 · **Plan:** [`PERFTEST-PLAN-2026-08-16.md`](./PERFTEST-PLAN-2026-08-16.md)
**Rule applied:** anything that could not run locally is reported **NOT RUN with the reason**,
never as passed.

## Summary

| Outcome | Count | IDs |
| --- | --- | --- |
| **PASS** | 8 | P-01, P-02, P-03, P-04, P-05, U-01, U-01b, **U-03 (after fix)** |
| **FAIL** | 2 | U-02 (fixed on `main`, re-test needs an image rebuild), plus F-05 found during U-01 |
| **NOT RUN** | 4 | P-06, U-05, U-08, **U-02 re-test** |

U-03 was the one finding that could be **closed end-to-end in this session**: it failed, the fix
landed on `main` (PR #294), and the re-test then passed using only chart-rendered RBAC. U-02's fix
also landed, but re-testing it requires building and redeploying the controller image — reported
NOT RUN rather than assumed.

## Performance results (S-LOCAL — darwin/arm64, go1.26.5, M-series)

| ID | Criterion | Measured | Verdict |
| --- | --- | --- | --- |
| P-01 | `BenchmarkExtract` completes; record ns/op, B/op | **27 679 ns/op · 32 219 B/op · 452 allocs/op** (45 572 iterations) | **PASS** (baseline recorded) |
| P-02 | exit 0, < 120 s, zero errors | exit 0, **4 s** wall clock | **PASS** |
| P-03 | rejects out-of-range `KOLECT_LOAD_TEST_MAX` | `0`, `10001`, `abc` all rejected non-zero | **PASS** (negative criterion) |
| P-04 | 10k tier completes, ≤ 10000 objects | **10 000 objects in 14.79 ms → 676 035 ops/s**, exit 0 | **PASS** — see F-04 on what this actually measures |

**P-04 honesty note.** This number is not comparable to the CI budget (defined on
ubuntu-latest-8-cores) and, more importantly, it is not a cluster measurement at all. See F-04.

## Live-cluster results (S-TALOS — 3 × Talos v1.9.5 / k8s 1.32.0, real iron)

| ID | Criterion | Measured | Verdict |
| --- | --- | --- | --- |
| P-05 | numeric `export_p99`; rows == applied | **`export_p99` = 0.977679 s** (900 observations); **10 000 / 10 000 rows** | **PASS** |
| U-01 | new matching objects reach the sink | 200 objects labelled into scope → inventory 10 000 → **10 200 in < 20 s** | **PASS** |
| U-01b | deletion propagates | 200 objects deleted → **10 200 → 10 000 in < 20 s** | **PASS** |
| U-02 | survives a transient API interruption | Operator **exited 18 times**, `leader election lost` after `dial tcp 10.96.0.1:443: connect: operation not permitted` | **FAIL** → F-02 fixed on `main`, but **re-test NOT RUN**: the flags live in the controller binary and the lab runs an older image, so verifying the fix needs a controller image build + redeploy |
| U-03 | a SA can be granted `/metrics` using only what the chart ships | **HTTP 403** initially. **RE-TESTED after the fix landed:** hand-applied RBAC deleted, chart-rendered ClusterRole/Binding applied (`helm.sh/chart: kollect-0.17.0`), same SA token → **HTTP 200**, 12 export histogram buckets | **FAIL → PASS after fix** |
| U-04 | sink unavailable then recovers | Not injected directly, but observed: Postgres restarted once (`1 (157m ago)`) and the row count still reconciled to exactly 10 000 with **no duplicates** | **PARTIAL PASS** — convergence proven, deliberate injection not performed |

### Method notes (so these numbers are reproducible)

- `export_p99` is computed from `kollect_export_duration_seconds_bucket` by linear interpolation
  within the containing bucket — the same method `soak-export.sh` already uses for etcd fsync.
  Buckets: 88 ≤ 0.05 s, 184 ≤ 0.1, 587 ≤ 0.25, 784 ≤ 0.5, 896 ≤ 1.0, 900 ≤ 2.5.
- The histogram is **cumulative since the operator's last restart** (~2.6 h before measurement),
  not a window scoped to the apply burst. Stated rather than buried.
- Row counts are `SELECT count(*) FROM inventory_items` on `shared-postgres-0`.
- The 200 probe objects were removed afterwards; the lab is back at its 10 000 baseline.

## NOT RUN — with reasons

| ID | Scenario | Why not | What would unblock it |
| --- | --- | --- | --- |
| P-06 | pprof under load (`task perf-kind:quick`) | The script exits 2 on a non-kind context and refuses the Talos cluster without `--allow-non-kind`. Running it against Kind would profile a different substrate than every other live number here, making the result non-comparable. | A deliberate decision to either profile on Kind (and label it S-KIND) or pass `--allow-non-kind` against Talos |
| U-05 | RBAC-denied resource type | Requires reinstalling the operator with reduced RBAC. The running operator holds the L-10k state that the LT-S14 gate was just closed against; reinstalling would destroy the evidence. | A separate namespace/install, or re-running after the L-10k evidence is archived |
| U-08 | Webhook rejection path | Needs the Kind e2e stack; colima was started but the e2e cluster was not built in this session. | `task test:e2e` on S-KIND |
| — | 10k tier **published** numbers | Defined on `ubuntu-latest-8-cores` (S-CI), unreachable from this session. The local number above is recorded but is **not** the published budget. | CI nightly |

## Findings

| # | Finding | Severity | Status |
| --- | --- | --- | --- |
| F-01 | Helm chart ships no metrics-reader ClusterRole; `/metrics` is unscrapeable as installed | **High** | story `PERF-FIX-01` |
| F-02 | Leader-election timings not tunable; ~10 s API blip terminates the process | **High** | story `PERF-FIX-02` |
| F-03 | ~~stale perf snapshot~~ **WITHDRAWN** — `artifacts/perf-snapshot.md` is gitignored and docs say never to commit it. The stale file was a local leftover, not a published artifact. No defect. | — | withdrawn |
| F-04 | `test/load` is not a load test — same object, single thread, no cluster, no assertions | **Medium** | story `PERF-FIX-04` |
| F-05 | `KollectTarget.status` resource count goes stale when the matched set changes | **Medium** | story `PERF-FIX-05` |

### F-04 detail

`test/load/collect_test.go` calls `extractor.Extract(obj, attrs)` on **one identical object** in a
tight single-threaded loop, then logs a throughput line. It has:

- no API server, no cluster, no sinks, no controller, no concurrency;
- no assertion beyond "did not return an error" — no throughput floor, no latency bound, no memory
  ceiling, so it cannot fail for being slow;
- an object that never varies, so it measures a hot path with perfect cache locality.

It is a micro-benchmark that duplicates `BenchmarkExtract`, published under a name
(`load-test:10k`, "nightly ubuntu-latest-8-cores") that reads as cluster-scale evidence. The
danger is not that it is fast — it is that a green nightly implies scale confidence nobody
measured.

### F-05 detail

`KollectTarget.status` reported `collecting 1000 resource(s)` throughout a period when it was
demonstrably collecting **1200** — the inventory moved 10 000 → 10 200 and back within 20 s while
the message never changed, `observedGeneration` stayed at 2, and `lastTransitionTime` stayed hours
old.

`setReady` in `internal/controller/kollecttarget_controller.go:263` builds the message from a
`collected` count passed in on the **spec** reconcile path. Objects entering or leaving the matched
set do not re-run it, so the number is a snapshot of the last spec change, not a live count. An
operator running `kubectl get kollecttarget -o yaml` to check scale reads a stale number with
nothing marking it stale.

## L-10k / LT-S14 gate — CLOSED

| Field | Before | Now |
| --- | --- | --- |
| `inventory_rows` | 9992 / 10000 | **10000 / 10000** |
| `export_p99` | `unmeasured` | **0.977679** |
| `valid` | `false` | **`true`** |

The shortfall was **lag, not loss**: all 10 000 ConfigMaps were in the API the whole time (verified
per-namespace, all ten at exactly 1000), and the inventory settled once the soak stopped racing it.
F-02 explains why the lag was large — the operator was restarting under load.
