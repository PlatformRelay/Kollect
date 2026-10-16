# Loop — ci-failure-reporting (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/ci-failure-reporting` · Branch: `feat/ci-failure-reporting` · Base: `origin/main` @ `81637dcde` (change 6 archive merged) · Started: 2026-10-16 · Reviewers: astra + claude (duo)
Next: hand-off — integrator: pr-description, ready PR, all checks green, merge; post-merge: main CI by headSha + first ci-failure-report run after the next E2E nightly; archive after that run
Budget: Claude review legs uncapped (operator goal 2026-10-16) · run under GOAL-2026-10-16-announcement-ready (decisions by model alignment, logged in agent-context/decisions.md)

Change 7 of 10 (issue #444). Spec set: proposal.md, specs/ci-failure-reporting/spec.md, tasks.md.

## Stages

- [x] 0 orient — re-measured against `origin/main` @ `81637dcde` (2026-10-16); scope decision A applied (decisions.md 2026-10-16 "Change 7 ci-failure-reporting scope")
- [x] P plan + tasks — present; spec set updated 98ed12ff4 (re-measure, decision A, red-first order)
- [x] R spec-set review — round 1 (43a6b4525): astra NO-GO (3 CRITICAL, verified → WARNING-level, treated as accepted), opus CONCERNS; R1–R15 fixed 75a764bc4, R16 → non-goal. Round 2 (last): astra GO WITH FIXES, opus CONCERNS; 9 mechanical findings fixed b7fcce05a (A1 gh issue close has no --body-file; C3 gh issue list default limit 30). No CRITICAL remains. Records `reviews/R-spec-set/` + `round-2/`
- [x] L task loop — 1.1–5.1 closed (6.1 is hand-off); task subagents recorded their own rows
- [x] B branch review — round 1 (2e02ba15a): task check exit 0 (1091 s); per-commit gate green; astra REQUEST_CHANGES (B1, B2 WARNING), claude/opus:spec CLEAN (CFR-1..7 hold); no CRITICAL → no round 2; fixes as tasks 7.1–7.4 with normal task review
- [x] Hand-off — `pr-description.md` (ready PR, integrator)

## Fitness functions

| Characteristic | Command | Kind | Baseline | Per task / at B |
|---|---|---|---|---|
| CI guard meta-tests | `bash hack/test/<guard>_test.sh` and `--self-test` (incl. `ci_failure_report_test.sh` once it exists) | triggered | green | per task |
| CI wiring and zizmor gate (CWS-1..7) | `hack/test/ci_workflow_security_test.sh` and `--self-test`; `zizmor --offline --no-progress --min-severity=high --config .github/zizmor.yml .github/` | triggered | green; zizmor no findings | per task touching `.github/` or guards |
| Shell hygiene | `task lint:shell` | triggered | no warnings | per task |
| Markdown | `task lint:markdown` (tracked files only) | triggered | 0 issues | per task + every orchestrator state edit |
| OpenSpec strict | `task spec:validate` | triggered | green | per spec-set edit |
| Evidence is Markdown | `hack/test/openspec_evidence_markdown_only_test.sh` and `--self-test` | triggered | green | per task |
| Full local gate | `task check` | holistic | green on base | at B |

Holistic run at B: not run yet.

## Triage

| Stage | Finding (one line) | Sev | Models | Disposition | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| 0 | scope: run-conclusion only, keep Go patch lag, label created by the workflow | decision | astra+opus | fix (A) | spec set 98ed12ff4; race gap is a non-goal pointing at SINK-BREAKER-RACE-01 | operator-accepted |

| B1 | B1 malformed/empty gh output with exit 0 → duplicate issue/comment | WARNING | astra | fix | 7.1 | no |
| B1 | B2 deleting both lint steps undetected outside the test (not in CFC-6 list) | WARNING | astra | fix | 7.2 | no |
| B1 | B3 workflow-level concurrency lets a dispatch displace a pending scheduled report | NOTE | opus | fix | 7.3 (cheap) | no |
| B1 | H1 hack/check.sh grep -v \| grep -q SIGPIPE silently skips 3 large guards' --self-test (pre-existing) | WARNING | holistic | fix | 7.4, on this branch (touches the same gate) | no |
| B1 | B4 failed-job list over cap → no issue (impossible with 11 jobs); B5 self-wiring matches any ci.yaml line | NOTE | opus | accept | B4 bounded by job count; B5 CWS-6 requires the required job | no |
| L/5.1 | 3-min --self-test kept in lint (not tool-pins-selftest) despite the 2026-10-16 ruling on slow self-tests | decision | orchestrator | accept | ruling's aim was bounding lint (was heading to ~20 min); lint ~11–12 → ~15 min stays under tool-pins-selftest (19–20 min, the slowest required job), which adding it there would lengthen | no |
| L/T1.1 | F1 `--label` routes gh issue list to search; F2 comment authors lack is_bot; F3 started = non-empty steps; F5 empty scheduled-run list unspecified | WARNING (spec) | probe | fix | spec: author-only lookup + client-side label; comment-author rule; started-job definition; empty list → no write, exit 0 (orchestrator) | no |

## Tasks

| Task | Verdict | Review (legs, rounds, Claude?) | Gaps / decision request |
|---|---|---|---|
| 1.1 | CLOSED-WITH-GAPS | astra, 1 round (REQUEST_CHANGES, evidence only, fixed); no Claude leg (fixtures and evidence only) | timed-out job shape not seen live (F7); concurrency replacement and 65536 cap not probable read-only; F1 lookup argv has no `--label`, F2 comment author shape, F5 empty run list: inputs for 3.1–3.3 |
| G2 (2.1, 2.2) | CLOSED-WITH-GAPS | astra + claude/opus:diff (CI touched), 2 rounds: R1 astra REQUEST_CHANGES, opus CONCERNS (fixed); R2 astra APPROVE, opus CONCERNS (one test-strength gap, deferred) | line-cut at 4000 bytes unasserted (M20 survives): add `[line cut]` assertion and a cut-removed mutant in 5.1 |
| G3 (3.1, 3.2, 3.3) | CLOSED-WITH-GAPS | astra + claude/opus:diff (token-writing script: security posture), 1 round: astra APPROVE, opus CONCERNS (payload-guard test gap, fixed with 5 cases) | live gh shapes read by the implementer (row 29); lookup ceiling 500 shared with every github-actions issue (recorded in G3 Harness gaps) |
| G4 (4.1, 4.2) | CLOSED-WITH-GAPS | astra + claude/opus:diff (privileged `workflow_run` workflow, CI and security posture), 1 round: astra REQUEST_CHANGES, opus CONCERNS (guard-strength gaps and wording, all fixed, fix-line mutants 9/9) | header comment agent-review; workflow firing on GitHub is the 6.1 post-merge row; one test change: CWS-2 self-test mutant anchors (`rules: {}` gone) |
| 5.1 | CLOSED-WITH-GAPS | astra + claude/opus:diff (CI wiring), 2 rounds: R1 astra APPROVE, opus CONCERNS (bump control could become a no-op; no cws control; both fixed, fix-line mutant red); R2 astra APPROVE, opus CLEAN (comment wording fixed; multi-name filter typo note deferred) | filter env var is believed harmless in CI (no CI step sets it); `task check` is the stage-B row |
| G7 (7.1–7.4) | CLOSED-WITH-GAPS | astra + claude/opus:diff (CI wiring, privileged workflow, local gate), 2 rounds: R1 astra APPROVE, opus CONCERNS (R1-1 jobs read unchecked: behaviour, fixed red-first; two comment fixes); R2 astra APPROVE, opus CONCERNS (notes: two wording fixes, one rejected) | 7.3 runtime semantics believed (job `if:` before job concurrency); final `task check` 29:58, one ENVIRONMENT red (demo_04: mise kubectl shim unset in the worktree), guard rerun green with a real kubectl |

## Known red

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|
| `hack/test/ci_workflow_security_test.sh`: seven CWS-2 self-test mutants and control | ci-workflow-hardening | G4 | anchored on the literal `rules: {}`, which tasks.md 4.2 replaces; mutants no longer applied. Same rule text and expected message, anchor now the `rules:` line (evidence/G4.md *Test changes*) |
| `hack/test/ci_workflow_security_test.sh`: CFC-6 `cfc_guards` list, one mutant added | cross-file-consistency-gates (CFC-6) | G7 (7.2) | B2: the list omitted a guard that checks its own wiring only when run; entry and mutant added, nothing weakened (evidence/G7.md *Test changes*) |
| `hack/test/ci_failure_report_test.sh` self-test: G3 M17 mutant | 5.1 | G7 (7.1) | B1's comments shape check made the mutant equivalent (survived); it now also drops that check, same case and message |
| `hack/test/ci_failure_report_test.sh`: concurrency wiring assertion, W4 mutant | G4 (4.1), 5.1 | G7 (7.3) | tasks.md 7.3 moves the block to the job: same group and value, now also no workflow-level block |

## Lessons

This change (ci-failure-reporting):

- `gh issue list --label` runs GraphQL search (gh 2.98.0); only label-free `--author 'github-actions[bot]'` uses the list API: check `GH_DEBUG=api` before calling an argv "not search" (1.1)
- A job started iff its `steps` array is non-empty; `startedAt` is set on jobs that never ran (1.1)
- `gh issue view --json comments` spells the bot `github-actions` (no `is_bot`), issues spell it `app/github-actions` (1.1)
- The Bash tool's zsh does not word-split `$var`: run loops over flag strings under `bash` (1.1)
- A codex review leg here has no network: live-shape checks stay with the implementer (1.1)
- A size-cap case must make the input too big to fit uncut, or a removed cut passes it (G2)
- A test's own Markdown fence oracle needs a fixed-text control case (G2)
- Each escaped character class needs its own killing case: three escapes survived deletion at 15/15 (G2)
- Review legs here cannot write /tmp: put any check they must rerun in the evidence, not in a /tmp script (G2)
- A "no write" case for a bad id must also assert the call that returned it was the last; later checks hide its unchecked use (G3)
- Scope a forbidden-flag grep to the call it guards: `--label` on `issue create` is legitimate (G3)
- Read a fake's `--limit` from the recorded argv, then serve exactly that many items (G3)
- Scan raw `run:` bodies for `${{`: GitHub expands expressions before the shell, comment lines included; forbid every `${{` (G4)
- A file-name zizmor exception covers every trigger in the file: pin `on:` keys exactly (G4)
- Before replacing a literal in a config, grep existing self-tests for mutants anchored on it (G4: `rules: {}`)
- A per-mutant self-test of a slow guard needs a case filter in the guard first: 43 s plain x 85 mutants is an hour, filtered 3.5 min (5.1)
- Mutate shell code with an exactly-once literal replace, not sed: no escaping, and a vanished anchor fails loudly (5.1)
- Before moving a self-test out of `lint`, compare job wall times: lint 12 min < tool-pins-selftest 20 min, so lint was the free slot (5.1)
- After adding a defensive check, rerun the self-test: it can make an old mutant equivalent (G7: the B1 shape check silenced G3 M17)
- Apply a finding's rule to every call of that kind, not only the ones it lists (G7: the jobs read had B1's defect, found in review)
- `grep >/dev/null` avoids pipefail's SIGPIPE only because GNU grep drains its input; keep a large-file test on the detection (G7)
- Fixing a silent skip turns on never-run work: `task check` went from ~18 to ~30 min, and the skipped self-tests ran for the first time (G7)
- A fresh worktree's mise sets no kubectl: put a real kubectl first on PATH for the kustomize guards in `task check` (G7)

