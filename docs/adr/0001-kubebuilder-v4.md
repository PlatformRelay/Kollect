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

## Resolved questions (2026-06-05)

- **Helm chart ships day 1** — `charts/kollect/`, CI `helm template` / unittest path ([REQUIREMENTS.md](../REQUIREMENTS.md)).
- **Validating webhooks early** — Profile CEL/JSONPath and Sink type enum at admission ([ADR-0004](0004-crd-model.md), [ADR-0015](0015-static-vs-reconciled.md)); not deferred to reconcile-time workarounds.
