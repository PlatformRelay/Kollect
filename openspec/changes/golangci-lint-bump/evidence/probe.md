# Probe record — golangci-lint bump v2.11.4 → v2.13.1 (task 1.1)

Date: 2026-10-07 · Branch `fm/kollect-dev-continue` · pre-task sha `7825750b` ·
machine: macOS arm64, go1.26.6 (mise), treehouse worktree slot 6. All commands run in the repo
root. State file: `evidence/1.1.md` (links back here). Full logs were taken to a session temp
directory (not committed); every claim below carries the command, exit code and the trimmed raw
output that supports it. Unless marked *believed*, a claim is verified (the command was run in
this session, today).

## 1. Before baseline — `task lint` at the current pin v2.11.4

Binary in place: `bin/golangci-lint`, symlink-replaced custom build,
`./bin/golangci-lint version` → `golangci-lint has version v2.11.4-custom-gcl-47DEQpj8HBSa… built with go1.26.6 from ? on 2026-09-25 00:54:23 +0000 UTC`.

- **Attempt 1** — `task lint`: **exit 2 (failed)**; golangci-lint's own 5-minute internal
  timeout (`run.timeout: 5m` in `.golangci.yaml`) hit before analysis finished; it had printed
  `0 issues.` at the point of the kill, then
  `level=error msg="Timeout exceeded: try increasing it by passing --timeout option"` and
  `make: *** [lint] Error 4`. Classified ENVIRONMENT (cold caches / machine load); product
  config untouched; retried once.
- **Attempt 2 (baseline run)** — `task lint`: **exit 0**, ~1:27 wall. Raw output (trimmed to
  the task markers):

  ```
  task: [install] go mod download
  task: [install] go mod verify
  all modules verified
  task: [lint] make lint
  "/Users/kheimel/.treehouse/kollect-79dca7/6/kollect/bin/golangci-lint" run
  0 issues.
  task: [arch-lint] go run -mod=mod github.com/fe3dback/go-arch-lint@v1.15.0 check --arch-file .go-arch-lint.yml
  …
  OK - No warnings found
  ```

**Before findings count: 0 issues** (v2.11.4 custom build; `make lint` exit 0). go-arch-lint:
OK — no warnings. LTB-3 "before" number.

On attempt 1's odd ordering (`0 issues.` printed *before* the timeout error): in golangci-lint
v2 the summary printer emits the issues counted so far, and a run killed at its internal
deadline reports that partial count plus the timeout error — so `0 issues.` there is an
interim count of a killed run, not a full-run result (*believed* — standard printer behaviour,
not reconciled against source; it does not affect the baseline, which is attempt 2's complete
exit-0 run; noted by the round-1 review leg as unexplained).

## 2. Pin bump (uncommitted, reverted at the end)

- `Makefile:184` — `GOLANGCI_LINT_VERSION ?= v2.11.4` → `v2.13.1`; verified with
  `git diff --stat`: `Makefile | 2 +-`, `hack/tooling/.custom-gcl.yml | 2 +-` (2 files).
- `hack/tooling/.custom-gcl.yml:6` — `version: v2.11.4` → `version: v2.13.1`.
- `rm -f bin/golangci-lint*` — removed `bin/golangci-lint` (custom v2.11.4) and
  `bin/golangci-lint-v2.11.4` (vanilla v2.11.4). Necessary: `make -n golangci-lint
  GOLANGCI_LINT_VERSION=v2.13.1` on the stale file target printed `Nothing to be done`
  (pre-verified, loop.md round-2 finding 7); after the removal `make -n` printed the full
  install + custom-build recipe.

## 3. `make golangci-lint` (custom build with plugins at v2.13.1)

- `go list -m sigs.k8s.io/logtools@latest` **before** the build: `sigs.k8s.io/logtools v0.10.1`.
- `make golangci-lint`: **exit 0**, ~1:21 wall (module downloads warm after ~60 s of
  `go: downloading …` lines; the `golangci-lint custom` step is silent). Log tail:
  `Building custom golangci-lint with plugins...` and the recipe completed (`mv` produced the
  final binary). The make log does not print the plugin version it resolved.

## 4. Vanilla binary cannot pass this config (structural proof)

Tested with the vanilla v2.13.1 binary that the go-install-tool scheme leaves at
`bin/golangci-lint-v2.13.1` (the custom build overwrites `bin/golangci-lint` itself):

