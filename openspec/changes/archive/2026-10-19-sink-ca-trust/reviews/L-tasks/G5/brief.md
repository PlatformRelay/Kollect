# Review brief — sink-ca-trust G5 (tasks 5.1, 5.2: operator and CLI parity)

You are reviewing one task group, independently of whoever implemented it. Read-only: do not
edit, commit or push. Repository: the current directory (branch `feat/sink-ca-trust`).

Inputs:

1. TASK — `openspec/changes/sink-ca-trust/tasks.md`, section *5. Operator and CLI parity* (5.1,
   5.2), plus the test rules at the top of that file. Spec:
   `openspec/changes/sink-ca-trust/specs/sink-tls-trust/spec.md` requirement SCT-5. Dispatch
   rules: the operator (controller) and the CLI (`kollect-pipeline`) must resolve the CA and fail
   identically, compared with `errors.Is`, through the shared constructors. CLI coverage is end to
   end for S3/GCS (the CLI loads only snapshot sinks); Postgres/MongoDB parity is at the shared
   constructors (probe versus export). These guards may pass when written; their red is a
   mutation.
2. MATRIX and SENSORS — `openspec/changes/sink-ca-trust/evidence/G5.md` (verification matrix,
   sensors run, iterations, mutation table, *Deviations from the task text*). The full
   `task`-tier suite is pending: it runs once after review, not missing.
3. DIFF — the uncommitted working tree against `45ed24e7a`: `git diff 45ed24e7a` (new files are
   intent-to-add, so they appear). Test files only; no product code changed. Product code the
   tests exercise (landed in groups 1-4, not under review): `internal/pipeline/wire.go`
   (`resolveCAPEM`, `cliBuildContext`, `runOneContext`, `withCLIBackend`),
   `internal/sink/{build_context,ca_source,registry,probe}.go`, `cmd/cli/{collect,exitcode}.go`.
4. RULES — `AGENTS.md` (repo root) and `docs/development/spec-workflow.md`. No private key or PEM
   fixture committed; certificates generated at test time. Real-TLS tests are
   `//go:build integration` (netguard refuses loopback otherwise).

Judge, in order:

a. INTENT — does the diff do what 5.1 and 5.2 ask, all of it, and nothing more? Check C1a, C1b,
   C2, C3 and both named mutations; judge the recorded deviations (C1b placed in `cmd/cli`; C2's
   Postgres "no sslmode" case replaced because of the recorded unset→verify-full decision).
b. MATRIX — for each row: is the sensor real, is the class honest, would it fail if the claim
   were false? You may mutation-verify in a scratch copy OUTSIDE the repository, judged by exit
   status. Commands: `go test -run 'TestCAParity|TestCLICredentials' ./internal/pipeline/
   ./internal/sink/`; `go test -tags integration -run TestCATrustIntegration_cliExportsToPrivateCA
   ./internal/pipeline/`; C1b needs envtest assets (`KUBEBUILDER_ASSETS`):
   `go test -run TestCollectCmd_caErrorExitsFatalAfterCollection ./cmd/cli/`.
c. RULES — anything in AGENTS.md the diff violates (test conventions, secrets, parallel safety of
   swapped package vars and `t.Setenv`).
d. Failure modes — can the operator and the CLI still diverge in a way these guards miss (a CLI
   path with its own validation, a probe not plumbed, an error compared by text instead of
   identity, a CLI that writes before failing, an exit code reached for the wrong reason);
   tautological tests (oracle computed from the code under test), weakened assertions, prose in
   the evidence that claims more than the tests show.

For each P0/P1 you are about to file, argue the opposite in one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

RETURN at most 15 lines, nothing else, starting with a `## Verdict` heading:

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you did not verify>
summary: <one line>
```

P0 (security, invariant broken, vacuous gate presented as proof) → BLOCK. P1 (correctness gap,
untested new behaviour, dishonest evidence class, missing half of the task) → REQUEST_CHANGES.
P2 → note only.
