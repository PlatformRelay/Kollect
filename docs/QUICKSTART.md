# Quick start

Get Kollect running on a local **kind** cluster and apply the sample custom resources. This path
is optimized for evaluation and feedback — not production deployment.

!!! tip "Assumptions"
    This guide assumes Docker, [kind](https://kind.sigs.k8s.io/), and kubectl are installed. For
    build-from-source steps you also need Go and [Task](https://taskfile.dev/) — see
    [DEVELOPMENT.md](DEVELOPMENT.md). New to CRDs, CEL extraction, or sink roles? Start with
    [Understand the basics](UNDERSTAND-THE-BASICS.md).

## What Kollect does

Platform teams need **stakeholder-visible inventory**: what workloads run in which namespaces, which
images and labels are in use, and how that state changes over time — without bespoke collectors
for every CRD or hand-maintained spreadsheets.

**Kollect** is a Kubernetes operator that watches arbitrary resource types (by GVK), extracts
attributes with JSONPath or CEL, aggregates results, and exports snapshots to pluggable sinks (Git,
GitLab, S3, GCS, Postgres, Kafka, NATS). Exporting to Git gives
auditable, diffable history that developer portals and compliance workflows can consume alongside
live API access.

## Prerequisites

- Docker, [kind](https://kind.sigs.k8s.io/), kubectl
- Go and [Task](https://taskfile.dev/) (to build from source) — see [DEVELOPMENT.md](DEVELOPMENT.md)

## Install (copy-paste)

<!-- markdownlint-disable MD046 -->

=== "kind (local dev)"

    From the repo root, **one command** builds the manager, creates the `kollect-dev` cluster, loads the
    image, and installs the operator (ingress/Grafana addons unless `KOLLECT_DEV_MINIMAL=1`):

    ```sh
    git clone https://github.com/konih/kollect.git && cd kollect
    task dev-up                       # build + kind cluster + operator + sample CRs
    ```

    `task dev-up` already applies `config/samples/`. To wait for the manager and re-apply samples
    yourself:

    ```sh
    kubectl -n kollect-system rollout status deployment/kollect-controller-manager --timeout=120s
    kubectl apply -k config/samples/   # profile → sink → target → inventory
    ```

    ??? note "Prefer explicit steps? (build and deploy by hand)"

        ```sh
        # 1. Cluster
        kind create cluster --name kollect-dev

        # 2. Build
        task build

        # 3. CRDs + operator image
        task install:crds
        task docker:build
        kind load docker-image kollect-controller-manager:dev --name kollect-dev
        task deploy:operator

        # 4. Wait for the manager pod
        kubectl -n kollect-system rollout status deployment/kollect-controller-manager --timeout=120s

        # 5. Sample inventory pipeline (profile → sink → target → inventory)
        kubectl apply -k config/samples/
        ```

        Consolidated install manifest (alternative):

        ```sh
        make build-installer IMG=kollect-controller-manager:dev
        kubectl apply -f dist/install.yaml
        kubectl apply -k config/samples/
        ```

=== "Helm"

    For a cluster you already have (including kind after `kind create cluster`), install the chart from
    the repo. Production-oriented values and upgrade steps: [Operator manual](OPERATOR-MANUAL.md).

    ```sh
    helm install kollect ./charts/kollect -n kollect-system --create-namespace
    kubectl -n kollect-system rollout status deployment/kollect-controller-manager --timeout=120s
    kubectl apply -k config/samples/
    ```

    From a release (pin chart and image version):

    ```sh
    helm install kollect oci://ghcr.io/konih/kollect --version 0.1.0 -n kollect-system --create-namespace
    kubectl apply -k config/samples/
    ```

<!-- markdownlint-disable MD046 -->

## Sample CRs

| File | Kind | Role |
| --- | --- | --- |
| `kollect_v1alpha1_kollectprofile.yaml` | `KollectProfile` | Extract Deployment image + labels |
| `kollect_v1alpha1_kollectsink.yaml` | `KollectSink` | Git sink (example repo URL) |
| `kollect_v1alpha1_kollecttarget.yaml` | `KollectTarget` | Collect Deployments using the profile |
| `kollect_v1alpha1_kollectinventory.yaml` | `KollectInventory` | Aggregate and export to the sink |

Narrative walkthrough with expected behavior notes:
[examples/deployment-inventory.md](examples/deployment-inventory.md).

### Optional: workload to collect

Deploy a sample Deployment so a target has something to watch:

```sh
kubectl create deployment nginx --image=nginx:1.27 --replicas=1
kubectl label deployment nginx app.kubernetes.io/name=nginx
```

The sample `KollectTarget` selects Deployments labeled `app.kubernetes.io/name=nginx`.

### Watch opt-in / opt-out (optional)

Control collection with labels and annotations ([ADR-0205](adr/0205-watch-labels.md)):

| Key | Values | Effect |
| --- | --- | --- |
| `kollect.dev/watch` (label on namespace or resource) | `enabled` / `disabled` | Opt in or out a namespace or single resource |
| `kollect.dev/namespace-watch` (annotation on namespace) | `enabled` / `disabled` | Opt in or out all resources in the namespace |

`KollectTarget.spec.watchMode` defaults to `All` (collect matching selectors except `disabled`).
Set `watchMode: OptIn` to collect only explicitly `enabled` namespaces/resources.

Sample opt-in target: `config/samples/kollect_v1alpha1_kollecttarget_opt-in.yaml`.

### Connection test (sink)

Samples set `spec.connectionTest: true` on `KollectSink`. The operator probes Git/Postgres/Kafka
and sets **`ConnectionVerified`** ([ADR-0403](adr/0403-connection-test.md)).

```sh
kubectl wait --for=condition=ConnectionVerified kollectsink/git-inventory \
  -n default --timeout=60s
kubectl describe kollectsink git-inventory -n default
```

Re-test without editing spec:

```sh
kubectl annotate kollectsink git-inventory -n default kollect.dev/test-connection=true --overwrite
```

### Optional: Git credentials

The sample Git sink references a placeholder repository. For real exports, create a Secret with
credentials and point `spec.secretRef` at it. Connection tests can pass without a writable remote;
export requires valid credentials and endpoint reachability.

## Verify

### Manager logs

```sh
kubectl -n kollect-system logs deployment/kollect-controller-manager -f
```

Expect the manager to start, register controllers, and reconcile without panics. On a live cluster
with sample CRs applied you should see informer registration for the profile GVK, reconcile loops
on `KollectTarget` and `KollectInventory`, extraction errors surfaced as conditions, and export
attempts with `status.itemCount`, `status.lastExportTime`, and sink-specific status (Git commit SHAs,
Postgres row counts, etc. — not full payloads; see [ADR-0103](adr/0103-etcd-limit.md)).

### CR status

```sh
kubectl get kollectprofiles,kollectsinks,kollectinventories
kubectl get kollecttargets -A
kubectl describe kollectinventory -n default team-inventory
```

When export works, check your Git sink repository for committed inventory JSON/YAML.

## Current maturity

!!! warning "Pre-beta API"
    Kollect is **`v1alpha1` pre-beta**. CRD fields, controller behavior, and sample YAML may change
    without notice. See [ROADMAP.md](ROADMAP.md) for item-level status before production use.

Be honest about where the project stands:

| Phase | Status | What works today |
| --- | --- | --- |
| **0** | ✅ Done | CRDs (9 kinds), RBAC, validating webhooks, manager on kind, CI, Helm chart, samples, docs |
| **1** | 🚧 Mostly shipped | Dynamic informers, CEL/JSONPath extraction, `KollectTarget`/`KollectInventory` controllers, seven sink types (Git, GitLab, S3, GCS, Postgres incl. delete recon, Kafka, NATS), connection test, `KollectScope` multitenant gate, inventory HTTP API (partial) |
| **2** | 🚧 In progress | Hub/spoke via Helm `mode`, ingest/merge, pluggable queue transport (in-process, Redis, NATS, Kafka) |
| **3** | 🚧 Partial | `KollectClusterTarget` + `KollectClusterInventory` controllers (rollup + export); `KollectClusterProfile` admission-only |

End-to-end export on kind is green in CI; some items (event-driven export path, S3 Parquet, finalizers)
remain 🚧/⬜ in [ROADMAP.md](ROADMAP.md). Track detail in [ARCHITECTURE.md](ARCHITECTURE.md).

## Next steps

- [UNDERSTAND-THE-BASICS.md](UNDERSTAND-THE-BASICS.md) — prerequisite concepts and curated links
- [CR-REFERENCE.md](CR-REFERENCE.md) — per-kind fields, RBAC, failure modes
- [DATA-FLOWS.md](DATA-FLOWS.md) — export debouncing and collection diagrams
- [PLATFORM-DECISIONS.md](PLATFORM-DECISIONS.md) — locked architecture summary
- [DEVELOPMENT.md](DEVELOPMENT.md) — codegen, tests, lint, pitfalls
- [ARCHITECTURE.md](ARCHITECTURE.md) — CRD model, reconciliation, portal use case
- [ROADMAP.md](ROADMAP.md) — build-order phases
- [examples/deployment-inventory.md](examples/deployment-inventory.md) — annotated YAML walkthrough

## Cleanup

```sh
kubectl delete -k config/samples/ --ignore-not-found
make undeploy ignore-not-found=true
kind delete cluster --name kollect-dev
```
