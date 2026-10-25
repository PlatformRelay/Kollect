Target read. Now the surrounding context — the code the spec claims is dead, the repo conventions, and the cited review report — plus I'll re-run the spec's own zero-caller probes.
Now I'll re-run the spec's own zero-caller probes myself, plus check the repo conventions and cited files. Note: the `reviews/` records under this change are earlier review records — I won't read them, to keep this review independent.
Probes largely confirm the spec's claims so far. Now the deeper context: store shard lifecycle (the deleted version-monotonicity test), auth cache semantics, git wrapper, engine binding, and repo conventions.
Key claims verified so far. Now the deeper checks: which `RunExportEnvelope` tests exist today (is deleting the 10 `RunExportItems` tests safe?), the breaker test migration, engine binding internals, git wrapper internals, and the repo's own gates.
Test-name analysis raises two real candidates: the "only unreachable path" claim for the 10 deleted `RunExportItems` tests, and the unmigrated alias self-tests. Checking both precisely, plus auth usage, the production binding path, repo gates, and the round-2 diff.
Nearly there — three final checks to make the findings concrete: does any surviving test assert the relational `SupportsDelete` routing, the NaN marshal-failure contract, and the skip branch?
All evidence gathered; probes re-run repo-wide, claims cross-checked against the actual code. Writing the report.

## Verdict: CONCERNS

One WARNING on the load-bearing "no reachable coverage lost" assumption (verified false in two places); everything else survived attack-defend-revise. I re-ran every zero-caller probe repo-wide (`rg` over all `*.go`), confirmed all 8 deletion targets are genuinely zero-production-caller at this HEAD, confirmed `RemoveTarget`/`Remove` never delete shards (store.go:194-203,263-277 — the DR-4 rationale holds), confirmed the production `RegisterTarget` synthetic-object shape (kollectclustertarget_controller.go:214-235), `export.go:267` for `exportThroughBreaker`, build tags on both git integration files, `git.Export`'s wrapper derivation (export.go:38-45), and that `task test/lint/coverage/spec:validate` exist (Taskfile.yml:147,176,201,206). Auth-cache finding: dropping `user` is sound — cache key is token-hash-scoped (auth_cache.go:72-73), the cached value is discarded (`_ = user`, auth.go:122), decision enforcement unchanged; security pass clean.

## Findings

- [WARNING] Task 1.3's deletion premise is false: two deleted `TestRunExportItems_*` tests assert reachable behaviour, and no task migrates them — `openspec/changes/dead-exported-surface/proposal.md:90-92`, `tasks.md:14`
  Failure: (a) `TestRunExportItems_exportsSnapshot` (`export_test.go:120-167`) is the only test in `internal/sink` asserting the git-sink document happy path — resolved `inventory/team-a/platform.yaml` + YAML payload (`stub.lastPath`/`lastBody`; repo-wide grep: no other sink or controller test uses these recorders) — a reachable ADR-0419 contract via `resolveSnapshotExport` (export.go:251); (b) `TestRunExportItems_marshalFailureIsTerminal` (`export_test.go:445-461`) is the only NaN fail-closed test of `MarshalEnvelope`, which production callers reach (kollectclusterinventory_controller.go:331, cleanup.go:352, partition.go:59) — no equivalent in `internal/export` tests. Deleting both with no migration instruction, justified by an absolute claim that is demonstrably wrong; the DR-4b review row runs after deletion and its Check scope is ambiguous.
  Fix: amend proposal.md:90-92 to enumerate the exceptions; extend task 1.3 to migrate (a) as a `RunExportEnvelope`-based test (as the breaker tests already are) and (b) as a direct `internal/export` NaN test; require the DR-4b reviewer to check per-test equivalence, not just the store deletions.
  Confidence: 85

- [NOTE] "6 production references" for `RunExportEnvelope` is not reproducible — `proposal.md:92`
  Failure: repo-wide count is 4 call sites (export.go:132 is the dead one deleted in task 1.2; surviving: kollectclusterinventory_controller.go:331, kollectinventory_controller.go:404, cleanup.go:364) or 10 word-match lines — neither is 6; a reviewer who tries the count doubts the rest of the evidence chain.
  Fix: restate as "3 surviving call sites" or record the actual grep output.
  Confidence: 90

- [NOTE] DR-1…DR-10 requirement labels are defined nowhere in the change — `tasks.md:10,53`
  Failure: proposal.md has no requirements section (`.openspec.yaml` `skip_specs: true`, no specs/ deltas); the Verification table's Req column and `DR-4b` are unresolvable for a fresh reader.
  Fix: one line in the proposal mapping DR-1…DR-10 to the eight bullet groups, or drop the labels from tasks.md.
  Confidence: 95

- [NOTE] The Why section's evidence source does not exist in the repo — `proposal.md:5,74`
  Failure: `data/kollect-xconsol-final/report.md` (§4 Sweep 2, §7 item 4) — no `data/` directory at root; a reader checking the cited sweep counts hits a dead path.
  Fix: cite it as an out-of-repo workbench record or inline the counts the tasks already carry.
  Confidence: 90

- [NOTE] DR-6 deletes the only direct flag assertion for `RelationalStore()` (SupportsDelete) without folding it into `cap` — `tasks.md:39`
  Failure: a future edit flipping `cap.RelationalStore()` (`capabilities.go:41`) off SupportsDelete would silently re-route relational cleanup (cleanup.go:197) — BUT this is transitively pinned by `internal/sink/postgres/capabilities_test.go:20-24` and bigquery's, so risk is low; the other three constructors are covered by `cap/capabilities_test.go:8-20`.
  Fix: while deleting the four self-tests, move the `RelationalStore()` constructor assertion into `TestCapabilityConstructors`.
  Confidence: 80

- [NOTE] DR-7's escape hatch "(or the narrowest same-package equivalent)" does not exist for the 8 controller-package test files — `tasks.md:45`
  Failure: `internal/controller` tests are cross-package to `internal/collect`; the only remaining exported writer is `RegisterTarget`, which needs profile + `RegisterTargetOptions` scaffolding per the production shape — feasible (collect unit tests do it), but 14 sites risk 14 hand-rolled seeds or silently dropped `NamespacesForClusterTarget` reader coverage (task 6.3 then fails late).
  Fix: task 6.2 should name one shared test helper (synthetic target + profile + ceiling) used by all 14 sites.
  Confidence: 65

## Could not check

- Nothing was executed: `go build/vet/test`, `task test/lint/coverage/spec:validate` unrun (read-only review, pre-implementation) — the gates' feasibility (envtest availability, 90% floor on the final tree) is asserted by the spec, not verified.
- The cited consolidated report and PR 451 (PlatformRelay/Kollect) — absent from / outside this repo; out of bounds.
- External consumers of `api/v1alpha1` constants — the compile cannot prove this half; the spec records it as an accepted limit.
- Whether some controller-level test asserts the YAML document export through a differently-named recorder (my greps covered `lastPath`/`lastBody` and `RunExportEnvelope` in tests only) — residual on the WARNING's confidence, not its existence.
- Round-1 review records under `reviews/` deliberately not read, to keep this leg independent.
