Now let me verify the T11 fixes hold at HEAD by running the targeted tests (read-only; build cache only).
One more verification — confirm the envtest delete-wiring spec actually ran (not skipped) in the suite I executed:
## Verdict: CLEAN

Ran at HEAD `3bc38bdc` (not just read): full unit suites green for `internal/controller` (29.1s, incl. the focused envtest delete-watch spec: 1/54, SUCCESS), `internal/sink` (30.1s), `internal/sink/git` (247s), `internal/collect`, `internal/validation`, `test/schema`, `api/v1alpha1`; `go build ./...` clean.

Spec compliance (per requirement):
- **ERA-1 holds** — third axis `per_sink_export.go:43,82-84,115`; both controllers read the annotation once (`kollectinventory_controller.go:292,474`, `kollectclusterinventory_controller.go:256`); preview fed the same value (`kollectinventory_controller.go:494`), preview never records so it cannot consume the trigger; `KollectTarget` correctly excluded; doc row scoped to the two kinds (`docs/ANNOTATIONS-LABELS.md:101`).
- **ERA-2 holds** — stamped after prune+scrub from the source object (`internal/collect/prune.go:79,95-108`); generation 0 stamped; no-metadata → no stamp; Attributes mode untouched. Round-1 #1 fixed: spec wording now says "whenever the copy retains a metadata section" and names the `SpecAndStatus` default (`specs/export-annotations/spec.md:46-53`); doc states the include requirement.
- **TSP-1 holds** — fields mirror namespaced (`kollectclustertarget_types.go:41-53`), printer columns Collected/Updated/Age in both CRDs, shared helper `collected_count.go:22`, ONE write site via `filterChanged || countChanged` (`kollectclustertarget_controller.go:338,350-355`), Degraded keeps count (`setDegraded` never touches the fields).
- **BEP-1/BEP-2 hold** — UID eviction + ns/name fallback (`backend_pool.go:86-92,301-307`); tombstone discards re-store (`:186-188`); delete-only handler (`family_sink_controller.go:103-109`); TTL backstop unchanged (`:43,228-244`). Tombstone-unwrap assumption verified against controller-runtime v0.24.1 module source (`pkg/internal/source/event_handler.go:133-136`).
- **GTE-1/2/3 hold** — enum go-git-only in CRDs+golden; admission `validation/git.go:88-94`; config `git/config.go:169-171` names go-git; `GitEngineCLI`/`Config.Engine` gone; file:// + ls-remote routing intact (`git/export.go:125`, `delete.go:107`, `connection.go:159`); KEX additions appended after the existing eight (`ssh_auth.go:36-39`); no live `engine: cli` advert left (repo-wide grep; release-notes, Dockerfiles, chart README, manuals, ADRs truthed). Helm `mode` correctly untouched (D7 parked).

T11 fixes held: (a) terminal classification verified end-to-end — `git/backend.go:33` wraps, demotion sites removed (`sink/export.go:174-185`, `cleanup.go:188-192`), `ClassOf` mapping confirmed equivalent for unclassified errors (`internal/errors/errors.go:86-108`), sink condition maps to `ReasonExportTerminal` (`sink_status.go:150-152`); (b) eviction-path tombstone sweep `backend_pool.go:326` bounds the export-less-manager case; (c)/(d)/(e) verified in diff.

Deferred findings — none is a missed behaviour defect: #6 cadence is spec-silent (count was already recomputed per reconcile for the Ready prose pre-branch); #7 is the deliberate aa967f23 owning-release shape, which satisfies "discarded, not pooled"; #8 is a robustness bet, not a spec clause; #9/#10 are pre-existing gate issues. #11 (NATS evict-during-use connection leak) is a genuine residual but is tracked, not missed: T12's owner-gated list names it, and BEP-1 sanctions the failing export, not the leak.

## Findings
- [NOTE] Branch Verification table cites rev `81bea0b0` for every row, predating T11's code fixes (`3bc38bdc`); table evidence is stale though T11 evidence re-ran the full suite and my HEAD re-runs are green — `openspec/changes/2026-10-06-product-decision-convergence/tasks.md:384-396`
  Failure: none observed; a future reader trusting the table would validate the wrong SHA.
  Fix: re-stamp table rows with a post-T11 rev at T12 hand-off.
  Confidence: 90
- [NOTE] `TestNewBackend_configFaultsAreTerminal` makes ALL `ConfigFromSpec` faults terminal (TLS, pushPolicy, cloneDepth) — stronger than GTE-1's engine-only ask; safe because `ConfigFromSpec` is pure spec parsing with no transient input, but it is an observable tightening for non-git-sink error classification at the two acquire sites — `internal/sink/git/backend.go:31-34`
  Failure: no legitimate transient caller exists (verified: `ResolveSecret` rewrites NotFound to plain error, `credentials.go:44-46`); a future transient config resolution added there would silently go terminal.
  Fix: none now; keep `ConfigFromSpec` side-effect-free.
  Confidence: 70

## Could not check
- Integration tier (`task test-integration`, Docker) and `task verify`/`lint`/coverage gate — not executed here (they regenerate files / need Docker); relied on recorded evidence plus my own unit-tier runs at HEAD.
- `-race` re-run at HEAD (recorded at pre-T11 rev for most packages; T11 recorded `-race -count=2` for `internal/sink` and `internal/sink/git` at the final tree).
- Sibling knowledge-base artifacts (`data/kollect-xconsol-*`, outside this repo) — the five source decisions taken on relay only.
- Whether any snstrup761-only SSH server exists in the wild (accepted residual per proposal).