- `./bin/golangci-lint-v2.13.1 config verify` → **exit 0** — config *verification* alone does
  NOT catch the unregistered plugin. (The old vanilla v2.11.4 binary no longer existed to
  repeat this on; *believed* it behaves the same — same plugin system, and the mechanism below
  is version-independent: `logcheck` is a module plugin, vanilla binaries cannot register it.)
- `./bin/golangci-lint-v2.13.1 run --timeout 90s ./cmd/...` → **exit 3**, immediate startup
  failure, raw output:

  ```
  Error: build linters: plugin(logcheck): plugin "logcheck" not found
  The command is terminated due to an error: build linters: plugin(logcheck): plugin "logcheck" not found
  ```

**Conclusion (verified):** a vanilla golangci-lint v2.13.1 cannot run this repo's
`.golangci.yaml` — the enabled `logcheck` linter is provided by the custom module-plugin build.
The failure surface is linter-set construction at run start, not `config verify`. This is the
structural proof task 1.3 relies on: a clean `task lint` at the pin implies the custom build
(vanilla would exit 3 before producing any findings).

## 5. `task lint` under the built v2.13.1 custom binary

`task lint` → **make lint exit 1, task exit 2** (expected: findings exist until task 1.3;
LTB-3 "runnable" = binary starts and reports findings — satisfied). Summary line:

```
57 issues:
* goconst: 44
* gosec: 1
* staticcheck: 12
make: *** [lint] Error 1
task: Failed to run task "lint": exit status 2
```

**After findings count: 57** (44 goconst, 1 gosec, 12 staticcheck SA1019). The arch-lint step
did not run inside `task lint` (make lint failed first), so the Go-layering fitness function
was run separately: `task arch-lint` → exit 0, `OK - No warnings found`.

Full issue list (57 lines, `path:line:col: message (linter)` — the input for task 1.3):

