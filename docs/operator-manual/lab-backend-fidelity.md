# Lab backend and emulator fidelity

Local and lab substitutes prove **protocol wiring** (export shape, auth path, ClusterIP reachability).
They are **not** managed-cloud IAM, quota, billing, or control-plane proof. Emulator or in-cluster
lab success must be recorded as `PASS_WITH_LIMITATION` with an explicit fidelity statement — never
summarized as green SaaS parity.

This page is the LAB-DOC-04 matrix for every type registered in `internal/sink/registry.go`
(`NewRegistry`). General backend-profile automation (LAB-H07) is **deferred**
([ADR-0707](../adr/0707-lab-harness.md)); until then operators follow the commands and caveats
below. Publishable evidence shape: [lab evidence bundle](lab-evidence-bundle.md). Runner walkthrough:
[local lab runbook](local-lab-runbook.md).

## How to read the matrix

| Column | Meaning |
| --- | --- |
| Type | `spec.type` / registry key |
| Local / lab substitute | What CI or `quick+sinks` actually exercises |
| Pin / image guidance | Prefer pinned images in Taskfile / testcontainers / lab manifests |
| Proven locally | Protocol / behaviour covered offline or on a named lab schedule |
| Excluded (not proven) | Managed-service behaviour you must **not** claim from the substitute |
| Command / entry | Exact offline or schedule reference |

**Edge rules (binding):**

- **MinIO / S3-compatible paths are not Google IAM/API proof** — including GCS exercised via the
  S3-compatible XML path.
- **Redpanda is not managed Kafka control-plane / SaaS proof.**
- **Forgejo is not GitHub or GitLab SaaS proof.**
- **BigQuery emulator is not GCP IAM, quota, or billing proof.**

Secret and endpoint examples on this page are **lab-local**. TLS/auth assumptions are called out
per row. No real production credential or public SaaS endpoint is required for the offline /
ClusterIP paths.

## Primary fidelity matrix

