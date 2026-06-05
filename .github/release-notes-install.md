## Container image

```
${IMAGE_REPO}:${VERSION}
```

Multi-arch (`linux/amd64`, `linux/arm64`), distroless nonroot base.

OCI attestations (SBOM + SLSA provenance) are attached in GHCR. Verify the signature:

```sh
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/${GITHUB_REPOSITORY}/.+' \
  ${IMAGE_REPO}@${IMAGE_DIGEST}
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

Verify checksums with `sha256sum -c checksums.txt`.
