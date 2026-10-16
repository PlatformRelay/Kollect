# Loop — developer-toolchain-pins (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/developer-toolchain-pins` · Branch: `feat/developer-toolchain-pins` · Base: `origin/main` @ `966e17e10` (change 4 merged) · Started: 2026-10-16 · Reviewers: astra + claude (duo)
Next: L — dispatch 1.1 (new consistency_toolchain_pins_test.sh + lint wiring)
Budget: Claude review legs uncapped (operator goal 2026-10-16) · run under GOAL-2026-10-16-announcement-ready (decisions by model alignment, logged in agent-context/decisions.md)

Change 5 of 10 (issue #442). Spec set: proposal.md, specs/developer-toolchain/spec.md, tasks.md.

## Stages

- [x] 0 orient — re-measured against change 4's tip (2026-10-16): 6 mechanical edits + 2 model-aligned decisions
- [x] P plan + tasks — present; spec set updated 649e5811c (re-measure, decisions A pin ownership and C guard shape)
- [x] R spec-set review — `reviews/R-spec-set/` (b3fd2d944) on 9621c98aa: astra REQUEST CHANGES, claude/opus CONCERNS; no CRITICAL; 11 WARNING + 6 NOTE all verified; fixes applied faa6754fd (R2 placement deviation: lint wiring lands with 1.1 because CFC-6 reds an unwired consistency_* guard; R9 mutants in 2.2)
- [ ] L task loop — see *Tasks*
- [ ] B branch review
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

## Known red

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|

## Lessons

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
