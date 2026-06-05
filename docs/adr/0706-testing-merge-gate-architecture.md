# ADR-0706: Testing and merge-gate architecture

> The test pyramid, CI merge gates, opt-in tiers (integration, load, e2e), and how quality gates map to
> `task` targets and GitHub Actions — lifted from [GUIDELINES.md](https://github.com/konih/kollect/blob/main/GUIDELINES.md) into a durable
> ADR.

**Theme:** 07 · Project & meta · **Status:** Current

## Context

Kollect is **TDD-first** with binding rules in [GUIDELINES.md](https://github.com/konih/kollect/blob/main/GUIDELINES.md) §4 and
[NFR-TEST-* in REQUIREMENTS.md](../REQUIREMENTS.md). CI (`.github/workflows/ci.yaml`) enforces a
subset on every PR; heavier tiers run nightly or on demand. The split — what **blocks merge** vs what
**signals risk** — was implicit in Taskfiles and workflows but not recorded, which made it hard to add
gates (e.g. Q16 RBAC audit, Q15 supply-chain attestations) without debate.

## Decision

### Test pyramid (fast → slow)

| Tier | Scope | Tooling | Typical `task` |
| --- | --- | --- | --- |
| **L0 — Unit** | Pure packages, table-driven; mocks via mockery on small interfaces | `go test`, race detector | `task test` / `task coverage` |
| **L1 — Controller / API** | envtest (Ginkgo), webhook handlers, reconciler branches | controller-runtime envtest | included in `task coverage` |
| **L2 — Golden / contract** | OpenAPI fragments, sample YAML decode, extractor goldens | checked-in `test/` + `config/samples/` | `task test`; samples per [ADR-0301](0301-event-driven-informers.md) |
| **L3 — Integration** | Real Postgres, Kafka, Git, S3, GCS, Redis, NATS via **testcontainers** | `-tags=integration` | `task test-integration` |
| **L4 — E2E** | Kind cluster: Helm install, smoke, export asserts | `hack/kind/e2e/` | `task test:e2e`; nightly workflow |
| **L5 — Load / perf (opt-in)** | Bounded synthetic scale (≤2000 objects) | `-tags=load`, `KOLECT_LOAD_TEST=1` | `task load-test`; `task bench`; `task perf-report` |

**Direction:** most tests live at L0–L2; every new sink backend must reach **L3** before merge
([NFR-EXT-3](../REQUIREMENTS.md)); L4 catches wiring regressions webhooks/RBAC/informers miss
([ADR-0301](0301-event-driven-informers.md)).

### Merge gates (PR / push to `main`)

Binding jobs in `.github/workflows/ci.yaml`:

| Gate | Command | Blocks merge |
| --- | --- | --- |
| Secret scan | `gitleaks detect` | Yes |
| Codegen drift | `task verify` | Yes |
| Vulnerabilities | `task vulncheck` | Yes |
| Format + lint | `task format:check`, `task lint` | Yes |
| Unit + envtest + coverage floor | `task coverage` (`COVERAGE_MIN`, default **60** on `./internal/...`) | Yes |
| Compile | `task build` | Yes |
| Sink / transport integration | `task test-integration` (Docker) | Yes |
| Helm packaging | `task helm-test` (lint + unittest) | Yes |
| Docker image build | `task docker:build` | Yes |
| Perf snapshot | `task perf-report` | **No** (`continue-on-error: true`) |

**Pre-commit / local:** `task verify`, `task lint`, `task scrub` ([ADR-0104](0104-security-model.md)
scrub list) before commit; `CONTRIBUTING.md` documents the contributor loop.

**Preflight workflow** (`.github/workflows/preflight.yaml`) runs `task verify` on demand for fast drift
checks.

### Scheduled / manual tiers (non-blocking on PR)

| Workflow | Trigger | Purpose |
| --- | --- | --- |
| `e2e-nightly.yaml` | cron + `workflow_dispatch` | Kind setup/smoke, git-export assert, integration asserts |
| `test-e2e.yaml` | manual / path filters | Extended e2e when needed |
| `release.yaml` | tag | Supply chain gates ([ADR-0705](0705-release-supply-chain.md)) |

E2E does **not** block every PR (cost/latency); **NFR-TEST-3** is satisfied by nightly + release
validation, not per-commit kind.

### Scale and load bounds

- Default **`task test` / `task coverage`**: synthetic object caps **≤500** ([ADR-0603](0603-performance-scalability.md)).
- **`task load-test`**: requires `KOLECT_LOAD_TEST=1`; hard cap **2000** objects — never in default CI.
- **`task bench`**: micro-benchmarks on hot paths (CEL/JSONPath extract); safe in dev/CI excerpt via
  `task perf-report`.

### Planned gates (decided, not yet wired)

- **Q16 — RBAC audit:** kubeaudit-style CI job on rendered RBAC ([ADR-0104](0104-security-model.md)).
- **Q15 — Supply chain:** cosign attestations, chart signing, OpenSSF scorecard post-rc
  ([ADR-0705](0705-release-supply-chain.md)).
- **Coverage target:** raise `COVERAGE_MIN` toward **70%** on `./internal/...` once backlog stabilizes
  (coordinator roadmap).

### Definition of done (per change)

From [GUIDELINES.md](https://github.com/konih/kollect/blob/main/GUIDELINES.md) §6, enforced by review:

- Relevant tests green at the appropriate tier; lint clean; **`task verify` no drift**.
- New external I/O: timeouts + backoff; no secrets in logs; status conditions updated.
- Non-trivial decisions → ADR update.

## Consequences

- Contributors know exactly which commands CI runs and which are nightly/opt-in.
- Sink backends cannot merge without integration proof — aligns with registry design
  ([ADR-0406](0406-sink-registry.md)).
- Perf/load work stays opt-in — default CI stays fast and deterministic.
- Packaging regressions fail via `helm-test` ([ADR-0704](0704-helm-chart-crd-lifecycle.md)).

## Open questions

- **OPEN:** Promote **`task perf-report`** from optional to blocking once baseline is stable in
  `PERF-SNAPSHOT`?
- **OPEN:** Per-PR **path-filtered e2e** for webhook/cert changes ([ADR-0105](0105-webhook-serving-cert-management.md))
  vs nightly-only?
- **OPEN:** **`COVERAGE_MIN=70`** effective date — tie to v0.1.0-rc or v0.1.0 tag?
- **OPEN:** Integration job sharding (Postgres vs Kafka vs object-store) if `test-integration` runtime
  exceeds ~15 minutes?
