# Kollect — engineering guidelines

Binding guidelines for the Kollect operator: error handling, robustness, security, and testing.
Enforced by lint, CI, and review. ADRs in `../adr/` capture major decisions.

**Related docs:** Go style and lint rules → [coding-standards.md](coding-standards.md);
contribution process → [CONTRIBUTING.md](../../CONTRIBUTING.md); product requirements and NFRs →
[REQUIREMENTS.md](../REQUIREMENTS.md).

## 0. Product priorities (summary)

- **Custom CA TLS** on Git/GitLab sinks from early phases (`caBundle` / `caSecretRef`).
- **Validating webhooks early** for Profile CEL/JSONPath and Sink `type` enum.
- **Helm chart day 1**; **Prometheus metrics** testable in CI; **connection test** with clear status.
- **HTTP inventory API** is core, not optional later.
- **Aggregation** — one export per logical change; design for ~60 clusters without blocking single-cluster.
- **Reject `KollectPublication` / doc-sync** — use Git/Kafka/Postgres export + external CI ([ADR-0702](../adr/0702-doc-sync-templating.md)).
- **Postgres + Kafka sinks** are first-class export targets ([ADR-0402](../adr/0402-sink-backends-database-kafka.md)).

## 1. Error handling

Operator-specific error taxonomy drives reconcile behavior. For Go wrapping conventions (`%w`,
`errors.Is` / `errors.As`), see [coding-standards.md § Go conventions](coding-standards.md#go-conventions).

- **Typed error taxonomy** drives requeue behavior:
  - `ErrTransient` (network, throttling, conflicts) → requeue with backoff; `Synced=False`, `Reason=Progressing`.
  - `ErrTerminal` (bad config, invalid CEL/JSONPath) → no requeue; `Degraded=True` + Warning Event.
  - `ErrForbidden` (SAR/RBAC denied) → degrade scope; record `skipped:forbidden`; do not fail the whole reconcile.
- **No `panic`** in reconcilers or libraries (except `main`).
- **Context deadlines** on every external call; propagate reconcile `ctx`.
- **Structured logs** (`logr`): stable messages + keys. Never log secrets, tokens, or full payloads.
  Logging package policy: [coding-standards.md § Logging](coding-standards.md#logging).

## 2. Robustness and reliability

- **Idempotent, level-based reconcile** — safe to run repeatedly.
- **Event-driven collection** — dynamic informers; long resync as a backstop only.
- **Finalizers** when external cleanup is required.
- **Optimistic concurrency** — on `Conflict`, requeue quietly.
- **Bounded resource use** — paginated `List`, scoped caches, tuned concurrency and rate limits.
- **Circuit breakers** around external sinks and doc backends.
- **Lifecycle** — leader election, graceful shutdown, `/healthz` and `/readyz`, PDB where deployed.
- **etcd size guard** — `status` holds summaries/counts/conditions only; payloads go to sinks.
- **Status discipline** — `Ready` / `Synced` / `Degraded` + `observedGeneration`.
- **Determinism** — stable ordering for sink output and golden tests.

## 3. Security

- **Least-privilege RBAC** — minimal generated roles; SAR pre-check before list/watch.
- **Tenancy** — optional `KollectScope` (future) for allowed GVKs, namespaces, sinks.
- **Secrets** — credentials only via `secretRef`; never in spec/status or logs.
- **Container hardening** — non-root runtime image (UID 65532), read-only rootfs, dropped capabilities, seccomp.
- **Network** — restrictive `NetworkPolicy` for production egress.
- **Transport** — TLS verification required for sink and doc endpoints; support org **custom CA** (no disable-verify in prod).
- **Input validation** — CEL in CRD OpenAPI + **validating webhooks before reconcile workarounds**.
- **Supply chain** — pinned dependencies and GitHub Action SHAs; scans enforced in CI.
  Tooling and gates: [coding-standards.md § Security](coding-standards.md#security).

## 4. Testing

Operator test expectations. Pyramid tiers, coverage floors, and CI gates:
[testing.md](testing.md) and [coding-standards.md § Testing](coding-standards.md#testing).

- **Tests alongside code** — unit, envtest, golden contracts, integration (testcontainers), kind e2e.
- **Mocks** — mockery on small interfaces only.
- **Metrics** — assert Prometheus counters/histograms in controller tests where behavior changes.
- **Scale tests bounded** — default `task test` caps synthetic objects (500); load tests require
  `KOLECT_LOAD_TEST=1` and `-tags=load` (max 2000). Never run 10k-object suites in default CI.

## 5. Performance and scalability

- **Scale target:** 10,000+ watched objects per operator with scoped informers ([ADR-0603](../adr/0603-performance-scalability.md)).
- **Memory bounded** — paginated `List`, namespace/label selectors, shared informer per GVK; no full
  payload in etcd status ([ADR-0103](../adr/0103-etcd-limit.md)).
- **Parallel controllers** — tune `MaxConcurrentReconciles`; workqueue rate limiter + exponential
  backoff on `ErrTransient`; separate concurrency for heavy vs light reconcilers where needed.
- **Backpressure** — monitor workqueue depth and reconcile latency metrics; SAR `ErrForbidden` degrades
  scope for one target without blocking the whole queue.
- **Rate limits and circuit breakers** — per-sink `gobreaker`; transient sink/API errors requeue with
  jitter; terminal config errors stop requeue ([ADR-0602](../adr/0602-error-taxonomy.md)).
- **Profiling** — pprof on `:6060` behind feature gate (default off); document in [PERFORMANCE.md](../PERFORMANCE.md).
- **Benchmarks** — `task bench` (`-short`, `-benchmem`); `BenchmarkExtract` for CEL/JSONPath hot path.

## 6. Definition of done (per change)

- Relevant tests green; lint clean; `task verify` shows no drift.
- New external calls have timeouts and backoff where appropriate.
- Status conditions and Events updated; no secrets in logs.
- ADR updated when the decision is non-trivial.

Full contributor checklist: [CONTRIBUTING.md § Pull request process](../../CONTRIBUTING.md#pull-request-process).