```
internal/collect/helmdecode.go:34:2: string `namespace` has 3 occurrences, make it a constant (goconst)
internal/collect/helmdecode.go:35:2: string `config` has 3 occurrences, make it a constant (goconst)
internal/collect/prune.go:125:37: string `namespace` has 3 occurrences, make it a constant (goconst)
internal/controller/finalizer.go:34:23: SA1019: (sigs.k8s.io/controller-runtime.Result).Requeue is deprecated: Use `RequeueAfter` instead. (staticcheck)
internal/controller/kollectclusterinventory_controller.go:703:23: SA1019: (sigs.k8s.io/controller-runtime.Result).Requeue is deprecated: Use `RequeueAfter` instead. (staticcheck)
internal/controller/kollectclusterinventory_controller.go:748:25: string `Exported` has 3 occurrences, make it a constant (goconst)
internal/controller/kollectclusterinventory_controller.go:825:23: SA1019: (sigs.k8s.io/controller-runtime.Result).Requeue is deprecated: Use `RequeueAfter` instead. (staticcheck)
internal/controller/kollectclusterinventory_controller.go:89:24: SA1019: (sigs.k8s.io/controller-runtime.Result).Requeue is deprecated: Use `RequeueAfter` instead. (staticcheck)
internal/controller/kollectclustertarget_controller.go:77:24: SA1019: (sigs.k8s.io/controller-runtime.Result).Requeue is deprecated: Use `RequeueAfter` instead. (staticcheck)
internal/controller/kollectconnectiontest_controller.go:204:23: SA1019: (sigs.k8s.io/controller-runtime.Result).Requeue is deprecated: Use `RequeueAfter` instead. (staticcheck)
internal/controller/kollectconnectiontest_controller.go:243:23: SA1019: (sigs.k8s.io/controller-runtime.Result).Requeue is deprecated: Use `RequeueAfter` instead. (staticcheck)
internal/controller/kollectinventory_controller.go:130:24: SA1019: (sigs.k8s.io/controller-runtime.Result).Requeue is deprecated: Use `RequeueAfter` instead. (staticcheck)
internal/controller/kollectinventory_controller.go:561:25: string `Exported` has 3 occurrences, make it a constant (goconst)
internal/controller/kollectinventory_controller.go:602:23: SA1019: (sigs.k8s.io/controller-runtime.Result).Requeue is deprecated: Use `RequeueAfter` instead. (staticcheck)
internal/controller/kollectinventory_controller.go:639:23: SA1019: (sigs.k8s.io/controller-runtime.Result).Requeue is deprecated: Use `RequeueAfter` instead. (staticcheck)
internal/controller/reconcile_guard.go:36:25: SA1019: (sigs.k8s.io/controller-runtime.Result).Requeue is deprecated: Use `RequeueAfter` instead. (staticcheck)
internal/controller/target_finalizer.go:36:23: SA1019: (sigs.k8s.io/controller-runtime.Result).Requeue is deprecated: Use `RequeueAfter` instead. (staticcheck)
internal/metrics/aggregation.go:28:22: string `gvk` has 8 occurrences, make it a constant (goconst)
internal/metrics/aggregation_labeled.go:101:43: string `gvk` has 8 occurrences, make it a constant (goconst)
internal/metrics/metrics.go:138:12: string `gvk` has 8 occurrences, make it a constant (goconst)
internal/metrics/metrics.go:256:30: string `series` has 6 occurrences, make it a constant (goconst)
internal/metrics/metrics.go:38:10: string `kollect_inventory_items_total` has 3 occurrences, make it a constant (goconst)
internal/metrics/metrics.go:45:10: string `kollect_collect_items_total` has 3 occurrences, make it a constant (goconst)
internal/metrics/metrics.go:55:23: string `gvk` has 8 occurrences, make it a constant (goconst)
internal/metrics/metrics.go:63:12: string `controller` has 12 occurrences, make it a constant (goconst)
internal/metrics/metrics.go:71:12: string `kind` has 6 occurrences, make it a constant (goconst)
internal/metrics/metrics.go:82:12: string `sink_type` has 4 occurrences, make it a constant (goconst)
internal/metrics/metrics_catalog.go:100:24: string `group` has 8 occurrences, make it a constant (goconst)
internal/metrics/metrics_catalog.go:200:42: string `series` has 6 occurrences, make it a constant (goconst)
internal/metrics/metrics_catalog.go:20:15: string `kollect_inventory_items_total` has 3 occurrences, make it a constant (goconst)
internal/metrics/metrics_catalog.go:21:15: string `gauge` has 9 occurrences, make it a constant (goconst)
internal/metrics/metrics_catalog.go:27:15: string `kollect_collect_items_total` has 3 occurrences, make it a constant (goconst)
internal/metrics/metrics_catalog.go:36:35: string `gvk` has 8 occurrences, make it a constant (goconst)
internal/metrics/metrics_catalog.go:43:15: string `counter` has 17 occurrences, make it a constant (goconst)
internal/metrics/metrics_catalog.go:44:24: string `controller` has 12 occurrences, make it a constant (goconst)
internal/metrics/metrics_catalog.go:52:24: string `kind` has 6 occurrences, make it a constant (goconst)
internal/metrics/metrics_catalog.go:67:15: string `histogram` has 3 occurrences, make it a constant (goconst)
internal/metrics/metrics_catalog.go:68:24: string `sink_type` has 4 occurrences, make it a constant (goconst)
internal/pipeline/init_sampling.go:254:15: string `string` has 5 occurrences, make it a constant (goconst)
internal/pipeline/init_wizard.go:583:49: string `string` has 5 occurrences, make it a constant (goconst)
internal/sink/bigquery/backend.go:187:11: string `cluster` has 4 occurrences, make it a constant (goconst)
internal/sink/bigquery/backend.go:190:11: string `exported_at` has 4 occurrences, make it a constant (goconst)
internal/sink/git/delete.go:668:20: string `origin` has 6 occurrences, make it a constant (goconst)
internal/sink/git/exec_git.go:343:28: string `origin` has 6 occurrences, make it a constant (goconst)
internal/sink/git/export.go:566:9: string `origin` has 6 occurrences, make it a constant (goconst)
internal/sink/git/mirror.go:187:16: string `origin` has 6 occurrences, make it a constant (goconst)
internal/sink/git/remote_branch.go:50:9: string `origin` has 6 occurrences, make it a constant (goconst)
internal/sink/git/sync_remote.go:45:18: string `origin` has 6 occurrences, make it a constant (goconst)
internal/sink/gitlab/client_test.go:135:16: G710: Open redirect via taint analysis (gosec)
internal/sink/mongodb/backend.go:201:9: string `inventory_namespace` has 3 occurrences, make it a constant (goconst)
internal/sink/mongodb/backend.go:202:9: string `inventory_name` has 3 occurrences, make it a constant (goconst)
internal/sink/mongodb/backend.go:203:9: string `target_name` has 3 occurrences, make it a constant (goconst)
internal/sink/mongodb/backend.go:204:9: string `source_uid` has 3 occurrences, make it a constant (goconst)
internal/sink/mongodb/export_plan.go:30:3: string `inventory_namespace` has 3 occurrences, make it a constant (goconst)
internal/sink/mongodb/export_plan.go:31:3: string `inventory_name` has 3 occurrences, make it a constant (goconst)
internal/sink/preview/render.go:46:24: string `team-a` has 4 occurrences, make it a constant (goconst)
internal/sink/preview/render.go:47:24: string `api` has 3 occurrences, make it a constant (goconst)
```

