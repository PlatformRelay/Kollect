# Lab harness (`hack/lab/`)

Maintainer **L4.5** evidence runner for an **existing** kubeconfig (driving-range Talos,
Ubuntu laptop, or Kind as one consumer). Architecture: [ADR-0707](../../docs/adr/0707-lab-harness.md).
Operator walkthrough: [Local lab runbook](../../docs/operator-manual/local-lab-runbook.md).

Non-Kind kubeconfig is first-class. Scripts **never** create or destroy a cluster.

## Substrate allowlist (LAB-DEKIND)

Lab tooling port-forwards, installs Helm values and applies load, so it refuses to run
against a cluster it does not recognise. The permitted kube contexts are enumerated in
[`substrates.conf`](substrates.conf) and enforced by [`lib/substrate.sh`](lib/substrate.sh):

| Context pattern | Substrate | Image delivery |
| --- | --- | --- |
| `kind-*` | `kind` | build + `kind load docker-image` |
| `kumulus-lab` (cluster `kumulus`) | `talos` | pinned registry reference only |

**Default-deny**: an unlisted context — including a maintainer's ambient production
context — is refused with exit **2**. Widen the list only via
`KOLLECT_LAB_ALLOWED_CONTEXTS` (`pattern[=substrate[=cluster]]`, comma separated) or
`KOLLECT_LAB_SUBSTRATES_FILE`; both are validated the same way and a wildcard-only or
under-specific pattern (`*`, `*-prod`, `k*`) fails the load closed. When an entry names an
expected cluster, a cluster-name mismatch can only **refuse** — it never admits a context
the pattern did not already match.

Non-Kind substrates have **no** `kind load` equivalent, so the operator image must come from
a registry at an immutable reference (`ghcr.io/platformrelay/kollect:v<semver>`). A local-only
or mutable tag (`:dev`, `:latest`, untagged) is rejected before anything is installed rather
than silently reusing whatever the nodes already cached.

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

## Perf pprof quick path (LAB-H10 / PERF-LAB-01)

Quick pprof path for **any allowlisted substrate** — Kind or the kumulus Talos lab. CI verifies
the offline `--dry-run` machine-encoded quick path; live capture uses localhost port-forward +
`curl`/`go tool pprof` (maintainer opt-in).

```sh
task perf-lab:quick          # alias: task perf-kind:quick (unchanged)
bash hack/lab/perf-kind.sh --dry-run --run-id perf-demo --objects 500 --seed 42 --duration 60s

# Live on Kind (release "kollect" in kollect-system)
bash hack/lab/perf-kind.sh --run-id perf-live --objects 500

# Live on the kumulus Talos lab (release kollect-op1 in namespace kollect-op1)
KUBECONFIG=<kumulus kubeconfig> bash hack/lab/perf-kind.sh --run-id perf-live \
  --release kollect-op1 --namespace kollect-op1 --objects 500
```

Both live paths need `pprof.enabled: true` on the release — the manager does not serve
`:6060` otherwise and the run exits **BLOCKED** rather than emitting placeholder profiles.

Live port-forward (never a public Service):

```sh
kubectl -n kollect-system port-forward deploy/kollect-controller-manager 16060:6060
```

| Flag | Meaning |
| --- | --- |
| `--dry-run` | Offline fixture: DOC-02 evidence + `profiles/index.md` with `.pb.gz.stub` placeholders |
| `--objects` | `100` \| `500` \| `2000` for live converge/churn (metadata-only in `--dry-run`) |
| `--duration` | Phase dwell hint (default `60s`); CPU profile sample capped at `30s` |
| `--namespace` | Manager namespace for port-forward (default: `kollect-system`) |
| `--release` | Helm release name → `deploy/<release>-controller-manager` (default: `kollect`) |
| `--seed` | Deterministic fixture metadata in `--dry-run` |
| `--allow-non-kind` | Maintainer override for a context that is **not** on the substrate allowlist. Allowlisted lab clusters (including kumulus) need no flag |
| `--keep-lab` | Hint: retain `kollect.dev/lab-run=<RUN_ID>` labeled workload |

Context fixtures for offline meta-tests: `--fixture=context-kind | context-kumulus |
context-non-kind | context-ambiguous | context-kumulus-lookalike | context-prod-lookalike`.

Phases: **idle → converge(N) → churn → recover**. pprof is off in product Helm by default; enable
`pprof.enabled: true` on the release. Access via **localhost port-forward only** — the Service spec
does not expose `:6060`; forward the deployment container port (matches load-test-runbook).

On interrupt: tear down port-forward; delete only resources labeled `kollect.dev/lab-run=<RUN_ID>`;
keep partial `profiles/` artifacts. Live runs that cannot reach the pprof endpoint exit **BLOCKED**
with reason (no silent `.stub` placeholders).

Meta-tests: `hack/test/lab_perf_kind_meta_test.sh`, `hack/test/lab_substrate_meta_test.sh`,
`hack/test/lab_image_delivery_meta_test.sh` (offline only; run together via
`bash hack/test/lab_harness_meta_suite.sh`).

## Webhook rejection against an existing cluster (U-08)

The webhook scenario no longer needs a freshly created Kind stack. With
`KOLLECT_E2E_EXISTING_CLUSTER=1`, `hack/e2e/webhook-smoke.sh` asserts against whatever
allowlisted cluster the current context points at, **read-only** (every apply is a
server-side dry run), so it is safe to run against a live release that is holding evidence:

```sh
KUBECONFIG=<kumulus kubeconfig> KOLLECT_E2E_EXISTING_CLUSTER=1 \
  KOLLECT_RELEASE=kollect-op1 KOLLECT_NAMESPACE=kollect-op1 \
  task lab:webhook-smoke
```

Exit **2** = context not on the allowlist; exit **4** = the release has no validating webhook
configuration (install/upgrade it with webhooks enabled first — the operator's
`--validating-webhooks-enabled=false` is a precondition failure, not a product bug).
