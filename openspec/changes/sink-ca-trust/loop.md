# Loop — sink-ca-trust (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/sink-ca-trust` · Branch: `feat/sink-ca-trust` · Base: `origin/main` @ `7f97d59ec` · Started: 2026-10-19 · Reviewers: astra + claude (duo)
Next: L task loop — dispatch T0.1 (probe) with task-prompt.md
Budget: claude review legs uncapped (operator goal 2026-10-16) · decisions by model alignment, logged in agent-context/decisions.md

Backlog SINK-CA-UNUSED-01 (decisions 2026-10-04 "→ A, honour it" and 2026-10-19 "CA semantics", outcome A). Release note: v0.22.0. Spec set: proposal.md, design.md, specs/sink-tls-trust/spec.md, tasks.md.

## Stages

- [x] 0 orient — new change, created with `task spec:cli -- new change sink-ca-trust`; file:line references measured against `origin/main` @ `7f97d59ec` (2026-10-19)
- [x] P plan + tasks — written; spec set committed: `70c847a8e`
- [x] R spec-set review — round 1 on `b3da7b2c9`: astra BLOCK, opus CONCERNS; A-F1 (MongoDB `sslInsecure` alias) and A-F3 (CLI loads no database sinks) CRITICAL accepted and fixed, all others fixed or narrowed (`reviews/R-spec-set/verification.md`). Round 2 (last) on `07ac4d2a3`: astra CONCERNS, opus CONCERNS, no CRITICAL; all findings fixed (`reviews/R-spec-set/round-2/verification.md`). No CRITICAL remains
- [ ] L task loop — see *Tasks*
- [ ] B branch review — `reviews/B-branch/round-N/` · rounds: <n> · verdict: <…>
- [ ] Hand-off — <date> · `pr-description.md` · PR: <none>

## Fitness functions

| Characteristic | Command | Kind | Baseline (number, list, allow-list) | Per task / at B |
|---|---|---|---|---|
| Unit tests, changed packages | `go test -v ./internal/sink/... ./internal/pipeline/...` | triggered | green | per task |
| Integration-tagged build | `go vet -tags integration ./internal/sink/... ./internal/pipeline/...` | triggered | clean | per task |
| Integration TLS tests | `go test -tags integration -v -run '<names>' <pkg>` (Docker for Postgres/MongoDB) | triggered | new tests run, none skipped where Docker exists | per task |
| Race | `go test -race -count=3` on changed packages | triggered | green | per task |
| Architecture | `task lint` (golangci-lint v2 and go-arch-lint; `internal/sink/**` is one component, `.go-arch-lint.yml:63-64`) | triggered | no new findings | per task |
| Codegen drift | `task verify` | triggered | green | G6 and at B |
| Markdown | `task lint:markdown` (tracked files only; stage new files first) | triggered | 0 issues | per task + every orchestrator state edit |
| OpenSpec strict | `task spec:validate` | triggered | green | per spec-set edit |
| Full local gate | `task check` | holistic | green on base | at B |

Holistic run at B: not run yet.

## Triage