Carried over from change 5 (developer-toolchain-pins):

- A shrink-only bash list needs a closed list of allowed mentions, not a list of write forms (1.1)
- Self-test mutant values must never be a plausible next release: use 99.0.0 or derive from the tree (1.1)
- A mutant probe must tag inserted lines with a marker; a regex matching existing lines proves nothing (1.1)
- In single-quoted awk inside bash, an apostrophe in an awk comment ends the quote (1.1)

- Run hand mutants on ported code, not only the ported cases (1.2: section-off survived all 12 old cases)
- A comment naming the check that enforces a rule must match that check's real scope (1.2)
- Run hand-mutant self-tests one at a time; parallel runs gave spurious exit 2 (1.2)

- When two clauses compare one value, give the value to one and keep presence in the other (2.1)
- Line-address a hand-mutant sed when the target text repeats (2.1)

- A "reads the variable" check must forbid task-level overrides (Task's task vars beat global vars) (2.2)
- Match the bot's extraction regex exactly (one space, no trailing blanks) (2.2)

- Before adding a rule over a file earlier mutants edit, run every mutant touching that file first (2.3)
- Hand-mutate each regex alternative; one was redundant (2.3)

- A self-test keeping one tree copy per case exhausts shared /tmp inodes: free each copy before the next (G3)
- A Renovate model must reject what RE2 rejects (lookaround, backrefs, atomic groups) (G3)

- A tamper fixture must still extract (flip gzip MTIME byte 4) so the test fails on the checksum, not tar; add a correct-digest-under-another-asset case (G4)

- Run long self-tests on a throwaway snapshot (tracked files, git init, add -A -f) so editing can continue (G6)
- `^v[0-9]` lets a go query like `v1` through: require full semver (G6)

Carried over from change 4 (cross-file-consistency-gates):

- `task lint:markdown` lints only git-tracked files: a probe file needs `git add -N` or it is skipped with exit 0 (G8)
- Orchestrator: run `task lint:markdown` on every state-file edit before committing (B2 regression from 800a33e69)
- A variable image tag can contain `/` and `:` and name another repo: fail on any `$` in an image, never parse around it (G7)
- The Bash tool runs zsh here: `${PIPESTATUS[0]}` is empty — use `set -o pipefail` or bash -c (G7)
- A wiring rule must take its required modes from the spec, not from the guard's own code; anchor sed mutants on stable prefixes, not exact config lines (G6)
- Sensors: match whole tokens of the target grammar (Prometheus `[A-Za-z0-9_:]`), and make a parser fail on leftover input it does not recognise (G5)
- The block-ai-mentions hook checks the whole Bash command string: keep model names in evidence out of any command that also runs `git commit` (G5)
- Validate numbers as bounded + canonical before bash `(( ))`: inside `if` an arithmetic error is false and big values wrap → false green in a fail-closed sensor (G4)
- Under pipefail, `grep -v | grep -q` can SIGPIPE and read as no-match; use `grep >/dev/null` (G4)
- A fresh worktree lacks bin/operator-sdk: run `make operator-sdk` before the dist_* guards (G4)
- RBAC coverage must respect role kind and scope: never pool a namespaced Role with a ClusterRole; a Role never covers a cluster-scoped resource or subresource (G3)
- An exception class must be a closed list, or "any other X fails" is undone by one line (G3)
- A negative mutant must pass every check except the clause it names; one gate mutant per clause proves it; a file-deleting mutant needs an absence probe (G3)
- Write variable rm targets as "${R:?}"/... — the safety hook refuses `rm -f $R/*` (G3)
- Check a spec's exception list against the product docs' "by design" sections before cutting tasks (G3)
- `helm template --show-only` errors when the template renders nothing (clusterrole.yaml in tenant mode): render all, filter on `# Source:` (G3)
- `python3 -I` loses user-site PyYAML on this host; use plain `python3` as the repo guards do (G3)
- Wait on PIDs (`wait $pid`, `while kill -0 $pid`), NEVER `pgrep -f <pattern>` where the pattern appears in your own command line — it matches itself and never ends (G2, ~80 min lost)
- A bash helper called as `$(helper)` runs in a subshell and loses global state: register cleanup/temp roots in the parent shell (G2)
- `go mod download` under an exact GOTOOLCHAIN refuses a module needing a newer Go: parse "requires go >=" (G2)
- Guard smoke builds must build each pin at its own version and assert the built version; never reuse bin/ (G2)
- Clean up temp dirs in an EXIT trap; a leaking guard left 298 dirs in /tmp (G2)
- A Go minor bump breaks every pinned tool embedding golang.org/x/tools (golangci-lint, go-arch-lint, govulncheck at 1.27.2); 2.1 must cover all `go run`/`go install` tools; re-run `task lint` after each tool fix — the next broken step only shows once the earlier passes (G1)
- Run `task test` with `umask 022` on this host; umask 002 reds two internal/sink/git mirror tests at base too (G1)
- mise shims for gitleaks/jq fail in some dirs: use ~/.local/share/mise/installs/gitleaks/8.30.1/gitleaks; keep mise.toml out of scratch copies (G1)
- Renovate: the LAST matching rule wins, so a group rule must be the last groupName-setting rule; prove grouping with a local `renovate --platform=local` lookup dry-run (~1 min) (G1)

## Where a human should look first

- `.github/workflows/ci-failure-report.yaml` — new privileged `workflow_run` workflow (issues:write); exactly 3 steps, job-level concurrency, no checkout of the triggering ref
- `.github/zizmor.yml` — commented `dangerous-triggers` file exception for that workflow
- `hack/ci/ci-failure-report.sh`, `hack/ci/ci-failure-issue-body.sh` — the reporter and its hardened body builder
- `hack/check.sh` — `task check` now really runs the --self-test of 5 large guards it silently skipped (SIGPIPE); `task check` ~18 → ~30 min
- `hack/test/ci_workflow_security_test.sh` — CFC-6 list gains ci_failure_report_test.sh; CWS-2 mutants re-anchored (test changes)
- `.github/workflows/ci.yaml` — lint job gains the reporter test steps (~3 min self-test); lint ~12 → ~16 min

<!-- filled at hand-off -->

## Proposed harness changes

| Lesson | Seen in | Proposed change |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
