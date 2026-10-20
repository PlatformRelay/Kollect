## Verdict: CLEAN

## Findings
- [NOTE] Nothing tests that the cluster reconciler skips the requeue after a terminal failure; only the namespaced reconciler has that control test — `internal/controller/kollectclusterinventory_controller.go:226-229`
  Failure: if someone drops the `result.RequeueAfter = 0` on the cluster side, a terminal `0s` cluster inventory is retried every 30 s and no test fails. The code is correct today; tasks.md 2.3 names only the namespaced control.
  Fix: add a cluster version of `TestApplyInventoryExportOutcome_zeroIntervalTerminalNoRequeue`.
  Confidence: 85
- [NOTE] Two of the three control rows can't tell the interval apart from the watchdog. "30s" and "unset" both expect 30 s, which equals `ZeroIntervalWatchdog`. Only "5m" catches a "watchdog for every interval" mutant — `internal/controller/zero_interval_requeue_test.go:332-334`
  Failure: if the 5m row were removed, a requeue that always used the watchdog would pass. The table is adequate as it stands.
  Fix: none needed. Optionally add a 45s row.
  Confidence: 90
- [NOTE] The watchdog requeue goes beyond SCR-7. Every `0s` degraded path now requeues every 30 s (Suspended, scope denied, sink unreachable, spill gate, registry nil), but SCR-7 (`spec.md:254-260`) covers only a transient total export failure — `internal/controller/kollectinventory_controller.go:145,150,158,220`
  Failure: a suspended `0s` inventory now runs a reconcile and a status write every 30 s instead of never. No export runs. Round 1 already raised this and rejected it.
  Fix: none in G2 (operator decision). The G4 docs should state it.
  Confidence: 80
- [NOTE] A stale metadata read can retire a newer pooled entry — `internal/sink/backend_pool.go:277-291`
  Failure: acquire A's metadata GET finishes before a rotation. Acquire B then stores the rebuilt entry. When A takes the lock, its key differs, so A deletes B's entry and closes it while B's export may still be using it. A then rebuilds the same value. Before this change, `storePooledBackend` key-matching kept B's entry. The design's retire rule allows this, and the G3 holders and generation fence (design step 4) cover it.
  Fix: G3, as planned. Make sure G3's tests include a stale-read interleaving.
  Confidence: 55
- [NOTE] A changed CA key goes unretired until the next good metadata read. If the credential metadata read times out and `caRulesFirst` finds the CA Secret present but missing its CA key (terminal `ErrCAKeyMissing`), `isSecretGone` is false, so the entry stays — `internal/sink/backend_pool.go:171-174`
  Failure: the change is observed but not acted on; the next successful metadata read retires the entry via `lookupOrRetire`. The acquire fails either way, so no stale backend is served on this call.
  Fix: none required. Optionally retire when the error is `ErrCAKeyMissing` too.
  Confidence: 60

Checked and holding:
- **Retire rules:** not-found and empty retire, through the metadata read or the CA-rules read (`isSecretGone` on both errors). Timeout, cancel and forbidden keep the entry. An observed key change retires before the rebuild.
- **Empty check on every read path:** `fetchSecrets`, `getSecret` (pool-disabled path and `ResolveSecret`), the CA-rules read in `caRulesFirst`, and the CLI `resolveSecretValues` after `stringData` and env substitution. All use the same `EmptySecretError` text.
- **Error classes:** the new errors are unclassified, so transient.
- **Stream cleanup:** the retain happens before the acquire. `TestStreamSinkTypes_matchRetainingBackends` ties the set to the registry and to each backend's capabilities and `ExportCleaner` assertion.
- **No ambient-credential fallback:** an emptied S3 Secret never reaches the factory, with the pool on or off (test, `backend_pool_retire_test.go:1109`).
- **No regressions:** BEP-1 tombstones are untouched by `retirePoolEntry`. The CA-before-credential precedence from G1 is kept.
- **No leaked values:** the new log and error text holds only namespace and name. The test fixtures use `sentinel-*` values.
- **Round-1 fix:** the fix holds (M13 and M14 kill it). The two "green on base" claims are true: G1's `getSecretObject` already mapped NotFound to the plain text.

## Could not check
- I ran no tests or mutants. Pass and fail results, mutation kills and race runs come from `evidence/G2.md`.
- I didn't read `checkCASecret`, `secretMeta`, `buildContextFromSecrets` or the test helpers (`rotationClient`, `enableRotationPool`, `resolveThroughCLI`, `secretRecordingRegistry`) in full. I inferred their behaviour from the call sites and the recorded test results.
- I didn't check whether `conflictRequeueAfter` could ever equal the watchdog value, which would let the envtest spin until timeout.
- I couldn't see the round-1 review legs or the G3 tasks, beyond what the brief and the evidence quote.
