# Backlog stories — perftest findings 2026-08-16

Five stories from the phase-2 perftest run. Evidence in
[`../PERFTEST-RESULTS-2026-08-16.md`](../PERFTEST-RESULTS-2026-08-16.md).
INVEST vertical slices, Given/When/Then acceptance.

---

## PERF-FIX-01 — Ship a metrics-reader ClusterRole in the Helm chart (P1)

**Finding F-01.** `metrics.secure: true` is the chart default and `metrics.serviceMonitor.enabled`
exists, but the chart renders nothing that can authorize a scrape. `metrics-reader` lives only in
`config/rbac/metrics_reader_role.yaml`, which is the kustomize path — a Helm install never gets it.
The result is a metrics endpoint that is on by default, advertised as ServiceMonitor-ready, and
returns 403 to everything including the operator's own ServiceAccount.

This is not theoretical: it is why `export_p99` sat at `unmeasured` for a day and blocked the
LT-S14 gate. Someone measuring kollect has to reverse-engineer the RBAC.

**Scope**
- New template `charts/kollect/templates/clusterrole-metrics-reader.yaml`, rendering the
  `nonResourceURLs: ["/metrics"] / verbs: ["get"]` ClusterRole.
- A values switch (`metrics.readerRole.create`, default `true` when `metrics.secure` is true) and,
  optionally, `metrics.readerRole.bindTo` for extra subjects (e.g. a Prometheus SA).
- helm-unittest coverage; `helm-docs` regenerated.

**Acceptance**
- **Given** a default `helm install`, **when** a ServiceAccount is bound to the rendered reader
  role and calls `GET /metrics` with its token, **then** it receives **HTTP 200** — with no
  hand-written RBAC.
- **Given** `metrics.secure: false`, **when** the chart renders, **then** the reader role is
  omitted (nothing to authorize).
- **Given** `metrics.serviceMonitor.enabled: true`, **when** the chart renders, **then** the scrape
  identity it implies can actually read `/metrics`.
- helm-unittest asserts the role's `nonResourceURLs` and `verbs` exactly.

---

## PERF-FIX-02 — Make leader-election timings tunable and survivable (P1)

**Finding F-02.** `cmd/main.go` sets `LeaderElection` and `LeaderElectionID` and nothing else, so
controller-runtime defaults apply: 15 s lease, 10 s renew deadline, 2 s retry. The chart exposes
only `leaderElection.enabled`. Roughly ten seconds of API unreachability therefore terminates the
process.

Observed under load: **18 restarts**, each `leader election lost` → `exit 1`, after
`dial tcp 10.96.0.1:443: connect: operation not permitted`. Every exit interrupted in-flight
exports and is the direct cause of the export lag that made the L-10k inventory look short.

A single-replica operator gains nothing from an aggressive renew deadline — there is no peer
waiting to take over, so exiting fast just means downtime.

**Scope**
- Plumb `LeaseDuration`, `RenewDeadline`, `RetryPeriod` into `ctrl.Options` from config/flags.
- Expose them in `values.yaml` under `leaderElection`, with defaults that are deliberately more
  patient than controller-runtime's when `replicas == 1`.
- Document the trade-off (longer deadline = slower failover, fewer spurious exits).

**Acceptance**
- **Given** `leaderElection.leaseDuration/renewDeadline/retryPeriod` set in values, **when** the
  chart renders and the manager starts, **then** the flags reach `ctrl.Options` (asserted in a
  unit test, not just rendered).
- **Given** a single-replica install and a **15 s** API-server interruption, **when** connectivity
  returns, **then** the process has **not** exited and reconciliation resumes.
- **Given** defaults, **when** nothing is configured, **then** behaviour is at least as patient as
  today and the values are documented in the chart README.

---

## PERF-FIX-03 — WITHDRAWN (not a defect)

Verification before writing code showed `artifacts/` is **gitignored**, and
`docs/development/setup.md` states plainly: "never commit either path". The stale
`perf-snapshot.md` was a local leftover from a June run on this workstation, not a published
artifact — no reader of the repo ever sees it. Finding withdrawn; no work required.

---

## PERF-FIX-04 — Make the load test load-test something (P2)

