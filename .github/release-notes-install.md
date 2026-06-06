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

## Container image (kollect-ui)

Optional read-only UI SPA — enable with Helm `ui.enabled=true`.

```
${UI_IMAGE_REPO}:${VERSION}
```

Multi-arch (`linux/amd64`, `linux/arm64`), nginx alpine static server.

```sh
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/${GITHUB_REPOSITORY}/.+' \
  ${UI_IMAGE_REPO}@${UI_IMAGE_DIGEST}
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
  --set image.tag=${VERSION} \
  --set ui.image.tag=${VERSION}
```

## Install (Helm — GitHub Release tarball)

```sh
helm upgrade --install kollect kollect-${VERSION}.tgz \
  --namespace kollect-system \
  --create-namespace \
  --set image.repository=${IMAGE_REPO} \
  --set image.tag=${VERSION} \
  --set ui.image.tag=${VERSION}
```

Verify checksums with `sha256sum -c checksums.txt`. Each release asset includes a
`<file>.sigstore.json` Sigstore bundle; `release-provenance.intoto.jsonl` attests all assets.
See [docs/RELEASE.md](https://github.com/${GITHUB_REPOSITORY}/blob/main/docs/RELEASE.md#verify-after-release).
