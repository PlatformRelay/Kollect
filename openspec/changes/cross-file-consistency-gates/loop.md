# Loop — cross-file-consistency-gates (spec-loop run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/cross-file-consistency-gates` · Branch: `feat/cross-file-consistency-gates` · Base: `origin/main` @ `0c1456b5d` · Started: 2026-10-14 · Reviewers: astra + claude (duo) — `/spec-loop-duo` from G3 on (operator, 2026-10-15); G1–G2 ran the earlier codex + claude/sonnet two-leg per-task sizing
Next: G8 (round-2 behaviour fixes C1, C2; no further review per cap), then hand-off: PR
Budget: claude review legs 11/12 (cap lifted by operator goal 2026-10-16) (B reserve 3) · active hours 0/8 (sessions: 2026-10-14–) · source: default

OpenSpec repo (no `.specify/`): the feature dir is `openspec/changes/cross-file-consistency-gates/`
(proposal.md, specs/cross-file-consistency/spec.md, tasks.md). Landing order position 4 of 10;
1–3 merged (#450, #452, #453). Issue #441.

## Stages

- [x] 0 orient — proposal / spec delta / tasks present, nothing implemented (all unticked); branch and base recorded
- [x] P plan + tasks — skipped: present · spec-set fix committed with this file (see Triage)
- [x] R spec-set review — done by other means before #437 merged: commits `a0aba9880` (independent Opus review), `10bc634ed` (round two), `50580a3ad` (cold adversarial), `1745b60ba` (post-merge status). Not repeated. Re-measure on 2026-10-14 found 3 WARNINGs (Triage), no CRITICAL; mechanical fixes applied
- [x] L task loop — G1–G6 closed (all CLOSED-WITH-GAPS); 6.2 owner/post-merge
- [ ] B branch review — round 1 (`reviews/B-branch/round-1/`, b18354a39): astra:adversarial BLOCK, claude/opus:spec CONCERNS; holistic: task check 2/18 red (lint:markdown = branch, demo_04 = host kubectl), coverage 91.3% = base, vulncheck 0. Triage below; fixes → G7; round 2 (`reviews/B-branch/round-2/`, aa2241523): astra BLOCK, opus CONCERNS; task check 17/18 (lint:markdown = orchestrator regression); race ./internal/... green; A2/A4/O3 closed, H1 regressed, A1 partial → G8. Rounds: 2/2
- [ ] Hand-off

## Fitness functions

| Characteristic | Command | Kind | Baseline | Per task / at B |
|---|---|---|---|---|
| CI guard meta-tests | `bash hack/test/<guard>_test.sh` and `--self-test` (lint job) | triggered | green suite | per task (touched + new guards) |
| Shell hygiene | `task lint:shell` | triggered | no warnings | per task |
| Workflow supply chain | zizmor gate via `hack/test/ci_workflow_security_test.sh` | triggered | 0 high findings | per task touching `.github/` |
| Go build/vet | `go build ./... && go vet ./...` | triggered | clean | per task touching Go/go.mod |
| Lint incl. arch | `task lint` (golangci v2.13.1 + go-arch-lint) | triggered | green | task 1.3 and at B |
| OpenSpec strict | `task spec:validate` | triggered | 13/13 | per spec-set edit |
| Full local gate | `task check` | holistic | green on base | at B |
| Coverage floor | `task coverage` (COVERAGE_MIN 90) | holistic | ≥90% | at B (Go bump) |
| Vulns | `task vulncheck` | holistic | recorded at 1.3 | task 1.3, at B |

Holistic run at B: pending

## Triage

| Stage | Finding (one line) | Sev | Models | Disposition | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| B2 | H1b loop.md MD032 regression from orchestrator commit 800a33e69 | CRITICAL | astra+holistic | fix | orchestrator, blank line; lint:markdown re-run | no |
| B2 | C1 `golang:1.27.2@${D}` / `golang:1.27.2-${X}` pass CFC-1 (golang exempt from `$` rule) | WARNING | opus | fix | 8.1 (no review, cap) | no |
| B2 | C2 comment line inside a `FROM \` continuation hides the image (pre-existing) | WARNING | astra | fix | 8.2 (no review, cap) | no |
| B2 | C3 `mirror.gcr.io/library/golang:x`, `localhost:5000/golang:x` not recognised | WARNING | opus | defer | BACKLOG CFC1-REGISTRY-PREFIX | no |
| B2 | C4 heredoc body `FROM ${B}` trips the `$` rule (false red, fails closed) | WARNING | astra | defer | BACKLOG | no |
| B2 | C5 duplicated labeled collector accepted twice by client_golang; outside CFC-4's name invariant | WARNING | astra | defer | BACKLOG | no |
| B2 | C6 tool-pins-selftest not in protect-main | WARNING | opus | defer | post-merge owner row | no |
| B1 | H1 lint:markdown red (MD032/MD038) in this change's loop/evidence files; preflight would fail | CRITICAL | holistic | fix | 7.1 | no |
| B1 | A1 `ARG B=golang:x` + `FROM ${B}` passes CFC-1 | WARNING | astra | fix | 7.2 | no |
| B1 | A2 metrics guard counts any literal as registered; MustRegister removal stays green | WARNING | astra | fix | 7.3 (Go registry test) | no |
| B1 | A3 kustomize/controller-gen/setup-envtest get only the go-directive check | WARNING | astra | reject | required verify/test jobs compile them under go.mod's Go; CFC-2 noted partial | no |
| B1 | A4 go_toolchain self-test leaks 31 temp dirs per run | WARNING | astra | fix | 7.4 | no |
| B1 | O1 tool-pins-selftest not in protect-main | WARNING | opus | defer | post-merge owner row (orchestrator) | no |
| B1 | O2 required lint job now network-bound (tool-pins plain) | WARNING | opus | reject | task lint in the same job already downloads modules; trade-off in decisions.md | no |
| B1 | O3 rbac guard tracebacks on malformed exception line (still red) | NOTE | opus | fix | 7.5 | no |
| B1 | O4 literal scan over-counts; O5 Summary `_sum`/`_count` rejected | NOTE | opus | defer | BACKLOG (CFC-4 follow-ups) | no |
| R | proposal quotes stale versions (1.26.6 / 1.27.1); now go.mod 1.26.9, Dockerfiles 1.27.2 | WARNING | re-measure | fix | proposal.md updated | no |
| R | `.devcontainer/devcontainer.json` pins `golang:1.26.9`; CFC-1 scanned only Dockerfiles, so it drifts after the bump | WARNING | re-measure | fix | CFC-1 + scenario + tasks 1.2/1.3 include the devcontainer image | no |
| L/G2 | CFC-2 said `GOTOOLCHAIN=local`; on a host whose Go differs that tests the wrong Go | WARNING | claude/sonnet | fix | spec + proposal amended to `GOTOOLCHAIN=go<version>` (exactly go.mod's Go) | no |
| L/G3 | CFC-3 tenant scenario exempted only cluster-scoped triples; workload reads are deliberately team-granted (team-operator.md §3) | WARNING (spec) | probe | fix | operator ruling A 2026-10-15: tenant scenario lists workload-collection reads as exceptions | done |
| R | `lint` is not a `protect-main` required check; CFC-6 "required job" depends on change 1's operator step 5.1 | WARNING | re-measure | fix | operator ruling 2026-10-15; ruleset 18802410 now also requires lint, workflow-security, dependency-review (applied via gh api) | done |

## Tasks

| Task | Verdict | Review (legs, rounds, Claude?) | Gaps / decision request |
|---|---|---|---|
| G1 (1.1–1.3) | CLOSED-WITH-GAPS | codex + claude/sonnet, 2 rounds on c2b744ac0..fc959302c; tool-pin commits reviewed separately (codex APPROVE, sonnet CLEAN); no opus | Was BLOCKED on 1.3; operator chose A: golangci-lint v2.14.0 (fa9bc99be), govulncheck v1.9.0 (812c7f274), plus go-arch-lint v1.15.0→v1.19.0 (9d38f7431) — NOT in the ruling, same x/tools-under-Go-1.27 failure, version-only, accepted by orchestrator. Gaps: CodeQL on Go 1.27 proven only by CI Analyze (Go); guard unwired until 6.1; Renovate grouping proven by a one-off local dry-run |
| G2 (2.1) | CLOSED-WITH-GAPS | codex + claude/sonnet, 2 rounds (round 1 REQUEST_CHANGES verified/fixed; round 2 REQUEST_CHANGES verified/fixed, no round 3); no opus | Commits 13b263c1f, f62e76e01, b564d1f49. Gap: unwired until 6.1. Self-test ~5 min → own lint step at 6.1. Deferred to B: parallel-unsafe fixture deletion from the shared module cache; misleading message for a literal golangci pin in a Makefile recipe (fails closed). Orchestrator lost ~80 min to a self-matching `pgrep -f` wait loop (killed). CFC-2 `GOTOOLCHAIN=local` → amended to exactly go.mod's Go (orchestrator decision, spec-set commit) |
| G3 (3.1) | CLOSED (3.1); 3.2 was BLOCKED → resolved | none (probe evidence only, 8178695c2) | Default render covers all 113 role.yaml triples (no drift). Tenant render misses 36: 30 cluster-scoped (spec-named) + 6 namespaced workload reads (deployments, certificates g/l/w) → SPECIFICATION block; operator chose A 2026-10-15: exempt workload reads per team-operator.md §3 (spec amended). Deferred to B/backlog: default ClusterRole over-grants services CRUD + deployments write; tenant Role grants profiles/scopes write (CFC-3 is one-directional) |
| G3 (3.2) | CLOSED-WITH-GAPS | astra + claude/opus:diff (RBAC = security posture), 2 rounds: r1 astra BLOCK / opus CONCERNS (team-workload accepted any read incl. secrets; leader-election Role counted as ClusterRole; Role covering cluster-scoped) all verified+fixed; r2 astra REQUEST_CHANGES (test isolation, fixed) / opus CLEAN | Commits 913a08ffa, f3489c06f, 1b5b7b78d. Mode-scoped exception lines with closed reason classes; 37 tenant exceptions, no default drift. SSAR (cluster-scoped, granted to all via system:basic-user) added to the spec's cluster-scoped list by orchestrator. Deferred: tenant Role's no-op SSAR rule (chart change); unwired until 6.1 |
| G4 (4.1) | CLOSED-WITH-GAPS | astra + claude/opus:diff (new workflow), 2 rounds: r1 astra REQUEST_CHANGES (octal leading-zero false green, curl past job timeout) / opus CONCERNS, fixed; r2 astra REQUEST_CHANGES (2^63 overflow false green, fixed unreviewed per cap) / opus CLEAN | fba3eef06 (review-fix commits squashed in, local). Live sensor green (1.27.2 current). Needs `&include=all`. Deferred: tie curl retry budget to timeout-minutes (6.1/later). Gaps: wire both modes at 6.1; first scheduled run = post-merge row. Fresh worktree needed `make operator-sdk` for dist_operatorhub_pr_test.sh (ENVIRONMENT) |
| G5 (5.1–5.2) | CLOSED-WITH-GAPS | astra only, 2 rounds: r1 REQUEST_CHANGES (prefix typo passed; gate-mutant runner exit 0 on survivor) fixed; r2 REQUEST_CHANGES (const mirror entry dropped; backtick import missed) fixed unreviewed per cap, each with mutants | Real drift: 5 registered metrics missing from metrics.md → docs-only fix 0dbc6302a. Guard 05842fe59..dcf2be244. Gaps: concatenated/Sprintf metric names invisible to the literal scan (none today); wiring at 6.1 |
| G6 (6.1) | CLOSED-WITH-GAPS | astra + claude/opus:diff (CI), 2 rounds (r1 astra APPROVE / opus CONCERNS; r2 astra REQUEST_CHANGES / opus CONCERNS, all verified fixed); decision-A follow-up: astra only (L Claude legs exhausted; B opus covers it), APPROVE | 300801927, 3915645d4, 366dc0840; decision A (operator 2026-10-16): tool-pins self-test in its own unconditional job `tool-pins-selftest` (timeout 45 min, in verify-eligibility.sh required_checks) d770c56f3, b94b69066 (unique fixture module per run — G2 race fixed), 2a9b5b946. ci_workflow_security CFC-6 rule now fails on an unwired guard. Gaps: first CI run on PR head; add `tool-pins-selftest` to protect-main AFTER merge; runner runtimes estimated |
| G7 (7.1–7.5) | CLOSED-WITH-GAPS | astra only, 2 rounds (r1 REQUEST_CHANGES: registry-port bypass, Register() untested — fixed; r2 REQUEST_CHANGES: `localhost:${B}` bypass — fixed unreviewed per cap, mutants) | a3880bc47..7d5a4d790. Any `$` in a FROM image fails; metrics registration proved via Describe + AlreadyRegisteredError; TestRegister now uses registerOnce (test change: double Register panics). Deferred to BACKLOG: `localhost:5000/golang:x` not recognised as golang; `go test -count=2 ./internal/metrics/` panics on base |

## Known red

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|

## Lessons

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

## Proposed harness changes

| Lesson | Seen in | Proposed change |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
| post-merge | add `tool-pins-selftest` to protect-main required checks (orchestrator, gh api, right after merge) |
| 6.2 | merge with the `post-merge` row open; archive in a follow-up evidence PR after the first scheduled `go-patch-lag` run |
