# ADR-0705: Release engineering and supply chain

> How Kollect builds, signs, and ships releases: multi-arch images, cosign signing, SBOM + provenance,
> vulnerability gating, OCI chart publishing, and git-cliff versioning.

**Theme:** 07 · Project & meta · **Status:** Current

## Context

As OSS infrastructure that runs with broad cluster read access, Kollect's **supply-chain trust** is a
product feature: adopters need to verify what they run. The release pipeline (`.github/workflows/release.yaml`,
`hack/release-assets.sh`) implements this, but the decisions weren't recorded. `docs/RELEASE.md` is the
operator how-to; this ADR is the rationale.

## Decision

### Trigger and versioning

- Releases fire on **`vX.Y.Z` tags** (semver, validated in-workflow); `workflow_dispatch` can rebuild a
  tag's assets for testing.
- **Conventional Commits + gitmoji** drive versioning and changelog via **git-cliff** (`cliff.toml`);
  release notes are generated, not hand-written (per `AGENTS.md`).

### Image build

- **Multi-arch** `linux/amd64,linux/arm64` via buildx; pushed to **GHCR**:
  - Operator: `ghcr.io/<owner>/kollect`
  - UI SPA: `ghcr.io/<owner>/kollect-ui` ([ADR-0409](0409-kollect-ui-deployment.md))
- **Distroless, non-root** runtime base for the operator ([ADR-0101](0101-kubebuilder-v4.md)); UI uses nginx alpine static server.

### Supply-chain attestations (binding)

1. **SBOM** — buildx `sbom: true` plus an SPDX-JSON SBOM (`anchore/sbom-action`) published as a release
   asset (`dist/sbom.spdx.json`).
2. **Provenance** — buildx `provenance: mode=max` (SLSA-style).
3. **Signing** — **cosign keyless** (Sigstore OIDC, `id-token: write`) signs the image **by digest**.
4. **Vulnerability gate** — **Trivy** scans both release images and **fails the release** on fixable
   `CRITICAL`/`HIGH` (`ignore-unfixed: true`).

### Action hardening

- All GitHub Actions are **pinned by commit SHA**; workflow permissions are least-privilege
  (`contents: read` default, elevated per-job only where needed) ([ADR-0104](0104-security-model.md)).

### Release artifacts

Each GitHub Release publishes: `install-crds.yaml` and `install.yaml` (kubectl install paths —
[ADR-0704](0704-helm-chart-crd-lifecycle.md)), the Helm chart `.tgz` **also pushed as OCI** to GHCR,
`sbom.spdx.json`, `sbom-ui.spdx.json`, and `checksums.txt`.

## Consequences

- Adopters can `cosign verify` by digest and inspect the SBOM before deploying — trust is verifiable.
- A new fixable CRITICAL/HIGH CVE blocks the release until the base/deps are bumped (intentional friction).
- SHA-pinned actions mean Dependabot/maintenance keeps the pipeline current; stale pins are a known cost.
- Tag-driven releases keep `main` always-releasable ([ADR-0703](0703-platform-architecture-pivot.md)).

## OpenSSF Scorecard follow-ups

The project publishes an [OpenSSF Scorecard badge](https://securityscorecards.dev/viewer/?uri=github.com/konih/kollect)
(see `README.md`). A scheduled workflow (`.github/workflows/scorecard.yaml`) runs on every `main` push and
weekly; SARIF results are uploaded to GitHub Code Scanning.

**Solo-maintainer policy:** checks that require multi-person review gates or block direct pushes to `main`
are **documented and deferred** — not enabled — so one maintainer can ship without self-approval friction.

| Check | Score (snapshot) | Status | Rationale |
| --- | ---: | --- | --- |
| Dangerous-Workflow | 0 critical | **Done** | No `pull_request_target`; workflow inputs passed via env vars; actions SHA-pinned |
| Token-Permissions | 0 high | **Done** | Default `contents: read`; `security-events: write` scoped to CodeQL/Scorecard analyze jobs only; release job documents why `contents: write` is required |
| Pinned-Dependencies | 0 medium | **Done** | Actions pinned to commit SHA; distroless base image by digest; Helm tarball SHA256-verified; pip docs deps hash-locked (`--require-hashes`) |
| SAST | 0 medium | **Done** | `golangci-lint` + `govulncheck` in CI; **CodeQL** for Go (`.github/workflows/codeql.yaml`) |
| Vulnerabilities | 0 | **Done** | `govulncheck` on every PR; grpc ≥1.79.3 and otel/sdk ≥1.43.0; Trivy gates release images; Dependabot alerts enabled |
| Security-Policy | 10 | **Done** | `SECURITY.md` |
| Dependency-Update-Tool | 10 | **Done** | Dependabot |
| Binary-Artifacts | 10 | **Done** | No committed binaries |
| License | 10 | **Done** | MIT |
| Code-Review | 0 high | **Deferred** | Branch protection + required reviewers blocks solo push-to-main workflow |
| Branch-Protection | 0 high | **Deferred** | Optional for single maintainer; CI merge gates substitute for GitHub branch rules |
| Maintained | 0 high | **Deferred** | Activity-based; improves with regular releases and issue triage post-`v0.1.0` |
| Fuzzing | 0 medium | **Partial** | Native Go fuzz (`FuzzContentHash`, `internal/aggregate`) in CI (`fuzz` job, 30s); OSS-Fuzz deferred |
| CII-Best-Practices | 0 low | **Deferred** | Core Infrastructure Initiative badge application not pursued pre-GA |
| Contributors | 0 low | **N/A** | Solo OSS; diversity metric not applicable |

**CodeQL (SAST):** GitHub CodeQL for Go runs on every push/PR to `main`, weekly on Mondays, and uploads
results to GitHub Code Scanning (`.github/workflows/codeql.yaml`). Complements `golangci-lint` security
linters and `govulncheck` — not a replacement.

**Native Go fuzz:** CI job **fuzz** runs `go test -fuzz=FuzzContentHash -fuzztime=30s` on
`internal/aggregate` (export coalesce checksum path). Full OSS-Fuzz integration remains deferred.

## Decided follow-ups (2026-06-05, planned post-`v0.1.0-rc`)

- **YES:** Publish signed **provenance + SBOM attestations** (`cosign attest`) attached to the image,
  in addition to the release-asset SBOM.
- **DONE:** OpenSSF **Scorecard** workflow + badge (`.github/workflows/scorecard.yaml`).
- **DONE:** **Sign the Helm chart** (`cosign sign` the OCI chart artifact) — see release workflow.
- **TODO:** Add **`slsa-verifier`** check in release CI for provenance verification.
