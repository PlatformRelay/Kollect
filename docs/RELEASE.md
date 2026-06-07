# Releasing a new version

Step-by-step guide for maintainers publishing a **Kollect** release.

Related: [CONTRIBUTING.md](../CONTRIBUTING.md) (commits), [DEVELOPMENT.md](DEVELOPMENT.md) (local tasks),
[ROADMAP.md](ROADMAP.md) (feature status).

## Overview

Releases are **tag-driven**: push a tag `vX.Y.Z` on `main` and
[`.github/workflows/release.yaml`](../.github/workflows/release.yaml) builds, scans, signs, and
publishes artifacts. Version numbers are **not** bumped by CI — commit `charts/kollect/Chart.yaml`
and `CHANGELOG.md` on `main` first.

While the API is `v1alpha1`, use **minor** (`0.3.0`, `0.4.0`, …) for themed feature tranches or
breaking operator behaviour; **patch** (`0.2.1`) for fixes on the current minor line. Breaking
commits use `!` in the subject (see [CONTRIBUTING.md](../CONTRIBUTING.md)).

## Versioning policy

Kollect uses **frequent pre-1.0 minors** with a **presentation gate at ~v0.10.0** — not a single
early GA at v0.1.0.

| Policy | Detail |
| --- | --- |
| **Cadence** | Target one minor (or rc → minor) every **1–3 weeks** while building toward v0.10 |
| **RC tags** | `vX.Y.Z-rc.N` — soak on green `main`; use `workflow_dispatch` with `draft` + `prerelease` |
| **Breaking changes** | `feat!:` / `BREAKING CHANGE:` → **minor** bump pre-v1.0 |
| **Phases vs semver** | [ROADMAP.md](ROADMAP.md) phases 0–4 = build order; semver bands = release tranches |

### What `v0.2.0-rc.1` shipped (2026-06-07)

The first post-strategy tranche was **platform / sinks**, not UI:

- Sink family CRDs ([ADR-0414](adr/0414-sink-family-crds.md)) and removal of monolithic `KollectSink`
- Family sink reconcilers, validating webhooks, e2e bootstrap for family CRDs
- Git transport retry, SSH host keys, Forgejo/Gitea MR auth fixes

Read API + UI milestones moved to the **v0.5–v0.10** band ([ADR-0408](adr/0408-read-api-ui-architecture.md)).

### Version ladder (summary)

| Band | Theme |
| --- | --- |
| **0.2.x** | Platform / sink families — **rc.1 shipped** |
| **0.3.x** | Quality, perf, coverage ratchet (next rc target) |
| **0.4.x** | Parquet sink, supply-chain attestations |
| **0.5.x – 0.6.x** | Read API freeze + UI foundation |
| **0.7.x** | Read-only UI MVP |
| **0.8.x – 0.9.x** | Portal backend + drift UI |
| **0.10.0** | **Presentation gate** — demo-ready release |

Full ladder: [ROADMAP.md § Read API + UI](ROADMAP.md#read-api-ui-console-planned-adr-0408).

## Retroactive version anchors

History before the first GitHub release is split with lightweight tags (changelog anchors only):

| Tag | Commit | Milestone |
| --- | --- | --- |
| `v0.0.1` | `13546aff` | Kubebuilder scaffold |
| `v0.0.2` | `1e6f6719` | Core `v1alpha1` CRDs |
| `v0.0.3` | `66421337` | Helm chart, extraction, inventory HTTP |
| `v0.0.4` | `4234960b` | ADR-0703 platform pivot MVP |
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
on GitHub Actions.

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

### Current release status

| Item | Status |
| --- | --- |
| `v0.2.0-rc.1` | ✅ Shipped 2026-06-07 — sink families + git hardening |
| Next target | **`v0.3.0-rc.1`** — perf (PERF-01/03), 70% coverage ratchet |
| Presentation gate | **`v0.10.0`** — UI MVP + demo + docs |

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

Edit [`charts/kollect/Chart.yaml`](../charts/kollect/Chart.yaml):

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

On green `main` at the commit you intend to ship:

```sh
git tag v0.3.0
git push origin main
git push origin v0.3.0   # triggers release workflow — only after CI green on this SHA
```

CI publishes the GitHub Release, GHCR image, OCI Helm chart, and attached assets.

**Dry-run locally** before tagging:

```sh
VERSION=0.3.0 task release-dry-run
ls -la dist/
```

**Rebuild assets** for an existing tag: Actions → **Release** → **Run workflow** → enter the tag
(optional `draft` / `prerelease` inputs).

## What CI publishes

| Output | Location |
| --- | --- |
| Container image (operator) | `ghcr.io/konih/kollect:<version>` (and `:v<version>`), `linux/amd64` + `arm64` |
| Container image (kollect-ui) | `ghcr.io/konih/kollect-ui:<version>` (and `:v<version>`), `linux/amd64` + `arm64` |
| OCI SBOM + SLSA provenance | GHCR attestations on both images |
| GitHub Release | git-cliff section + install footer; assets below |
| `install-crds.yaml` | CRD bundle |
| `install.yaml` | Full operator install (image pinned to tag) |
| `kollect-<version>.tgz` | Helm chart tarball |
| `sbom.spdx.json` | SPDX SBOM for operator image (Syft) |
| `sbom-ui.spdx.json` | SPDX SBOM for kollect-ui image (Syft) |
| `checksums.txt` | SHA256 of release files |
| `<asset>.sigstore.json` | Sigstore bundle for each release asset (cosign keyless) |
| `release-provenance.intoto.jsonl` | Combined SLSA provenance attestation for release assets |
| Helm chart (OCI) | `oci://ghcr.io/konih/kollect` |

Release notes are assembled by [`hack/assemble-release-notes.sh`](../hack/assemble-release-notes.sh)
and [`.github/release-notes-install.md`](../.github/release-notes-install.md).

## Verify after release

### Container images (GHCR)

```sh
TAG=v0.2.0-rc.1   # or your release tag
REPO=konih/kollect

OP_DIGEST="$(crane digest ghcr.io/${REPO}/kollect:${TAG#v})"
UI_DIGEST="$(crane digest ghcr.io/${REPO}/kollect-ui:${TAG#v})"

cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/konih/kollect/.+' \
  "ghcr.io/${REPO}/kollect@${OP_DIGEST}"

cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/konih/kollect/.+' \
  "ghcr.io/${REPO}/kollect-ui@${UI_DIGEST}"
```

SLSA provenance and SPDX SBOM attestations are published to GHCR (via `actions/attest`) and the
repository [Attestations](https://github.com/konih/kollect/attestations) page:

```sh
gh attestation verify "ghcr.io/${REPO}/kollect@${OP_DIGEST}" \
  --owner konih --repo kollect
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
  --certificate-identity-regexp '^https://github.com/konih/kollect/.+' \
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
