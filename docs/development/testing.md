# Testing strategy

Kollect is **TDD-first**. Quality gates follow a six-tier test pyramid (L0–L5) defined in
[ADR-0706: Testing and merge-gate architecture](../adr/0706-testing-merge-gate-architecture.md).

!!! tip "Quick local loop"
    Before opening a PR: `task lint` · `task coverage` · `task coverage:race` (recommended) ·
    `task verify` · `task scrub`. See [Coding standards](coding-standards.md) and
    [CONTRIBUTING.md](https://github.com/platformrelay/kollect/blob/main/CONTRIBUTING.md) for the full checklist.

## Test pyramid (L0–L5)

| Tier | Scope | Blocks merge? | Typical command |
| --- | --- | --- | --- |
| **L0 — Unit** | Pure packages, table-driven tests, mocks | Yes | `task test` |
| **L1 — Controller / API** | envtest reconcilers, webhooks | Yes | `task coverage`; nightly advisory `task coverage:race` |
| **L2 — Golden / contract** | OpenAPI fragments, sample YAML, extractor goldens | Yes | `task test` |
| **L3 — Integration** | Real Postgres, Kafka, Git, S3, GCS, Redis, NATS (testcontainers) | Yes | `task test-integration` |
| **L4 — E2E** | Kind cluster: Helm install, smoke, export asserts | **PR smoke (required)** + nightly / extended | `task test:e2e` |
| **L5 — Perf budget / benchmarks** | In-process extractor hot path (no cluster, sinks or concurrency) | Default + nightly | `task extract-budget` · `task bench` · `task perf-report` |

**Direction:** Most tests live at L0–L2. Every new sink backend must reach **L3** before merge
([NFR-EXT-3](../REQUIREMENTS.md)). L4 catches wiring regressions that unit tests miss. L5 is an
in-process budget, not a scale tier: it fails a >25% regression in the extractor's `B/op` or
`allocs/op` (its `ns/op` ceiling is only a catastrophic-regression net), and says nothing about
cluster scale. Cluster scale is the opt-in `scale-envtest-10k` job.

## Coverage target

Statement coverage on `./internal/...` is enforced by `hack/coverage.sh` via `task coverage`:

| Setting | Value |
| --- | --- |
| **Target (pre-v0.10)** | Hold 90% floor; further ratchet only after measured CI coverage sustains ≥90.5% (COV-90-00) |
| **Current CI floor** | **90%** (`COVERAGE_MIN` in `Taskfile.yml` and `.github/workflows/ci.yaml`) |
| **Codecov project target** | See `codecov.yml` (advisory relative to CI floor) |

Regressions below the enforced floor fail CI. Raise the floor only after coverage has grown
sustainably — see ADR-0706 for the ratchet policy.

```sh
task coverage          # unit + envtest + floor check → coverage.out (CI path; no -race)
task coverage:race     # COVERAGE_RACE=1 + CGO_ENABLED=1 (local + nightly advisory CI)
task coverage:report   # go tool cover -func summary
task coverage:html     # coverage.html for browser review
```

Integration-tagged tests (`-tags=integration`) and e2e packages are excluded from the default
coverage profile.

## What CI runs on every PR

**Path filters:** **Preflight** and **CodeQL** run on every PR. Preflight always runs
`task lint:markdown`, so Markdown in code, docs-only, and mixed PRs is checked consistently.
**CI** and **E2E smoke** also *start* on every PR, but their expensive jobs skip when the PR
changes *only* documentation paths (`docs/**`, `mkdocs.yml`, `README.md`, `CHANGELOG.md`,
`CONTRIBUTING.md`, `LICENSE`, and issue templates). Other root Markdown files still run
everything.
The path-scoped **Docs** workflow runs the complete `task docs:verify` gate, then (on
`main` push) deploys to
[platformrelay.github.io/Kollect](https://platformrelay.github.io/Kollect/). Any change under
`api/`, `internal/`, `charts/`, `cmd/`, `config/`, `hack/`, `test/`, `go.mod`, or
`.github/workflows/` — or a mixed docs+code PR — runs the full gate below. Release tags no longer
trigger docs deploys; the site tracks `main` only.

**Docs-only PRs merge without a bypass (CI-DOCSGATE-01).** They used to need a maintainer
**ruleset bypass** or `gh pr merge --admin`: `paths-ignore` on `pull_request` meant CI and E2E
smoke were never *dispatched*, and a workflow GitHub never dispatches does not report its
contexts as skipped — it does not report them at all, so the required checks `test` and
`kind-smoke` stayed permanently absent and the PR sat at `BLOCKED`. Both workflows now trigger
on every PR and make the decision themselves:

- a `changes` job classifies the PR (documentation-only, or not) from its diff, failing safe to
  "not documentation-only" for anything it cannot classify;
- the real work lives in `test-suite` (CI) and `kind-smoke-run` (E2E smoke), gated on that
  verdict;
- the jobs that carry the *required context names* — `test` and `kind-smoke` — run
  `if: always()` and only report. They fail unless the real job actually succeeded whenever the
  PR touches anything outside the documentation path set, so the no-op path cannot stand in for
  a real run.

`paths-ignore` remains on the `push` trigger of both workflows, where no required context is at
stake. `hack/test/ci_docs_gate_test.sh` locks all of this in — including that CodeQL and
Preflight stay unfiltered on `pull_request`, since they own the other two required contexts.

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

**E2E smoke (L4 Tier 0):** `.github/workflows/e2e-smoke.yaml` job **`kind-smoke-run`** on every
non-docs PR and push to `main` (same documentation path set as CI). The required branch-protection
context **`kind-smoke`** is the reporting job that wraps it — see
[coding-standards.md](coding-standards.md).

**Non-blocking on PR:** `e2e-extended.yaml` (Tier 1 matrix + webhook profile; label `e2e/full` or
path-filtered), `task perf-report` (promoted to blocking at **v0.4** per ADR-0706);
**SonarCloud** scan (`sonarcloud` job — needs `SONAR_TOKEN`; see
[tooling-setup.md](tooling-setup.md)).

### Holistic maintainability (SonarCloud)

SonarCloud mirrors coverage trends and surfaces duplication / technical-debt ratios over time —
complementing point-in-time `dupl` and Codecov. Configured in `sonar-project.properties`; optional
until the maintainer adds `SONAR_TOKEN`. Does not replace `task lint` or arch-lint.

**Codecov** uploads run in the **`test-suite`** job (OIDC auth, non-blocking). PR patch comments require
the maintainer to install the [Codecov GitHub App](https://github.com/apps/codecov) once — see
[tooling-setup.md § Codecov](tooling-setup.md#codecov-maintainer-setup).

## Scheduled and manual tiers

| Workflow | Tier | Purpose |
| --- | --- | --- |
| `preflight.yaml` | All PRs | Markdown lint, codegen and changelog drift, module consistency |
| `docs.yaml` | Docs paths | Markdown lint, strict MkDocs build, GitHub Pages deploy (`main` only) |
| `e2e-smoke.yaml` | L4 Tier 0 | **Mandatory** kind smoke on PR + `main` (job `kind-smoke-run`, reported as `kind-smoke`) |
| `e2e-extended.yaml` | L4 Tier 1 | Optional git-export, multitenant, tenant-mode, webhook profile |
| `e2e-nightly.yaml` | L4 Tier 2 | Full Kind matrix + bench/perf (deduped L3) + advisory race |
| `test-e2e.yaml` | L4 Tier 3 | Manual full matrix (`workflow_dispatch`) |
| `release.yaml` | Supply chain | Image signing, SBOM, chart publish |

### Nightly advisory race detector (HY-07 / TEST-02)

`e2e-nightly.yaml` runs a **`race`** job (`continue-on-error: true`) that executes
`task coverage:race` with `CI=true`, `CGO_ENABLED=1`, and `COVERAGE_MIN=0` (racing is the
signal — the merge-gate floor stays on the non-race `test` job). On a `WARNING: DATA RACE`
finding the step summary prints an excerpt; **file a GitHub issue labeled `race`/`flake`**
and do not ignore it. Timeouts/compile failures are summarized distinctly from race reports.
Timeout is generous (`timeout-minutes: 45`) because `-race` is typically 2–10× slower.

Set repository variable **`GIT_EXPORT_TEST_REPO`** (Settings → Actions → Variables) to enable full
remote git SHA assert in git-export scenarios. Without it, jobs verify inventory HTTP hash only.

For **local** runs the variable is optional: export `GIT_EXPORT_TEST_REPO` to a dedicated test repo
(or `file://` bare remote) when exercising push assertions; unit and envtest tiers do not require it.

## Multi-node lab evidence

Kind L4 proves single-node wiring. Separately, published **v0.16.0** was exercised on a Talos lab
with **1 control plane + 2 workers** (`quick+sinks`, **ready with conditions**). Maintainer
multi-node / existing-cluster evidence sits as **L4.5** beside Kind L4 and perf-budget L5 —
[ADR-0707: Lab harness architecture](../adr/0707-lab-harness.md). Publishable shape, redaction,
and an example matrix live in the
[lab evidence bundle contract](../operator-manual/lab-evidence-bundle.md).

**Observed (bounded)**

- Two manager replicas on distinct workers; leader failover without unexpected restarts
- Inventory collection and export to in-cluster Postgres
- ClusterIP Postgres / MinIO / NATS with Helm `allowPrivateSinks: true`
  ([resolved-address policy](../security/resolved-address-policy.md))
- Private GitHub and GitLab snapshot export (connection + push)
- Certificate scrape non-zero (partial versus live cluster certificates)
- Idle pprof captured; Wave-4 / Tier-S load **not** run

**Not claimed from that run**

- Wave-4 / Tier-S load (500–2k synthetic rows) or full pprof under churn
- 100k rows/cluster design proof ([load-test runbook](../operator-manual/load-test-runbook.md))
- Full Ubuntu D-suite / every sink backend / NetPol deny-path / worker drain / Argo scrape

Treat multi-node lab results as **bounded evidence for a named pin**, not a substitute for Kind CI
or the 100k cloud gate. Raw protocols stay local-only — see the
[evidence bundle contract](../operator-manual/lab-evidence-bundle.md).

## Local development commands

| Task | Purpose |
| --- | --- |
| `task docs:verify` | Tracked Markdown, truth/freshness contracts, samples, strict site build, and browser layout when Chrome is available |
| `task test` | Unit + envtest (no floor check; no race detector) |
| `task coverage` | Unit + envtest + 90% floor (CI; CGO off, no `-race`) |
| `task coverage:race` | Same as coverage with race detector (local + nightly advisory) |
| `task test-integration` | L3 sink/transport integration (Docker) |
| `task test:e2e` | L4 kind smoke (setup → smoke → teardown) |
| `task bench` | Micro-benchmarks on hot paths |
| `task extract-budget` | L5 extractor hot-path budget — >25% gate on B/op + allocs/op only (in-process; not cluster scale) |
| `task perf-report` | Benchmark + unit pass summary (local only, gitignored output) |

Full local setup: [development/setup.md](../development/setup.md).

`task docs:verify` is the canonical local reproduction for the required Docs workflow. It skips
only the real-browser layout check when Chrome/Chromium is unavailable. To reproduce CI's hard
requirement exactly, run `DOCS_REQUIRE_CHROME=1 task docs:verify`; set `CHROME_BIN` when the browser
is not on `PATH`.

## Writing chart tests (helm-unittest gotchas)

Two matcher behaviours produce assertions that **pass unconditionally**. Both were found by a
review that mutated the implementation and watched the suite stay green.

- **`contains` / `notContains` compare WHOLE list elements, not substrings.** So
  `notContains: --my-flag=` can never match a rendered `--my-flag=60s`, and the test passes
  whether or not the flag is emitted. When asserting that something is *absent* from a rendered
  arg list, assert the **exact list** instead:

  ```yaml
  # kollect-doc: ignore helm-unittest assertions, not a kollect CR
  - equal:
      path: spec.template.spec.containers[0].args
      value: [--leader-elect, --health-probe-bind-address=:8081, ...]
  ```

- **`containsDocument` is evaluated against EVERY rendered document.** A multi-document template
  (e.g. a ClusterRole plus its ClusterRoleBinding) can therefore never satisfy two different
  `containsDocument` assertions at once. Assert per `documentIndex` with `isKind` instead.

**Before trusting a new chart assertion, mutate the template and confirm the test goes red.** A
chart test that cannot fail is worse than no test — it reports coverage that does not exist.

## Definition of done

Per-change checklist: [guidelines § 6](guidelines.md#6-definition-of-done-per-change).
PR workflow: [CONTRIBUTING.md § Pull request process](https://github.com/platformrelay/kollect/blob/main/CONTRIBUTING.md#pull-request-process).

## Further reading

- [ADR-0706: Testing and merge-gate architecture](../adr/0706-testing-merge-gate-architecture.md)
- [ADR-0707: Lab harness architecture](../adr/0707-lab-harness.md) (L4.5 maintainer evidence)
- [Engineering guidelines](https://github.com/platformrelay/kollect/blob/main/docs/development/guidelines.md) §4 (testing rules)
- [REQUIREMENTS.md](../REQUIREMENTS.md) — NFR-TEST-* priorities
- [operator-manual/performance.md](../operator-manual/performance.md) — scale bounds and perf-report workflow
- [tooling-setup.md](tooling-setup.md) — arch-lint, depguard, SonarCloud maintainer steps
