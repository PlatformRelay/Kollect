# Loop — golangci-lint-bump (spec-loop run state)

Repo: `/Users/kheimel/.treehouse/kollect-79dca7/6/kollect` (treehouse pool worktree slot 6;
primary checkout untouched) · Branch: `fm/kollect-dev-continue` · Base: `3ee21266`
(origin/main after PR #450) · Started: 2026-10-06 · Reviewers: fanout (free only)
Implementer model: GLM-5.3 (task processes). Review pairing: task diff legs
`DeepSeek-V4.1-Flash:diff,Qwen3.8-Flash-Next:diff` (both families other than the implementer's);
gate legs GLM-5.3 / DeepSeek-V4.1-Flash / Qwen3.8-Flash-Next plus strong free leg
`Qwen3.8-2.4T-A95B-NVFP4` where spec-loop would use `claude/opus`. NO Claude leg, ever (brief).
Budget: claude review legs 0/0 · active hours 0/8 (default) · source: default (no claude part)

This is an OpenSpec repo (no `.specify/`); the spec set lives in `openspec/changes/golangci-lint-bump/`
(proposal.md, specs/lint-toolchain/spec.md, tasks.md). The orchestrator and every reviewer leg are
free internal models only, per the firstmate brief.

## Stages

- [x] 0 orient — feature dir, what existed (proposal / spec delta / tasks, all present); branch and base recorded
- [ ] P plan + tasks — skipped: present (authored when the change was proposed; re-verified not implemented: tasks.md all unchecked, both version pins still v2.11.4)
- [x] R spec-set review — round 1 `reviews/R-spec-set/` on base: BLOCK, 9 CRITICALs verified (8 fixed in the spec set, 1 rejected as false) + 1 warning deferred; round 2 `reviews/R-spec-set-round2/` on aebb276b: 5/5 legs CONCERNS (unified BLOCK via the promote rule; no leg rated a CRITICAL itself), 8 findings — all mechanical, fixed below; two spec-set rounds spent, no further spec-set re-review per the round limit · CRITICAL: none surviving
- [ ] L task loop — see *Tasks*
- [ ] B branch review — round 1 `reviews/B-branch/round-1/` (9/9 legs, unified BLOCK: 3 CRITICAL verified → 2 fixed, 1 premise rejected after verification; 3 WARNINGs → 2 fixed, 2 deferred) · round 2 pending · rounds: 1/2
- [ ] Hand-off — <date> · `pr-description.md` · PR: <none | link>

## Fitness functions (inventory at orient)

| Characteristic | Command | Kind | Baseline | Per task / at B |
|---|---|---|---|---|
| Go layering | `go-arch-lint check` (via `task lint`) | triggered | allow-list | per task |
| Go coverage floor | `task coverage` with `COVERAGE_MIN=90` (internal/) | holistic, per change | ≥90% | at B |
| Shell hygiene | `task lint:shell` (shellcheck --severity=warning over hack/**) | triggered | no warnings | per task |
| Workflow supply chain | zizmor `--offline --min-severity=high` over `.github/` (required lint job) | triggered | 0 findings at high | per task |
| CI guard meta-tests | `hack/test/*_test.sh` suite in the required lint job | triggered | green suite | per task |
| Static Go analysis | golangci-lint (`.golangci.yaml`, v2) + Sonar + vulncheck | holistic | green on main | at B (this branch MOVES the linter) |
| Markdown lint | `task lint:markdown` (required) | triggered | clean | per task |

This branch is version-only + lint-fix commits; the linter move itself is the fitness event:
findings count before (v2.11.4) vs after (v2.13.1) is the recorded measure (LTB-3).

Holistic run at B (branch tip a674783e, code tree c9e795bb): `task coverage` — full suite
green on the final tree (all internal/ packages ok; closes the 1.3 record gap — the earlier
full run was at e2941062), internal/ coverage **91.3%** (floor 90%): pass. The branch adds
no new code paths (constants + requeue swaps + 2 reasoned nolints), so the coverage floor is
not at risk; the base-branch value was not re-measured. `task lint` 0 findings, arch-lint
OK, markdown 0, shell 0, zizmor 0 (1.3 evidence), CI guard meta-tests: CI-covered at PR time.

## Triage

| Stage | Finding (one line) | Sev | Models | Disposition | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| B1 | 1 LabelProfile decoupling cosmetic — const unused, label-name sites still read the unrelated StaticRefTypeProfile enum | CRITICAL | 4/8 | fix | VERIFIED REAL: made the decoupling real — the 8 label-name sites (metrics.go ×2, aggregation.go, aggregation_labeled.go, metrics_catalog.go ×4) now use LabelProfile; values unchanged ("profile"); metrics + collect tests green | no |
| B1 | 2 conflict-requeue: "the anti-hot-loop argument kept at reconcile_guard.go:36 was not applied" to the 11 conflict sites; 9/11 untested | CRITICAL | 4/8 | fix + reject-premise + defer | Premise REJECTED after verification: the two paths have different, documented rationales — panic = no progress and no watch event (reconcile_guard.go:36 nolint), conflict = transient + watch re-enqueues (finalizer.go:16-20 const comment, b1d32712); 3 of the 4 models judged the disclosed delta acceptable (NOTE). Fix applied: the const comment now contrasts the panic path explicitly. Deferred: the 9/11 per-site requeue tests are pre-existing coverage (the branch preserved "a requeue happens" semantics, 2 tests pin it), named in the archive record's tech-debt notes. Logged as an operator-re-openable decision. | no |
| B1 | 3 close-out nolint count stale: records say 2, branch carries 3 (guard-test assertion nolint unrecorded) | CRITICAL | 2/2 | fix | records updated (tasks.md LTB-2 row, evidence/1.3.md row 7 + disposition): 3 reasoned nolints — 2 product + 1 TEST-class | no |
| B1 | 4 gomodguard deprecated (warns on every run), migration to gomodguard_v2 owned by no task | WARNING | 3/5 | defer | archive record names it (warning-only, exit 0; the migration is a future tooling change) | no |
| B1 | 5 namespaceField (helm-release alias) reused for envelope pruning — coupling created by this branch's goconst hoist (verified via 5b97c562 diff) | WARNING | 3/3 | fix | VERIFIED REAL, branch-created: dropEnvelopeIdentity now owns its own envelope constants with a no-alias comment; helmdecode comment states the prune owns its own | no |
| B1 | 6 probe.md:214 logtools build-version still "believed" despite the go version -m proof | WARNING | 2/2 | fix | probe.md §8 updated to verified (1.2's go version -m + four stage-B legs) | no |
| R | 1 LTB-1 pin-mismatch scenario delivered by no task (`\|\| true` custom-build downgrade; guard deferred to change 5) | CRITICAL | 4/7 | fix | scenario re-scoped to this change's pin-equality check; runtime guard stays change 5's | no |
| R | 2 LTB-2 nolint/exclusion reason rule absent from task 1.3 and the LTB-2 check row | CRITICAL | 4/6 | fix | task 1.3 + verification row now cover reasonless nolint/exclusions | no |
| R | 3 LTB-3 after-count/fixed/justified numbers have no producing task | CRITICAL | 5/7 | fix | task 1.3 records count after, fixed/justified numbers, binary version | no |
| R | 4 `task format:check` runs the bumped binary; formatter drift uncovered | CRITICAL | 4/7 | fix | probe 1.1 runs format:check; 1.3 gate requires it clean; LTB-1 row covers it | no |
| R | 5 spec "each finding its own commit" vs task "one commit per group" | CRITICAL | 4/7 | fix | spec reconciled: commits separate from the version-only commit, grouped when one rule produces many findings | no |
| R | 6 "the Go bump is in no landing list" (proposal premise unverifiable) | CRITICAL | 4/7 | reject | FALSE after verification: cross-file-consistency-gates (change 4) explicitly bumps go.mod to go 1.27.1 in its proposal; DeepSeek-adv's conf-55 doubt of the ordering mechanism also rejected — the ordering constraint holds regardless | no |
| R | 7 no task verifies the executed binary is the pinned custom build | CRITICAL | 2/2 | fix | task 1.3 records `bin/golangci-lint version` (must report the pin); a vanilla binary would fail config validation on logcheck, so a silent downgrade to vanilla is loud at task lint | no |
| R | 8 logcheck plugin pinned `version: latest` floats | CRITICAL/WARN | 2-3 | defer | change 5 (developer-toolchain-pins): out of scope per this proposal's non-goals; GLM legs' NOTE agrees; destination named in the archive record | no |
| R | 9 LTB-3 "runnable" undefined once findings appear | CRITICAL | 3/7 | fix | spec defines runnable = binary starts and reports findings (exit may be non-zero until 1.3) | no |
| R | 10 probe 1.1 not executable as written (vanilla binary fails config validation on the custom plugin) | WARNING | 3/7 | fix | task 1.1 names the procedure: temp bump of BOTH pins, make golangci-lint, record, revert | no |

Decision logged per the skill's CRITICAL-with-mechanical-fix rule: no CRITICAL survived
verification as needing a stop — every fix is a spec-set edit the spec set already implies,
so the round-2 re-review replaced the stop (logged 2026-10-06).

### Round 2 (5/5 legs, on aebb276b) — all findings WARNING-grade (promoted only by the 2+ rule)

- 1 plugin-pin deferral hollow (change 5 covers only the golangci pins — verified, zero
  plugin/logtools/latest mentions there): FIX IN THIS CHANGE — scope extension, logged as a
  decision: LTB-4 added, the pin lands in the version-only commit. Rationale: the file is
  already in scope, the pin directly serves LTB-3's reproducibility, and no other change owns
  it.
- 2 proposal still promised a mismatch probe "(task 1.1)": FIXED (assumption reworded).
- 3 "the review record" never named: FIXED (`evidence/probe.md`, `evidence/1.3.md`).
- 4 loop.md bookkeeping wrong (pre-announced round 2; count conflict): FIXED (triage row
  above rewritten; stage line now says round 2 done).
- 5 version string cannot distinguish custom from vanilla build: FIXED (task 1.3 states the
  structural proof: clean lint requires the custom build — vanilla fails config validation on
  logcheck; established as probe evidence).
- 6 spec promised change 5 a "runtime task lint failure" guard: FIXED (reworded: DTP-3 drift
  test red-flags the mismatch in CI; verified DTP-3 names both golangci sites).
- 7 make skips the stale `$(GOLANGCI_LINT)` file target on a version change: VERIFIED REAL
  (`make -n golangci-lint GOLANGCI_LINT_VERSION=v2.13.1` → "Nothing to be done"); FIXED in the
  probe procedure (`rm -f bin/golangci-lint*` first). The latent Makefile staleness itself is
  a pre-existing defect beyond this change's version-only scope — deferred, named in the
  archive record.
- 8 `task format:check` swallows stderr (`2>/dev/null` in Taskfile.yml:324): VERIFIED REAL;
  FIXED for this change's evidence (raw output + exit codes recorded; do not rely on the
  swallowed view). The Taskfile weakness is pre-existing — deferred, named in the archive
  record.
- 9 the v2.11.4 "before" count was never produced: FIXED (task 1.1 records it first).

## Tasks

| Task | Verdict | Review (legs, rounds) | Gaps / decision request |
|---|---|---|---|
| 1.1 probe | CLOSED | 1 diff leg DeepSeek-V4.1-Flash, round 1 — `reviews/L-tasks/1.1/` (CONCERNS; 1 WARNING verified, evidence fix, no round 2) | none |
| 1.2 version-only commit | CLOSED | 2 diff legs DeepSeek-V4.1-Flash + Qwen3.8-Flash-Next, round 1 — `reviews/L-tasks/1.2/` (CONCERNS → APPROVE: 1 ERROR = pre-existing stale-target hazard, loop-deferred, both legs non-gating; 1 WARNING closed by `go version -m` pin verification) | none |
| 1.3 fix findings | CLOSED-WITH-GAPS (closed by the orchestrator from the task's files after the run-task 5400s cap killed it mid-final-lint; final gates re-run by the orchestrator: lint 0 / format 0 / arch 0 / markdown 0, exit 0 each) | 2 diff legs DeepSeek-V4.1-Flash + Qwen3.8-Flash-Next, round 1 — `reviews/L-tasks/1.3/` (CONCERNS; 3 findings verified real, all fixed in c9e795bb; no CRITICAL, no behaviour defect → no round 2). 57 before → 0 after: 55 fixed, 2 justified (reasoned nolints); `.golangci.yaml` untouched | gaps: CI guard meta-tests not-run locally (CI-covered, inputs untouched); SA1019 conflict-requeue policy delta (rate-limited → fixed 1 s) accepted by review, monitoring note recorded |

## Known red

(none — the 57-finding feature red was cleared by task 1.3: `task lint` 0 findings, exit 0,
at c9e795bb and re-verified by the orchestrator after the 1.3 process timed out mid-close.
The 2 task-prompt.md markdown errors were fixed earlier (a3cd387e; loop.md's b26702e6
attribution here was wrong and is corrected) and a third MD032 introduced by the 1.3
dispatch commit was fixed by the task process.)

## Test changes
(none)

## Lessons
<!-- master list; __LESSONS__ is rendered from it -->
- tooling (task 1.1): never read a long sensor log through `tail` — capture to a file, then
  grep; a truncated finding list propagated a wrong count into evidence until review caught it.
- tooling (task 1.1): after the custom-build `mv`, `bin/golangci-lint` is a plain file, so the
  next `make golangci-lint` re-runs install + custom build (~40 s warm) instead of skipping.
- tooling (task 1.1): golangci-lint `config verify` does not catch an unregistered module
  plugin — the vanilla-build failure is at `run` start (exit 3, "plugin not found"); word the
  structural proof accordingly.
- tooling (task 1.2): `go version -m bin/golangci-lint` reads the resolved plugin module
  version out of the custom binary — one-command proof that a plugin pin is what the build
  used (proposed to change 5 as a DTP-3 companion check).
- tooling (task 1.2): go-task 3.51.1 exits 201 for any failed task and prints the inner code
  (`exit status 2`) only in the message — do not mix the two conventions across gate records.
- tooling (task 1.2): the stale-`bin/golangci-lint`-target hazard also swallows pin-only
  edits (make skips on an already-current binary); only `rm -f bin/golangci-lint*` + a fresh
  build proves the committed pins produce the binary.
- tooling (task 1.3): golangci-lint's `uniq-by-line` (default true) hides goconst findings on
  lines that already carry one — size goconst work with `--uniq-by-line=false` or a "0
  visible" gate is measured against a truncated set (proposed as a change-5 CI check).
- tooling (task 1.3): goconst v1.11.0 emits per (string, file), excludes direct call args
  (`ExcludeTypes: [Call]`), and const declarations never count — constants may share values
  freely without re-triggering goconst.
- tooling (task 1.3): controller-runtime v0.24 `Requeue:true` = AddRateLimited (exponential
  backoff), `RequeueAfter` = fixed delay — the deprecated-field migration is a policy choice,
  per-site justification required.
- process (task 1.3): a focused sensor run that times out on one package must not be dropped
  from the record — the git package's 480s timeout hid two controller test corrections until
  the full-suite run caught them.
- tooling (task 1.3): zizmor is not preinstalled locally; `bash hack/install-zizmor.sh <dir>`
  reproduces the CI-pinned 1.30.1 offline audit in seconds.
- process (task 1.3, orchestrator): the biggest task of a run can outlive the run-task 5400s
  cap (three commits + review + fixes here) — dispatch heavy tasks with a raised limit and
  expect the close to be reconciled from files on a 124.
- process (stage B, orchestrator decision, logged): the round-1 fix batch (6 findings, 2 code
  files + record updates) was applied orchestrator-side instead of a fresh task dispatch — a
  single coherent micro-batch whose context was already loaded; round 2 reviews it with fresh
  legs, which is the protection the L stage would have provided.

## Where a human should look first
<!-- filled at hand-off -->

## Proposed harness changes

| Lesson | Seen in | Proposed change |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
| 2.1 archive lands as PR's last commit | merged by the merge authority; PR must be green at that point |
