# R spec-set review, round 2 (last): verification

Revision reviewed: `a0ac1aefe`. Fresh legs: reviewer-a (spec lens, cold brief
`brief-reviewer-a.md`) → BLOCK, with WARNINGs only; reviewer-b (spec lens, fanout.sh
`--no-unify`) → CONCERNS. No CRITICAL. Each finding was checked against the tree by the
orchestrator on 2026-10-19; all verified ones are fixed, so no further round runs.

| # | Finding | Sev (leg) | Verified | Disposition |
| --- | --- | --- | --- | --- |
| A1 / B5 | Store fence keyed on the entry's key fails after the acquirer's own retire, and treating "absent" as permission lets an older builder re-populate; fence/share order unstated | WARNING (a), NOTE (b) | yes: design step 4 as written | fixed: per-sink generation counter that survives removal, share-then-fence order (design step 4); tests `_slowBuilderAfterRetireNotPooled`, `_sameKeyBuildersShare`; SCR-3 text |
| A2 / B1 | Cleanup paths not wired to the uncached reader (`sink_cleanup.go:145-159`, nested request in `cleanup.go:369-379`) | WARNING (both) | yes | fixed: reader threaded through `cleanupSinkExports` and `pruneRelationalExport`; reconcilers default the reader in `SetupWithManager`; SCR-1 cleanup scenario; test rows and `TestInventoryReconcilers_defaultSecretReader` |
| A3 | Postgres column-default assertion fails on the `ON CONFLICT` update path | WARNING (a) | yes: `postgres/bulk_export.go:67-69`, `:133-135` | fixed: second export adds a new source UID (insert path); observer connection for `pg_stat_activity` |
| A4 | Several 2.1 reds already green on the G1 base; `_rotateBack` exposure misnamed | WARNING (a) | yes | fixed: marked *control* (restore, cancelled read, Git cleanup); new red `_observedChangeRetiresOnFailedRead`; `_rotateBack` exposure reworded; `_sameDataNewVersionRebuilds` pins metadata identity |
| A5 | Metadata GET still carries annotations (`last-applied-configuration` holds Secret data) | WARNING (a) | yes (kubectl apply behaviour) | fixed: SCR-4 scoped to `data`/`stringData` plus "keeps only four fields"; design *Secret data lifetime*; sentinel-in-annotation test |
| B2 | SCR-2 vs SCR-3 contradict on a failed full read after an observed change | WARNING (b) | yes | fixed: an observed change retires; only a failed metadata read keeps the entry (SCR-2, SCR-3, design retire rules, new scenario and test) |
| B3 | Troubleshooting edit for terminal Git auth would be false (no requeue) | WARNING (b) | yes: `kollectinventory_controller.go:262-264` | fixed: task 4.1 keeps the re-annotate step for terminal errors |
| B4 | SCR-3 lists TTL as an at-once removal; design skips held entries | NOTE (b) | yes | fixed: SCR-3 says the TTL never removes a held entry |
| B6 | "Registry's stream types" do not exist | NOTE (b) | yes: `registry.go:44-58` | fixed: `streamSinkTypes` set with a drift test |
| B7 | `DefaultSecretNamespace` fallback misattributed | NOTE (b) | yes: `build_context.go:34-36` | fixed |
| B8 | Empty `Name` refs and duplicate `spec.secretRef` for Kafka/NATS | NOTE (b) | yes: `credentials.go:33-35`, `build_context.go:138-141`, `:189-192` | fixed: skipped / listed once (design) |
| B9 | CLI message parity when a config-dir Secret has no namespace | NOTE (b) | yes: `wire.go:132-141` | fixed: message form stated; CLI test row |
| B10 | Emptied CA Secret changes from terminal to retryable; not in compatibility table | NOTE (b) | yes | fixed: compatibility row |
| B11 | 10 s read timeout in design only | NOTE (b) | yes | kept as a design detail (no requirement needed; it bounds an acquire, not a user-visible rule) |

No CRITICAL remains.
