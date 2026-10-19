## Verdict: BLOCK
## Findings
- [WARNING] The store fence conflicts with immediate retirement and loses invalidation history — `openspec/changes/sink-credential-rotation/design.md:53`
  Failure: An acquire observes B1, retires it as required at lines 84–86, then cannot store B2 because the entry no longer has B1’s key. Treating absence as permission instead allows an older builder to repopulate after a newer entry was retired.
  Fix: Define a per-sink generation token that survives entry removal; test replacement from an occupied pool and an older builder completing after a newer retirement.
  Confidence: 96
- [WARNING] API-reader wiring omits an explicitly nested cleanup acquisition — `internal/sink/cleanup.go:369`
  Failure: Database cleanup acquires through the new reader, then `pruneRelationalExport` constructs another export request. Without forwarding `SecretReader`, that acquire falls back to the cached client, allowing stale credentials or an unknown-namespace failure.
  Fix: Explicitly require forwarding through this request and controller cleanup helpers; add a database-cleanup test with stale cached data and a restricted namespace cache.
  Confidence: 98
- [WARNING] The Postgres column-default assertion can fail after a correct rotation — `openspec/changes/sink-credential-rotation/tasks.md:101`
  Failure: Exporting the same inventory rows as `u2` takes `ON CONFLICT DO UPDATE`; neither upsert implementation updates the additional audit column, so its existing default remains `u1` (`internal/sink/postgres/bulk_export.go:68`, `:134`).
  Fix: Specify fresh row identities for the second export, or an INSERT/UPDATE audit trigger; check sessions through a separate observer connection. Keep `NOLOGIN`, which does not revoke existing sessions.
  Confidence: 99
- [WARNING] Several required reds are already green on their specified base — `openspec/changes/sink-credential-rotation/tasks.md:118`
  Failure: G1 already performs metadata reads and rebuilds on changed identity. Consequently restoration, cancelled-read preservation, and deleted-Git-auth failure can pass before G2. Also, sequential A→B→A rebuilds three times with a content key, so `_rotateBack` cannot expose the defect it names.
  Fix: Mark existing behavior as controls; use a same-data update with changed UID/resourceVersion to distinguish metadata identity from content hashing.
  Confidence: 98
- [WARNING] Metadata-only GET does not guarantee that credential values are absent from the response — `openspec/changes/sink-credential-rotation/design.md:120`
  Failure: A Secret annotation containing credentials, including a last-applied configuration, remains in `PartialObjectMetadata.ObjectMeta`. Controller-runtime copies that metadata response, so a pool hit can transfer credential material despite SCR-4’s absolute promise; checking the helper’s return type does not prove otherwise.
  Fix: Scope the guarantee to excluding Secret `data`/`stringData` fields and retaining only identity/version fields; document the annotation limitation and test that annotations are discarded.
  Confidence: 99
## Could not check
- Did not run tests, envtest probes, Docker integrations, race checks, or gates in this read-only review.
- Runtime API-server resourceVersion behavior, RBAC enforcement, and real database-session drainage remain unverified; controller-runtime metadata GET support was checked in cached source.
