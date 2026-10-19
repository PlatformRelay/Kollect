# Review brief — group G1 (tasks 1.1 and 1.2), sink-credential-rotation

You are reviewing one task group independently of whoever implemented it. Read-only: do not edit
files. Repo root is the current directory (branch `feat/sink-credential-rotation`, base commit
`a67bdb01b`).

Inputs:

1. TASK — `openspec/changes/sink-credential-rotation/tasks.md`, items 1.1 (tests) and 1.2
   (implementation) under "## 1. Rotation rebuilds the backend (SCR-1, SCR-4 key)". Spec:
   `specs/sink-secret-rotation/spec.md` (SCR-1, SCR-4); design: `design.md` sections *Readers*,
   *Referenced Secrets, one list*, *Fingerprint and acquire* steps 1-3, *Logging*. Out of scope
   for G1 (later groups): retire rules, the empty-Secret check and NotFound reclassification (G2),
   pool holders and the store fence (G3), docs (G4).
2. MATRIX — `openspec/changes/sink-credential-rotation/evidence/G1.md` (verification matrix,
   recorded reds, mutation gate, sensors).
3. DIFF — the staged, uncommitted change: `git diff --cached a67bdb01b` (Go code, tests, the
   evidence file and this brief). New files: `internal/sink/secret_refs.go`,
   `internal/sink/backend_pool_rotation_test.go`, `internal/sink/rotation_integration_test.go`,
   `internal/controller/secret_reader_wiring_test.go`,
   `internal/controller/secret_rotation_envtest_test.go`.
4. SENSORS — recorded in the evidence's *Sensors run*, *Iterations* and *Mutation gate*. Trust
   exit codes as reported; rerun anything under a minute you dispute (for example
   `go test ./internal/sink/ -count=1 -run 'TestAcquireBackend_|TestReferencedSecretRefs|TestSecretMeta'`).
   Envtest needs `KUBEBUILDER_ASSETS=/home/koni/Projects/PlatformRelay/kollect/bin/k8s/1.36.0-linux-amd64`.
5. RULES — this is credential handling: no Secret value may reach a pool key, a log line or an
   error added by this change; secret values in tests are sentinels (`sentinel-*`); tests that dial
   loopback or a container carry `//go:build integration`; nothing private (company names, real
   endpoints, keys) in any file; evidence is Markdown only.

Judge, in order:

a. INTENT — does the diff do all of 1.1 and 1.2 and nothing outside them? Every test the task
   names, with every row it names (8 references × namespace omitted/explicit; export, Retain and
   Delete-prune rows; both reconcilers; both envtest specs; both integration tests)?
b. MATRIX — would each test fail if its claim were false? Mutation-verify in a scratch copy
   outside the repo where cheap. Are the reds genuine (target assertion, not compile errors or
   skips)? Are evidence classes honest?
c. CORRECTNESS and SECURITY — does `referencedSecretRefs` list exactly the Secrets the old
   `BuildContextFromSpec` read, per type, with the same namespace defaulting and fallbacks (Kafka
   and NATS to `spec.secretRef`, git auth only for type `git`, CA only without `caBundle`)? Is the
   key derived from metadata only? Do key and values come from one read? Is every production
   request (export in both reconcilers, both finalizer cleanup paths, the nested prune export)
   wired to the uncached reader, and does `SetupWithManager` default it? Does any path now
   regress (error classes, the pool-disabled path, connection probes, BEP-1 tombstones)?
d. AI-code failure modes — tautological tests, weakened assertions, a dropped half of the task,
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
