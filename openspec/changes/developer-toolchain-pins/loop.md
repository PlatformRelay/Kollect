# Loop — developer-toolchain-pins (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/developer-toolchain-pins` · Branch: `feat/developer-toolchain-pins` · Base: `origin/main` @ `966e17e10` (change 4 merged) · Started: 2026-10-16 · Reviewers: astra + claude (duo)
Next: B round 2 (last) on the G6 range, then hand-off
Budget: Claude review legs uncapped (operator goal 2026-10-16) · run under GOAL-2026-10-16-announcement-ready (decisions by model alignment, logged in agent-context/decisions.md)

Change 5 of 10 (issue #442). Spec set: proposal.md, specs/developer-toolchain/spec.md, tasks.md.

## Stages

- [x] 0 orient — re-measured against change 4's tip (2026-10-16): 6 mechanical edits + 2 model-aligned decisions
- [x] P plan + tasks — present; spec set updated 649e5811c (re-measure, decisions A pin ownership and C guard shape)
- [x] R spec-set review — `reviews/R-spec-set/` (b3fd2d944) on 9621c98aa: astra REQUEST CHANGES, claude/opus CONCERNS; no CRITICAL; 11 WARNING + 6 NOTE all verified; fixes applied faa6754fd (R2 placement deviation: lint wiring lands with 1.1 because CFC-6 reds an unwired consistency_* guard; R9 mutants in 2.2)
- [x] L task loop — 1.1–4.3 closed (5.1 archive is post-merge)
- [ ] B branch review — round 1 `reviews/B-branch/round-1/` (650d7c765): task check exit 0 (1528 s), vulncheck 0; astra REQUEST_CHANGES (B1 P1), opus CONCERNS; no spec row contradicted/absent; fixes → G6
- [ ] Hand-off

## Fitness functions

| Characteristic | Command | Kind | Baseline | Per task / at B |
|---|---|---|---|---|
| CI guard meta-tests | `bash hack/test/<guard>_test.sh` and `--self-test` | triggered | green | per task |
| CI wiring (CFC-6) | `hack/test/ci_workflow_security_test.sh` both modes | triggered | green | per task touching ci.yaml or guards |
| Shell hygiene | `task lint:shell` | triggered | no warnings | per task |
| Markdown | `task lint:markdown` (tracked files only) | triggered | 0 issues | per task + every orchestrator state edit |
| Tool pins (CFC-2) | `hack/test/consistency_tool_pins_test.sh` | triggered | green | task 2.2 |
| OpenSpec strict | `task spec:validate` | triggered | green | per spec-set edit |
| Evidence is Markdown | `hack/test/openspec_evidence_markdown_only_test.sh` | triggered | green | per task |
| Full local gate | `task check` | holistic | green on base | at B |

## Triage

| Stage | Finding (one line) | Sev | Models | Disposition | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| B1 | B1 hard-coded git-cliff v2.13.1 sentinel: a correct bump reds --self-test and blocks the shared group | P1 | astra (reproduced) | fix | 6.1 + all-pins bump control | no |
| B1 | B2 new 233 s --self-test in required lint, against operator ruling 2026-10-16 (slow self-tests out of lint) | WARNING | opus | fix | 6.2: step in existing tool-pins-selftest job (no new required context); orchestrator applies precedent, no new decision | no |
| B1 | golangci-lint in shared Renovate group; a red golangci bump blocks Task/gitleaks/git-cliff/govulncheck | WARNING | opus (unmentioned behaviour) | fix | 6.7 own group | no |
| B1 | queued: macOS shasum fallback; helm-docs 64-hex; CFC-2 `latest` via var; case-insensitive uses:; task=latest case | — | earlier tasks | fix | 6.3–6.6 | no |
| B1 | B3 stale line ref; B4 dotted numbers fail closed; B5 one dead file pattern fails manager; B6 extends presets ignored | NOTE | opus | fix (text) | 6.8 | no |
| R | R1 shellcheck manager bump would always be red (digest table has only 0.11.0); shared group blocks | WARNING | opus | fix | own group, all 3 sites, missing-digest guard case | no |
| R | R2 1.1 left red until 2.x | WARNING | opus | fix | each check lands with its fix task | no |
| R | R3 DTP-7 vs installer-scan owner | WARNING | opus+astra | fix | fetch-lib gate owns the scan; tamper tests in new guard --self-test | no |
| R | R4 UNVERIFIED_BY_DESIGN shrink-only never tightened | WARNING | opus | fix | 4.3 limit = empty | no |
| R | R5 liveness mechanism unspecified | WARNING | opus | fix | 3.1 records mechanism + Renovate version | no |
| R | R6 tamper tests only negative | WARNING | astra | fix | valid-archive positive case first | no |
| R | R7 comment-to-pin rule undefined | WARNING | astra | fix | block-above/inline rule + Node/Python/Taskfile cases | no |
| R | R8 CWS-6 exits before CFC-6 | WARNING | astra | fix | scenario + both-steps-removed case | no |
| R | R9 vulncheck var not proven | WARNING | astra | fix | other-var + missing-def mutants (2.2) | no |
| R | R10 git-cliff pins at release.yaml:420, verify-changelog.sh:11 missed | WARNING | reviewer-verifier | fix | DTP-3/DTP-4 cover both (literal; action input can't read a Taskfile var) | no |
| R | R11 setup.md:39 restates Task version | WARNING | reviewer-verifier | fix | 2.1 rewrites without version | no |
| R | R12/R13/R15/R17 datasource, rule order, template pointer, gitleaks v-prefix | NOTE | mixed | fix | spec set | no |
| R | R14 sha512sum missing on stock macOS | NOTE | opus | defer | BACKLOG DTP-SHA512-MACOS | no |
| R | R16 change 4 archive state | NOTE | opus | reject | CFC-2/6 are merged code; archive state irrelevant | no |
| 0 | DTP-2/DTP-3 floors + golangci agreement duplicate CFC-2 | decision | astra+opus | fix (A) | dropped; CFC-2 owns analysis-tool pins | logged, pending validation |
| 0 | guard shape: extend dev_mise vs new consistency_* guard | decision | astra+opus | fix (C) | new consistency_toolchain_pins_test.sh; dev_mise folded (task 1.2) | logged, pending validation |

## Tasks

| Task | Verdict | Review (legs, rounds, Claude?) | Gaps / decision request |
|---|---|---|---|
| 1.1 | CLOSED-WITH-GAPS | astra + claude/opus:diff (CI), 2 rounds, all behaviour findings fixed with mutants | 30a377146. Also fixed a pre-existing flake on main: ci_workflow_security_test.sh guard_modes `grep -v \| grep -q` under pipefail dropped --self-test 73/500 → 0/500. Gaps: round-2 fixes unreviewed (B: read `unverified_other_uses`, `cliff_action_sites` first); DTP-6 scan blind to computed-name eval/sourced writes. Deferred: SIGPIPE at ci_workflow_security_test.sh:756 (its own self-test) |
| 1.2 | CLOSED-WITH-GAPS | astra APPROVE + claude/opus:diff CONCERNS (wording, fixed), 1 round | 6fe60b5a7. dev_mise_pin_drift_test.sh folded: 12 cases ported 1:1 + a 13th (section-off survived all 12 old cases); old guard + ci.yaml step deleted; refs updated. Test changes: old guard deleted, CWS-6 mutant retargeted. Deferred to 2.1: step-reset-off (setup-task step without `version:` silently skipped) |
| 2.1 | CLOSED | astra APPROVE + claude/opus:diff CLEAN, 1 round | 8d9a13d4c. Task 3.52.0 at 18 setup-task sites + mise.toml; setup.md:39 version-free; step-reset-off closed (setup-task step without `version:` reds). Test changes: 1.2 cases task-drift/ci-self-drift now expect DTP-1 message; no-ci-pins split. Deferred to B: case-sensitive `uses:` match (shared reader); no `task = "latest"` case |
| 2.2 | CLOSED-WITH-GAPS | r1 astra APPROVE + claude/opus:diff CONCERNS (2 WARNING verified: `latest` passed both guards; task-level vars override ran v1.1.4 → fixed); r2 astra APPROVE | fc1135a99. GOVULNCHECK_VERSION var + renovate annotation (dry-run extracts it); `task vulncheck` green. Test change: 2 CFC-2 mutants re-anchored. Deferred to B (fix there): CFC-2 accepts `latest` via any Taskfile var (consistency_tool_pins_test.sh:159-163) |
| 2.3 | CLOSED-WITH-GAPS | r1 astra REQUEST_CHANGES (bare majors, -rc truncation, case-sensitive count rule → fixed) + claude/opus:diff CLEAN (lint step renamed); r2 astra APPROVE | 25efc58f9. DTP-5 comment-to-pin rule; mise.toml :24/:28/:39/:52 rewritten as pointers; Task comment no longer restates a version (Renovate would red every bump). Test changes: 4 ported DEV-MISE-01 mutants move/delete attached comments. Gaps: dotted prose numbers fail closed; `# gitleaks 8` passes |
| G3 (3.1–3.3) | CLOSED-WITH-GAPS | r1 astra REQUEST_CHANGES + claude/opus:diff CONCERNS; r2 astra REQUEST_CHANGES (2 P2, fixed unreviewed per cap) | c01c6ab49, d2354912a. Liveness = file hit AND ≥1 extracted dep; mechanism = python model reading renovate.json (matches Renovate 44.115.4 extract: 29 deps today / 41 final); Renovate not run in lint (groups need lookup + GitHub token). New gitleaks/shellcheck/shellcheck-py/git-cliff/golangci managers; shellcheck own group before go toolchain. Test changes: 3 earlier cases rewritten, ALSO_RED on 7 cases. Gaps: model ≠ Renovate (presets, minimatch); rewrite simulated from replace string |
| G4 (4.1–4.3) | CLOSED-WITH-GAPS | astra APPROVE + claude/opus:diff CONCERNS (1 WARNING, 3 NOTE), 1 round | 5bb2e11ff, e814eeddf, 3b05a8148. git-cliff verifies per-asset .sha512 (inline sha512sum), helm-docs verify_sha256 vs checksums.txt; UNVERIFIED_BY_DESIGN and guard limit EMPTY; real installs on linux/amd64. Test changes: mutant relabel; fetch-lib scan messages SHA256→checksum. **Orchestrator override:** macOS (no sha512sum/sha256sum) now fails closed → fix at B with `shasum -a` fallback, not deferred (operator works on macOS). Deferred to B: helm-docs digest not checked as 64 hex. Gaps: Darwin/arm64 install paths probed by URL only |
| G6 (6.1–6.8) | CLOSED-WITH-GAPS | r1 astra REQUEST_CHANGES (2 P2) + claude/opus:diff CONCERNS (1 WARNING, 6 NOTE) fixed; r2 astra APPROVE | 632246c28..58c516203. P1 reproduced then fixed (all-pins bump control, 41 sites); toolchain self-test moved to tool-pins-selftest job (7.6+7.4 min, 45 min timeout); shasum fallback; helm-docs 64-hex; CFC-2 full-semver for literal + Taskfile-var pins (`latest`, `v1` floated); case-insensitive uses:; golangci own group. Test changes: change-4 CFC-2/CFC-6 mutants adjusted. CAVEAT: 10 commits split from one tested tree, intermediates not run individually. Gaps: no real Darwin run; shim branch for no-shasum hosts never ran. Deferred: Makefile var `latest` / literal `@latest` in CFC-2 → BACKLOG (ENVTEST_VERSION branch pin needs its own decision) |

## Known red

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|

## Lessons

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

## Proposed harness changes

| Lesson | Seen in | Proposed change |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
