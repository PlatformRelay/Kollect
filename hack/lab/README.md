# Lab harness (`hack/lab/`)

Maintainer **L4.5** evidence runner for an **existing** kubeconfig (driving-range Talos,
Ubuntu laptop, or Kind as one consumer). Architecture: [ADR-0707](../../docs/adr/0707-lab-harness.md).
Operator walkthrough: [Local lab runbook](../../docs/operator-manual/local-lab-runbook.md).

Non-Kind kubeconfig is first-class. Scripts **never** create or destroy a cluster.

## Preflight (LAB-H01)

```sh
bash hack/lab/preflight.sh
bash hack/lab/preflight.sh --force          # allow prior kollect-lab-* / lab-run residue
bash hack/lab/preflight.sh --fixture=clean  # offline meta-test mode
```

| Exit | Meaning |
| --- | --- |
| 0 | OK (clean, or residue with `--force`) |
| 1 | usage / invalid fixture / hard host failure |
| 2 | isolation residue without `--force` |
| 3 | `KOLLECT_LAB_PREFLIGHT_STRICT=1` and optional tools missing |

Offline fixtures: `--fixture=clean|residue` or `KOLLECT_LAB_PREFLIGHT_FIXTURE`. Meta-tests live under
`hack/test/lab_*_meta_test.sh` and must not call live `kubectl`/`helm`.

## Runner (LAB-H02)

Resumable schedule runner. Depends on preflight (invoked automatically; dry-run uses the clean
fixture unless `KOLLECT_LAB_PREFLIGHT_FIXTURE` is set).

```sh
bash hack/lab/run.sh --schedule quick --dry-run --run-id demo-1
bash hack/lab/run.sh --schedule quick+sinks --run-id demo-2 --resume --seed 42
bash hack/lab/run.sh --schedule quick --tier auto --keep-lab
```

| Flag | Meaning |
| --- | --- |
| `--schedule` | `quick` \| `quick+sinks` (required). `full-lab-day` / `soak` are declared but **refuse** (`BLOCKED`) until implemented |
| `--run-id` | Lab run id → `artifacts/lab/<RUN_ID>/results.json` |
| `--resume` | Skip scenarios already `PASS` / `PASS_WITH_LIMITATION` in `results.json` |
| `--seed` | Deterministic seed forwarded to scenario scripts |
| `--keep-lab` | Retain lab namespaces/resources (default cleans up) |
| `--tier` | `auto` \| `S` \| `M` \| `L` (accepted; capacity gating may no-op in v1) |
| `--dry-run` | Offline stubs only — no live `kubectl`/`helm` mutations |

Schedules live under `hack/lab/schedules/`. Scenario stubs under `hack/lab/scenarios/`. Serial Wave-2b
backends must tear down before the next (`hack/lab/lib/serial-backend.sh`). Skip / limit / blocked
rows always carry a machine-emitted reason — never an empty green cell.

**Stub honesty:** without `--dry-run`, scenario scripts emit `BLOCKED` (“live scenario not
implemented”) — they never paper-green as `PASS`. `--keep-lab` / default cleanup are **hints only**
until live scenario bodies exist; dry-run does not create or delete cluster resources.

## Workload (LAB-H03)

Minimal labeled batch/churn helper: `bash hack/lab/workload.sh --run-id <id> --dry-run --out-dir <dir>`
(always labels `kollect.dev/lab-run=<RUN_ID>`; not required in default `quick`/`quick+sinks`).

## Perf Kind pprof (LAB-H10 / PERF-LAB-01)

Kind-oriented quick pprof path. CI verifies the offline `--dry-run` machine-encoded quick path;
live Kind e2e is maintainer opt-in.

```sh
task perf-kind:quick
bash hack/lab/perf-kind.sh --dry-run --run-id perf-demo --objects 500 --seed 42
bash hack/lab/perf-kind.sh --run-id perf-live --objects 500   # requires kind-* context
```

| Flag | Meaning |
| --- | --- |
| `--dry-run` | Offline fixture: DOC-02 evidence + `profiles/index.md` (no kubectl) |
| `--objects` | `100` \| `500` \| `2000` synthetic object budget |
| `--seed` | Deterministic profile stub metadata |
| `--allow-non-kind` | Skip Kind context gate (never creates/destroys cluster) |
| `--keep-lab` | Hint: retain `kollect.dev/lab-run=<RUN_ID>` labeled workload |

Phases: **idle → converge(N) → churn → recover**. pprof is off in product Helm by default; lab
enables it only on a **dev** release (for example `kollect-dev`) via Helm values. Access via
**localhost port-forward only** — never expose pprof through a public Service.

On interrupt: tear down port-forward; delete only resources labeled `kollect.dev/lab-run=<RUN_ID>`;
keep partial `profiles/` artifacts.

Meta-tests: `hack/test/lab_perf_kind_meta_test.sh` (offline only).
