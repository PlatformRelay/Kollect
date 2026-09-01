# Install Kollect

Install the operator on a local kind cluster, then continue to a Git-backed inventory you can
inspect and diff.

!!! note "Before you start"
    You need Docker, [kind](https://kind.sigs.k8s.io/), `kubectl`, Git, Go, and
    [Task](https://taskfile.dev/). The API is `v1alpha1`; pin a release for shared environments.

## Local evaluation

From a fresh checkout:

```sh
git clone https://github.com/platformrelay/kollect.git
cd kollect
task build
task kind-dev-up
kubectl -n kollect-system rollout status \
  deployment/kollect-controller-manager --timeout=120s
```

`task kind-dev-up` creates the `kollect-dev` cluster and installs only the operator and its CRDs.
It deliberately does not apply `config/samples/`, so the next guide starts with an empty Kollect
API and a collision-free Git-only pipeline.

Verify that the operator is ready:

```sh
kubectl get pods -n kollect-system
kubectl api-resources --api-group=kollect.dev
```

## Helm on an existing cluster

The default chart enables validating webhooks and creates cert-manager `Issuer` and `Certificate`
resources. Install [cert-manager](https://cert-manager.io/docs/installation/) first and verify its
CRDs are available:

```sh
kubectl get crd certificates.cert-manager.io issuers.cert-manager.io
```

Install the published OCI chart, or use the chart in your checkout:

```sh
helm install kollect oci://ghcr.io/platformrelay/charts/kollect \
  --namespace kollect-system --create-namespace
kubectl -n kollect-system rollout status \
  deployment/kollect-controller-manager --timeout=120s
```

!!! info "The chart coordinate moved to `charts/kollect`"
    The chart used to be published at `ghcr.io/platformrelay/kollect` (no `charts/` segment) — the
    same OCI repository that serves the **controller image**. Artifact Hub's documented contract is
    one chart per repository (`oci://registry/namespace/chart-name`), and a repository holding two
    artifact kinds cannot satisfy it: every `v`-prefixed image tag was loaded as a chart and
    failed. The chart therefore lives at **`ghcr.io/platformrelay/charts/kollect`**, and the
    controller image stays at `ghcr.io/platformrelay/kollect` — it does not move, because its
    digest is pinned immutably in already-published OLM bundles
    ([ADR-0709](../adr/0709-chart-image-oci-path-separation.md)).

**Existing installs keep working.** The chart history was copied to the new path, so both
coordinates serve byte-identical manifests at identical digests and nothing breaks at the moment of
the move. But **only the new path receives new versions**: repoint anything that pins or automates
against the old coordinate — GitOps `HelmRelease`/`Application` sources, Renovate or Dependabot
rules, CI `helm pull` steps — or it will silently stop seeing releases. `image.repository` values
are unaffected; that is the image, not the chart.

For production values, restricted watch scope, secrets, and webhook TLS, use the
[operator manual](../operator-manual/index.md). See the [release page](../RELEASE.md) before
upgrading a pinned installation.

If cert-manager cannot be installed, either provide the webhook serving Secret and CA injection
yourself before installing with `webhooks.certManager.create=false`, or explicitly disable webhooks
for a constrained development environment. The chart does not generate certificates when
`webhooks.certManager.create=false`; disabling admission is not the recommended production path.

## Discoverability on package hubs

Helm OCI on GHCR remains the primary install path ([ADR-0705](../adr/0705-release-supply-chain.md)).
Additional distribution wiring ships under [ADR-0708](../adr/0708-operator-distribution-hubs.md):

- **Artifact Hub** — the chart repository is registered and listed as
  [`kollect`](https://artifacthub.io/packages/search?repo=kollect); it indexes the same OCI chart
  (`oci://ghcr.io/platformrelay/charts/kollect`), so Artifact Hub is a discovery surface, not a
  separate install path — `helm install` from GHCR exactly as above. The registration points at the
  `charts/` path because Artifact Hub indexes one chart per repository entry
  ([ADR-0709](../adr/0709-chart-image-oci-path-separation.md)); the same repository ID, stars and
  Verified Publisher status carried over, so the listing URL is unchanged.
- **OperatorHub / OpenShift** — OLM bundles are generated at release and submitted to the community
  operator catalogs when `OPERATORHUB_PAT` is configured. The **OpenShift community catalog**
  submission is merged, so on OpenShift the operator installs from the console's OperatorHub tab
  using package **`kollect`**, channel **`stable`**.
  The **OperatorHub.io** listing is *not* live yet: our upstream submission is open and green on our
  side, but its workflow runs sit at `action_required`, waiting for an upstream maintainer to
  approve them. Until that merges there is no OperatorHub.io package page to link to — use Helm, the
  OpenShift catalog, or the manifests on the [release page](../RELEASE.md).
  The OLM bundle runs with `--validating-webhooks-enabled=false` (no webhook Service/cert in the
  bundle). Prefer Helm OCI for the full admission path; CRD schema validation still applies under OLM.

Hub URLs on this page and in the README point only at listings that actually resolve. A hub URL that
answers HTTP 200 and then renders "package not found" is a dead link, not a graceful landing page
([ADR-0708](../adr/0708-operator-distribution-hubs.md)).

## Next step

Continue to [Your first inventory](first-inventory.md) to collect Deployments and verify the
export in Git.

## Clean up

For the local path:

```sh
task kind-dev-down
```

If installation fails, check the manager logs and the
[troubleshooting guide](../operator-manual/troubleshooting.md).