**Finding F-04.** `test/load/collect_test.go` calls `extractor.Extract` on **one identical object**
in a tight single-threaded loop. No API server, no sinks, no controller, no concurrency, and no
assertion beyond "did not return an error" — so it cannot fail for being slow, using too much
memory, or regressing. It duplicates `BenchmarkExtract` while carrying the name `load-test:10k` and
a nightly ubuntu-8-core CI job, which reads as cluster-scale evidence to anyone who has not opened
it.

The risk is not that the test is fast. It is that a green nightly implies scale confidence nobody
measured.

**Scope** (pick one, do not do both silently)
- **(a) Make it real:** varied objects, concurrent workers, an envtest API server, at least one
  sink, and numeric assertions — a throughput floor and a memory ceiling that can actually fail.
- **(b) Make it honest:** rename to what it is (an extractor throughput micro-benchmark), drop the
  duplicate of `BenchmarkExtract`, and remove the implication of scale from the task and CI job
  names.

**Acceptance**
- **Given** the suite, **when** extraction throughput regresses by >25% or the memory ceiling is
  exceeded, **then** the test **fails** (today it cannot).
- **Given** a reader of the task list, **when** they read the load-test task description, **then**
  it states what is and is not exercised.
- **Given** option (b) is chosen, **then** no CI job name or doc claims cluster scale from it.

---

## PERF-FIX-05 — Report a live resource count on KollectTarget (P2)

**Finding F-05.** `setReady` (`internal/controller/kollecttarget_controller.go:263`) builds
`profileRef %q resolved; collecting %d resource(s)` from a count passed on the **spec** reconcile
path. Objects entering or leaving the matched set do not re-run it.

Observed: the status read `collecting 1000 resource(s)` throughout a period when it was
demonstrably collecting **1200** — the inventory moved 10 000 → 10 200 and back within 20 s while
the message never changed, `observedGeneration` stayed at 2, and `lastTransitionTime` stayed hours
old. Nothing marks the number as stale, so `kubectl get kollecttarget -o yaml` silently misreports
scale.

**Scope**
- Either refresh the count when the matched set changes, or move it to a `status.collectedCount`
  field with its own `lastUpdated` so staleness is visible.
- Prefer a numeric status field over a prose message — it is machine-readable and can back a
  printer column (`kubectl get kollecttarget` showing COLLECTED).

**Acceptance**
- **Given** a target collecting N objects, **when** M more objects enter its selector, **then**
  within one resync the reported count is N+M — asserted in an envtest.
- **Given** objects leave the selector, **then** the count decreases correspondingly.
- **Given** `kubectl get kollecttarget`, **then** the collected count is visible without `-o yaml`.
- **Given** the count cannot be refreshed, **then** its staleness is explicit (a timestamp), not
  implied.

---

## PERF-FIX-06 — `vulncheck` is red on `main` (pre-existing, found while landing #294)

**Not from this lane** — verified failing on clean `main` before PR #294 was merged, and again on
the merge commit. Recorded here because it was the only genuinely red thing left after the lane
landed, and an unexplained red gate decays into an ignored one.

`task vulncheck` reports **7 vulnerabilities in the Go standard library** that this code actually
calls, against `go 1.26.5` in `go.mod`. Reachable call paths named by govulncheck include:

- `internal/inventory/server.go:77` — `Server.Start` → `http.Server.ListenAndServe`
- `internal/sink/mongodb/backend.go:136` — `Backend.Close` → `mongo.Client.Disconnect`
- `internal/sink/s3/backend.go:111` — `Backend.Export` → `s3.Client.PutObject`

Plus 1 in an imported package and 1 in a required module, neither of which this code calls.

Because they are **standard library** advisories, the fix is a toolchain bump rather than a
dependency bump: raise the `go` directive (and the CI `GOTOOLCHAIN`/setup-go version) to a patch
release that carries the fixes, then re-run `task vulncheck` and confirm zero *called*
vulnerabilities.

**Acceptance**
- **Given** `main`, **when** `task vulncheck` runs, **then** it exits 0 — or every remaining
  advisory is explicitly triaged in `osv-scanner.toml` with a reason.
- **Given** CI, **when** the `vulncheck` job runs, **then** it is either green or its
  non-blocking status is deliberate and documented, so a real regression is still visible.

**Note:** `vulncheck` is **not** one of `protect-main`'s required checks (`preflight`, `test`,
`kind-smoke`, `Analyze (Go)`), so this does not block merges today — which is precisely how it has
stayed red.
