# kollect perftest plan — phase 1 (scenarios, numeric criteria, harness map)

**Date:** 2026-08-16 · **Author:** agent session · **Repo tip:** `b4399fbd7`
**Status:** phase 1 proposal. Nothing here is a result. Phase 2 results live in
[`PERFTEST-RESULTS-2026-08-16.md`](./PERFTEST-RESULTS-2026-08-16.md).

## 0. Why this document exists before any run

A threshold without a named substrate is not a criterion. `task test:load-test:10k` is documented
as "nightly ubuntu-latest-8-cores"; the same test on an M-series laptop under colima produces a
different p99 by a wide margin, and neither number is wrong — they answer different questions.
So every row below names **where it runs**, and a result measured anywhere else does not satisfy
it.

The second reason: the operator asked for scenarios *not covered by the e2e suite*. That list is
derived from reading `hack/kind/e2e/smoke.sh`, not from imagination. See §2.

## 1. Baseline established before proposing anything

| Fact | Evidence |
| --- | --- |
| `main` unit tests are **green** | `task test:run` → exit 0, all packages `ok` (2026-08-16) |
| `artifacts/perf-snapshot.md` says unit tests FAIL | **STALE** — dated 2026-06-07, superseded by the run above |
| Talos lab cluster is **Ready**, 3 nodes | `kubectl get nodes` — controlplane/worker-1/worker-2, v1.32.0, Talos v1.9.5 |
| Docker on the workstation | was DOWN (colima stopped); started for this session |

## 2. What the e2e suite actually covers — and therefore what it does not

`hack/kind/e2e/smoke.sh` is 57 lines. In full, it asserts:

1. seven CRDs reach `Established`;
2. one `KollectSnapshotSink` named `e2e-snapshot-sink` exists in `default`;
3. `GET /inventory` returns a body containing `"itemCount"` within 60s;
4. a generic CRD (cert-manager `Certificate`) is collected.

That is a **happy-path installation smoke test**. It contains no failure injection, no restart, no
concurrency, no scale, no backpressure, and no RBAC-denial path. Everything in §4 is therefore
uncovered by construction — this is not a criticism of the smoke test, which is doing its job
(catching a broken install), but it does mean "e2e passes" carries no information about behaviour
under stress.

## 3. Substrates

| ID | Substrate | What it is | Used for |
| --- | --- | --- | --- |
| **S-LOCAL** | Workstation Go toolchain (darwin/arm64, go1.26.5) | No cluster, no Docker | micro-benchmarks, bounded synthetic load |
| **S-KIND** | kind v0.33.0 on colima (4 CPU / 8 GB) | Single-node, on the laptop | e2e parity, pprof capture, testcontainers |
| **S-TALOS** | kumulus lab, 3 × Talos v1.9.5 / k8s 1.32.0 | Real multi-node cluster on real iron | scale, failure injection, multi-node scheduling |
| **S-CI** | ubuntu-latest-8-cores (GitHub) | Not available locally | the 10k tier's *published* numbers |

**S-CI is not reachable from this session.** Any scenario whose criterion is defined on S-CI is
reported **NOT RUN**, never as passed, per the operator's instruction.

## 4. Scenario matrix

Numeric criteria. Where a threshold is a first measurement rather than a defended budget, it is
marked **(baseline)** — meaning the run establishes the number and the criterion is only "it
completes and the number is recorded", with the threshold binding on *subsequent* runs. Inventing
a p99 budget with no prior art and then passing it would be theatre.

### 4a. Performance — the perftest proper

| ID | Scenario | Harness | Substrate | Numeric pass criterion |
| --- | --- | --- | --- | --- |
| P-01 | Extractor micro-benchmark | `task test:bench` | S-LOCAL | `BenchmarkExtract` completes; ns/op and B/op recorded **(baseline)**; no allocation regression >20% vs the recorded baseline on re-run |
| P-02 | Bounded synthetic load, 1k objects | `KOLECT_LOAD_TEST=1 task test:load-test` | S-LOCAL | exit 0; wall clock < 120s; zero `t.Error` |
| P-03 | Synthetic object cap enforcement | `TestSyntheticObjectCap` | S-LOCAL | rejects `KOLECT_LOAD_TEST_MAX` outside 1..10000 with a non-zero exit — a *negative* criterion |
| P-04 | 10k tier | `task test:load-test:10k` | S-LOCAL (attempt) / S-CI (published) | completes, ≤ 10000 objects, exit 0. Laptop timing recorded but **not comparable** to the CI budget |
| P-05 | 10k live scale — export latency | live operator on S-TALOS, metrics histogram | S-TALOS | `export_p99` is a **number** (not `unmeasured`) and inventory rows == applied objects |
| P-06 | pprof capture under load | `task perf-kind:quick` | S-KIND | heap+CPU profiles captured; RSS ceiling < the 4Gi container limit |

### 4b. Scenarios NOT covered by e2e — the operator's second ask

