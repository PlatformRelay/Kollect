# Releasing a new version

Step-by-step guide for maintainers publishing a **Kollect** release.

Related: [CONTRIBUTING.md](https://github.com/platformrelay/kollect/blob/main/CONTRIBUTING.md) (commits), [development/setup.md](development/setup.md) (local tasks),
[ROADMAP.md](ROADMAP.md) (feature status).

## Overview

Releases are **tag-driven**: push a tag `vX.Y.Z` on `main` and
[`.github/workflows/release.yaml`](https://github.com/platformrelay/kollect/blob/main/.github/workflows/release.yaml) builds, scans, signs, and
publishes artifacts. Version numbers are **not** bumped by CI — commit `charts/kollect/Chart.yaml`
and `CHANGELOG.md` on `main` first.

While the API is `v1alpha1`, use **minor** (`0.3.0`, `0.4.0`, …) for themed feature tranches or
breaking operator behaviour; **patch** (`0.2.1`) for fixes on the current minor line. Breaking
commits use `!` in the subject (see [CONTRIBUTING.md](https://github.com/platformrelay/kollect/blob/main/CONTRIBUTING.md)).

## Versioning policy

Kollect uses **frequent pre-1.0 minors**. The current release is shown in
[GitHub Releases](https://github.com/platformrelay/kollect/releases); the authoritative history is
the [changelog](https://github.com/platformrelay/kollect/blob/main/CHANGELOG.md).

| Policy | Detail |
| --- | --- |
| **Cadence** | Release when a coherent, validated change set is ready; no fixed calendar |
| **RC tags** | `vX.Y.Z-rc.N` — soak on green `main`; use `workflow_dispatch` with `draft` + `prerelease` |
| **Breaking changes** | `feat!:` / `BREAKING CHANGE:` → **minor** bump pre-v1.0 |
| **Compatibility** | Patch releases stay compatible with their minor line; breaking changes require a minor |

Before 1.0, a minor release may include breaking API or default changes. Mark them with `feat!:` or
`BREAKING CHANGE:`, document migration steps in the changelog, and call them out in release notes.
Patch releases stay compatible with their minor line.

## Retroactive version anchors

History before the first GitHub release is split with lightweight tags (changelog anchors only):

| Tag | Commit | Milestone |
| --- | --- | --- |
| `v0.0.1` | `13546aff` | Kubebuilder scaffold |
| `v0.0.2` | `1e6f6719` | Core `v1alpha1` CRDs |
| `v0.0.3` | `66421337` | Helm chart, extraction, inventory HTTP |
| `v0.0.4` | `4234960b` | ADR-0201 platform pivot MVP |
| `v0.1.0-rc.1` – `rc.3` | 2026-06-05 – 06 | Pre-strategy RCs (finalizers, helm, e2e, release pipeline) |
| **`v0.2.0-rc.1`** | 2026-06-07 | Sink-family tranche |

Push changelog anchor tags once (if not already on the remote):

```sh
git tag v0.0.1 13546aff
git tag v0.0.2 1e6f6719
git tag v0.0.3 66421337
git tag v0.0.4 4234960b
git push origin v0.0.1 v0.0.2 v0.0.3 v0.0.4
```

## Pre-release checklist

```sh
git checkout main && git pull
RELEASE_SHA="$(git rev-parse HEAD)"
echo "Tagging: ${RELEASE_SHA}"
```

```sh
task verify
task lint
task test
task helm-test
task changelog:verify
```

Ensure **CI**, **preflight**, and **`kind-smoke`** (`e2e-smoke.yaml`) are green on `${RELEASE_SHA}`
on GitHub Actions. Docs-only or path-filtered commits that skipped a required job are **not**
eligible — re-dispatch the skipped workflow on that exact SHA.

Then run the read-only **Release gate** against that immutable SHA. It rejects commits not reachable
from protected `main`, missing/cancelled/unsuccessful required checks, and commits that are not the
merge commit of a merged-to-`main` PR. **Non-author APPROVE is not required** (solo-maintainer
policy; Environment `release` + tag ruleset still gate publication):

```sh
gh workflow run release-gate.yaml -f sha="${RELEASE_SHA}"
gh run list --workflow release-gate.yaml --limit 1
```

The publishing workflow independently repeats this eligibility check **before** registry login,
signing, attestations, or release uploads. Gate scripts are always loaded from the default branch
so a candidate tag cannot supply its own verifier. Publication checks out the proven immutable SHA
(not a movable tag ref alone) and refuses to continue if the tag no longer resolves to that SHA.

### Operator protection (required)

Configure these in the GitHub repo settings (not expressible in workflow YAML alone):

1. **Environment `release`** — required reviewers (and optionally a wait timer) on the write-capable
   Release job. Without this, a malicious tagged workflow copy that drops the eligibility job could
   still obtain write/`id-token` permissions.
2. **Tag rules** — restrict `v*.*.*` creation to protected `main` / allowed actors so arbitrary
   commits cannot be tagged into the release path.

### L4 pre-release gate

Before tagging, require **one** of:

1. Green **`e2e-nightly`** workflow run on `${RELEASE_SHA}` (re-run via `workflow_dispatch` if the
   scheduled cron has not yet picked up the commit), or
2. Manual **`test-e2e`** workflow dispatch on that SHA, or
3. Local **`task test:e2e`** on the release commit (document run ID / timestamp in the release notes).

L3 integration (`test-integration` in CI) remains the merge gate for sink backends; nightly L4
no longer duplicates export-integration or object-store jobs.

### Git export test repository (optional)

For full remote git SHA assert in **`e2e-nightly`**, **`e2e-extended`**, and **`test-e2e`**
git-export jobs, set repository variable **`GIT_EXPORT_TEST_REPO`** in GitHub → Settings →
Actions → Variables (clone URL of a dedicated test repo). Workflows pass `${{ vars.GIT_EXPORT_TEST_REPO }}`
with `GITHUB_TOKEN`; this cannot be set from workflow YAML. Without the variable, git-export jobs
verify inventory HTTP hash only (degraded mode).

### RC pre-release on GitHub Actions

The release workflow accepts `draft` and `prerelease` inputs only on **`workflow_dispatch`**.
Pushing a tag matching `v*.*.*` triggers a **non-draft** release automatically — use rc tags with
dispatch inputs when you need draft/prerelease metadata.

**Steps** (maintainer, on green `main`):

```sh
git checkout main && git pull
RELEASE_SHA="$(git rev-parse HEAD)"
git tag v0.3.0-rc.1 "${RELEASE_SHA}"
git push origin v0.3.0-rc.1
```

Then trigger a draft pre-release rebuild if needed:

```sh
gh workflow run release.yaml \
  -f tag=v0.3.0-rc.1 \
  -f draft=true \
  -f prerelease=true
```

Monitor: `gh run list --workflow=release.yaml --limit 3`

**Skip tag push** if you only want local validation — `task release-dry-run` covers assets without
publishing to GHCR or GitHub Releases.

## Version and changelog

### 1. Preview unreleased notes

```sh
task changelog
```

### 2. Choose the version

| Change | Example bump |
| --- | --- |
| Themed feature tranche / breaking operator behaviour | `0.2.0` → `0.3.0` |
| Bug fixes on current minor | `0.2.0` → `0.2.1` |
| Soak before minor GA | Tag `0.3.0-rc.1` first |

### 3. Bump the Helm chart

Edit [`charts/kollect/Chart.yaml`](https://github.com/platformrelay/kollect/blob/main/charts/kollect/Chart.yaml):

```yaml
version: 0.3.0
appVersion: "0.3.0"
```

Align `version` and `appVersion` with the git tag (`v0.3.0` → `0.3.0`).

### 4. Regenerate CHANGELOG.md

```sh
task changelog:write
git add charts/kollect/Chart.yaml CHANGELOG.md
git commit -m ":bookmark: chore(release): prepare v0.3.0"
```

## Cut a release

Land the release prep through a **protected-main PR** (rebase-merge). A second human review is not
required for solo-maintainer releases. Refetch and record the exact resulting `main` SHA:

```sh
git fetch origin main
git switch main
git pull --ff-only origin main
RELEASE_SHA="$(git rev-parse HEAD)"
gh workflow run release-gate.yaml -f sha="${RELEASE_SHA}"
```

Only after that gate succeeds on the same immutable SHA:

```sh
git tag v0.3.0 "${RELEASE_SHA}"
git push origin v0.3.0   # triggers release workflow, which repeats eligibility before login
```

Do **not** `git push origin main` outside branch protection, and do not move an existing release tag.

CI publishes the GitHub Release, GHCR image, OCI Helm chart, and attached assets.

**Dry-run locally** before tagging:

```sh
VERSION=0.3.0 task release-dry-run
ls -la dist/
```

**Rebuild assets** for an existing tag: Actions → **Release** → **Run workflow** → enter the tag
(optional `draft` / `prerelease` inputs). Rebuilds still require eligibility on the tag's commit SHA
and the protected `release` environment.

## What CI publishes

| Output | Location |
| --- | --- |
| Container image (operator) | `ghcr.io/platformrelay/kollect:<version>` (and `:v<version>`), `linux/amd64` + `arm64` |
| Container image (kollect-ui) | `ghcr.io/platformrelay/kollect-ui:<version>` (and `:v<version>`), `linux/amd64` + `arm64` |
| Container image (pipeline CLI) | `ghcr.io/platformrelay/kollect-pipeline:<version>` (and `:v<version>`), `linux/amd64` + `arm64` |
| OCI SBOM + SLSA provenance | GHCR attestations on all three images |
| GitHub Release | git-cliff section + install footer; assets below |
| `install-crds.yaml` | CRD bundle |
| `install.yaml` | Full operator install (image pinned to tag) |
| `kollect-<version>.tgz` | Helm chart tarball |
| `sbom.spdx.json` | SPDX SBOM for operator image (Syft) |
| `sbom-ui.spdx.json` | SPDX SBOM for kollect-ui image (Syft) |
| `sbom-pipeline.spdx.json` | SPDX SBOM for kollect-pipeline image (Syft) |
| `checksums.txt` | SHA256 of release files |
| `<asset>.sigstore.json` | Sigstore bundle for each release asset (cosign keyless) |
| `release-provenance.intoto.jsonl` | Combined SLSA provenance attestation for release assets |
| Helm chart (OCI) | `oci://ghcr.io/platformrelay/kollect` |

Release notes are assembled by [`hack/assemble-release-notes.sh`](https://github.com/platformrelay/kollect/blob/main/hack/assemble-release-notes.sh)
and [`.github/release-notes-install.md`](https://github.com/platformrelay/kollect/blob/main/.github/release-notes-install.md).

## Verify after release

### Container images (GHCR)

```sh
TAG=v0.2.0-rc.1   # or your release tag
REPO=platformrelay/kollect

OP_DIGEST="$(crane digest ghcr.io/${REPO}/kollect:${TAG#v})"
UI_DIGEST="$(crane digest ghcr.io/${REPO}/kollect-ui:${TAG#v})"
PIPELINE_DIGEST="$(crane digest ghcr.io/${REPO}/kollect-pipeline:${TAG#v})"

cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/platformrelay/kollect/.+' \
  "ghcr.io/${REPO}/kollect@${OP_DIGEST}"

cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/platformrelay/kollect/.+' \
  "ghcr.io/${REPO}/kollect-ui@${UI_DIGEST}"

cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/platformrelay/kollect/.+' \
  "ghcr.io/${REPO}/kollect-pipeline@${PIPELINE_DIGEST}"
```

SLSA provenance and SPDX SBOM attestations are published to GHCR (via `actions/attest`) and the
repository [Attestations](https://github.com/platformrelay/kollect/attestations) page:

```sh
gh attestation verify "ghcr.io/${REPO}/kollect@${OP_DIGEST}" \
  --owner platformrelay --repo kollect
gh attestation verify "ghcr.io/${REPO}/kollect-ui@${UI_DIGEST}" \
  --owner platformrelay --repo kollect
gh attestation verify "ghcr.io/${REPO}/kollect-pipeline@${PIPELINE_DIGEST}" \
  --owner platformrelay --repo kollect
```

### GitHub Release assets (OpenSSF Scorecard Signed-Releases)

Each release asset ships with a Sigstore bundle (`<file>.sigstore.json`) and a combined SLSA
provenance bundle (`release-provenance.intoto.jsonl`). Verify a downloaded artifact:

```sh
TAG=v0.2.0-rc.1
VERSION="${TAG#v}"
gh release download "${TAG}" --pattern 'kollect-*.tgz' --dir /tmp/kollect-verify
cd /tmp/kollect-verify

cosign verify-blob \
  --bundle "kollect-${VERSION}.tgz.sigstore.json" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/platformrelay/kollect/.+' \
  "kollect-${VERSION}.tgz"
```

Checksums: `sha256sum -c checksums.txt` after downloading all unsigned assets.

### Rebuild an existing tag with signing

```sh
gh workflow run release.yaml \
  -f tag=v0.2.0-rc.1 \
  -f draft=false \
  -f prerelease=true
```

Confirm `CHANGELOG.md` on `main` has an empty **Unreleased** section (run `task changelog:write`
after tagging if needed).
