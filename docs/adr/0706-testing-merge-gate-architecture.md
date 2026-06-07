# ADR-0706: Testing and merge-gate architecture

> The test pyramid, CI merge gates, opt-in tiers (integration, load, e2e), and how quality gates map to
> `task` targets and GitHub Actions — lifted from [engineering guidelines](https://github.com/konih/kollect/blob/main/docs/development/guidelines.md) into a durable
> ADR.

**Theme:** 07 · Project & meta · **Status:** Current

## Context

Kollect is **TDD-first** with binding rules in [engineering guidelines](https://github.com/konih/kollect/blob/main/docs/development/guidelines.md) §4 and
[NFR-TEST-* in REQUIREMENTS.md](../REQUIREMENTS.md). CI (`.github/workflows/ci.yaml`) enforces a
subset on every PR; heavier tiers run nightly or on demand. The split — what **blocks merge** vs what
**signals risk** — was implicit in Taskfiles and workflows but not recorded, which made it hard to add
gates (e.g. Q16 RBAC audit, Q15 supply-chain attestations) without debate.

## Decision

### Test pyramid (fast → slow)

| Tier | Scope | Tooling | Typical `task` |
| --- | --- | --- | --- |
| **L0 — Unit** | Pure packages, table-driven; mocks via mockery on small interfaces | `go test` | `task test` / `task coverage` (CI: no `-race`) |
| **L1 — Controller / API** | envtest (Ginkgo), webhook handlers, reconciler branches | controller-runtime envtest | included in `task coverage`; use `task coverage:race` locally |
| **L2 — Golden / contract** | OpenAPI fragments, sample YAML decode, extractor goldens | checked-in `test/` + `config/samples/` | `task test`; samples per [ADR-0301](0301-event-driven-informers.md) |
| **L3 — Integration** | Real Postgres, Kafka, Git, S3, GCS, Redis, NATS via **testcontainers** | `-tags=integration` | `task test-integration` |
| **L4 — E2E** | Kind cluster: Helm install, smoke, export asserts | `hack/kind/e2e/`, `hack/e2e/` | `task test:e2e`; nightly workflow |
| **L5 — Load / perf (opt-in)** | Bounded synthetic scale (≤2000 objects) | `-tags=load`, `KOLECT_LOAD_TEST=1` | `task load-test`; `task bench`; `task perf-report` |

**Direction:** most tests live at L0–L2; every new sink backend must reach **L3** before merge
([NFR-EXT-3](../REQUIREMENTS.md)); L4 catches wiring regressions webhooks/RBAC/informers miss
([ADR-0301](0301-event-driven-informers.md)).

**L4 canonical path:** shell scripts under `hack/kind/e2e/` (cluster lifecycle + smoke) and
`hack/e2e/` (export, tenant, cert-manager asserts). CI and `task test:e2e` use these scripts;
`test/e2e/fixtures/` holds multitenant YAML templates consumed by `hack/e2e/multitenant.sh`. The
Kubebuilder Ginkgo scaffold (`go test -tags=e2e ./test/e2e/...`) was removed — it duplicated smoke
with `make deploy` instead of Helm and did not cover export or cert-manager collection paths.

### Merge gates (PR / push to `main`)

Binding jobs in `.github/workflows/ci.yaml`:

| Gate | Command | Blocks merge |
| --- | --- | --- |
| Secret scan | `gitleaks detect` | Yes |
| Codegen drift | `task verify` | Yes |
| Vulnerabilities | `task vulncheck` | Yes |
| Format + lint | `task format:check`, `task lint` | Yes |
| Unit + envtest + coverage floor | `task coverage` (`COVERAGE_MIN`, default **65** on `./internal/...`) | Yes |
| Compile | `task build` | Yes |
| Sink / transport integration | `task test-integration` (Docker) | Yes |
| Helm packaging | `task helm-test` (lint + unittest) | Yes |
| Docker image build | `task docker:build` | Yes |
| Perf snapshot | `task perf-report` | **No** (`continue-on-error: true`) |
| RBAC audit (Q16) | `bash hack/audit-rbac.sh` (Polaris danger + kubeaudit error on `config/rbac/role.yaml`) | Yes |
| Native Go fuzz | Matrix: `FuzzContentHash` (`internal/aggregate`); `FuzzExtractJSONPath`, `FuzzExtractCEL`, `FuzzValidateAttributePath` (`internal/collect`) — 30s each | Yes |

**Pre-commit / local:** `task verify`, `task lint`, `task coverage:race` (recommended),
`task scrub` ([ADR-0104](0104-security-model.md) scrub list) before commit;
`CONTRIBUTING.md` documents the contributor loop.

**Preflight workflow** (`.github/workflows/preflight.yaml`) runs `go mod tidy`/`go mod verify`,
`task verify`, and `task changelog:verify` — fast drift checks without lint or tests.

### Scheduled / manual tiers (non-blocking on PR)

| Workflow | Trigger | Purpose |
| --- | --- | --- |
| `e2e-smoke.yaml` | PR + push `main` (non-docs) | **Mandatory** kind smoke — inventory HTTP + family sink sample |
| `e2e-extended.yaml` | PR path / label `e2e/full` / dispatch | Tier 1 git-export, multitenant, tenant-mode, webhook profile |
| `e2e-nightly.yaml` | cron + `workflow_dispatch` | Full Kind matrix + bench/perf (L3 deduped) |
| `test-e2e.yaml` | `workflow_dispatch` | Manual full nightly matrix |
| `release.yaml` | tag | Supply chain gates ([ADR-0705](0705-release-supply-chain.md)) |
| `codeql.yaml` | push/PR `main`, weekly | CodeQL SAST for Go; SARIF → Code Scanning |

E2e **Tier 0 (`kind-smoke`)** blocks merge on every non-docs PR and `main` push; **NFR-TEST-3** full
matrix remains nightly + manual dispatch.

### Scale and load bounds

- Default **`task test` / `task coverage`**: synthetic object caps **≤500** ([ADR-0603](0603-performance-scalability.md)).
- **`task load-test`**: requires `KOLECT_LOAD_TEST=1`; hard cap **2000** objects — never in default CI.
- **`task bench`**: micro-benchmarks on hot paths (CEL/JSONPath extract); safe in dev/CI excerpt via
  `task perf-report`.

### Coverage floor

Statement coverage on `./internal/...` is enforced by `hack/coverage.sh` / `task coverage`:

| Phase | `COVERAGE_MIN` | When |
| --- | --- | --- |
| **Now (PR / `main`)** | **65%** | `.github/workflows/ci.yaml`, `Taskfile.yml`, `hack/coverage.sh` default |
| **Release candidate / tag** | **70%** | Ratchet when measured coverage is **≥ 70%** sustained on `main`, or at **`v0.3.0-rc`** tag cut — whichever comes first |

Measured coverage after the TEST-PYRAMID #3 tranche (2026-06-05): **~69.4%**. A follow-on unit-test tranche
(2026-06-05) raised measured `./internal/...` coverage to **~74%**; aspirational target before the **70%**
ratchet is **~80%** — merge gate stays at **65%** until then. **Codecov** target remains **70%** (see
`codecov.yml`).

### Planned gates (decided, not yet wired)

- **Q15 — Supply chain:** cosign attestations, chart signing, OpenSSF scorecard post-rc
  ([ADR-0705](0705-release-supply-chain.md)).
- **OSS-Fuzz:** upstream integration for CEL/JSONPath parsers — native `testing.F` fuzz now covers
  `internal/aggregate` and `internal/collect` extractors in CI; OSS-Fuzz remains post-GA.

### Definition of done (per change)

From [engineering guidelines](https://github.com/konih/kollect/blob/main/docs/development/guidelines.md) §6, enforced by review:

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

- **OPEN:** Promote **`task perf-report`** from optional to blocking at **v0.4** once baseline is stable in
  `PERF-SNAPSHOT`?
- **RESOLVED (2026-06-07):** Mandatory **Tier 0 `kind-smoke`** on all non-docs PRs via
  `.github/workflows/e2e-smoke.yaml`; webhook profile moved to optional **`e2e-extended.yaml`**
  ([ADR-0105](0105-webhook-serving-cert-management.md)).
- **RESOLVED (2026-06-05):** Per-PR **path-filtered e2e** for webhook/cert changes — superseded by
  Tier 0 smoke + Tier 1 extended webhook job (formerly `e2e-webhook-path.yaml`).
- **RESOLVED (2026-06-07):** **`COVERAGE_MIN=70`** — ratchet at **`v0.3.0-rc`** tag or when
  measured `./internal/...` coverage is **≥ 70%** sustained on `main` (see **Coverage floor** above).
  PR floor remains **65%** until then.
- **OPEN:** Integration job sharding (Postgres vs Kafka vs object-store) if `test-integration` runtime
  exceeds ~15 minutes?
