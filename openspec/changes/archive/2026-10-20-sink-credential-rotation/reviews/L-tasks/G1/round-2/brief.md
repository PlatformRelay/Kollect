# Review brief — group G1 (tasks 1.1 and 1.2), round 2, sink-credential-rotation

You are reviewing one task group independently of whoever implemented it. Read-only: do not edit
files. Repo root is the current directory (branch `feat/sink-credential-rotation`, base commit
`a67bdb01b`). This is the second and last review round.

Inputs:

1. TASK — `openspec/changes/sink-credential-rotation/tasks.md`, items 1.1 and 1.2 (section
   "## 1. Rotation rebuilds the backend"); spec `specs/sink-secret-rotation/spec.md` (SCR-1,
   SCR-4); design sections *Readers*, *Referenced Secrets, one list*, *Fingerprint and acquire*
   steps 1-3, *Logging*. Retire rules, the empty check and NotFound reclassification are G2;
   holders and the store fence are G3.
2. MATRIX — `openspec/changes/sink-credential-rotation/evidence/G1.md`. Its *Independent review*
   section lists round 1's findings and what was done with each.
3. DIFF — the whole staged change: `git diff --cached a67bdb01b`. Focus first on what changed
   since round 1:
   - `internal/sink/build_context.go`: `checkCASecret` and its call in `BuildContextFromSpec`
     (the CA Secret is read and its CA key selected before any credential Secret);
   - `internal/sink/backend_pool.go`: `caRulesFirst`, applied when the metadata or full Secret read
     fails; the rebuild log line now after `NewBackend`;
   - tests: `TestAcquireBackend_caRulesPrecedeCredentialLookup`
     (`internal/sink/backend_pool_rotation_test.go`), the cluster-export subtest in
     `internal/controller/secret_reader_wiring_test.go`, the export counter in
     `internal/controller/secret_rotation_envtest_test.go`.
4. SENSORS — evidence *Sensors run*, *Iterations*, *Mutation gate*. Trust exit codes; rerun
   anything under a minute you dispute.
5. RULES — credential handling: no Secret value in a pool key, log line or error added here; test
   secrets are sentinels; loopback/container tests carry `//go:build integration`; nothing private
   in any file; evidence is Markdown only.

Judge, in order:

a. Does the CA-precedence fix restore the old `BuildContextFromSpec` behaviour exactly (CA source
   check, then a missing CA Secret, then a CA Secret without a CA key, then credential Secrets),
   on both the probe path and the pool path? Can `caRulesFirst` mask an error it should not (for
   example replace a forbidden or timeout error with a CA error, or read CA data on a pool hit)?
b. Are round 1's dispositions sound (the rejected b-W2 against SCR-2 and design *Compatibility*,
   the deferral b-N2 to G3)?
c. Anything else in the full diff a first round could have missed: correctness, security,
   tautological tests, missing rows of 1.1.

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

Severity: P0 (security, secret leaked, invariant broken, vacuous proof) → BLOCK. P1 (correctness
gap, untested new behaviour, missing half of the task, dishonest class) → REQUEST_CHANGES. P2 →
note only.
