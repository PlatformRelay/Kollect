# Local lab runbook (adaptive laptop / existing cluster)

Maintainer **L4.5** validation against an **existing** Kubernetes test cluster (Kind, K3s,
Talos driving-range, or similar). Zero cloud cost: the harness **discovers** the current
`KUBECONFIG` context and **never creates or destroys** the cluster.

This page documents real flags from [`hack/lab/`](https://github.com/platformrelay/kollect/tree/main/hack/lab)
([ADR-0707](../adr/0707-lab-harness.md)). Publishable evidence shape and redaction live in the
[lab evidence bundle](lab-evidence-bundle.md). Per-sink local substitutes and emulator limits:
[lab backend fidelity](lab-backend-fidelity.md). Function→scenario coverage:
[lab scenario matrix](lab-scenario-matrix.md). The [load test runbook](load-test-runbook.md) remains
the separate **100k / two-cluster cloud** claim gate — do not word laptop or Talos results as that
gate.

## Discover an existing cluster

1. Confirm context and nodes (do **not** provision a new cluster for this path):

   ```sh
   kubectl config current-context
   kubectl get nodes -o wide
   kubectl version --client
   ```

2. Run preflight against that kubeconfig:

   ```sh
   bash hack/lab/preflight.sh
   # residue from a prior lab run:
   bash hack/lab/preflight.sh --force
   ```

3. Drive schedules with the resumable runner (preflight is invoked automatically unless
   `--skip-preflight`):

   ```sh
   bash hack/lab/run.sh --schedule quick --tier auto --run-id lab-demo-1
   bash hack/lab/run.sh --schedule quick+sinks --run-id lab-demo-2 --resume --seed 42
   ```

Offline / CI meta-tests use `--dry-run` (and preflight `--fixture=clean|residue`). Live scenario
bodies may still emit `BLOCKED` until implemented — stubs never paper-green as `PASS`.

## Isolation contract

All mutation for a run is confined to that run’s identity. Unrelated namespaces must survive.
Until live scenario bodies land, `--keep-lab` and the runner’s default “cleanup” messages are
**hints only** (`hack/lab/run.sh` prints intent; it does **not** automatically delete lab
namespaces/resources). The operator must clean labeled resources deliberately after a run.

| Mechanism | Value |
| --- | --- |
| Lab label | `kollect.dev/lab-run=<RUN_ID>` on lab resources |
| Namespaces | `kollect-lab-<RUN_ID>-*` (and other `kollect-lab-*` residue checked by preflight) |
| Helm release (when used) | `kollect-lab` in the lab install namespace |
| Cleanup | **Manual** — delete `kollect-lab-*` / `kollect.dev/lab-run=<RUN_ID>` resources when finished; `--keep-lab` only suppresses the runner’s cleanup *hint* |

Preflight exit **2** means isolation residue without `--force` — clear `kollect-lab-*` /
`kollect.dev/lab-run` resources, or pass `--force` deliberately.

## Capacity tiers (`tier=auto`)

`--tier auto|S|M|L` is accepted by `hack/lab/run.sh`. The S/M/L steps, plateau checks, and stop
thresholds below are **documented guidance** for the operator — the flag may **no-op in v1**
(capacity gating is not fully enforced by the runner yet). Use **measured** host headroom
(available RAM after Docker/Kubernetes), not a marketing model name:

```sh
free -h
lscpu
df -h
kubectl get nodes
kubectl top nodes   # if metrics-server is present
```

| Tier | Available RAM (guide) | Suggested scale steps (collected rows) | Max attempt ceiling | Backends during load |
| --- | ---: | --- | ---: | --- |
| **S** | `<12 GiB` | 500 → 2k → 5k | 10k | local/bare Git only |
| **M** | `12–24 GiB` | 500 → 2k → 5k → 10k | 20k | Git; Postgres in a **separate** serial step |
| **L** | `>24 GiB` | 500 → 2k → 5k → 10k → 20k | 50k | Git; Postgres separately |

The maximum is a **ceiling**, not a target. When following the `tier=auto` guidance, start at the
tier matching available RAM and grow only after a **15-minute stable plateau** (steady
reconcile/export, no growing backpressure). For the largest successful step, prefer a short churn
window then a longer soak only when the schedule supports it — `soak` itself is not implemented yet
(see Schedules).

### Stop thresholds → `LIMIT_REACHED`

Stop growth, preserve diagnostics under `artifacts/lab/<RUN_ID>/`, and record the attempted step as
`LIMIT_REACHED` (with a reason) when any condition persists for more than about five minutes — or
**immediately** on OOM / data-integrity failure:

- host free memory below ~10%, or swap growth above ~2 GiB
- filesystem holding container data above ~85%
- operator or node OOMKill, `MemoryPressure` / `DiskPressure`, or repeated eviction
- API p95 latency above ~2s or sustained client throttling / HTTP 429
- operator CPU throttling above ~25% while reconcile backlog grows
- dispatch queue staying near capacity without recovery after churn stops
- inventory convergence exceeding ~10 minutes at a new step
- exported item count / checksum diverging from the expected source set

`LIMIT_REACHED` is **valid upper-bound evidence**. Never coerce it (or `BLOCKED` / `SKIPPED`) to
`PASS`.

## Schedules

Checked-in registries live under `hack/lab/schedules/`. Expected wall time is approximate maintainer
guidance for a warm multi-node lab; laptop Kind runs vary.

| Schedule | Status | Approx duration | Prerequisites | Serial backends | Cleanup | Artifacts |
| --- | --- | --- | ---: | --- | --- | --- |
| `quick` | **Implemented** registry | ~30–90 min | Existing cluster; preflight OK; product pin / chart | None (no Wave-2b sinks) | Hint only until live scenarios; operator cleans labeled resources manually | `artifacts/lab/<RUN_ID>/` (+ report draft) |
| `quick+sinks` | **Implemented** registry | ~2–4 h | Same as `quick` plus ability to stand ClusterIP / temp remotes for sinks | Wave-2b backends **serial** — tear down before next (`hack/lab/lib/serial-backend.sh`) | Same (manual; `--keep-lab` is a hint) | Same |
| `full-lab-day` | **Declared; refuses** | — | — | — | — | Exit **2** `BLOCKED` until capacity gates + scenario scripts land |
| `soak` | **Declared; refuses** | — | — | — | — | Exit **2** `BLOCKED` until overnight Tier M/L schedule lands |

```sh
bash hack/lab/run.sh --schedule quick --tier auto --run-id <id>
bash hack/lab/run.sh --schedule quick+sinks --run-id <id> --seed 42
# These refuse until implemented:
bash hack/lab/run.sh --schedule full-lab-day --run-id <id>   # → BLOCKED
bash hack/lab/run.sh --schedule soak --run-id <id>           # → BLOCKED
```

Do not advertise `quick+sinks` as a full Ubuntu D-suite or Wave-4 / 100k proof. Public wording stays
**READY WITH CONDITIONS** with an explicit limitations list ([lab evidence bundle](lab-evidence-bundle.md)).

## Resume by scenario ID

Results accumulate in `artifacts/lab/<RUN_ID>/results.json`. Re-run with the **same** `--run-id` and
`--resume`:

```sh
bash hack/lab/run.sh --schedule quick+sinks --run-id lab-demo-2 --resume
```

Scenarios already recorded as `PASS` or `PASS_WITH_LIMITATION` are skipped; other rows (including
`FAIL`, `SKIPPED`, `LIMIT_REACHED`, `BLOCKED`) can be retried without a destructive full redo of the
cluster or of completed evidence. Seed (`--seed`) stays stable across resume when you pass the same
value.

## Harness flags

### `hack/lab/run.sh`

| Flag | Meaning |
| --- | --- |
| `--schedule NAME` | `quick` \| `quick+sinks` \| `full-lab-day` \| `soak` (last two refuse until implemented) |
| `--run-id ID` | Lab run id → `artifacts/lab/<RUN_ID>/` (default: generated `lab-<utc>-…`) |
| `--resume` | Skip scenarios already `PASS` / `PASS_WITH_LIMITATION` in `results.json` |
| `--seed N` | Deterministic seed forwarded to scenario scripts (default `0`) |
| `--keep-lab` | Hint only — suppress default cleanup *message*; does not auto-delete resources until live scenarios land (operator cleans manually) |
| `--tier auto\|S\|M\|L` | Capacity tier hint; S/M/L table above is documented guidance (`auto` may **no-op in v1**) |
| `--dry-run` | Offline stubs only — no live `kubectl`/`helm` mutations |
| `--artifacts-root DIR` | Results root (default `<repo>/artifacts/lab`) |
| `--skip-preflight` | Advanced / nested tests only |

Exit codes: **0** OK (excluded rows may be `SKIPPED` with reasons); **1** usage / hard fail /
scenario `FAIL`; **2** schedule refused (`BLOCKED` / unimplemented).

### `hack/lab/preflight.sh`

| Flag / env | Meaning |
| --- | --- |
| `--force` | Allow prior `kollect-lab-*` / `kollect.dev/lab-run` residue |
| `--fixture=clean\|residue` | Offline meta-test mode (`KOLLECT_LAB_PREFLIGHT_FIXTURE`) |
| `KOLLECT_LAB_PREFLIGHT_STRICT=1` | Missing optional tools → exit **3** |

### Evidence + report

```sh
bash hack/lab/collect-evidence.sh --run-id <id> [--out-root artifacts/lab] [--dry-run]
bash hack/lab/report.sh --run-id <id>
# or: bash hack/lab/report.sh --run-dir artifacts/lab/<id>
```

`report.sh` emits `summary.md` + `checksums.txt` and runs the redaction gate. Never commit raw
artefacts, kubeconfigs, or tokens.

Companion helpers: `hack/lab/workload.sh` (labeled batch/churn; not required for default
`quick` / `quick+sinks`). Script-level detail:
[`hack/lab/README.md`](https://github.com/platformrelay/kollect/blob/main/hack/lab/README.md).

## Verdicts that are not PASS

Machine-emitted rows must keep an explicit reason. **Never coerce** non-pass outcomes to `PASS`.

| Verdict | Counts as pass? |
| --- | --- |
| `PASS` | yes |
| `PASS_WITH_LIMITATION` | yes, with an explicit limitation |
| `FAIL` | no |
| `SKIPPED` | no — schedule exclusion or precondition |
| `LIMIT_REACHED` | no — capacity / time / stop threshold |
| `BLOCKED` | no — unimplemented schedule or live scenario stub |

Align published matrices with the [lab evidence bundle](lab-evidence-bundle.md) contract.

## Related

- [ADR-0707: Lab harness architecture](../adr/0707-lab-harness.md)
- [Lab evidence bundle](lab-evidence-bundle.md)
- [Load test runbook (100k cloud gate)](load-test-runbook.md)
- [Performance and scaling](performance.md)
- [Testing strategy — multi-node lab evidence](../development/testing.md#multi-node-lab-evidence)
- [`hack/lab/README.md`](https://github.com/platformrelay/kollect/blob/main/hack/lab/README.md)
