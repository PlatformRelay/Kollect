# kollect-pipeline sample configs

Copy-paste-ready config directories for `kollect-pipeline collect` (the kubeconfig-based CLI
collection mode, ADR-0801). Each subdirectory is a complete, self-contained config directory you
can point `--config` at. Full walkthrough: [`docs/guides/pipeline-cli.md`](../../../docs/guides/pipeline-cli.md).

| Directory | Collects | GVK | Sink |
| --- | --- | --- | --- |
| [`helm-releases/`](helm-releases/) | Helm 3 release chart/app versions | `v1/Secret` (via `helm:release.*`) | `--output` (local files) |
| [`deployment-images/`](deployment-images/) | Container image versions | `apps/v1/Deployment` | `--output` (local files) |
| [`ingress-hosts/`](ingress-hosts/) | External ingress hostnames | `networking.k8s.io/v1/Ingress` | `--output` (local files) |
| [`namespaces/`](namespaces/) | Namespace list + phase/labels | `v1/Namespace` (cluster-scoped) | `--output` (local files) |
| [`git-sink/`](git-sink/) | Image versions, committed to git by kollect | `apps/v1/Deployment` | `KollectSnapshotSink type: git` |

## Two ways to write output

- **Local files (`--output`)** — the `helm-releases`, `deployment-images`, `ingress-hosts` and
  `namespaces` directories ship no sink manifest. Pass `--output ./inventory/` and your CI job
  owns the `git add`/`commit`/`push` (see the guide's GitLab CI / GitHub Actions templates).
- **Git sink** — the `git-sink` directory ships a `KollectSnapshotSink` of `type: git`, so kollect
  itself commits and pushes. A config directory holds **at most one** sink manifest; do not combine
  a sink manifest with `--output`.

## Try it

```sh
kollect-pipeline collect \
  --kubeconfig ~/.kube/config \
  --config config/samples/pipeline/deployment-images \
  --output ./inventory \
  --dry-run
```

Drop `--dry-run` to actually write files. These directories are validated in CI against the real
config loader and CRD validation (`test/samples/pipeline_samples_test.go`).