| Type | Local / lab substitute | Pin / image guidance | Proven locally | Excluded (not proven) | Command / entry |
| --- | --- | --- | --- | --- | --- |
| `local` | Filesystem export under a lab path | N/A (host/pod FS) | Path write + resume-safe overwrite | Remote object-store IAM, multi-AZ durability | `task test-integration` (local backend); lab `DR-2b.1` deferred from `quick+sinks` |
| `git` | Bare git / in-cluster Forgejo / temp GitHub remote | Pin Forgejo image in `hack/demo/hero/manifests/forgejo.yaml` when using the hero path | Push/commit export; ConnectionTest probe | GitHub Enterprise SSO, org webhooks, SaaS rate-limit classes | `task demo-up` / Forgejo bootstrap; lab `DR-2b.2` (bare-git, deferred), `DR-2b.11` (github, in `quick+sinks`) |
| `gitlab` | GitLab-compatible API against temp remote or self-hosted | Pin lab GitLab/Forgejo-compatible endpoint in protocol notes | Project file export via GitLab API | GitLab.com SaaS IAM, group inheritance, managed runners | Lab `DR-2b.12` (in `quick+sinks`); `task test-integration` GitLab cases |
| `s3` | MinIO (S3 API) ClusterIP or testcontainers | Pin MinIO image used by kind/lab protocol | PutObject-style export; path layout | AWS IAM roles, KMS, bucket policies, CloudFront | Lab `DR-2b.4` / serial token `minio` (in `quick+sinks`); `task test-integration` |
| `gcs` | MinIO via S3-compatible XML API (`internal/sink/gcs` integration uses `minio/minio`) / BQ+GCS lab pair | Pin `minio/minio` major used by `-tags=integration` (same stand-in as S3 lab) | Object write through the GCS client's S3-compatible path | **Google IAM, ADC in GCP, bucket IAM conditions, Soft Delete** — MinIO/S3-compatible success is never GCS IAM/API proof | Lab `DR-2b.10` (`bq-gcs`) deferred from `quick+sinks`; `task test-integration` |
| `postgres` | Official Postgres testcontainers / ClusterIP Postgres | Pin Postgres major used in integration Taskfile | Upsert + delete-reconcile; ConnectionTest | Cloud SQL IAM DB auth, proxy, regional HA failover semantics | Lab `DR-2b.3` (in `quick+sinks`); `task test-integration` |
| `mongodb` | MongoDB testcontainers / ClusterIP | Pin Mongo image from integration suite | Document upsert export | Atlas IAM, VPC peering, managed backup restore | Lab `DR-2b.7` deferred from `quick+sinks`; `task test-integration` |
| `kafka` | Redpanda (Kafka API) testcontainers / ClusterIP | Pin Redpanda image from ADR-0402 / integration Task | Produce inventory events | Managed Kafka ACLs, schema registry SaaS, multi-AZ rebalance SLOs — Redpanda ≠ managed Kafka control plane | Lab `DR-2b.8` deferred from `quick+sinks`; `task test-integration` |
| `nats` | NATS JetStream testcontainers / ClusterIP | Pin NATS image from integration suite | JetStream publish path | Synadia Cloud accounts, leafnode SaaS, managed auth | Lab `DR-2b.5` (in `quick+sinks`); `task test-integration` |
| `bigquery` | [goccy/bigquery-emulator](https://github.com/goccy/bigquery-emulator) via testcontainers | Pin emulator image used by `-tags=integration` | `MERGE`/export SQL contract ([ADR-0420](../adr/0420-bigquery-database-sink.md)) | GCP IAM, WIF, quotas, job billing, partition pruning — emulator ≠ GCP | Lab `DR-2b.10` deferred; `task test-integration`; live GCP e2e is maintainer-only |

## Lab serial backends vs registry types

Wave-2b serial tokens (`hack/lab/lib/serial-backend.sh`) map onto registry types. Membership below
is from `hack/lab/schedules/quick+sinks.json` (implemented scenarios vs schedule exclusions).

| Serial token | Scenario | Registry type(s) | In `quick+sinks` today? | Fidelity verdict when green |
| --- | --- | --- | --- | --- |
| `postgres` | `DR-2b.3` | `postgres` | Yes | `PASS` or `PASS_WITH_LIMITATION` if ClusterIP-only caveats apply |
| `minio` | `DR-2b.4` | `s3` (S3 API stand-in) | Yes | **`PASS_WITH_LIMITATION`** — not AWS IAM |
| `nats` | `DR-2b.5` | `nats` | Yes | `PASS` / limitation if JetStream-only features used |
| `github` | `DR-2b.11` | `git` | Yes | `PASS_WITH_LIMITATION` if temp remote ≠ org SaaS policy proof |
| `gitlab` | `DR-2b.12` | `gitlab` | Yes | Same SaaS caveat as matrix row |
| `local-fs` | `DR-2b.1` | `local` | Deferred (exclusion) | Gap until scheduled |
| `bare-git` | `DR-2b.2` | `git` | Deferred | Gap until scheduled |
| `forgejo` | `DR-2b.6` | `git` (GitLab-compatible hero also uses Forgejo) | Deferred | **`PASS_WITH_LIMITATION`** — Forgejo ≠ GitHub/GitLab SaaS |
| `mongodb` | `DR-2b.7` | `mongodb` | Deferred | Gap until scheduled |
| `redpanda` | `DR-2b.8` | `kafka` | Deferred | **`PASS_WITH_LIMITATION`** — Redpanda ≠ managed Kafka |
| `fan-out` | `DR-2b.9` | multi-sink | Deferred | Gap until scheduled |
| `bq-gcs` | `DR-2b.10` | `bigquery` + `gcs` | Deferred | **`PASS_WITH_LIMITATION`** — BQ emulator / MinIO-as-GCS ≠ GCP IAM/billing |

## Recording emulator results

When a report row depends on MinIO (including as the GCS S3-compatible stand-in), Forgejo,
Redpanda, or the BigQuery emulator:

1. Verdict: **`PASS_WITH_LIMITATION`** (not bare `PASS`).
2. Notes: name the substitute and paste the matching **Excluded** cell from the matrix.
3. Limitations section of the evidence bundle must repeat that managed-cloud IAM/quota/control-plane
   behaviour was **not** claimed.

## Lab-local Secret / endpoint examples

Use disposable ClusterIP Services and lab Secrets. Examples assume in-cluster DNS and TLS off or
lab-only self-signed — call that out in the protocol. Do **not** paste production PATs, cloud keys,
or public SaaS hostnames into committed docs or evidence.

```yaml
# Illustrative only — lab-local ClusterIP MinIO; no real credential required to copy the shape.
apiVersion: v1
kind: Secret
metadata:
  name: lab-minio
  namespace: kollect-lab-example
type: Opaque
stringData:
  accessKey: labminio
  secretKey: labminio-secret
```

## Related

- [Local lab runbook](local-lab-runbook.md) — schedules, isolation, harness flags
- [Lab evidence bundle](lab-evidence-bundle.md) — publishable verdicts and redaction
- [ADR-0707: Lab harness](../adr/0707-lab-harness.md) — H07 profiles deferred
- [ADR-0406: Sink registry](../adr/0406-sink-registry.md) — registration contract
- [ADR-0420: BigQuery sink](../adr/0420-bigquery-database-sink.md) — emulator fidelity limits
