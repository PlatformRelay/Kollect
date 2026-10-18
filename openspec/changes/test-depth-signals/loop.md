# Loop — test-depth-signals (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/test-depth-signals` · Branch: `feat/test-depth-signals` · Base: `origin/main` @ `b05651d20` (change 7 merged as #469) · Started: 2026-10-18 · Reviewers: astra + claude (duo)
Next: L — dispatch G1 (1.1–1.5 fuzz runner)
Budget: Claude review legs uncapped (operator goal 2026-10-16) · run under GOAL-2026-10-16-announcement-ready (decisions by model alignment, logged in agent-context/decisions.md)

Change 8 of 10 (issue #445). Spec set: proposal.md, specs/test-depth/spec.md, tasks.md.

## Stages

- [x] 0 orient — re-measured against `origin/main` @ `b05651d20` (2026-10-18); decisions applied (decisions.md 2026-10-16 "change 8": shuffle scope A, fuzz retry A)
- [x] P plan + tasks — present; spec set updated da4009a78 (re-measure, decisions A/A, red-first order with given/when/then cases)
- [x] R spec-set review — round 1 (58a84b979): astra BLOCK, opus CONCERNS; V1 CRITICAL (Ginkgo refuses -count≠1 → RunSpecs packages run 3× at -count=1) accepted and fixed inside decision A; V2–V14, V16 fixed (9a42d7591), V15 reject, V17 → 1.1. Round 2 (last): astra BLOCK, opus CLEAN; R2-1..R2-9 fixed (99949a331; R2-2 exact flake sequence). No CRITICAL remains
- [ ] L task loop — see *Tasks*
- [ ] B branch review — `reviews/B-branch/round-N/` · rounds: · verdict:
- [ ] Hand-off — `pr-description.md` · PR: none

## Fitness functions

| Characteristic | Command | Kind | Baseline | Per task / at B |
|---|---|---|---|---|
| CI guard meta-tests | `bash hack/test/<guard>_test.sh` and `--self-test` (incl. `fuzz_retry_test.sh`, `test_shuffle_test.sh`, `readme_quickstart_test.sh` once they exist, and `ci_failure_report_test.sh` for 2.3) | triggered | green | per task |
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
| 0 | shuffle scope A (envtest in, e2e only out; Ginkgo order a non-goal) and fuzz retry A (fail closed) | decision | astra+opus | fix (A, A) | spec set da4009a78; Ginkgo `--randomize-all` named as the follow-up in proposal *Non-goals* | pending operator validation |

## Tasks

| Task | Verdict | Review (legs, rounds, Claude?) | Gaps / decision request |
|---|---|---|---|
| G1 (1.1-1.5) | CLOSED-WITH-GAPS | astra + claude/opus:diff (CI touched), 2 rounds; r1 astra REQUEST_CHANGES (deny markers bypassed on exit 0, fixed), opus CLEAN; r2 astra APPROVE, opus CONCERNS (deferred) | CI on the PR head not run; deferred to B: empty-pkg F0 case, crash-fixture replay; spec question: `go: downloading` lines on a cold cache make a real flake unrecognised (filter them in TDS-1, or an uncaptured `go mod download` step) |
| 1.6 | CLOSED-WITH-GAPS | astra + claude/opus:diff (CI touched), 1 round; astra APPROVE, opus CONCERNS (wording fixed, two NOTEs deferred to B) | cold-cache probe local only; F16 ignores `if:`/`continue-on-error:` (deferred to B, fails closed) |
| 2.1 | CLOSED | astra (evidence only), 1 round | found two repetition-dependent tests (collect, metrics), fixed in their own commits before G2 |
| G2 (2.2-2.5) | CLOSED | astra + claude/opus:diff (CI touched), 2 rounds; r1 astra REQUEST_CHANGES (empty SHUFFLE_SEED accepted, go list argv unasserted: fixed), opus CLEAN; r2 astra APPROVE, opus CLEAN | first scheduled run + reporter run stay the post-merge row; deferred to B: `make setup-envtest` failure has no `::error::` line |
| G3 (3.1-3.2) | CLOSED-WITH-GAPS | astra + claude/opus:diff (CI touched), 2 rounds; r1 astra REQUEST_CHANGES (kubectl flag before verb, single `&`: fixed), opus CONCERNS; r2 astra REQUEST_CHANGES (backslash continuation, indented fence: fixed after the last round), opus CONCERNS (evidence: fixed) | round-2 fixes unreviewed (mutation-checked); deferred to B: `internal: true` tasks count as existing, `bash`/`shell` fences unread (spec reads `sh` only); README had no drift |

## Known red

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|
| `hack/test/ci_failure_report_test.sh`: exact `workflow_run.workflows` list, comment, message (+ mutant) | change 7 (ci-failure-reporting) | G2 | not wrong, extended: TDS-2 requires `Test shuffle` in the list (spec.md TDS-2, task 2.4) |
| `internal/collect` `TestRecordLabeledMetricSeries_CapsCardinalityDeterministically`; `internal/metrics` `TestRegister` | before this change | 2.1 fixes `a52189aa9`, `297952dee` | absolute reads of process-global counters: red at `-shuffle=off -count=3` (evidence/G2.md) |

## Lessons

<!-- master list: one line per reusable lesson with the task ids that hit it; two or more ids → a proposed harness change at hand-off -->

This change (test-depth-signals):

- `gh run view <run> --log --job <id>` ignores the run id: list the run's jobs first and take the id from that run (G1)
- An exactly-once literal mutant anchor fails when a comment repeats the flag: keep each mutated clause on one line at one site (G1)
- A CFC-6 "mutant did not apply" grep must match the step's `bash hack/test/<guard>` text, not the guard's name, once a job comment names the guard (G1)
- Give every exit path of a decision script a case, the success path included: deny markers after the exit-0 branch passed `panic:` on exit 0 (G1)
- Keep the evidence Status pending until the review is recorded: a reviewer reads a premature CLOSED as a claim (1.6)
- `make setup-envtest` prints the assets path to stderr with no newline: keep it off the line that carries a seed or marker (2.1)
- Run the new nightly's real command once before writing its guard: the 2.1 split found two counters read as absolutes, red at every `-count=3` (2.1)
- Under `set -e`, a `$(grep … | head | cut)` with no match aborts the whole guard: add `|| true` to every probe substitution, or the red stops at the first missing line (G2)
- A mutant that adds a bad entry to a validated list reds every case that runs the script: name the clause it targets and accept the spill, or mutate the validation instead (G2)
- A Markdown-fence guard must treat a fence that runs into the next `##` heading as unclosed: a later bare fence "closes" it and the section boundary is lost (G3)
- A hand mutant that drops a tokenizer step can survive every task-named case: add one case with two commands on one line (G3)
- A command extractor needs cases for every shell joint, not only `&&`: single `&`, `$(…)`, a flag before the verb, `\` continuation and indented fences all slipped past a reviewed first version (G3)

Carried over from change 7 (ci-failure-reporting):

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

<!-- filled at hand-off -->

## Proposed harness changes

| Lesson | Seen in | Proposed change |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
