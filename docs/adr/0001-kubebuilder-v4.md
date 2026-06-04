# ADR-0001: Kubebuilder v4 + controller-runtime

## Status

Accepted

## Context

kollect is a Kubernetes operator that watches arbitrary GVKs, extracts attributes, aggregates
results, and exports to pluggable sinks and doc backends. We need a mature scaffolding stack
with CRD generation, envtest, RBAC markers, and a large ecosystem.

Prior personal-operator scaffolding validated Kubebuilder v4 layout: `api/`, `internal/controller`,
`config/` kustomize, Helm chart, Taskfile CI parity, and golden OpenAPI contract tests.

OSS references (external-secrets, Flux controllers, Argo CD) all build on controller-runtime
patterns — thin reconcilers, workqueues, conditions, and leader election.

## Decision

- **Framework:** Kubebuilder **v4.14** with **controller-runtime v0.24.x**.
- **Language:** Go **1.26**.
- **Layout:** standard Kubebuilder v4 project structure (`api/v1alpha1`, `internal/{controller,collect,sink}`,
  `cmd/main.go`, `config/`, `charts/kollect/`).
- **API version:** `kollect.dev/v1alpha1` (alpha until export/doc flows stabilize).
- **Image:** distroless nonroot; evaluate `ko` for reproducible builds (see PLAN Phase 0 tooling).

## Consequences

### Positive

- Codegen (deepcopy, CRD manifests, RBAC) is automated and gateable via `hack/verify.sh`.
- envtest + Ginkgo integration matches industry practice and the project's proven test harness.
- controller-runtime gives informer cache, workqueue, conditions, and metrics for free.

### Negative

- Kubebuilder scaffold defaults (placeholder README, basic reconcilers) must be replaced with
  kollect-specific logic — intentional, not a reason to avoid the framework.
- v1alpha1 API may need breaking changes before beta; document migration in release notes.

## Open questions

- **OPEN:** Ship Helm chart from day one (external-secrets model) or defer until Phase 1 sink works?
- **OPEN:** Enable webhooks in Phase 0 scaffold or add validating webhook only when CEL on CRDs is insufficient?
