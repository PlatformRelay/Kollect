# Quick start

Get kollect running on a local **kind** cluster and apply the sample custom resources. This path
is optimized for evaluation and feedback — not production deployment.

!!! tip "Assumptions"
    This guide assumes Docker, [kind](https://kind.sigs.k8s.io/), and kubectl are installed. For
    build-from-source steps you also need Go and [Task](https://taskfile.dev/) — see
    [DEVELOPMENT.md](DEVELOPMENT.md).

## What kollect does

Platform teams need **stakeholder-visible inventory**: what workloads run in which namespaces, which
images and labels are in use, and how that state changes over time — without bespoke collectors
for every CRD or hand-maintained spreadsheets.

**kollect** is a Kubernetes operator that watches arbitrary resource types (by GVK), extracts
attributes with JSONPath or CEL, aggregates results, and exports snapshots to pluggable sinks (Git,
GitLab, S3, GCS, Postgres, Kafka). Exporting to Git gives
auditable, diffable history that developer portals and compliance workflows can consume alongside
live API access.

## Prerequisites

- Docker, [kind](https://kind.sigs.k8s.io/), kubectl
- Go and [Task](https://taskfile.dev/) (to build from source) — see [DEVELOPMENT.md](DEVELOPMENT.md)

## Install on kind (copy-paste)

From the repo root, **`task kind-dev-up`** creates the `kollect-dev` cluster, loads the operator image,
and deploys Helm (ingress/Grafana addons unless `KOLLECT_DEV_MINIMAL=1`). See [DEVELOPMENT.md](DEVELOPMENT.md).

```sh
# 1. Cluster
kind create cluster --name kollect-dev

# 2. Clone and build (skip if already in repo root)
git clone https://github.com/konih/kollect.git && cd kollect
task build

# 3. CRDs + operator
task install:crds
task docker:build
kind load docker-image kollect-controller-manager:dev --name kollect-dev
task deploy:operator

# 4. Wait for manager pod
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
credentials and point `spec.secretRef` at it (implementation-dependent). Until Phase 1 sink code
lands, credentials are not required for validating CR acceptance.

## Verify

### Manager logs

```sh
kubectl -n kollect-system logs deployment/kollect-controller-manager -f
```

**Phase 0 (current bootstrap):** expect the manager to start, register controllers, and reconcile
without panics. Reconcilers may log minimal activity until collection logic is implemented.

**Phase 1 (target):** logs should show informer registration for the profile GVK, reconcile
loops on `KollectTarget`, and extraction errors surfaced as conditions.

**Phase 1 (inventory + Git sink):** expect export attempts, `status.itemCount`, `status.lastExportTime`,
and Git commit SHAs in status (not full payloads — see [ADR-0103](adr/0103-etcd-limit.md)).

### CR status

```sh
kubectl get kollectprofiles,kollectsinks,kollectinventories
kubectl get kollecttargets -A
kubectl describe kollectinventory -n default team-inventory
```

When export works, check your Git sink repository for committed inventory JSON/YAML.

## Current maturity

!!! warning "Pre-beta API"
    kollect is **`v1alpha1` pre-beta**. CRD fields, controller behavior, and sample YAML may change
    without notice. Applying samples validates schema and wiring; full end-to-end export depends on
    the phase items in [ROADMAP.md](ROADMAP.md).

Be honest about where the project stands:

| Phase | Status | What works today |
| --- | --- | --- |
| **0** | In progress | CRDs, RBAC, manager on kind, CI, samples, docs |
| **1** | Planned | Dynamic informers, attribute extraction, Git/GitLab export |

Controllers may still contain scaffold `TODO(user)` reconcile loops. Applying samples **validates
API schema and wiring**; end-to-end export requires Phase 1 implementation. Track progress in
the repo and [ARCHITECTURE.md](ARCHITECTURE.md).

## Next steps

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