Shape of the 57: 44 × goconst "string has N occurrences, make it a constant" (repeated
literals — metric label values `gvk`/`controller`/`kind`/`sink_type`, git remote `origin`,
mongodb column names, catalog type names); 12 × staticcheck SA1019
`(sigs.k8s.io/controller-runtime.Result).Requeue is deprecated: Use RequeueAfter instead`; 1 ×
gosec G710 "Open redirect via taint analysis" in a gitlab client **test**
(`internal/sink/gitlab/client_test.go:135`).

## 6. `task format:check` under v2.13.1

**exit 0** — no formatter drift. Raw output:

```
task: [install] go mod download
task: [install] go mod verify
all modules verified
task: [format:check] test -z "$(gofmt -l . | grep -v '^$' || true)"
task: [format:check] make golangci-lint
Downloading github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1
Building custom golangci-lint with plugins...
task: [format:check] test -z "$(bin/golangci-lint fmt --diff 2>/dev/null | grep -v '^$' || true)"
```

(The `make golangci-lint` inside format:check re-ran the install + custom build — ~40 s, module
cache warm — because the probe had left `bin/golangci-lint` as a plain file rather than a
symlink, so the freshness check fell through to a rebuild. Final binary verified unchanged in
kind: still `v2.13.1-custom-gcl…`, built 2026-10-06 22:34:00 UTC.) Note: this task's raw exit
is trusted per the round-2 finding 8 — the `2>/dev/null` inside the task's third line swallows
stderr, but the recorded exit code 0 and the absence of the `fmt --diff` output are the
evidence; a drift would have made the `test -z` fail.

## 7. Executed binary version

`bin/golangci-lint version` →
`golangci-lint has version v2.13.1-custom-gcl-47DEQpj8HBSaTImW5JCeuQeRkm5NMpJWZG3hSuFU built
with go1.26.6 from ? on 2026-10-06 22:28:59.65155 +0000 UTC` (post-format:check rebuild stamps
22:34:00 UTC, same version string). Version carries `-custom-gcl-<hash>` — a custom build is
distinguishable from vanilla in the version string, but a vanilla binary still fails the config
at run start (§4), which is the harder proof.

## 8. logtools plugin resolution

- `.custom-gcl.yml` plugin block at probe time: `version: latest` (floats; LTB-4 — pinned in
  task 1.2 using this record).
- `go list -m sigs.k8s.io/logtools@latest` run **before** the build and **after** it:
  `sigs.k8s.io/logtools v0.10.1` both times (verified). The plugin build uses the same
  `@latest` resolution; the bracketing reads agree within the build window, so the build used
  **v0.10.1** (*believed* for the build itself — the `golangci-lint custom` temp module is
  deleted after the build and prints no resolved version; the two bracketing resolutions plus
  the same module cache make v0.10.1 the only consistent value).
- Task 1.2 should pin `version: v0.10.1`.

## 9. Revert

`git checkout -- Makefile hack/tooling/.custom-gcl.yml` — working tree back to clean except new
evidence files (`git status`: only `?? openspec/changes/golangci-lint-bump/evidence/`;
`git diff 7825750b -- Makefile hack/tooling/.custom-gcl.yml .golangci.yaml` → empty, exit 0).
Binaries left in `bin/` (gitignored): `golangci-lint` (custom v2.13.1),
`golangci-lint-v2.13.1` (vanilla). The v2.11.4 binaries were removed by the probe's `rm` step,
per the task procedure; the make version-suffix scheme rebuilds them on demand.

## Probe headline (for the review record, LTB-3/LTB-4)

| Measure | Value |
|---|---|
| Findings before (v2.11.4, `task lint`) | 0 issues (exit 0; attempt 1 timed out internally, attempt 2 clean) |
| Findings after (v2.13.1 custom, `task lint`) | 57 (goconst 44 · gosec 1 · staticcheck 12), `make lint` exit 1 |
| Formatter drift (v2.13.1) | none — `task format:check` exit 0 |
| `bin/golangci-lint version` | `v2.13.1-custom-gcl-…` (matches the pin; custom build proven also by §4) |
| logtools resolved | `v0.10.1` (latest today = v0.10.1; pin target for task 1.2) |
| Vanilla v2.13.1 | `config verify` exit 0; `run` exit 3 `plugin(logcheck): plugin "logcheck" not found` |
