# Review brief — group G2 (tasks 2.1 to 2.4), sink-credential-rotation

You are reviewing one task group independently of whoever implemented it. Read-only: do not edit
files. Repo root is the current directory (branch `feat/sink-credential-rotation`, base commit
`d307e2ec8`, the G1 commit).

Inputs:

1. TASK — `openspec/changes/sink-credential-rotation/tasks.md`, items 2.1 and 2.3 (tests) and 2.2
   and 2.4 (implementation) under "## 2. Deleted and emptied Secrets fail (SCR-2, SCR-4, SCR-5,
   SCR-7)". Spec: `specs/sink-secret-rotation/spec.md` (SCR-2, SCR-4, SCR-5, SCR-7); design:
   `design.md` sections *Fingerprint and acquire* (errors, retire rules), *Cleanup before acquire
   for sinks that cannot retract*, *CLI*, *Zero export interval*. Out of scope for G2: pool
   holders, deferred close and the store fence (G3: a retired backend is still closed at once
   here), docs (G4).
2. MATRIX — `openspec/changes/sink-credential-rotation/evidence/G2.md` (verification matrix,
   recorded reds, mutation gate, sensors). The full suite is `pending` (runs once after review).
3. DIFF — the uncommitted change: `git diff d307e2ec8` (new files are intent-to-add, so they show).
   New test files: `internal/sink/backend_pool_retire_test.go`,
   `internal/pipeline/sink_empty_secret_test.go`, `internal/controller/zero_interval_requeue_test.go`,
   `internal/controller/secret_retire_envtest_test.go`.
4. SENSORS — recorded in the evidence. Trust exit codes as reported; rerun anything under a
   minute you dispute (for example `go test ./internal/sink/ -count=1 -run
   'TestAcquireBackend_|TestSecretMeta_|TestCleanup_|TestS3_|TestResolveSecret_|TestRotationErrors_|TestStreamSinkTypes_'`).
   Envtest needs `KUBEBUILDER_ASSETS=/home/koni/Projects/PlatformRelay/kollect/bin/k8s/1.36.0-linux-amd64`
   and `-run TestControllers ./internal/controller/ -ginkgo.focus '<spec text>'`.
5. RULES — this is credential handling: no Secret value may reach an error, log line, status
   condition, Event or pool entry this change adds; test secret values are sentinels
   (`sentinel-*`); nothing private (company names, real endpoints, keys) in any file; evidence is
   Markdown only.

Judge, in order:

a. INTENT — does the diff do all of 2.1 to 2.4 and nothing outside them? Every test and row the
   tasks name? Where the evidence says a test was green on the base although tasks.md expected
   red (`TestSecretMeta_notFoundIsTransient`, the "deleted secret … redacted status" envtest
   spec), is the "outcome already holds" claim true?
b. MATRIX — would each test fail if its claim were false? Mutation-verify in a scratch copy
   outside the repo where cheap. Are the reds on the target assertion? Are evidence classes honest?
c. CORRECTNESS and SECURITY — retire rules exactly as design says (not-found and any observed key
   change retire; timeout, cancelled, forbidden do not); empty check on every read path (pool
   full read, pool-disabled `getSecret`, the CA-rules read in `caRulesFirst`, CLI) with one text;
   error classes stay transient; the stream retain cannot skip a sink type that needs cleanup;
   S3/GCS cannot fall back to ambient credentials for an entirely empty Secret; SCR-7 requeue
   applies only where the design says and terminal failures still do not requeue; no regression
   for BEP-1 tombstones, the pool-disabled path, or the CA-before-credential precedence G1 pinned.
d. AI-code failure modes — tautological tests, weakened assertions, a dropped half of a task,
   prose claiming more than the code does.

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
