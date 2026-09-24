# Implementation Plan: B2 — Admission secretRef guard + auth-mode enum

**Branch**: `fm/kollect-b2-secretref` (target `fix/secretref-scope-and-authmode`) | **Date**: 2026-09-24 | **Spec**: `specs/001-secretref-authmode/spec.md`

**Input**: Feature specification from `/specs/001-secretref-authmode/spec.md`

## Summary

Close two admission/startup defects with the smallest coherent change:

1. **K-04.** Add a process-wide allowlist of Secret namespaces (mirroring
   `validation.allowPrivateSinks`) and a validation helper that walks every
   `SecretReference` reachable from a family-sink spec, rejecting any non-empty
   namespace that is neither the sink's own namespace nor allowlisted. Wire the
   helper into the three family-sink validating webhooks. Expose the allowlist as
   `--allow-secret-ref-namespaces` and Helm `allowSecretRefNamespaces`.
2. **K-12.** Validate `--inventory-auth-mode` at startup against the closed set
   `{kubernetes, disabled}`, fail fast otherwise, and make the non-disabled mode
   require authorization.

No reconciler or API-type change; the reference field is not removed (C-1
preserves the fan-in pattern behind a flag).

## Technical Context

**Language/Version**: Go (module `github.com/platformrelay/kollect`; see `go.mod`).

**Primary Dependencies**: controller-runtime (`sigs.k8s.io/controller-runtime`),
`k8s.io/apimachinery/pkg/util/validation/field`; envtest + Ginkgo/Gomega for the
admission suite.

**Storage**: N/A.

**Testing**: `go test` with envtest (`KUBEBUILDER_ASSETS`), Ginkgo specs under
`internal/webhook/v1alpha1`, table tests in `cmd`. Repo gates: `make test`,
`make lint`, `make vet`, `make fmt`, `task arch-lint`.

**Target Platform**: Kubernetes operator (Linux).

**Project Type**: Single Go project (operator + CLI).

**Constraints**: No new dependency; no CRD schema change; existing same-namespace
and empty-namespace references keep working.

**Scale/Scope**: ~2 production files for K-12, ~3 for K-04 plus Helm and tests.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

The repo's own governance (`CONTRIBUTING.md`, `AGENTS.md` when present) is the
operative contract; the spec-kit constitution is an unfilled template and is not
used as a gate. The change must satisfy:

- **Behavioural test lock ships with the fix** (webhook envtest cross-ns
  reject/allow; startup enum validation) — satisfies the batch contract.
- **Deny by default**: empty allowlist rejects all cross-namespace refs.
- **No tenant-controllable widening**: the allowlist is a process flag only.
- **Architecture lint**: `internal/validation` may not import `internal/webhook`
  or `cmd`; the helper stays in `validation` and the webhook imports it, matching
  the existing `endpoint_guard` / `SetAllowPrivateSinks` shape.
- **No new dependency.**

## Project Structure

### Documentation (this feature)

```text
specs/001-secretref-authmode/
├── spec.md              # Feature spec
├── plan.md              # This file
└── tasks.md             # Task breakdown
```

### Source Code (repository root)

```text
internal/validation/
├── secret_ref_namespace.go        # NEW: allowlist + SecretReference walk (K-04)
├── secret_ref_namespace_test.go   # NEW: unit coverage for the walk
├── family_sink.go                 # (unchanged; spec-shape validation stays pure)
└── endpoint_guard.go              # (pattern reference: allowPrivateSinks)

internal/webhook/v1alpha1/
├── family_sink_webhook.go         # call the guard in each validator (K-04)
└── secret_ref_namespace_envtest_test.go  # NEW: reject/allow envtest (test lock)

cmd/
├── startup_flags.go               # NEW flag + validateInventoryAuthMode (K-12)
├── startup_flags_test.go          # enum-validation test (test lock)
└── main.go                        # wire allowlist; strict auth mode; authz required

internal/inventory/
└── auth.go                        # AuthDisabled(): drop the "none" alias (K-12)

charts/kollect/
├── values.yaml                    # allowSecretRefNamespaces: []
├── templates/deployment.yaml      # render --allow-secret-ref-namespaces
└── tests/deployment_secret_ref_namespaces_test.yaml  # NEW helm-unittest

docs/
├── crds/kollectsnapshotsink.md    # note cross-namespace rejection (K-04)
└── security/security-architecture.md  # note the allowlist (K-04)
```

**Structure Decision**: Single project. The guard lives in `internal/validation`
because that is where admission-only policy helpers already live
(`endpoint_guard.go`); the webhook package calls it exactly as it already calls
`ValidateSnapshotSinkSpec`.

## Complexity Tracking

No constitution violations. The allowlist is a flat list of target namespaces;
per-source scoping was rejected as over-engineering (recorded as residual risk in
the batch report).