| Stage | Finding (one line) | Sev | Models | Disposition (fix / reject / defer) | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| R1 | MongoDB `sslInsecure` alias and mixed-case keys bypass the rejection | CRITICAL | astra (+opus backstop) | fix | SCT-3, design *MongoDB*, M2/M4/M7 | no |
| R1 | CLI loads only `KollectSnapshotSink`; Postgres CLI scenario unexecutable | CRITICAL | astra | fix | SCT-5 scope, C1 in `cmd/cli`, non-goal | no |
| R1 | `s3://` + SDK env endpoint can be `http`; no redirect policy | WARNING (astra CRITICAL) | astra | fix | SCT-1, S1b, I4 | no |
| R1 | MongoDB driver errors leak hosts today | WARNING (astra CRITICAL) | astra | defer | non-goal + follow-up; spec narrowed to errors this change adds | no |
| R1 | Postgres: direct negotiation, env and home root defaults, ordering | WARNING | astra, opus | fix | SCT-2, design *Postgres*, D3-D5 | no |
| R1 | Both-set check order, parity by `errors.Is`, R2 scope | WARNING | astra, opus | fix | design *CA resolution*, R1/R2 | no |
| R1 | Missing mode/hostname/SRV cases; PI4 package | WARNING | astra | fix | PI2, PI5, PI4, M8 | no |
| R1 | Subprocess env, Go version, integration evidence | WARNING | astra, opus | fix | tasks test rules, 0.1 | no |
| R1 | Compatibility and release note scope | WARNING | astra | fix | design table, 6.1, SCT-6 | no |
| R1 | Stale lines and small wording | NOTE | astra, opus | fix | proposal, tasks | no |
| R2 | S3-specific endpoint resolved inside `s3.NewFromConfig` | WARNING | astra, opus | fix | design *S3 and GCS*, S1b | no |
| R2 | CLI C1/C3 unexecutable as written (error only printed; envtest absent in integration job) | WARNING | astra, opus | fix | C1a, C1b, C3; CLI-CA-EARLY-01 stays backlog | no |
| R2 | Recovery after a Secret fix has no trigger | WARNING | opus | fix | SCT-4, docs retry note, non-goal | no |
| R2 | `tlsInsecure=false` spec/design mismatch | WARNING | astra, opus | fix | SCT-3, M4 | no |
| R2 | Service-file `sslrootcert` fails before the backstop | WARNING | astra | fix | design step 1, D4 | no |
| R2 | Effective-sslmode wording; CLI fetch order; SRV wording | NOTE | astra, opus | fix | spec, design, proposal | no |
| R-post | Postgres sslmode with a CA: unset → `verify-full`, explicit `prefer`/`allow`/`disable` → error, `require` → chain check | decision | orchestrator (model-aligned, both legs B) | fix (orchestrator decision, mechanical edit, no review round) | decisions.md 2026-10-19, brief `decisions/2026-10-19-sink-ca-pg-sslmode.md`; SCT-2, design *Postgres*, D3, D3b, D5, PI6, release note | no |

## Tasks

| Task | Verdict | Review (legs, rounds, Claude?) | Gaps / decision request |
|---|---|---|---|
| 0.1 | CLOSED-WITH-GAPS: probe only, rows agent-review; no proposal or design claim contradicted | covered by the G1 review | evidence in `evidence/G1.md` (dispatch), not `T0.1.md` |
| 1.1 + 1.2 (G1) | CLOSED | astra + claude/opus:diff, 2 rounds (`reviews/L-tasks/G1/`, `round-2/`); Claude leg: trust handling (dispatch) | defer to B: `internal/controller/secret_ca.go` third resolver (test-only caller) and `git.ValidateTLSSpec`'s own both-sources text |
| 2.1 + 2.2 (G2) | CLOSED | astra + claude/opus:diff, 1 round (`reviews/L-tasks/G2/`); Claude leg: TLS trust (dispatch). astra REQUEST_CHANGES (P1 missing request counter, fixed), opus CLEAN | B: `task lint` does not lint integration-tagged files (9 pre-existing findings in `export*_integration_test.go`) |
| 3.1 + 3.2 (G3) | CLOSED-WITH-GAPS | astra + claude/opus:diff, 2 rounds (`reviews/L-tasks/G3/`, `round-2/`); Claude leg: TLS trust (dispatch). R1 astra BLOCK (bracketed-host misread, fixed), opus CONCERNS (backstops untested, no-CA error text, fixed); R2 astra BLOCK (empty `service=` precedence, fixed, not re-reviewed), opus CLEAN | B: read the round-2 fix (`caDSN` service presence); residual chain-only case when the reader sees a mode pgx does not (opus R2 N3) |
| 4.1 + 4.2 (G4) | CLOSED | astra + claude/opus:diff, 1 round (`reviews/L-tasks/G4/`); Claude leg: TLS trust (dispatch). astra APPROVE, opus CLEAN (4 notes, fixed: test and wording) | none; URI check lives in `clientOptions`, not `ConfigFromSpec` (`config.go` unchanged, `evidence/G4.md` *Notes*) |
| 5.1 + 5.2 (G5) | CLOSED (5.2: none) | astra + claude/opus:diff, 1 round (`reviews/L-tasks/G5/`); Claude leg: TLS trust (dispatch). astra APPROVE, opus CLEAN (4 notes, fixed: test and wording) | none; C1b lives in `cmd/cli` (exit mapping is in package `main`); C2 Postgres "no sslmode" case replaced per the R-post decision (`evidence/G5.md` *Deviations*) |
| 6.1 (G6) | CLOSED-WITH-GAPS | astra only, 1 round (`reviews/L-tasks/G6/`); no Claude leg (docs, astra did not BLOCK). astra REQUEST_CHANGES (5 P1 + 1 P2, all wording, verified and fixed) | B: `task docs:verify` not run locally (no pinned `kubectl`, mkdocs `redirects` plugin missing); CI Docs workflow |

