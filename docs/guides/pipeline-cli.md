# Pipeline CLI guide

`kollect-pipeline` collects Kubernetes inventory from a **kubeconfig**, in a single short-lived
run, without installing the kollect operator in the cluster. It is built for CI/CD pipelines: a
scheduled job points it at one or more clusters, it renders the inventory, and either writes files
for the job to commit or commits to git itself.

This is the CLI counterpart to the in-cluster operator. Both share the same collection engine and
the same `KollectProfile` / `KollectTarget` / `KollectSnapshotSink` resources — the CLI just reads
them from a local directory instead of the cluster API (ADR-0801).

## Overview

```
kubeconfig ──▶ kollect-pipeline collect ──▶ inventory files (--output)  ──▶ your CI commits them
                     ▲                   └─▶ git sink (KollectSnapshotSink) ──▶ kollect commits them
                     │
             --config <dir> of KollectProfile + KollectTarget [+ Sink] YAML
```

- **One-shot:** no controllers, no CRDs installed, no leader election. Runs, writes, exits.
- **Multi-context:** collect from many kubecontexts (or a glob) in one invocation.
- **Read-only:** only `list`/`get` against the target clusters; it never writes to them.

## Prerequisites

- A kubeconfig with read access (`list`/`get`) to the resources you want to collect.
- A config directory containing at least one `KollectProfile` and one `KollectTarget`
  (see [Configuration directory](#configuration-directory)).
- Either the `kollect-pipeline` binary or its container image (see [Install](#install)).

## Install

**Container image (CI):**

```sh
docker run --rm ghcr.io/konih/kollect-pipeline:<version> --help
```

> The `kollect-pipeline` container image is published to GHCR as part of the release that ships
> ADR-0801 P-006 (image + release wiring). Until that lands, build the binary from source with the
> options below.

**Build from source:**

```sh
git clone https://github.com/konih/kollect.git && cd kollect
task build:cli            # produces ./bin/kollect-pipeline
# or, without Task:
go build -o bin/kollect-pipeline ./cmd/cli
```

## Configuration directory

`--config <dir>` points at a directory of YAML manifests. `kollect-pipeline` loads every `*.yaml` /
`*.yml` file in that directory (non-recursive) and expects:

| Kind | Purpose | Count |
| --- | --- | --- |
| `KollectProfile` | **what** to extract from each resource | one or more |
| `KollectTarget` | **where** to collect from (GVK selectors); its `profileRef` names a profile in the same directory | one or more |
| `KollectSnapshotSink` | **where** to write, when kollect owns the write | **at most one** (or zero + `--output`) |
| `v1/Secret` | credentials referenced by a sink's `secretRef`; values may be `${env:VAR}` placeholders resolved from the CI environment | optional |

Unknown kinds are ignored with a warning. Every `KollectTarget.spec.profileRef` must match a
`KollectProfile` name loaded from the same directory, or the run fails before contacting any cluster.

### KollectProfile — what to collect

A profile selects a GVK and lists the attributes to extract from each matching object. Paths use
JSONPath (`$.…`), CEL (`cel:…`), or the Helm-release decoder (`helm:release.…`):

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectProfile
metadata:
  name: deployment-images
  namespace: kollect-system
spec:
  targetGVK:
    group: apps
    version: v1
    kind: Deployment
  attributes:
    - name: image
      path: "$.spec.template.spec.containers[0].image"
    - name: containerCount
      path: "cel:size(object.spec.template.spec.containers)"
      type: int
```

### KollectTarget — where to collect from

A target references a profile and narrows the set of objects (label selectors, namespaces, names):

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectTarget
metadata:
  name: deployment-images
  namespace: kollect-system
spec:
  profileRef: deployment-images
  # labelSelector: { matchLabels: { app.kubernetes.io/managed-by: Helm } }
  # includedNamespaces: [team-a, team-b]
```

For cluster-scoped kinds (for example `v1/Namespace`), namespace selectors do not apply; kollect
performs a single cluster-wide list.

### KollectSnapshotSink — where to write

Two ways to get output:

1. **`--output <dir>`** — write inventory files locally; your CI job commits them. Do **not** ship a
   sink manifest in this case.
2. **A `KollectSnapshotSink` of `type: git`/`gitlab`/`s3`/`gcs`** — kollect writes to that backend
   itself. At most one sink manifest per config directory. (`type: local` is not a sink kind — use
   `--output` for local files.)

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectSnapshotSink
metadata:
  name: inventory-git
  namespace: kollect-system
spec:
  type: git
  endpoint: https://gitlab.example.com/platform-team/cluster-inventory.git
  pathTemplate: clusters/{cluster}/{namespace}/{name}.yaml   # {cluster} {namespace} {name} only
  cluster: my-cluster
  git:
    branch: main
    pushPolicy: Commit
    auth:
      type: token
  secretRef:
    name: git-credentials      # a v1/Secret manifest (key: token) placed in the same --config dir
```

For a private repo, add the referenced `v1/Secret` manifest (with a `token` key) to the config
directory; `kollect-pipeline` resolves `secretRef` against Secrets in that directory, not the
cluster.

### Sink credentials from the CI environment (`${env:VAR}`)

Committing a real token in the Secret manifest is wrong for CI. Instead, keep the credential in
your CI system's variable store and reference it with an env placeholder — a `stringData` (or
decoded `data`) value that is **exactly** `${env:VAR_NAME}` is substituted from the process
environment when the sink's `secretRef` is resolved:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: git-credentials
  namespace: kollect-system
stringData:
  token: ${env:KOLLECT_GIT_TOKEN}
```

- **GitLab CI:** define `KOLLECT_GIT_TOKEN` as a masked (and, for protected branches, protected)
  variable under *Settings → CI/CD → Variables*; it is exported into the job environment
  automatically.
- **GitHub Actions:** map a repository secret into the step env:
  `env: { KOLLECT_GIT_TOKEN: "${{ secrets.KOLLECT_GIT_TOKEN }}" }`.

Rules: substitution applies only when the whole value is a single `${env:VAR}` placeholder
(values that merely contain one mid-string stay verbatim, so real credentials are never
rewritten); an **unset or empty** variable fails the run with an error naming the secret, key,
and variable; literal `data`/`stringData` values keep working unchanged, and `stringData` wins
over `data` for the same key. See the shipped
[`git-sink` sample](https://github.com/konih/kollect/tree/main/config/samples/pipeline/git-sink)
for the complete flow.

## Use case examples

Ready-to-run config directories live in
[`config/samples/pipeline/`](https://github.com/konih/kollect/tree/main/config/samples/pipeline):

| Use case | Directory | GVK |
| --- | --- | --- |
| Helm release versions | `helm-releases/` | `v1/Secret` via `helm:release.*` |
| Image versions | `deployment-images/` | `apps/v1/Deployment` |
| Ingress hostnames | `ingress-hosts/` | `networking.k8s.io/v1/Ingress` |
| Namespace list | `namespaces/` | `v1/Namespace` (cluster-scoped) |
| Image versions → git | `git-sink/` | `apps/v1/Deployment` + `type: git` sink |

### Helm release versions

Helm 3 stores each release as a `v1/Secret`. The `helm:release.` prefix decodes it and exposes
release metadata (`chartName`, `chartVersion`, `appVersion`, `status`, …) without reading raw Secret
data, so no secret-extraction opt-in is required. The target matches Helm's `owner=helm,
status=deployed` labels. See `config/samples/pipeline/helm-releases/`.

### Image versions (Deployments)

`$.spec.template.spec.containers[*].image` captures every container image; add
`$.spec.replicas` for scale. See `config/samples/pipeline/deployment-images/`.

### Ingress hostnames

`$.spec.rules[*].host` captures the exposed hostnames; `$.spec.ingressClassName` records the class.
See `config/samples/pipeline/ingress-hosts/`.

### Namespace list

A cluster-scoped profile over `v1/Namespace` records each namespace's `status.phase` and labels.
See `config/samples/pipeline/namespaces/`.

## Running locally

```sh
kollect-pipeline collect \
  --kubeconfig ~/.kube/config \
  --config    config/samples/pipeline/deployment-images \
  --output    ./inventory \
  --dry-run
```

`--dry-run` prints what would be written without touching the filesystem or git. Drop it to write
files under `./inventory`.

### Flags

| Flag | Meaning |
| --- | --- |
| `--config <dir>` | **Required.** Directory of profile/target/sink YAML. |
| `--kubeconfig <path>` | Kubeconfig to use. Default: `$KUBECONFIG`, then `~/.kube/config`. |
| `--output <dir>` | Write inventory files here (synthesizes a local sink). Mutually exclusive with a sink manifest. |
| `--context <name\|glob>` | Kubecontext(s) to collect from; repeatable and comma-separated; globs allowed. Default: current context. |
| `--namespace <ns>` | Restrict collection to a single namespace (overrides target selectors). |
| `--dry-run` | Collect and log what would be written; write nothing. |
| `--log-level <lvl>` | `debug` \| `info` \| `warn` \| `error` (default `info`). |

### Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success — all targets collected and written. |
| `1` | Partial — at least one target was skipped (forbidden / transient / GVK not found) but something was still collected. |
| `2` | Fatal — config invalid, cluster unreachable, or every target failed. |

In a multi-context run the process exit code is the worst outcome across all contexts.

## GitLab CI integration

Store a base64-encoded kubeconfig as a masked CI/CD variable `KUBECONFIG_B64` (base64 avoids
newline-mangling), and commit your config directory as `collect-config/`.

```yaml
# .gitlab-ci.yml
stages: [collect]

collect-inventory:
  stage: collect
  image: ghcr.io/konih/kollect-pipeline:<version>
  script:
    - echo "$KUBECONFIG_B64" | base64 -d > /tmp/kubeconfig
    - kollect-pipeline collect
        --kubeconfig /tmp/kubeconfig
        --config     ./collect-config
        --output     ./inventory
    - git config user.email "ci@example.com"
    - git config user.name  "CI"
    - git add inventory/
    - git diff --cached --quiet || git commit -m "chore: inventory snapshot [skip ci]"
    - git push "https://oauth2:${GIT_PUSH_TOKEN}@${CI_SERVER_HOST}/${CI_PROJECT_PATH}.git" HEAD:${CI_DEFAULT_BRANCH}
  rules:
    - if: '$CI_PIPELINE_SOURCE == "schedule"'
    - if: '$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH'
  artifacts:
    paths: [inventory/]
```

## GitHub Actions integration

Store the base64 kubeconfig as the `KUBECONFIG_B64` secret.

```yaml
# .github/workflows/collect-inventory.yml
name: Collect inventory
on:
  schedule:
    - cron: '0 * * * *'   # hourly
  workflow_dispatch:

permissions:
  contents: write

jobs:
  collect:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Write kubeconfig
        run: echo "${{ secrets.KUBECONFIG_B64 }}" | base64 -d > "$RUNNER_TEMP/kubeconfig"
      - name: Collect inventory
        run: |
          docker run --rm \
            -v "$RUNNER_TEMP/kubeconfig:/kubeconfig:ro" \
            -v "${{ github.workspace }}/collect-config:/config:ro" \
            -v "${{ github.workspace }}/inventory:/out" \
            ghcr.io/konih/kollect-pipeline:<version> \
            collect --kubeconfig /kubeconfig --config /config --output /out
      - name: Commit inventory
        run: |
          git config user.email "actions@github.com"
          git config user.name  "GitHub Actions"
          git add inventory/
          git diff --cached --quiet || git commit -m "chore: inventory snapshot [skip ci]"
          git push
```

## Multi-cluster setup

Merge your cluster contexts into one kubeconfig and select them with `--context` (repeatable,
comma-separated, glob-aware):

```sh
kollect-pipeline collect \
  --kubeconfig ./merged.kubeconfig \
  --context 'prod-*,staging-eu' \
  --config ./collect-config \
  --output ./inventory
```

Each context is collected sequentially; a fatal error in one does not stop the others, and the
process exit code is the worst outcome across all of them. `{cluster}` in a sink `pathTemplate`
defaults to the context name, so per-cluster output is partitioned automatically.

## Upgrade path to the operator

The same `KollectProfile` / `KollectTarget` / `KollectSnapshotSink` manifests apply unchanged when
you later install the in-cluster operator (`type: local`/`--output` becomes a real sink such as
git, s3, or a database). Start with the CLI in CI; graduate to the operator when you want
continuous, event-driven collection instead of scheduled runs.

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| `--config is required` | Pass `--config <dir>`; it has no default. |
| `profileRef "…" not found in config directory` | A `KollectTarget.spec.profileRef` names a profile that isn't in `--config`. |
| `N KollectSnapshotSink objects found … only one is supported` | Keep at most one sink manifest per config dir (or use `--output`). |
| `--output and a KollectSnapshotSink … are ambiguous` | Use `--output` **or** a sink manifest, not both. |
| `sink secretRef "…" not found` | Add the referenced `v1/Secret` manifest to the config dir. |
| `environment variable for secret placeholder not set` | A Secret value is `${env:VAR}` but `VAR` is unset/empty — define it in the CI job env (e.g. a GitLab masked variable). |
| Exit code `1` | Some targets were skipped (RBAC forbidden / GVK absent). Run with `--log-level debug` to see which. |
| Exit code `2` | Cluster unreachable or config invalid — check `--kubeconfig` and the manifests. |
