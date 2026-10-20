## Verdict: CONCERNS

## Findings
**SCR compliance.** Codes: H = holds, P = partial, C = contradicted, A = absent, S = stronger-than-spec. Production file paths are under `internal/`.
| Req | Verdict | Deciding code | Pinning test, scenario by scenario |
|---|---|---|---|
| SCR-1 | H | `sink/backend_pool.go:190,203-228`, `sink/secret_refs.go:49-90,228`, `controller/kollectinventory_controller.go:684` | pw rotation H `_rotatedSecretRebuilds` + integ `TestPostgresExport_passwordRotation`; CA H `TestS3Export_caRotation`; uncached H `_readsSecretsUncached`; cleanup uncached H (same test, Delete-prune row; `sink/cleanup.go:194,382`); cross-ns H (envtest); all refs H `_coversEverySecretReferenceField`; rotate-back H `_rotateBack`; unchanged / unrelated H `_unchangedSecretsReuse` / `_unrelatedSecretReuse` |
| SCR-2 | P | `sink/secret_refs.go:193-200`, `sink/backend_pool.go:313-329` | deleted H `_deletedSecretFails`; emptied H `_emptiedSecretFails`; S3 ambient H `TestS3_emptiedSecretNoAmbientCredentials`; CA deleted H `_caGoneBehind…`; restore H `_restoredSecretRecovers`; observed change + failed full read H `_observedChangeRetiresOnFailedRead` (`:384`); cancelled read keeps shared backend **P** (finding 1); stream cleanup H `sink/cleanup.go:177` `_streamSinkDeletedSecretRetained`; Git release H `_deletedGitAuthSecretFails` |
| SCR-3 | P | `sink/backend_pool.go:442-476,493-519,541` | long export H `_rotationDuringExportDefersClose`; spec change H `_specChange…`; concurrent H `_concurrentAcquiresAfterRotation` (-race); failed rebuild H `_failedRebuildRetiresOld`; sink deleted H `_deleteEviction…` (tombstone `:497`); slower builder H `_slowBuilderDoesNotReplaceNewer`; TTL H `_ttlSkipsHeldEntry`; real Postgres H (integ); "started before another stored/retired SHALL NOT be pooled" **P** (finding 3) |
| SCR-4 | H | `sink/secret_refs.go:137-153`, `sink/backend_pool.go:27-42` (no data kept) | sentinels `TestRotationErrors_noSentinel` + envtest; fingerprint / pool hit `TestSecretMeta_metadataOnly` |
| SCR-5 | H | `pipeline/wire.go:205` | `TestResolveSinkSecretData_emptySecretFails`, `TestRunOneContext_secretManifestChangeNextRun` |
| SCR-6 | H (per G4.md, `rg` exit 1) | docs/* | stale sentence removed: G4 row 1 |
| SCR-7 | H, but wider than asked (finding 2) | `controller/kollectinventory_controller.go:264-266,658`, `controller/kollectclusterinventory_controller.go:226-229,719` | 0s retried H (envtest + `TestSetInventoryDegraded_zeroIntervalRequeues`); cluster H `TestClusterSetDegraded_…`; terminal: namespaced H `_zeroIntervalTerminalNoRequeue`, **cluster has no test**; not re-exported H (envtest control) |

**The failed re-read path (`observeAndLookup`):**
- **The open question from the brief: SCR-3 holds here and SCR-2 is partial.** Retiring always drains through holders, so no export is cut and no stale backend is used. But the `staleKey` retire on a failed re-read can retire a correct, shared entry.
- **The accepted residual (a forced second read retires an entry stored during it):** I agree with the claim. Holders drain the entry and the rebuild reads after the retire, so the cost is one extra build and never stale use.
- **Cleanup paths:** none reads a Secret it should not. Stream sinks return before the acquire (`sink/cleanup.go:177`), and Retain on object stores returns before it (`:188-191`).
- **Closing a backend under an export:** no path does this. The only exception is `ResetBackendPoolForTest` (`:101`), which is test-only.

- [WARNING] A failed re-read retires a fresh shared entry whenever the first read was older than that entry — `internal/sink/backend_pool.go:325-326`
  Failure: inventory A takes its read generation and reads Secret v5 (slow). Inventory B reads v6, retires B5 and stores B6, which is correct and shared. A's lookup sees a mismatch and a changed generation, so it records staleKey=B6.key (`:378`). A's context is then cancelled on the re-read, and `retireEntryWithKey` retires B6. The same happens in the not-found ordering of `_staleNotFoundDoesNotRetireNewer` when the re-read fails. The result is a spurious retire and rebuild, which contradicts SCR-2's "SHALL NOT retire" for a cancelled or timed-out read. `_failedReReadKeepsObservedChange` covers only the opposite ordering, and the fix is recorded as unreviewed (G3.md:253). The retire also buys no freshness for later acquires: each one reads metadata before a hit (`:332`), so a stale B5 left pooled would be retired by the next acquire anyway.
  Fix: drop the `staleKey` retire, which means a failed re-read retires nothing. Or keep it, write the race into SCR-2 as an accepted residual, and add a test for the stale-first-read ordering.
  Confidence: 75
- [WARNING] The SCR-7 watchdog sits in `setInventoryDegraded`/`setDegraded`, so it requeues every degraded path of a 0s inventory, not only total export failure — `internal/controller/kollectinventory_controller.go:658`, `internal/controller/kollectclusterinventory_controller.go:719`
  Failure: a 0s inventory that is Suspended (`:145` / cluster `:99`), scope-denied (`:150`), has an unreachable sink (`:158`), is spill-gated (`:220`), has NoTargets or TargetDegraded (cluster `:120,126`), or has ExportUnavailable (`:206`) now reconciles every 30 s indefinitely. Each pass does a status update, and the spill-gated case also re-marshals the payload. Before this branch none of these paths requeued. No requirement asks for this and no test pins it. It does not violate the "no re-export" clause, because none of these paths exports.
  Fix: apply `RequeueAfterForZeroInterval` only in the total-failure branches (`:264` / cluster `:226`) and leave the helpers returning `exportDebounce`.
  Confidence: 80
- [NOTE] The store fence takes its generation at lookup (`backend_pool.go:387`), not at acquire start. An acquire that began before another acquire's retire can still have its build pooled. That breaks SCR-3's last sentence as written, but it is harmless: the stored key comes from the full read that came after the retire (`:207,228`). Fix: reword the spec. Confidence: 70
- [NOTE] The cluster terminal-no-requeue at `:227-229` holds by reading the code but has no test (tasks.md:322 records the gap). Fix: add a twin of `_zeroIntervalTerminalNoRequeue`. Confidence: 95

**Behaviour that no requirement mentions:**
- Each Secret read is capped at 10 s (`secret_refs.go:26`), which adds a new way for an acquire to fail.
- The "rebuilt sink backend" Info log names the Secrets that changed (`backend_pool.go:220`).
- When a credential read fails, `caRulesFirst` reads the CA Secret's data (`:243-271`), and it can retire the pooled entry on that CA result (`:319`).
- Every retire bumps the key's generation, even when no entry is pooled, and the generations map ages out with the TTL (`:443,569`).
- The empty-Secret refusal now applies through `BuildContextFromSpec` to the connection tests too (`controller/kollectconnectiontest_controller.go:91`, `controller/family_sink_connection.go:49`). Those tests still read through the cached client, so the cross-namespace cache error remains there.
- `CheckCASources` now runs before any Secret read on the pool path (`:183`).
- A relational prune acquires twice: once in `RunCleanupExport`, then again in `RunExportEnvelope` (`sink/cleanup.go:194,379`).

## Could not check
- I ran no tests or gates. Pass results and exit codes are taken from evidence/G1–G4.md and tasks.md.
- The envtest specs (Ginkgo) and the doc pages: I did not read their bodies.
- `task check` and CI on the branch head are recorded as not run (tasks.md:326-327).
- The `staleKey` fix (G3 iteration 5) is unreviewed per G3.md:253. My finding 1 is the first review of that path.
- I did not verify RBAC for the metadata GET of Secrets through the API reader in the chart.
