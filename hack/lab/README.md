# Lab harness (`hack/lab/`)

Maintainer **L4.5** evidence runner for an **existing** kubeconfig (driving-range Talos,
Ubuntu laptop, or Kind as one consumer). Architecture: [ADR-0707](../../docs/adr/0707-lab-harness.md).

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

## Workload (LAB-H03)

Minimal labeled batch/churn helper: `bash hack/lab/workload.sh --run-id <id> --dry-run --out-dir <dir>`
(always labels `kollect.dev/lab-run=<RUN_ID>`; not required in default `quick`/`quick+sinks`).