| ID | Scenario | Why it matters | Substrate | Numeric pass criterion |
| --- | --- | --- | --- | --- |
| U-01 | **Operator restart mid-export** | Real clusters restart operators. Does export resume or silently lose rows? | S-TALOS | after restart, inventory rows return to 100% of applied within 10 min; zero permanent loss |
| U-02 | **Transient API-server unavailability** | Leader election exits the process on a 10s blip | S-TALOS | operator survives a 15s API interruption without exit, OR the behaviour is documented and tunable |
| U-03 | **Secure metrics scrapability** | `metrics.secure: true` is the chart default | S-TALOS | a ServiceAccount can be granted `/metrics` and receives HTTP 200 **using only what the chart ships** |
| U-04 | **Sink unavailable then recovers** | Postgres restart is routine | S-TALOS | exports resume; no duplicate rows; row count converges to applied |
| U-05 | **RBAC-denied resource type** | Kollect is often installed with reduced RBAC | S-TALOS | denied types are skipped with a surfaced condition; no crash, no silent empty inventory |
| U-06 | **Large single object** (near the 1 MiB etcd limit) | ConfigMaps can be big | S-LOCAL/S-TALOS | handled or rejected with a clear error; no OOM, no truncation-without-error |
| U-07 | **Concurrent targets on one cluster** | Multi-tenant is a shipped feature | S-TALOS | no sink conflicts; each inventory contains only its own scope |
| U-08 | **Webhook rejection path** | Validating webhook ships enabled | S-KIND | invalid CR rejected with a usable message; valid CR accepted |

## 5. Findings already confirmed before phase 2

These were found while establishing the baseline and closing the L-10k gate. They are real, they
are reproduced, and each has a story.

| # | Finding | Severity | Evidence |
| --- | --- | --- | --- |
| F-01 | **The Helm chart ships no metrics-reader ClusterRole.** `metrics.secure: true` is the default and a `serviceMonitor` option exists, but nothing the chart renders can authorize a scrape. `config/rbac/metrics_reader_role.yaml` exists only on the kustomize path. | High | `/metrics` returned **403** to the operator's own ServiceAccount; 200 only after applying a hand-written ClusterRole. This is why `export_p99` sat at `unmeasured` for a day and blocked the LT-S14 gate. |
| F-02 | **Leader-election timings are not tunable.** `cmd/main.go` sets `LeaderElection` and `LeaderElectionID` and nothing else; the chart exposes only `leaderElection.enabled`. controller-runtime defaults (15s lease / 10s renew) mean ~10s of API unreachability terminates the process. | High | Controller-manager pod: **18 restarts**, `lastState.terminated: Error, exit 1`, `"Failed to run manager": "leader election lost"` after `dial tcp 10.96.0.1:443: connect: operation not permitted`. Each exit interrupted in-flight exports. |
| F-03 | ~~stale perf snapshot~~ **WITHDRAWN on verification.** `artifacts/` is gitignored and `docs/development/setup.md` says never to commit it — the stale file was a local leftover from a June run, never published. Not a defect. | — | `git check-ignore` confirms it is ignored. |

## 6. L-10k / LT-S14 honesty gate — CLOSED

Goal C makes this a precondition, so it was closed first, with measurements rather than assertions.

| Field | Before (2026-08-15) | Now (2026-08-16) | How |
| --- | --- | --- | --- |
| `inventory_rows` | 9992 / 10000 | **10000 / 10000** | `SELECT count(*) FROM inventory_items` on `shared-postgres-0` |
| CMs in API | assumed | **10000** confirmed | per-namespace `kubectl get cm` count, all ten at exactly 1000 |
| `export_p99` | `unmeasured` | **0.977679 s** | `kollect_export_duration_seconds_bucket`, 900 observations, linear interpolation within the bucket — the same method `soak-export.sh` uses for etcd fsync |
| `valid` | `false` | **`true`** | both gate conditions met: numeric `export_p99` AND applied == counted |

**The shortfall was lag, not loss.** All 10000 ConfigMaps were always present in the API; the
inventory simply had not caught up when the soak process died. Hypotheses H2 (deleted CMs) and H7
(quota) are disproven by the API count; H3 (informer/export lag) is confirmed — and F-02 explains
*why* the lag was so large, since the operator was restarting under load.

**Caveat, stated rather than buried:** the histogram is cumulative since the operator's last
restart (~2.6h before measurement), not a window scoped to the apply. It is a steady-state export
latency at 10k objects, which is the number the gate asks for, but it is not "the p99 during the
apply burst".

## 7. Phase 2 execution order

1. S-LOCAL first (P-01, P-02, P-03) — no dependencies, fastest signal.
2. S-TALOS (P-05 done; U-01, U-02, U-04, U-05, U-07) — the real cluster, already up.
3. S-KIND (P-06, U-08) — requires colima.
4. Anything left → reported **NOT RUN** with the reason.