## Known red

Tests a test task left red for a later task; excluded from "green" at close, nothing else is.

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

Tests from a test task changed by a later task (close-task `TEST` class); each one changes what green means.

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|
| `internal/pipeline/sink_credentials_test.go:TestCLICredentials_matchOperatorResolver` | before this change | 1.1 | compared error presence only (`:384`); `"inline bundle wins"` (`:356`) pinned a precedence SCT-4 forbids and `"caSecret no key"` the silent fallback. Now: `"both sources rejected"`, `"caSecret no key"` expects the missing-key error, `"caSecret ca.pem"` added, shared errors compared with `errors.Is` (`evidence/G1.md` *Test changes*) |
| `internal/sink/build_context_test.go:TestBuildContextFromSpec` case `"ca secret with no known PEM keys yields empty CAPEM"` | before this change | 1.1 | pinned the silent fallback SCT-4 forbids; moved to `TestBuildContextFromSpec_errors`, expects an error naming the Secret |

## Lessons

<!-- master list: one line per reusable lesson with the task ids that hit it -->

- G1: a mutant that leaves dead code after `return` fails `go vet` inside `go test` and reads as a kill; guard it with a condition.
- G1: check a reviewer's platform claim against the pinned toolchain's source; Go 1.27 reads `SSL_CERT_FILE` on macOS and Windows too.
- G1: a fresh worktree has no `bin/k8s`; `internal/controller` then fails in `BeforeSuite`. Set `KUBEBUILDER_ASSETS` to the main checkout's assets or run `make setup-envtest`.
- G1: `gitleaks` via mise needs `MISE_GITLEAKS_VERSION=8.30.1` here (the shim has no default).
- G2: net/http does not follow a 307 for an SDK PutObject (body not replayable); exercise a `CheckRedirect` guard with a body-less call (HeadBucket).
- G2: a mutant that leaves an import unused is a build failure, not a kill; keep imports used.
- G2: unit-tier "no request" claims cannot use a loopback counter (netguard); count in the integration tier and assert the counter before the error.
- G3: a reader that mirrors a library parser drifts on key presence versus empty value and on syntax corners (bracketed hosts); assert the reader's own output on a table of edge spellings and keep a post-parse shape check.
- G3: `net.Pipe` is unbuffered; a TLS client that rejects the certificate deadlocks against the server's flight until the context ends; give the server end a short deadline.
- G3: run `go test` with `umask 022`; under 002, `t.TempDir()` is 0775 and `internal/sink/git`'s mirror guard fails.
- G4: prove "no DNS lookup" for mongo-driver by swapping `dns.DefaultResolver.LookupSRV`/`LookupTXT` for test-failing functions in a serial test that runs the real build path; a check function without DNS code proves nothing.
- G4: a parser-mirroring reader's differential table must include URIs the library refuses (an `@` in the query with no user info); a mutant survived until it did.
- G5: a mutant that sets a value to `nil` can leave its source variable unused (a build failure, not a kill); use an empty value derived from it (`x[:0:0]`) and read each kill's failure line.
- G5: prove "fails after collection" through a seam reached only after collection: wrap the registry factory inside the real `runAllContexts`, assert the call count and the error prefix, and add a `--output` control run.
- G6: `kollect.dev/test-connection=true` is cleared only when automatic probes are off, so re-setting it can be a no-op; to touch a sink, change any annotation.
- G6: `task docs:verify` locally needs `MISE_KUBECTL_VERSION` and the mkdocs `redirects` plugin; without them it is ENVIRONMENT, not a docs failure.

## Where a human should look first

<!-- filled at hand-off -->

## Proposed harness changes

| Lesson | Seen in | Proposed change (AGENTS.md line / sensor / close-task.yml) |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
