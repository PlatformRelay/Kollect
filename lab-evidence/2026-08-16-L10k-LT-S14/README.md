# Archived — L-10k / LT-S14 evidence, captured 2026-08-16 before U-05

**Why this archive exists.** `PERFTEST-RESULTS-2026-08-16.md` reported U-05 (RBAC-denied resource
type) as **NOT RUN** for this reason:

> Requires reinstalling the operator with reduced RBAC. The running operator holds the L-10k state
> that the LT-S14 gate was just closed against; reinstalling would destroy the evidence.

This directory is that evidence, captured live off the cluster **before** any U-05 work begins, so
the gate closure stays verifiable independently of what happens to the `kollect-op1` install.

**Capture is read-only.** Nothing here mutated the cluster. The only non-read call was
`kubectl create token --duration=10m` to scrape the authorized metrics endpoint.

## Substrate at capture time

| Fact | Value |
| --- | --- |
| Cluster | kumulus lab — 3 × Talos v1.9.5, k8s v1.32.0 (`controlplane`, `worker-1`, `worker-2`) |
| kubectl context | `kumulus-lab` (cluster `kumulus`) |
| Operator install | `kollect-op1` ns, `kollect-op1-controller-manager`, age 17h |
| Sink | `shared-postgres-0` in `kollect-loadtest` ns |

## The gate numbers, re-measured at capture

| Field | 2026-08-15 | Gate closure (08-16 earlier) | **This capture** |
| --- | --- | --- | --- |
| `inventory_rows` | 9992 / 10000 | 10000 / 10000 | **10000 / 10000** |
| `export_p99` | `unmeasured` | 0.977679 s | **0.793881 s** |
| `valid` | `false` | `true` | **`true`** |

`export_p99` is computed by linear interpolation within the containing bucket — the same method
`soak-export.sh` uses for etcd fsync, and the same method the original gate closure used.

```
observations = 9428          p99 rank = 9333.72
containing bucket = (0.5, 1.0]   counts 9205 -> 9424
export_p99 = 0.793881 s      mean = 0.134804 s
```

**Why this p99 differs from the gate's 0.977679 s.** Both are cumulative-since-last-restart, and
the controller has restarted since (see below), so this is a *different, later window* over a
quieter period — not a re-measurement of the same interval. It is lower because the apply burst is
over. Neither number supersedes the other; the gate closed on the earlier one and this one
confirms the system is still healthy at 10k. Stated rather than buried.

## Standing evidence for the open findings

- **F-02 (leader-election)** — `restart-evidence.txt` and `controller-describe.txt` preserve the
  **18 restarts** on `kollect-op1-controller-manager-6467f5584f-7sm98`. This is the raw evidence
  behind PERF-FIX-02. The fix is on `main` but this pod predates it, which is exactly why the U-02
  re-test needs a rebuilt image.
- **F-05 (stale collected count)** — `kollecttargets.yaml` preserves the `KollectTarget` statuses
  whose `collecting N resource(s)` message does not track the live matched set. This is the input
  evidence for PERF-FIX-05.
- **U-03 (metrics RBAC)** — the metrics scrape in this capture returned **HTTP 200** using a
  ServiceAccount token. That proves the reader role currently on the cluster works. It does **not**
  prove the shipped chart delivers it — see the Helm section below, which found the role was applied
  out-of-band rather than installed. Filed as PERF-FIX-09.

## Contents

| File | What it is |
| --- | --- |
| `live/nodes.txt` | `kubectl get nodes -o wide` |
| `live/pods-all.txt` | all pods, all namespaces, wide |
| `live/kollect-op1-workloads.yaml` | full deploy/rs/pod/svc YAML for the operator |
| `live/controller-describe.txt` | `describe pod` — restart history, last terminated state |
| `live/restart-evidence.txt` | restart counts + `lastState` extracted per container |
| `live/kollecttargets.yaml` | every `KollectTarget` with status, all namespaces |
| `live/postgres-rowcount.txt` | `\d inventory_items` schema + `count(*)` = 10000 |
| `live/inventory-row-count.txt` | the bare number, for scripted comparison |
| `live/metrics-raw.txt` | full authorized `/metrics` scrape (HTTP 200) |

## Caveat on durability

