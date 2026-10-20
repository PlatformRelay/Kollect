# Review brief — group G3 (tasks 3.1 and 3.2), sink-credential-rotation, round 1

You are reviewing one task group independently of whoever implemented it. Read-only: do not edit
files. Repo root is the current directory (branch `feat/sink-credential-rotation`, base commit
`f08fb2b12`, the G2 commit).

Inputs:

1. TASK — `openspec/changes/sink-credential-rotation/tasks.md`, items 3.1 (tests) and 3.2
   (implementation) under "## 3. Retired backends drain (SCR-3)". Spec:
   `specs/sink-secret-rotation/spec.md` (SCR-3); design: `design.md` sections *Pool entries with
   holders* and *Fingerprint and acquire* step 4 (store fence). Also in scope, deferred here by
   earlier groups: last writer wins between two concurrent builds (G1), and a stale metadata read
   that retires a newer pooled entry (G2). The evidence explains the fix for the second: the
   acquire records the key's generation before its metadata read and retires (key change or
   not-found/empty) only while the generation is unchanged. Judge whether that extension of the
   design's retire rule is sound and does not weaken SCR-2/SCR-3 (no export may use an old
   backend after a read that observed the change). Out of scope: docs (G4).
2. MATRIX — `openspec/changes/sink-credential-rotation/evidence/G3.md` (verification matrix,
   recorded reds, mutation gate, sensors). The full suite is `pending` (runs once after review).
3. DIFF — the uncommitted change: `git diff f08fb2b12` (new files are intent-to-add, so they
   show). Production: `internal/sink/backend_pool.go`. New tests:
   `internal/sink/backend_pool_drain_test.go`.
4. SENSORS — recorded in the evidence. Trust exit codes as reported; rerun anything under a
   minute you dispute (for example `go test ./internal/sink/ -count=1 -run
   'TestAcquireBackend_|TestRelease_|TestEvictBackendPool'`, or with `-race`).
5. RULES — this is credential handling: no Secret value may reach an error, log line or pool
   entry this change adds; test secret values are sentinels (`sentinel-*`); nothing private
   (company names, real endpoints, keys) in any file; evidence is Markdown only.

Judge, in order:

a. INTENT — does the diff do all of 3.1 and 3.2 and nothing outside them? Every test the tasks
   name? Is the claim that `_concurrentAcquiresAfterRotation` is green on the base (and
   load-bearing only through mutants) true?
b. MATRIX — would each test fail if its claim were false? Mutation-verify in a scratch copy
   outside the repo where cheap. Are the reds on the target assertion? Are evidence classes honest?
c. CORRECTNESS — Close exactly once per backend on every path (hit, store, share, fenced build,
   tombstone build, retire with and without holders, TTL, delete eviction, test evictions,
   `ResetBackendPoolForTest` followed by a late release); never under `globalBackendPool.mu`;
   no deadlock (a release takes the mutex; nested acquires, e.g. `pruneRelationalExport` calling
   `RunExportEnvelope` under an outer hold); the fence order (tombstone, same-key share, unchanged
   generation, else owning release); generations never reissued and aged out safely; a held
   entry never TTL-pruned; the stale-read guard cannot leave an old backend in use after an
   acquire observed the change; BEP-1 semantics kept.
d. AI-code failure modes — tautological tests, weakened assertions, races in the tests
   themselves, a dropped half of a task, prose claiming more than the code does.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

Return at most 15 lines, in this shape, under a heading `## Verdict`:

```text
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you did not verify>
summary: <one line>
```

Severity: P0 (security, secret leaked, invariant broken, vacuous proof presented as proof) →
BLOCK. P1 (correctness gap, untested new behaviour, missing half of the task, dishonest class) →
REQUEST_CHANGES. P2 → note only.
