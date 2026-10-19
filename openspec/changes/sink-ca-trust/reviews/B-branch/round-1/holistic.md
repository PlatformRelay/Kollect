# Stage B round 1 — holistic run

Branch tip `0a8afca93` (`feat/sink-ca-trust`), 2026-10-19, on this host (`go1.27.2 linux/amd64`,
Docker 29.9.0), `umask 022`, `KUBEBUILDER_ASSETS` from the main checkout. `git fetch origin`:
`origin/main` is still `317279084`, the merge base, so no rebase; `git merge-tree --write-tree
origin/main HEAD` is clean.

## `task check`

Run in the background and waited on by PID: exit 201, 49 minutes. Every Go and docs gate is
green: `verify`, `format:check`, `lint:shell`, `lint:markdown`, `lint`, `vulncheck`, `test`,
`scrub`, `spec:validate`, `audit-rbac`, `build`, `helm`, `go-mod`, `gitleaks`, `gitleaks-detect`,
`workflow-security`, `workflow-security-audit`, and 118 of 121 `hack/test` guards. The script's
own exclusions (Docker, kind, `dependency-review`) are listed by it and run elsewhere.

Three guards failed, all ENVIRONMENT (host tooling), none touched by the branch:

| Guard | Failure line | Rerun with the tool supplied | Result |
|---|---|---|---|
| `demo_04_samples_kustomize_test.sh` | `mise ERROR No version is set for shim: kubectl` | `MISE_KUBECTL_VERSION=1.36.1` | exit 0, `ok (2 tree(s) via kubectl kustomize)` |
| `dist_olm_bundle_test.sh` | `operator-sdk is required for the OperatorHub validator gate` (the main checkout's `bin/operator-sdk` is a Mach-O arm64 binary: `Exec format error`) | `bash hack/install-operator-sdk.sh ./bin` (linux v1.42.3, gitignored `bin/`) | exit 0, `All dist OLM bundle tests passed.` |
| `dist_operatorhub_pr_test.sh` | `DRY_RUN operatorhub-pr.sh failed` (same missing `operator-sdk`) | same | exit 0, `All dist OperatorHub PR tests passed.` |

Verdict: green on the branch tip once the host supplies `kubectl` and a linux `operator-sdk`.

## `task test-integration` (Docker)

Exit 201, 86 s. Every package `ok`, but the evidence gate failed:

```text
integration evidence (required): expected 68, executed 65, passed 65, failed 0, skipped 3, missing 0
skipped (expected integration tests):
  github.com/platformrelay/kollect/internal/sink/mongodb.TestMongoCATrustChild_MI3
  github.com/platformrelay/kollect/internal/sink/postgres.TestPostgresCATrustChild_PI3
  github.com/platformrelay/kollect/internal/sink/s3.TestCATrustIntegrationChild_systemRootsKept
RESULT: FAIL
```

Finding H1 (WARNING, this branch): the three subprocess children that PI3, MI3 and I3 re-execute
are separate top-level tests in integration-tagged files. The gate
(`hack/tools/integrationevidence`) counts every such test as required and fails on a skip, and
each child skips in the parent by design. CI's `test-integration` job (`ci.yaml:753`, `task
test-integration`) would go red on this branch. The parents themselves passed (the child ran and
passed inside each). Fix: run the child branch inside its parent test, so no expected test skips
(`verification.md`, stage-B fix 8.1).

## Per-commit gate

For each code commit, a detached worktree under
`/home/koni/Projects/PlatformRelay/worktrees/kollect/b-sct-<sha>`: `go build ./... && go vet ./...`
and `go test -count=1` on the packages the commit touches. Each worktree removed afterwards
(`git worktree prune`).

| Commit | Exit | Seconds | Packages tested |
|---|---|---|---|
| `0cdece478` shared pool and CA rules | 0 | 28 | `internal/pipeline`, `internal/sink`, `internal/sink/sinktls`, `sinktlstest` |
| `d9023cca3` S3 and GCS | 0 | 24 | `internal/sink`, `internal/sink/gcs`, `internal/sink/s3` |
| `d5ead06c0` Postgres | 0 | 24 | `internal/sink`, `internal/sink/postgres`, `sinktlstest` |
| `45ed24e7a` MongoDB | 0 | 26 | `internal/sink`, `internal/sink/mongodb`, `sinktlstest` |
| `9584aa3c3` parity tests | 0 | 23 | `cmd/cli`, `internal/pipeline`, `internal/sink` |
| `0a8afca93` docs and field comments | 0 | 8 | `api/v1alpha1` |

The docs-only spec commits (`ef7bd000f` … `9abf134a3`) change no Go file and were not gated.
