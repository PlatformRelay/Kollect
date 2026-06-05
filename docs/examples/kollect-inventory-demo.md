# Example: Git inventory demo (`konih/kollect-inventory-demo`)

Minimal **kind** lab that rolls up deployment inventory and exports JSON to the public demo repo
[github.com/konih/kollect-inventory-demo](https://github.com/konih/kollect-inventory-demo).

For a **wide-scope** demo (5 GVK types, 26 workloads, multi-namespace Targets, churn script), see
`hack/demo/kind-wide-scope/` and the local playbook `agent-context/DEMO-KIND-WIDE-SCOPE.md`.

## Prerequisites

- `kind`, `kubectl`, `helm`, `task`, Docker
- GitHub push token with `repo` scope (never commit it): `export GITHUB_TOKEN="$(gh auth token)"`

## 1. Cluster + operator

```sh
task kind-dev-up   # or: KOLLECT_DEV_MINIMAL=1 task kind-dev-up
kubectl config use-context kind-kollect-dev
```

The controller image must mount a writable `/tmp` (Helm chart `emptyDir`) because git export clones into a
temp workdir while `readOnlyRootFilesystem` is enabled.

## 2. Sample workload

```sh
kubectl create deployment nginx-demo --image=nginx:1.27 -n default
kubectl label deployment nginx-demo -n default app.kubernetes.io/name=nginx
```

## 3. Git push credentials (local only)

```sh
kubectl create secret generic git-push-credentials -n default \
  --from-literal=token="$GITHUB_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -
```

## 4. kollect CRs

Apply from this repo:

```sh
kubectl apply -f config/samples/kollect_v1alpha1_kollectprofile.yaml
kubectl apply -f config/samples/kollect_v1alpha1_kollecttarget.yaml
```

Git sink + inventory (same namespace as `sinkRefs`):

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectSink
metadata:
  name: git-inventory-demo
  namespace: default
spec:
  type: git
  endpoint: https://github.com/konih/kollect-inventory-demo.git
  connectionTest: true
  secretRef:
    name: git-push-credentials
    namespace: default
---
apiVersion: kollect.dev/v1alpha1
kind: KollectInventory
metadata:
  name: team-inventory
  namespace: default
spec:
  sinkRefs:
    - git-inventory-demo
  suspend: false
```

Object path (ADR-0407): `inventory/default/team-inventory.json`.

## 5. Verify

```sh
kubectl wait --for=condition=Ready kollectinventory/team-inventory -n default --timeout=180s
kubectl port-forward -n kollect-system svc/kollect-controller-manager 8082:8082 &
curl -sf http://127.0.0.1:8082/inventory | jq .itemCount
```

Remote (optional):

```sh
export GIT_EXPORT_TEST_REPO=https://github.com/konih/kollect-inventory-demo.git
bash hack/e2e/git-export-assert.sh
```

Unit tests for the git sink:

```sh
go test ./internal/sink/git/...
```

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| `create workdir: mkdir /tmp/kollect-git-export-*: read-only file system` | Missing `/tmp` `emptyDir` on the manager pod |
| `ConnectionVerified=False` / no commits | Missing or wrong `secretRef` token |
| Inventory HTTP OK but no Git push | Sink unreachable from cluster egress/DNS; verify with a curl pod in `kollect-system` |
