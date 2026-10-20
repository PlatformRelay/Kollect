# R spec-set review, round 2 (last): verification

Revision reviewed: `138dada5c`. Fresh legs: reviewer-a (spec lens, cold brief
`brief-reviewer-a.md`) → BLOCK; reviewer-b (spec lens, fanout.sh `--no-unify`) → CONCERNS. Each
finding below was checked against the tree by the orchestrator on 2026-10-20. This is the last
round; verified findings are fixed without a further review.

| # | Finding | Sev (leg) | Verified | Disposition |
| --- | --- | --- | --- | --- |
| A1 | An inventory deleted and recreated under the same name (generation 1 again) is debounced against a backend its cleanup emptied | CRITICAL (a) | yes: tracker keyed by `namespace/name`, no `sinkCoalesce` use outside the export paths, the deletion cleanup does not call `record` or clear the entry | accepted: inventory UID added to the baseline (SDC-4, new scenario; design); G3 envtest "re-exports to a recreated inventory" (both kinds), mutation M3 |
| A2 / B6 | A Secret that repoints the sink (for example a Postgres DSN) changes the destination without changing sink UID or generation | CRITICAL (a), NOTE (b) | yes: the destination can come from the Secret (`internal/sink/postgres/config.go:65`); the skip runs before the backend pool's Secret check | rejected as a fix, accepted as wording: tracking Secret versions in the debounce would make a Secret change trigger an export, which the recorded decision 2026-10-19 `rotation-zero-interval` (option B) and the `sink-secret-rotation` capability rule out for `0s`. SDC-4 retitled "export inputs are unchanged" and names what it does not detect; proposal non-goal. Downgraded to a scope note, no CRITICAL remains |
| A3 / B2 | Read API still falls back to the inventory time for a matched entry with no `lastExportTime`; existing test row `cov_s06_test.go:124` encodes that | WARNING (a, b) | yes: `internal/inventory/status.go:147-166` | fixed: SDC-6 (empty time; new scenario), design, G1 test row and mutation M3, proposal *Test changes* |
| A4 / B5 | Only the job-failure `forget` is tested; namespaced snapshot (full path) no-churn untested | WARNING (a), NOTE (b) | yes: snapshot sinks never short-circuit in the preview (`kollectinventory_controller.go:360-363` vs `:498`) | fixed: one `failSinkExport` helper for every failure branch with a unit test, an `rg` check and mutation M4; snapshot row on the namespaced no-write envtest |
| A5 / B1 | The doc regex matches `docs/REQUIREMENTS.md:89` and cannot cross table cells | WARNING (a, b) | yes | fixed: named edits per file and line, plus a per-line check with FR-EXP-9 reviewed and allowed |
| A6 | Task 5.1 omits the archive-commit re-review and final CI | NOTE (a) | yes: `docs/development/spec-workflow.md` *Land* | fixed in 5.1 |
| B3 | G1 would drop T1/C1 on total failure until G2's carry-over | NOTE (b) | yes | fixed: the total-failure per-sink write and its test move to G2 |
| B4 | `TestCarryOverLastExportPaths` and direct `shouldSkip`/`record`/`nextDue` callers change | NOTE (b) | yes: `per_sink_export_test.go:386-416` | fixed: *Test changes* rows in 2.1 and 3.1, proposal *Impact* |
| B7 | Reds real on their bases; no-churn inputs stable; citations current | NOTE (b) | — | no action |

No CRITICAL remains: A1 is fixed in the spec and test plan, A2 is out of scope by a recorded
decision and stated as such.
