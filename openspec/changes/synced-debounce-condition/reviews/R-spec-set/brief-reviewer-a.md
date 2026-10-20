# Cold review brief: spec set of OpenSpec change `synced-debounce-condition` (kollect)

You are an independent reviewer with read-only access to this repository (cwd is the repo root,
branch `feat/synced-debounce-condition`). You have no network. Read the files; do not guess.
Change nothing. Do not name yourself, your model or its vendor in the report. Module sources are
in the Go module cache (`go env GOMODCACHE`), for example `k8s.io/apimachinery` and
`sigs.k8s.io/controller-runtime` at the versions in `go.mod`.

## Context

kollect is a Go Kubernetes operator that exports an inventory to one or more sinks. Each sink
binding has an export interval; the per-sink coalesce tracker
(`internal/controller/per_sink_export.go`) skips ("debounces") a sink whose payload is unchanged
since its last successful export while the interval has not elapsed. On a kind quick start, a
healthy `KollectInventory` with one Git sink and a 15 s interval reported
`Synced=False reason=PartiallySynced "1 sink(s) debounced"` most of the time, while
`docs/reference/conditions.md:10` defines `Synced` as "the latest observed inventory was exported
successfully". The operator wants this fixed before the next release. A decision (both
independent legs chose option A) settled the shape: per-sink `Synced=True reason=Debounced`,
aggregate `True/Exported` with one message for export and debounce passes, no new reason or
condition type.

The change under review is `openspec/changes/synced-debounce-condition/`: `proposal.md`,
`design.md`, `specs/inventory-sync-status/spec.md`, `tasks.md` (ignore `loop.md`). Repo rules:
`AGENTS.md`, `docs/development/spec-workflow.md`, `openspec/config.yaml` (red-first TDD; a test
task records a red that fails on the target assertion; test and implementation land in one
commit; evidence names the test that ran). Reconcilers:
`internal/controller/kollectinventory_controller.go`,
`internal/controller/kollectclusterinventory_controller.go`; read API
`internal/inventory/status.go`.

## Lens: spec

For each requirement (SDC-1 to SDC-7): does the design deliver it, and would executing `tasks.md`
as written prove it? Verify against the code, never against a document claiming it.

Look specifically for:

1. Whether "debounced" really implies "no pending change" on every path: `shouldSkip`, the
   preview path `previewAllSinksDebounced`, the cluster reconciler, snapshot sinks' partition
   checksum, generation and `requestedAt`, a pod restart, a sink CR deleted and recreated, a
   sink spec change that does not bump the inventory generation, scope floors. Any case where a
   sink would now read `Synced=True` while its backend does not hold the current payload.
2. Whether the proposal's root-cause claims are right: the True/False flip per interval, the
   15 s cadence, the self-re-enqueue on status update, and the claim that `status.sinkExports[]`
   is rebuilt from empty entries each pass (losing `lastExportTime`, `lastChecksum` and moving
   `lastTransitionTime`).
3. The no-churn requirement SDC-5: can the design really reach "no status write on a repeated
   debounced reconcile" (every field of the status, `metav1.Time` precision, map or slice order,
   `observedGeneration`, `lastExportTime`, other conditions such as `SinkReachable`, `Ready`,
   `ExportShardWarn`), and does the API server skip a no-op status update?
4. The carry-over and baseline-reset designs: correctness, concurrency (per-sink jobs run in
   goroutines), interaction with `lastExportPaths` and the deletion cleanup that reads them,
   tracker pruning, and the failure branches that the design names.
5. Consumers of the changed reasons and messages: tests, docs, read API, metrics, Events, printer
   columns, e2e scripts, Helm chart, anything that parses "exported to N sink(s)". Which ones the
   proposal missed.
6. Whether each test case can fail for the reason it names, whether the planned reds are real on
   the stated base (not compile errors), whether the envtest approach (direct `Reconcile` calls,
   recording registry, pool off) can observe `resourceVersion` and Events as the cases say, and
   whether any case is flaky by construction (time, second precision, goroutines).
7. Missing or misordered tasks, docs that will be wrong after the change, scope creep, and
   anything needing an operator decision.
8. Stale `file:line` claims: re-check every `path:line` cited in the four files against the
   current tree and report each that is wrong.

## Output, and nothing else (at most 60 lines, plain markdown)

## Verdict: BLOCK | CONCERNS | CLEAN
## Findings
- [CRITICAL|WARNING|NOTE] <one-line claim> — `path:line`
  Failure: <concrete input/state → wrong outcome>
  Fix: <smallest change that removes it>
  Confidence: <0-100>
## Could not check
- <what you did not read or run>
