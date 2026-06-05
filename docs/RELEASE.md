# Releasing a new version

Step-by-step guide for maintainers publishing a **kollect** release.

Related: [CONTRIBUTING.md](../CONTRIBUTING.md) (commits), [DEVELOPMENT.md](DEVELOPMENT.md) (local tasks).

## Overview

Releases are **tag-driven**: push a tag `vX.Y.Z` on `main` and
[`.github/workflows/release.yaml`](../.github/workflows/release.yaml) builds, scans, signs, and
publishes artifacts. Version numbers are **not** bumped by CI — commit `charts/kollect/Chart.yaml`
and `CHANGELOG.md` on `main` first.

While the API is `v1alpha1`, use **minor** (`0.2.0`) for user-visible features or breaking
operator behaviour; **patch** (`0.1.1`) for fixes only. Breaking commits use `!` in the subject
(see [CONTRIBUTING.md](../CONTRIBUTING.md)).

## Retroactive version anchors

History before the first GitHub release is split with lightweight tags (changelog anchors only):

| Tag | Commit | Milestone |
| --- | --- | --- |
| `v0.0.1` | `13546aff` | Kubebuilder scaffold |
| `v0.0.2` | `1e6f6719` | Core `v1alpha1` CRDs |
| `v0.0.3` | `66421337` | Helm chart, extraction, inventory HTTP |
| `v0.0.4` | `4234960b` | ADR-0032 platform pivot MVP |

Commits after `4234960b` (hub/cluster APIs, transport, multi-tenant) appear under **Unreleased**
until you tag **`v0.1.0` at current `main` HEAD** for the first public release.

Push changelog anchor tags once (if not already on the remote):

```sh
git tag v0.0.1 13546aff
git tag v0.0.2 1e6f6719
git tag v0.0.3 66421337
git tag v0.0.4 4234960b
git push origin v0.0.1 v0.0.2 v0.0.3 v0.0.4
```

Do **not** push `v0.1.0` until you intend to publish — that tag triggers the release workflow.

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

Ensure **CI** and **preflight** are green on `${RELEASE_SHA}` on GitHub Actions.

### v0.1.0 prep status (session 13)

| Gate | Status | Notes |
| --- | --- | --- |
| `task changelog:verify` | ✅ after `changelog:write` | Sync before tag |
| `task release-dry-run` | ✅ | Local `dist/` assets valid |
| Coverage floor **45%** | ✅ | `COVERAGE_MIN` in CI + `CONTRIBUTING.md` |
| E2E kind smoke | ✅ | Run `26996964559` @ `42183693` |
| Export asserts (Ready, git SHA, multitenant) | ✅ | `68667ca6` |
| GitLab MR REST client | ✅ | `8247f4e` — feature-branch push deferred |
| Phase 4 metrics stub | ✅ | `kollect_custom_resource_series` per ADR-0033 |

**Remaining before tag:** manual GitHub Actions **Release** `workflow_dispatch` with a test tag
(e.g. `v0.1.0-rc.1`) to verify cosign keyless, SBOM, GHCR push, and chart upload. Do **not** push
`v0.1.0` until that passes and CI is green on the release SHA.

## Version and changelog

### 1. Preview unreleased notes

```sh
task changelog
```

### 2. Choose the version

| Change | Example bump |
| --- | --- |
| Breaking (`feat!`, CRD contract) | `0.1.0` → `0.2.0` |
| New feature, non-breaking | `0.1.0` → `0.1.1` or `0.2.0` |
| Bug fixes only | `0.1.0` → `0.1.1` |

### 3. Bump the Helm chart

Edit [`charts/kollect/Chart.yaml`](../charts/kollect/Chart.yaml):

```yaml
version: 0.1.0
appVersion: "0.1.0"
```

Align `version` and `appVersion` with the git tag (`v0.1.0` → `0.1.0`).

### 4. Regenerate CHANGELOG.md

```sh
task changelog:write
git add charts/kollect/Chart.yaml CHANGELOG.md
git commit -m ":bookmark: chore(release): prepare v0.1.0"
```

## Cut v0.1.0 (first release)

On green `main` at the commit you intend to ship:

```sh
git tag v0.1.0
git push origin main
git push origin v0.1.0   # triggers release workflow — only after CI green on this SHA
```

CI publishes the GitHub Release, GHCR image, OCI Helm chart, and attached assets.

**Dry-run locally** before tagging:

```sh
VERSION=0.1.0 task release-dry-run
ls -la dist/
```

**Rebuild assets** for an existing tag: Actions → **Release** → **Run workflow** → enter `v0.1.0`
(optional `draft` / `prerelease` inputs).

## What CI publishes

| Output | Location |
| --- | --- |
| Container image | `ghcr.io/konih/kollect:0.1.0` (and `:v0.1.0`), `linux/amd64` + `arm64` |
| OCI SBOM + SLSA provenance | GHCR attestations on the image |
| GitHub Release | git-cliff section + install footer; assets below |
| `install-crds.yaml` | CRD bundle |
| `install.yaml` | Full operator install (image pinned to tag) |
| `kollect-0.1.0.tgz` | Helm chart tarball |
| `sbom.spdx.json` | SPDX SBOM (Syft) |
| `checksums.txt` | SHA256 of release files |
| Helm chart (OCI) | `oci://ghcr.io/konih/kollect` |

Release notes are assembled by [`hack/assemble-release-notes.sh`](../hack/assemble-release-notes.sh)
and [`.github/release-notes-install.md`](../.github/release-notes-install.md).

## Verify after release

```sh
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/konih/kollect/.+' \
  ghcr.io/konih/kollect@<digest>
```

Confirm `CHANGELOG.md` on `main` has an empty **Unreleased** section (run `task changelog:write`
after tagging if needed).
