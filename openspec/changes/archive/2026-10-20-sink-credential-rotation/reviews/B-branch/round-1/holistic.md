# Stage B round 1 — holistic run on the branch tip

Branch tip `df5170e57` (code tip `b6fca7d1e`), base `origin/main` @ `ae98ef106`. All runs
2026-10-20, `umask 022`, `KUBEBUILDER_ASSETS` from the main checkout (`bin/k8s/1.36.0-linux-amd64`),
`MISE_GITLEAKS_VERSION=8.30.1`. Every result below is verified (run here).

## Rebase

`git fetch origin`: `origin/main` is still `ae98ef106`, the merge base. No rebase needed;
`git merge-tree` is trivially clean (the branch is a fast-forward of `origin/main`).

## `task check`

Exit 201, five gates red. Each was rerun or traced to its cause:

| Gate | Result | Cause | Disposition |
|---|---|---|---|
| verify, format:check, lint:shell, lint:markdown, lint, vulncheck, test, scrub, spec:validate, audit-rbac, build, helm, gitleaks, workflow-security, workflow-security-audit, every other guard | ok | — | — |
| go-mod (`go mod tidy -diff && go mod verify`) | FAILED | the branch's controller tests import `k8s.io/utils/ptr` (`secret_reader_wiring_test.go:96`, `secret_retire_envtest_test.go:90`, `secret_rotation_envtest_test.go:44`), so tidy moves `k8s.io/utils` from indirect to direct. The Preflight workflow runs the same command, so the PR would be red. | **branch defect, fix** (stage-B fix H1: use the package's existing `boolPtr`) |
| gitleaks-detect (`--no-git`) | FAILED | one finding, `.cache/plugin/social/manifest.json`: the git-ignored mkdocs social-card cache that G4's `docs:verify` left in this worktree. With `.cache` moved aside the same command reports `no leaks found`. | environment (ignored build cache), not the branch |
| guard `demo_04_samples_kustomize_test.sh` | FAILED | `mise ERROR No version is set for shim: kubectl`. Rerun with `MISE_KUBECTL_VERSION=1.36.1`: exit 0 | environment |
| guard `dist_olm_bundle_test.sh` | FAILED | `operator-sdk is required` (the worktree `bin/` lacked it). After `bash hack/install-operator-sdk.sh ./bin` (v1.42.3), rerun with the kubectl pin: exit 0 | environment |
| guard `dist_operatorhub_pr_test.sh` | FAILED | same cause (`DRY_RUN operatorhub-pr.sh failed` without operator-sdk); rerun: exit 0 | environment |

Excluded by `task check` itself (its table): test-integration (run below), kind-smoke, docker-build,
pipeline-cli-smoke, dependency-review.

## `task test-integration`

First run: `RESULT: FAIL`, three MinIO tests (`TestExportMinIO`, `TestExportS3Compatible`,
`TestRunExportEnvelope_SpillsOversizedPayloadToMinIO`) failed with `XMinioStorageFull`: the host
root filesystem was 100 % full (4.6 GB free). Go build-cache entries unused for more than two days
were removed (85 GB; a cache, rebuilt on demand). Second run, exit 0:

```text
integration evidence (required): expected 68, executed 68, passed 68, failed 0, skipped 0, missing 0
RESULT: PASS - every expected integration test ran and passed
```

## Per-commit gate

Each code commit checked out alone in a temporary detached worktree under `/tmp` (removed after):
`go build ./...`, `go vet` and `go vet -tags=integration` on the touched packages, then
`go test -count=1` on them (controller includes the envtest suite).

| Commit | Packages | build | vet | test |
|---|---|---|---|---|
| `d307e2ec8` (G1) | `./cmd/...`, `./internal/sink/`, `./internal/controller/` | 0 | 0 | 0 (4 `ok`, 0 `FAIL`) |
| `f08fb2b12` (G2) | `./internal/sink/`, `./internal/controller/`, `./internal/pipeline/` | 0 | 0 | 0 (3 `ok`) |
| `b6fca7d1e` (G3) | `./internal/sink/` | 0 | 0 | 0 (1 `ok`) |

`df5170e57` (G4) is docs only and is the tip: `task check`'s lint:markdown ran on it here;
`task docs:verify` ran in G4 (`evidence/G4.md`).
