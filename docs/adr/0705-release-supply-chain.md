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

- **Multi-arch** `linux/amd64,linux/arm64` via buildx; pushed to **GHCR** (`ghcr.io/<owner>/kollect`).
- **Distroless, non-root** runtime base ([ADR-0101](0101-kubebuilder-v4.md)).

### Supply-chain attestations (binding)

1. **SBOM** — buildx `sbom: true` plus an SPDX-JSON SBOM (`anchore/sbom-action`) published as a release
   asset (`dist/sbom.spdx.json`).
2. **Provenance** — buildx `provenance: mode=max` (SLSA-style).
3. **Signing** — **cosign keyless** (Sigstore OIDC, `id-token: write`) signs the image **by digest**.
4. **Vulnerability gate** — **Trivy** scans the built image and **fails the release** on fixable
   `CRITICAL`/`HIGH` (`ignore-unfixed: true`).

### Action hardening

- All GitHub Actions are **pinned by commit SHA**; workflow permissions are least-privilege
  (`contents: read` default, elevated per-job only where needed) ([ADR-0104](0104-security-model.md)).

### Release artifacts

Each GitHub Release publishes: `install-crds.yaml` and `install.yaml` (kubectl install paths —
[ADR-0704](0704-helm-chart-crd-lifecycle.md)), the Helm chart `.tgz` **also pushed as OCI** to GHCR,
`sbom.spdx.json`, and `checksums.txt`.

## Consequences

- Adopters can `cosign verify` by digest and inspect the SBOM before deploying — trust is verifiable.
- A new fixable CRITICAL/HIGH CVE blocks the release until the base/deps are bumped (intentional friction).
- SHA-pinned actions mean Dependabot/maintenance keeps the pipeline current; stale pins are a known cost.
- Tag-driven releases keep `main` always-releasable ([ADR-0703](0703-platform-architecture-pivot.md)).

## Decided follow-ups (2026-06-05, planned post-`v0.1.0-rc`)

- **YES:** Publish signed **provenance + SBOM attestations** (`cosign attest`) attached to the image,
  in addition to the release-asset SBOM.
- **YES:** Add **`scorecard`/`slsa-verifier`** checks and an OpenSSF badge.
- **YES:** **Sign the Helm chart** (`cosign sign` the OCI chart artifact) and document chart verification.
