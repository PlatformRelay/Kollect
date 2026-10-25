# Stage B round 2 (last) — verification

Reviewed head `5213abae0` (on `origin/main` @ `70d62b013`, which brings the Go 1.27.3 toolchain
bump for GO-2026-6809). Legs: reviewer-a (adversarial, brief
`brief-reviewer-a.md`), reviewer-b (spec conformance, SDC-1..7). Verdicts: reviewer-a APPROVE,
reviewer-b CLEAN. No CRITICAL or WARNING. Checked by the orchestrator on 2026-10-25.

Commit ids: the footer reword of the G1 commit also moved the branch onto the newer `origin/main`.
Round-1 records name the old ids; the map is: `df2c9e501` → `9b98e11c5` (G4), `4c5cc3786` → `5b77df492` (G3), `05bc2c6e0` →
`1b35a7db6` (G2), `aa39da6c5` → `b74f2c678` (G1), round-1 fix `5f99ebcaf` → `5213abae0`. The
branch's own changes are unchanged except the G1 commit message.

## Per-commit gate on the rewritten range

Each commit in a temp worktree: `go build ./...` and
`go test -count=1 ./internal/controller/ ./internal/inventory/` (envtest assets from
`KUBEBUILDER_ASSETS`), worktree removed after each. Verified 2026-10-25.

| Commit | Build | Tests |
|---|---|---|
| `6103ddd7f` docs(spec): add the change | 0 | 0 |
| `e2d640218` docs(spec): loop state | 0 | 0 |
| `529520788` docs(spec): round-1 spec review | 0 | 0 |
| `7497147e5` docs(spec): round-2 spec review | 0 | 0 |
| `809718a08` docs(spec): T0.1 probe | 0 | 0 |
| `d95e7645d` docs(spec): preview fixture test change | 0 | 0 |
| `b74f2c678` fix(controller)!: G1 | 0 | 0 |
| `1b35a7db6` fix(controller): G2 | 0 | 0 |
| `5b77df492` fix(controller): G3 | 0 | 0 |
| `9b98e11c5` docs: G4 | 0 | 0 |
| `5213abae0` fix(controller): B1 | 0 | 0 |

## Findings

| # | Leg | Sev | Finding | Disposition |
|---|---|---|---|---|
| A1 | reviewer-a | NOTE | Under repeated status conflicts each retry re-exports the unchanged payload; design.md should say "per failed write" | fix (wording): design.md and an upgrade-guide line (event sinks receive the payload again). Made after round 2, not reviewed again |
| B1 | reviewer-b | NOTE | The SDC-5 status-write test calls `exportToSinks`/`updateStatus` directly, not `Reconcile`; the preview path is covered only because `shouldSkip` returns false on a forgotten state | accept as is: the preview and full paths share `shouldSkip` (`per_sink_export.go:100`), so the forgotten state cannot be debounced by either |
| B2 | reviewer-b | NOTE | Re-send to event sinks after a lost status write is not in the upgrade note | fix: same upgrade-guide line as A1 |
| B3 | reviewer-b | NOTE | Cluster `ExportSucceeded` on an all-debounced pass says "exported N item(s)" | reject: SDC-2 allows it, and the namespaced `Ready` message has had the same wording since before this change |
| B4 | reviewer-b | NOTE | Round-1 dispositions: all hold; B5 still deferred (SINK-KEY-SHARED-HELPER) | none |

## After round 2 (wording only, not reviewed again)

`design.md` (re-export per failed status write, conflict loop) and one upgrade-guide line (the
next reconcile exports again after a failed status write; event sinks receive the payload again).
Gates on the working tree, 2026-10-25: `task lint:markdown` exit 0 (0 issues, `upgrading.md`
in the linted list); the 14 `hack/test/docs_*.sh` guards exit 0; `bash hack/docs/verify.sh` with
`MISE_KUBECTL_VERSION=1.36.1` and a fresh venv from `docs/requirements-docs.txt` outside the
repository exit 0 (`task docs:verify` itself stopped on the host venv's missing `redirects`
plugin); `rg -n -i 'debounc' docs | rg 'Synced=False|PartiallySynced'` lists only
`upgrading.md:429` and `adr/0413-export-interval-scheduling.md:80`.
