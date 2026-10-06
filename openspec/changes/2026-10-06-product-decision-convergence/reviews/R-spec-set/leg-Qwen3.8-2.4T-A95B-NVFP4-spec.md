Now verifying the cited code facts. Reading the key files:
## Verdict: CONCERNS

## Findings

- [WARNING] Engine convergence makes four doc sites false that neither GTE-3 nor any task fixes — `charts/kollect/README.md.gotmpl:16`, `docs/operator-manual/index.md:131-143`, `docs/development/coding-standards.md:77`, `Dockerfile.pipeline:33-38`
  Failure: T09 lands (cli rejected everywhere); `charts/kollect/README.md:16` still advertises `spec.git.engine: cli`, `index.md:137` still has a `cli` engine table row, and `task helm-docs:verify` stays green because the claim lives in the template — the change ships documentation it itself falsified. Proposal defers "doc rot" to Sweep 1, but this rot is manufactured here, not pre-existing.
  Fix: widen GTE-3's SHALL to every doc naming `engine: cli`, and add a T09 bullet: edit README.md.gotmpl, operator-manual/index.md, coding-standards.md, Dockerfile.pipeline comment.
  Confidence: 90

- [WARNING] Proposal capability IDs do not match the deltas: promises ERA-1..3, TSP-1..2, BEP-1..3, GTE-1..4 (12) but specs define 8 — `proposal.md:52-55` vs `specs/*/spec.md`
  Failure: auditor of the archived change looks for ERA-3/TSP-2/BEP-3/GTE-4 and finds nothing; tasks.md's Verification table (correctly) covers only the 8 real IDs, so proposal and tasks disagree about scope. `openspec validate --strict` will not catch prose IDs.
  Fix: renumber the Capabilities section to ERA-1..2, TSP-1, BEP-1..2, GTE-1..3 (or add the missing requirements if the draft dropped one).
  Confidence: 95

- [WARNING] TSP-1 escape hatch interacts with the cluster path's existing filter-status hatch; D3/T07 never merge them — `internal/controller/kollectclustertarget_controller.go:328-341`, `design.md:47-50`
  Failure: reconcile where count and filter status both move while the Ready condition is byte-identical: a naive mirror of the namespaced `countChanged && !written` beside `persistFilterStatusIfSkipped` issues two sequential status writes (or one conflicts if it uses a re-fetched copy). No scenario covers count+filter moving together.
  Fix: D3 states one write gated by `(countChanged || filterChanged) && !written`; add a scenario to TSP-1.
  Confidence: 60

- [NOTE] BEP-1 scenario asserts "no connection outlives deletion beyond the eviction call", which T03's unit tests cannot prove (they assert entry-gone + Close-ran); an in-flight export holding the same backend gets Close mid-use — `specs/backend-pool/spec.md:17`, `internal/sink/backend_pool.go:244-253`
  Failure: sink deleted mid-export → backend Closed under a live export; self-heals (re-acquire rebuilds, sink gone), and "best-effort" covers it, but the AND clause overclaims.
  Fix: soften the clause to "eviction is the only cleanup; in-flight use is best-effort".
  Confidence: 65

- [NOTE] ADR-0803 skips 0802 with no recorded reservation; theme 08 index holds only 0801 — `docs/adr/README.md:115-119`
  Failure: a parallel change takes 0802 during review; numbering collision at merge.
  Fix: take 0802, or record the 0803 reservation in loop.md/README.
  Confidence: 70

Requirement status at HEAD (all verified against code): ERA-1/ERA-2 absent (0 grep hits for either annotation under `api/ internal/ cmd/`; debounce axes confirmed at `per_sink_export.go:54-105`, preview path calls `shouldSkip` but never `record()` so it cannot consume the trigger — D1 holds); TSP-1 absent (no fields on `kollectclustertarget_types.go`; namespaced mirror at `kollecttarget_controller.go:294-359`); BEP-1/2 absent (evict functions caller-less, `backend_pool.go:33-35`); GTE-1 contradicted at HEAD (cli accepted at `kollectsink_types.go:181`, `validation/git.go:88-96`, `git/config.go:170-178`); GTE-2 absent (8-entry pin at `ssh_auth.go:27-36`); GTE-3 contradicted (`docs/crds/kollectsnapshotsink.md:22`). All cited line numbers check out within ±4 lines. Unmentioned-by-spec work in the target: none beyond the D7 park. Executing tasks.md as written delivers the proposal except for finding 1's docs.

## Could not check
- `data/kollect-xconsol-final/report.md` §7 (the five decisions): path absent from checkout and never committed (`git log --all` empty) — the §7-honouring answer rests on the proposal's own claim (4 implemented + Helm `mode` parked, which matches the chart facts at `values.yaml:79`/`values.schema.json:38`); I could not confirm §7 itself, including whether it demanded implementing `mode` rather than parking it.
- `/tmp/fanout-r-spec/combined-spec-for-review.md`: outside repo; redundant (all four spec.md read in full in-repo).
- `reviews/` subdir of the change dir (other reviewers' legs).
- x/crypto claims verified via the Go module cache (`kex.go:409`, `common.go:54,93`, `kex.go:448`; no sntrup761 outside testdata), not in-repo; controller-runtime DeleteEvent assumption (`eventhandler.go:105-146`) not re-read, version v0.24.1 confirmed in `go.mod:42`.
- No execution: tests, codegen, and `task verify` not run (read-only review); Docker-tier rows stay CI-owned per tasks.md.
