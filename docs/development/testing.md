# Testing strategy

Kollect is **TDD-first**. Quality gates follow a six-tier test pyramid (L0–L5) defined in
[ADR-0706: Testing and merge-gate architecture](../adr/0706-testing-merge-gate-architecture.md).

!!! tip "Quick local loop"
    Before opening a PR: `task lint` · `task coverage` · `task verify` · `task scrub`. See
    [CONTRIBUTING.md](../../CONTRIBUTING.md) for the full checklist.

## Test pyramid (L0–L5)

| Tier | Scope | Blocks merge? | Typical command |
| --- | --- | --- | --- |
| **L0 — Unit** | Pure packages, table-driven tests, mocks | Yes | `task test` |
| **L1 — Controller / API** | envtest reconcilers, webhooks | Yes | `task coverage` |
| **L2 — Golden / contract** | OpenAPI fragments, sample YAML, extractor goldens | Yes | `task test` |
| **L3 — Integration** | Real Postgres, Kafka, Git, S3, GCS, Redis, NATS (testcontainers) | Yes | `task test-integration` |
| **L4 — E2E** | Kind cluster: Helm install, smoke, export asserts | Nightly / path-filtered PR | `task test:e2e` |
| **UI — Playwright** | React SPA smoke (MSW dev server); not backend Kind L4 | No (optional locally) | `task ui-e2e` · `task ui-e2e:docker` |
| **L5 — Load / perf** | Bounded synthetic scale (≤2000 objects), micro-benchmarks | Opt-in | `task load-test` · `task perf-report` |

**Direction:** Most tests live at L0–L2. Every new sink backend must reach **L3** before merge
([NFR-EXT-3](../REQUIREMENTS.md)). L4 catches wiring regressions that unit tests miss. L5 stays
opt-in so default CI stays fast.

## Coverage target

Statement coverage on `./internal/...` is enforced by `hack/coverage.sh` via `task coverage`:

| Setting | Value |
| --- | --- |
| **Target (pre-v0.1.0)** | **80%** — ratchet `COVERAGE_MIN` when measured coverage is sustained |
| **Current CI floor** | 65% (`COVERAGE_MIN` in `.github/workflows/ci.yaml`) |
| **Codecov project target** | 70% (`codecov.yml`) |

Regressions below the enforced floor fail CI. Raise the floor only after coverage has grown
sustainably — see ADR-0706 for the ratchet policy.

```sh
task coverage          # unit + envtest + floor check → coverage.out
task coverage:report   # go tool cover -func summary
task coverage:html     # coverage.html for browser review
```

Integration-tagged tests (`-tags=integration`) and e2e packages are excluded from the default
coverage profile.

## What CI runs on every PR

Binding jobs in `.github/workflows/ci.yaml` (see ADR-0706 for the full matrix):

- Secret scan (`gitleaks`), codegen drift (`task verify`), vulncheck, lint/format
- **Architecture fitness:** `go-arch-lint` via `task arch-lint` (import boundaries in
  `.go-arch-lint.yml`)
- **Dependency policy:** golangci-lint `depguard` + `gomodguard` (same `task lint` job)
- **L0–L2:** `task coverage` with coverage floor
- **L3:** `task test-integration` (Docker required)
- Helm packaging (`task helm-test`), image build (`task docker:build`)
- Native Go fuzz (CEL/JSONPath extractors, content hash)
- RBAC audit (`hack/audit-rbac.sh`)

**Non-blocking on PR:** `task perf-report` (promoted to blocking at **v0.2.0** per ADR-0706);
**SonarCloud** scan (`sonarcloud` job — needs `SONAR_TOKEN`; see
[tooling-setup.md](tooling-setup.md)).

### Holistic maintainability (SonarCloud)

SonarCloud mirrors coverage trends and surfaces duplication / technical-debt ratios over time —
complementing point-in-time `dupl` and Codecov. Configured in `sonar-project.properties`; optional
until the maintainer adds `SONAR_TOKEN`. Does not replace `task lint` or arch-lint.

## Scheduled and manual tiers

| Workflow | Tier | Purpose |
| --- | --- | --- |
| `e2e-nightly.yaml` | L4 | Kind smoke, git-export assert, integration re-check |
| `e2e-webhook-path.yaml` | L4 | Path-filtered kind smoke for webhook/cert changes |
| `test-e2e.yaml` | L4 | Extended e2e on demand |
| `release.yaml` | Supply chain | Image signing, SBOM, chart publish |

## Local development commands

| Task | Purpose |
| --- | --- |
| `task test` | Unit + envtest (no floor check) |
| `task test-integration` | L3 sink/transport integration (Docker) |
| `task test:e2e` | L4 kind smoke (setup → smoke → teardown) |
| `task ui-ci` | UI PR gate — Vitest, lint, build, mock drift (no Playwright) |
| `task ui-e2e` · `task ui-e2e:docker` | UI Playwright smoke — optional; distinct from backend L4 |
| `task bench` | Micro-benchmarks on hot paths |
| `KOLECT_LOAD_TEST=1 task load-test` | L5 bounded load (≤2000 objects, opt-in) |
| `task perf-report` | Benchmark + unit pass summary (local only, gitignored output) |

Full local setup: [DEVELOPMENT.md](../DEVELOPMENT.md).

## Definition of done

From project guidelines — enforced by review:

- Relevant tests green at the appropriate tier; lint clean; `task verify` shows no drift.
- New external I/O: timeouts + backoff; no secrets in logs; status conditions updated.
- Non-trivial decisions → ADR update in `docs/adr/`.

## Further reading

- [ADR-0706: Testing and merge-gate architecture](../adr/0706-testing-merge-gate-architecture.md)
- [GUIDELINES.md](https://github.com/konih/kollect/blob/main/GUIDELINES.md) §4 (testing rules)
- [REQUIREMENTS.md](../REQUIREMENTS.md) — NFR-TEST-* priorities
- [PERFORMANCE.md](../PERFORMANCE.md) — scale bounds and perf-report workflow
- [tooling-setup.md](tooling-setup.md) — arch-lint, depguard, SonarCloud maintainer steps
