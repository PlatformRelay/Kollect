# Lab evidence bundle and redaction

Maintainer **multi-node lab** runs (Talos driving-range or equivalent) produce **bounded**
evidence for a **named product pin**. This page defines the **publishable** evidence contract:
manifest schema, scenario result rows, required limitations, and redaction rules.

Raw lab protocols and artifact trees stay **local-only** (gitignored). Public docs may only
carry redacted summaries that satisfy this contract. Kind CI and the
[load test runbook](load-test-runbook.md) remain separate validation paths.

## Evidence bundle schema

A complete run records a **manifest**, a **scenario matrix**, a **limitations** section, and
(optionally) footprint / pprof indexes. Automating that layout is future harness work
(LAB-H); until then, human-authored protocols follow the same fields.

### Manifest fields

| Field | Required | Notes |
| --- | --- | --- |
| `RUN_ID` | yes | Stable id (for example `dr-YYYYMMDD-<hex>`) used as the lab label value |
| Schedule | yes | Which scenario set ran (for example `quick+sinks`) |
| Started / finished (UTC) | yes | Wall-clock bounds |
| Product pin | yes | Release tag and/or image digest; chart version when Helm-installed |
| Cluster topology | yes | Control-plane / worker counts; K8s (and CNI/OS) versions when known |
| Lab label | yes | For example `kollect.dev/lab-run=<RUN_ID>` |
| Helm release / namespace | yes | Install identity for the run window |
| Access path | redacted | Describe *how* (for example “Tailscale + kubeconfig”) — never paste kubeconfig contents |
| Notable values | yes | Replica count, `allowPrivateSinks`, pprof on/off, resource profile |

Optional but recommended: node allocatable memory/CPU, temp remote URLs **without** credentials,
cleanup confirmation checklist.

### Scenario result rows

Each scenario id (for example `DR-2.4`) appears as one row:

| Column | Meaning |
| --- | --- |
| ID | Stable scenario identifier |
| Verdict | One of the allowed values below |
| Evidence / notes | Short, redacted proof (log snippet summary, counts, durations) |

**Allowed verdicts** (none of the non-pass values may be summarized as pass):

| Verdict | Counts as pass? |
| --- | --- |
| `PASS` | yes |
| `PASS_WITH_LIMITATION` | yes, with an explicit limitation |
| `FAIL` | no |
| `SKIPPED` | no — reason required |
| `LIMIT_REACHED` | no — capacity/time bound |
| `not triggered` | no — precondition absent |

Failed, skipped, limit-reached, and not-triggered rows must state **why** and what evidence is
missing. Do not collapse them into a green program verdict.

## Limitations section

Every publishable summary **must** include an explicit **limitations** (or “not claimed”) list.
Typical items:

- Scenarios `SKIPPED` / `LIMIT_REACHED` / `not triggered`
- Wave-4 / Tier-S synthetic load not run
- `kubectl top` / metrics-server unavailable
- Partial scrape counts versus live cluster objects
- Temp remotes only partially cleaned up

A program-level verdict such as **READY WITH CONDITIONS** is allowed only when limitations are
listed beside what **was** proven for that pin.

## Redaction rules

Before any fragment leaves the maintainer machine (docs PR, issue, chat paste):

| Never publish | Why |
| --- | --- |
| Kubeconfig files or pasted `clusters:` / `users:` blocks | Cluster credentials |
| Kubernetes **Secret** values, tokens, PATs, SSH keys, `.env` | Credential leak |
| Raw sink credentials / connection strings with passwords | Credential leak |
| Internal RFC1918 lab IPs when the audience is public docs | Unnecessary topology leak — prefer “CP + 2 workers” wording |
| Home paths, usernames tied to private hosts | PII / fingerprinting |
| Company-private hostnames or forbidden scrub strings | See `task scrub` / gitleaks |

| May publish | Notes |
| --- | --- |
| Release tag, chart version, image **digest** prefixes | Provenance |
| Scenario ids + verdicts + redacted notes | Matrix |
| Row counts, failover duration, restart counts | Observability |
| **pprof tops** (function names + byte/count summaries) | Heap/goroutine tops are OK; omit host paths inside profiles if present |
| Public git remote **URLs** without embedded credentials | After redacting tokens from clone URLs |
| Explicit “not claimed” / limitations list | Honesty gate |

When in doubt, omit. Prefer counts and digests over payloads.

## What may be published

| Artifact | Location | Public? |
| --- | --- | --- |
| Full protocol markdown | Maintainer `lab-protocols/<RUN_ID>.md` | **No** — local-only / not committed |
| Raw evidence tree | `artifacts/lab/<RUN_ID>/` | **No** — gitignored |
| Redacted summary table | This docs site (or a short ROADMAP/testing note) | **Yes** — after redaction |
| Kind CI / nightly results | GitHub Actions | **Yes** — separate from lab |
| 100k cloud design proof | [Load test runbook](load-test-runbook.md) | Only when that gate has been executed |

`task perf-report` snapshots are also local-only; they are not a substitute for this contract.

## Example (redacted) — `dr-20260805-cd33ee` on v0.16.0

Bounded **quick+sinks** schedule on a Talos lab (**1 control plane + 2 workers**). Wave-4 Tier-S
load was **SKIPPED** (RAM/time). Program verdict: **READY WITH CONDITIONS**.

**Proven (selected)**

| ID | Verdict | Notes (redacted) |
| --- | --- | --- |
| DR-1.1 | PASS | Two manager replicas on distinct workers; single lease holder |
| DR-1.2 | PASS | Leader failover ~17s; restartCount **0** |
| DR-2.4 | PASS | Inventory export to Postgres (`count=3`) |
| DR-2b.3–2b.5 | PASS | ClusterIP Postgres / MinIO / NATS with `allowPrivateSinks: true` |
| DR-2b.11–2b.12 | PASS | GitHub and GitLab ConnectionOK **and** snapshot export |
| DR-3.1 | PASS_WITH_LIMITATION | Certificate scrape collecting **1** (partial vs live certs) |
| DR-4.3 | PASS_WITH_LIMITATION | Idle heap/goroutine pprof; ~20 MiB in-use top; no churn baseline |

**Limitations / not claimed**

- DR-1.3–1.5, DR-2.3 / 2.6–2.7, DR-2b.1–2 / 6–10, DR-3.2–3.3, DR-4.1–4.2 skipped (time/headroom)
- Wave-4 synthetic load and churn pprof not captured
- Metrics-server absent (`kubectl top` N/A)
- Full Ubuntu D-suite / every sink / NetPol deny-path / 100k cloud gate not in scope

Treat this as **bounded evidence for pin v0.16.0**, not a substitute for Kind CI or the 100k
design proof.

## Related

- [Testing strategy — multi-node lab evidence](../development/testing.md#multi-node-lab-evidence)
- [Performance and scalability](performance.md)
- [Load test runbook](load-test-runbook.md)
- [Resolved-address / private-sink policy](../security/resolved-address-policy.md)
