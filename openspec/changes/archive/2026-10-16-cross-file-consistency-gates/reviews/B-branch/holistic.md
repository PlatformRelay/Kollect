# Stage B — holistic fitness on the tip

Tip `1578f3a79` (feat/cross-file-consistency-gates), base `0c1456b5d` (origin/main). Run 2026-10-16,
`umask 022`, on the shared host (load ~6 on 16 cores, other repos' gates running in parallel).

| Command | Exit | Key line | Time |
|---|---|---|---|
| `task check` | 201 (task: exit status 1) | `task check: FAILED -- 2 gate(s) failed: lint:markdown guard hack/test/demo_04_samples_kustomize_test.sh`; the other 16 gates green, incl. format:check, lint, vulncheck, test, verify, every consistency_* guard plain and `--self-test` | 1279 s |
| `task coverage` (tip) | 0 | `internal/ coverage 91.3% (floor 90%)` | 63 s (test cache warm from `task check`) |
| `task coverage` (base, detached worktree `b-base-0c1456b`, removed after) | 0 | `internal/ coverage 91.3% (floor 90%)` | 257 s incl. `make setup-envtest` |
| `task vulncheck` | 0 | `Your code is affected by 0 vulnerabilities` (1 vuln in a required module, not called) | 9 s |

## The two red gates

1. **lint:markdown — branch-caused, real.** `Summary: 31 issues in 8 files`, all in files this branch
   adds: `openspec/changes/cross-file-consistency-gates/{loop.md (9), task-prompt.md (4),
   evidence/G1.md (2), G2.md (1), G3.md (4), G4.md (5), G5.md (3), G6.md (3)}` — MD032
   blanks-around-lists and MD038 no-space-in-code. `task lint:markdown` runs in
   `.github/workflows/preflight.yaml:50`, and `preflight` is a protect-main required context, so the
   PR would be red. The ignore list (`hack/tooling/markdownlint-cli2.yaml:53`) exempts only
   `openspec/changes/**/reviews/**`. Not a base artefact: none of these files exist on base.
2. **guard hack/test/demo_04_samples_kustomize_test.sh — ENVIRONMENT.** `mise ERROR No version is
   set for shim: kubectl` in this worktree. Re-run with mise's kubectl 1.37.0 on PATH: green
   (`demo-04 samples kustomize: ok (2 tree(s) via kubectl kustomize)`). The branch touches neither
   the guard nor `config/samples/`.

`format:check` and `docs:verify` caveats (gitignored references/, untracked ui/) did not apply:
format:check was green here.

Coverage: base 91.3% = tip 91.3% (the Go 1.26.9 → 1.27.2 bump moved nothing).

## Verification of the review legs' CRITICAL / WARNING findings (orchestrator, against the code)

Legs: `round-1/leg-astra-adversarial.md` (BLOCK), `round-1/leg-claude-opus-spec.md` (CONCERNS).

| id | Sev (leg) | Finding | Verified? | Evidence |
|---|---|---|---|---|
| H1 | CRITICAL (holistic) | lint:markdown red on 8 committed openspec files; `preflight` is required | yes | see above; `preflight.yaml:50` |
| A1 | CRITICAL (astra) | `ARG B=golang:1.99.0` + `FROM ${B}` escapes CFC-1 | yes, as fail-open; severity WARNING (no such pattern in the tree, needs a hand edit) | probe: copy of HEAD + `Dockerfile.evil` with that pair → guard prints `ok - go.mod, every golang image ... agree`, exit 0. Parser at `consistency_go_toolchain_test.sh:86-90` takes the first non-flag word and only reacts to a `golang` prefix |
| A2 | CRITICAL (astra) | metrics gate counts any `kollect_*` literal as registered; dropping a metric from `MustRegister` stays green | yes; spec-literal holds (CFC-4 says "registered in the non-test source"), invariant does not; severity WARNING | probe: deleted `ReconcileInFlight,` (`internal/metrics/metrics.go:315`) in a copy → guard `ok`, package builds; `consistency_metrics_test.sh:139` collects every literal; no Go test gathers the registry (`metrics_catalog_test.go` compares the catalog with the hand mirror only) |
| A3 | CRITICAL (astra) | CFC-2 builds only the three analysis tools; other pins are checked by go directive metadata only | yes; matches the spec's stated mechanism (`go mod download -json`), so spec-partial; severity WARNING | `consistency_tool_pins_test.sh:348-374` (metadata) vs `:385-401` (smoke only when `analysis_tool` names it). Other pins: kustomize, controller-gen, setup-envtest (Makefile go-install-tool) — these are compiled by the required `verify`/`test` jobs under setup-go's go.mod Go, so a build break surfaces there |
| A4 | WARNING (astra) | go_toolchain self-test leaks every scratch copy | yes | `new_copy` called as `$(new_copy)` at `consistency_go_toolchain_test.sh:328,342,441` loses `SCRATCH` (`:317`); probe with private TMPDIR: 31 dirs, 6.5 MB left after one `--self-test`. The rbac/metrics guards already fixed this pattern (`consistency_rbac_test.sh:282`, `consistency_metrics_test.sh:256`) |
| O1 | WARNING (opus) | `tool-pins-selftest` not in protect-main | yes | `gh api .../rulesets/18802410`: contexts = preflight, test, kind-smoke, Analyze (Go), lint, workflow-security, dependency-review. Already the loop's post-merge owner row (adding it before the job exists on main would block every other PR) |
| O2 | WARNING (opus) | required `lint` now needs the network for the tool-pins step | yes, factual (`ci.yaml:569-570`); marginal: `task lint` in the same job already builds the custom golangci-lint from the proxy, and go-cache downloads modules | — |
| O3–O5 | NOTE (opus) | rbac IndexError on `cluster-scoped` with no colon (fails closed); literal scan over-counts; SummaryOpts suffixes | not verified (NOTE) | — |

Compliance table (opus): CFC-1 holds, CFC-2 holds (smoke runs one package, not the whole module;
A3 above makes it partial), CFC-3 holds, CFC-4 holds (spec-literal; A2), CFC-5 holds, CFC-6 partial
(O1). No row contradicted or absent.
