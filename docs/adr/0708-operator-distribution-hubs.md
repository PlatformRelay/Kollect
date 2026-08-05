# ADR-0708: Operator distribution via Artifact Hub and OperatorHub

> How Kollect becomes discoverable on Artifact Hub and OperatorHub without replacing Helm OCI
> as the primary install path, and without in-repo FBC/`opm` catalog machinery.

**Theme:** 07 · Project & meta · **Status:** Exploring (Proposed — maintainer LGTM required)

<!-- AgDR: architect role · 2026-08-06 · trigger: hub distribution parity plan (Attune pattern) -->

## Context

Kollect already publishes signed multi-arch images and a signed Helm OCI chart to GHCR
([ADR-0705](0705-release-supply-chain.md)), with CRD lifecycle rules in
[ADR-0704](0704-helm-chart-crd-lifecycle.md). Chart metadata already carries
`artifacthub.io/license: MIT` but lacks operator/CRD/capability annotations and Verified
Publisher OCI metadata. There is no OLM bundle or OperatorHub submission path.

Attune (reference) demonstrates lean hub parity: Chart.yaml annotations, root
`artifacthub-repo.yml` pushed via `oras` as `:artifacthub.io`, hand-templated registry+v1
bundles, and dual community-operators PRs from release — soft-fail so core publish stays green.

**Note on GHCR layout (DR-FIND-07):** the Helm chart occupies
`ghcr.io/<owner>/kollect:<version>` (bare tag); the controller image uses the **v-prefixed**
tag only. Artifact Hub Verified Publisher metadata must target the **chart** OCI repository
(`…/kollect:artifacthub.io`), never clobber image tags.

Package identity: **`kollect`**, channel **`stable`**.
`kollect-versions` / `kollect-render` are out of scope.

## Options considered

| Option | Pros | Cons |
| --- | --- | --- |
| **A. Full Attune parity** (AH + OLM hand bundle + dual PRs) | Discoverability on both hubs; reuses release pipeline | Hand CSV drift risk; registration prerequisites |
| **B. Artifact Hub only** | Smaller; Helm already primary | No OpenShift / OperatorHub.io path |
| **C. operator-sdk generate + in-repo FBC** | Toolchain-native OLM | Heavy vs lean release engineering; FBC ops |
| **D. Do nothing** | Zero cost | Hubs stay dark after launch docs investment |

### Weighted trade-off (subjective scores 1–5)

| Criterion (weight) | A | B | C | D |
| --- | ---: | ---: | ---: | ---: |
| Adopter discoverability (3) | 5 | 3 | 5 | 1 |
| Operability / lean tooling (3) | 4 | 5 | 2 | 5 |
| Continuity with ADR-0704/0705 (2) | 5 | 5 | 3 | 3 |
| Release blast radius (soft-fail) (2) | 4 | 5 | 3 | 5 |
| Pattern familiarity (Attune) (1) | 5 | 3 | 2 | 1 |
| **Weighted total** | **55** | **46** | **35** | **35** |

## Decision

We chose **option A — full Attune-style Artifact Hub + OperatorHub distribution** because it
extends ADR-0705 without replacing Helm OCI, accepting hand-templated OLM bundles and
operator-owned registration, over B (incomplete), C (tooling weight), or D (no discoverability).

Contract:

1. **Helm OCI remains primary** ([ADR-0704](0704-helm-chart-crd-lifecycle.md)).
2. **Artifact Hub:** enrich `charts/kollect/Chart.yaml` annotations (`operator`, `category`,
   `operatorCapabilities`, `crds`, `links`, `images`, keep `license`); root
   `artifacthub-repo.yml` with placeholder `repositoryID` until registration; release
   `oras push ghcr.io/<owner>/kollect:artifacthub.io` after helm push + cosign.
3. **OperatorHub:** `config/olm/` templates + `make generate-olm-bundle` copying CRDs from
   `config/crd/bases`; package **`kollect`**, channel **`stable`**; digest-pin CSV
   deployment/`relatedImages` to `ghcr.io/<owner>/kollect@sha256:…` (image digest from release,
   not chart digest).
4. **RBAC in CSV** from real `config/rbac/role.yaml` (+ leader-election as needed).
5. **`hack/operatorhub-pr.sh`** + release job gated on `secrets.OPERATORHUB_PAT`, soft-fail
   (`continue-on-error`); OpenShift annotation default **`v4.19`**; fork owner default
   **`platformrelay`**.
6. **Skipped:** Krew, Docker Hub chart mirror, in-repo FBC/`opm`, CLOMonitor.

## Consequences

- Hub wiring can land before registration using placeholder repositoryIDs; live Verified
  Publisher and upstream PRs wait on operator registration.
- CSV must list owned CRDs Kollect actually ships (Profile, Target, Inventory, sink families,
  scopes, connection tests, cluster variants) — large owned list; keep generate step mechanical.
- Docs must not ship live Artifact Hub / OperatorHub badge URLs that 404 before listing.
- Soft-fail hub jobs preserve tag-release success when PAT/forks are absent.

## Alternatives considered

See table. FBC rejected as disproportionate for solo maintenance. Artifact-Hub-only rejected
because OpenShift catalog adopters are in scope for this track.
