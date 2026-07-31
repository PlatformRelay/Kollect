# Local and test container images

Production releases publish semver tags (`v0.3.0`, `0.3.0`, …) to
[`ghcr.io/platformrelay/kollect`](https://github.com/platformrelay/kollect/pkgs/container/kollect) via
[release workflow](https://github.com/platformrelay/kollect/blob/main/.github/workflows/release.yaml). For manual maintainer testing against a
real registry or a remote cluster, use **maintainer-only** tags that never collide with release
semver.

!!! warning "Not for production"
    Tags such as `local`, `test-*`, and `dev-*` are **maintainer-only**. Do not reference them in
    Helm values, GitOps repos, or customer-facing docs.

## Build for kind / minikube (load locally)

Builds a single-platform image into the local daemon (default `linux/amd64`):

```sh
task docker:build:local
# → ghcr.io/platformrelay/kollect:local

kind load docker-image ghcr.io/platformrelay/kollect:local --name kollect-dev
```

Override repository, tag, platform, or container tool:

```sh
IMAGE_REPO=ghcr.io/platformrelay/kollect IMAGE_TAG=dev-me PLATFORMS=linux/arm64 task docker:build:local
CONTAINER_TOOL=podman task docker:build:local
```

The legacy kind workflow (`task docker:build` → `kollect-controller-manager:dev`) remains
unchanged; see [development/setup.md](../development/setup.md#manual-kustomize-deploy-legacy).

## Build and push to GHCR

Push requires a one-time registry login (do not commit tokens):

```sh
docker login ghcr.io
# GitHub → Settings → Developer settings → Personal access tokens (read:packages, write:packages)
```

Default tag is the current commit short SHA (`test-<short-sha>`):

```sh
task docker:push:local
# → ghcr.io/platformrelay/kollect:test-a1b2c3d

IMAGE_TAG=test-$(git rev-parse --short HEAD) task docker:push:local
IMAGE_TAG=local-dev IMAGE_REPO=ghcr.io/platformrelay/kollect task docker:push:local
```

Multi-arch push (optional):

```sh
PLATFORMS=linux/amd64,linux/arm64 task docker:push:local
```

Semver-like tags (`v0.3.0`, `0.3.0-rc.1`) are **rejected** by the script so local tasks cannot
accidentally overwrite release tags.

## Environment reference

| Variable | Default | Purpose |
| --- | --- | --- |
| `IMAGE_REPO` | `ghcr.io/platformrelay/kollect` | Registry repository |
| `IMAGE_TAG` | `local` (build) / `test-<short-sha>` (push) | Maintainer-only tag |
| `PLATFORMS` | `linux/amd64` | Single platform; comma-separated for multi-arch push |
| `CONTAINER_TOOL` | `docker` | `docker` or `podman` |
