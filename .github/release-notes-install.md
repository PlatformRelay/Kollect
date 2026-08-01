## Container image (operator)

```
${IMAGE_REPO}:${VERSION}
```

Multi-arch (`linux/amd64`, `linux/arm64`), Debian bookworm-slim nonroot base (includes `git` and `openssh-client` for `spec.git.engine: cli`).

OCI attestations (SBOM + SLSA provenance) are attached in GHCR and on the repository
[Attestations](https://github.com/${GITHUB_REPOSITORY}/attestations) page. Verify the signature:

```sh
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/${GITHUB_REPOSITORY}/.+' \
  ${IMAGE_REPO}@${IMAGE_DIGEST}
```

## Container image (kollect-pipeline)

One-shot CI/CD collection CLI (ADR-0801) — collect inventory from a kubeconfig without installing
the operator. See the [pipeline CLI guide](https://github.com/${GITHUB_REPOSITORY}/blob/main/docs/guides/pipeline-cli.md).

```
${PIPELINE_IMAGE_REPO}:${VERSION}
```

Multi-arch (`linux/amd64`, `linux/arm64`), distroless static nonroot base. The git snapshot sink
uses the pure-Go go-git engine over HTTPS; `git.engine: cli` and `file://` remotes are not supported
in this minimal image.

```sh
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/${GITHUB_REPOSITORY}/.+' \
  ${PIPELINE_IMAGE_REPO}@${PIPELINE_IMAGE_DIGEST}
```

## Install (Kustomize)

```sh
kubectl apply -f install-crds.yaml
kubectl apply -f install.yaml
```

## Install (Helm — OCI)

```sh
helm upgrade --install kollect ${CHART_OCI}/kollect \
  --version ${VERSION} \
  --namespace kollect-system \
  --create-namespace \
  --set image.repository=${IMAGE_REPO} \
  --set image.tag=${VERSION}
```

## Install (Helm — GitHub Release tarball)

```sh
helm upgrade --install kollect kollect-${VERSION}.tgz \
  --namespace kollect-system \
  --create-namespace \
  --set image.repository=${IMAGE_REPO} \
  --set image.tag=${VERSION}
```

Verify checksums with `sha256sum -c checksums.txt`. Each release asset includes a
`<file>.sigstore.json` Sigstore bundle; `release-provenance.intoto.jsonl` attests all assets.
See [docs/RELEASE.md](https://github.com/${GITHUB_REPOSITORY}/blob/main/docs/RELEASE.md#verify-after-release).