`agent-context/` is gitignored repo-wide (`.gitignore:2`, zero tracked files under it), matching
the convention that lab protocols are local-only. **This archive therefore lives only on this
workstation** — it is not pushed anywhere. If the evidence needs to survive machine loss, it has to
be copied somewhere tracked or external deliberately. Flagged rather than assumed.

## Helm release state (captured before any upgrade)

Captured because a `helm upgrade --set image.tag=v0.18.0` without the original values would destroy
the scope that produced the 10k inventory. The rows survive in Postgres; the configuration that
generated them would not.

| File | What it is |
| --- | --- |
| `live/helm-list.txt` | `helm list -n kollect-op1` — chart `kollect-0.17.0`, **REVISION 1**, installed 2026-08-15 14:12 |
| `live/helm-values.yaml` | user-supplied values only — the ones that must be preserved on upgrade |
| `live/helm-values-all.yaml` | fully-resolved values including chart defaults |
| `live/helm-manifest.yaml` | every object Helm believes it owns (274 lines) |

Values that must survive any redeploy: `watchNamespaces` (the ten `kol-op1-ns-0NN`),
`allowPrivateSinks: true`, `resourcesProfile: large`, `replicaCount: 1`, the control-plane
anti-affinity, `metrics.enabled: true` / `serviceMonitor: false`, `webhooks.enabled: false`.

### Finding surfaced by this capture — U-03's evidence is narrower than it reads

`kollect-metrics-reader` (ClusterRole **and** ClusterRoleBinding) exist on the cluster with
`helm.sh/chart: kollect-0.17.0` labels — but they are **absent from `helm get manifest`**, and were
created `2026-08-16T00:29:50Z`, roughly ten hours *after* the release install. Combined with
**REVISION 1** (no upgrade ever ran), this means they were rendered from `main` and applied
out-of-band, not installed by the chart.

The Helm labels come from the template's own label block, so they do not prove Helm installed the
object — and because `Chart.yaml` on `main` is still `0.17.0` until release time, the chart-version
label could not distinguish "the released 0.17.0 chart" from "main-rendered". So U-03 proved the
*template renders a working role*, not that a plain `helm install` delivers one — which is what
PERF-FIX-01's acceptance actually asks. Filed as **PERF-FIX-09**; U-03 is re-verified against the
v0.18.0 release rather than inherited as a pass.

---

## POSTSCRIPT — the substrate was destroyed hours after this capture

This archive stopped being a precaution and became the **only surviving copy** on 2026-08-16, a few
hours after it was taken.

What happened, from the kum1 kernel log:

| Boot | SATA state | Result |
| --- | --- | --- |
| Aug 15 16:25 (cluster healthy) | `ata1: SATA link up 6.0 Gbps` + `sd 0:0:0:0: [sda] 1953525168 512-byte logical blocks: (1.00 TB/932 GiB)` | `kumdata` VG present, Talos controlplane running |
| Aug 16 00:21 (current) | `ata1: SATA link down` — **no `sda` at all** | `kumdata` VG gone |

kum1's 1 TB SATA disk stopped being detected between those two boots. `incusd` has since logged
`Failed mounting storage pool ... err="Volume group kumdata not found" pool=data` every 60 s. The
Talos **controlplane** instance and all its volumes are gone from Incus. On kum2 the `kumdata` VG
still exists but holds **0 logical volumes** — the **worker-1** guest and its volumes are likewise
gone. Only **worker-2** on kum4 survived.

With no controlplane, the cluster is dead, and `shared-postgres-0` — which held the 10,000
`inventory_items` rows this gate closed against — died with it.

**Nothing here can be re-measured.** The numbers in this directory are the record. That is why this
copy was moved out of the gitignored `agent-context/` and committed, at the operator's direction,
against the usual local-only convention.

The cause is physical (a SATA cable, a failed drive, or a BIOS setting reset by the recent firmware
flash), not a software defect in kollect, and is tracked on the kumulus side.

## Redaction applied on commit

Real LAN addresses were rewritten to **RFC 5737 TEST-NET-1** (`192.168.178.x` → `192.0.2.x`),
preserving the final octet so node identity is still readable: `.85` controlplane, `.86` worker-1,
`.87` worker-2. No credentials, tokens, JWTs or keys are present — verified by grep and
`hack/scrub.sh` before commit.
