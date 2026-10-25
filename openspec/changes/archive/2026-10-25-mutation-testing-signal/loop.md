# Loop — mutation-testing-signal (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/mutation-testing-signal` · Branch: `feat/mutation-testing-signal` · Base: `origin/main` @ `3c01608ff` (changes 7 and 8 merged) · Started: 2026-10-19 · Reviewers: astra + claude (duo)
Next: hand-off — integrator: ready PR, all checks green, merge; post-merge: main CI by headSha, first scheduled mutation run (01:30 UTC) + report artifact; forced-failure night via MUTATION_FORCE_FAILURE; archive after
Budget: Claude review legs uncapped (operator goal 2026-10-16) · run under GOAL-2026-10-16-announcement-ready (decisions by model alignment, logged in agent-context/decisions.md)

Change 9 of 10 (issue #446). Spec set: proposal.md, specs/mutation-testing/spec.md, tasks.md.

## Stages

- [x] 0 orient — re-measured against `origin/main` @ `3c01608ff` (2026-10-19); decisions applied (decisions.md 2026-10-19 "change 9": threshold sources A, fixture placement A, flat signal A)
- [x] P plan + tasks — present; spec set updated fa3fdf389 (re-measure, decisions A/A/A, red-first order with given/when/then cases)
- [x] R spec-set review — round 1 (9a5426f3a): astra BLOCK, opus CONCERNS; V1-V5 CRITICAL (config extensions, inert CLI/env thresholds, fail-open scans, unpinned commands, env narrowing) verified by a scratch gremlins probe and fixed with V6-V15 (2b14ee6c2), V7 partly rejected, V16 no action. Round 2 (last, 2b14ee6c2): astra BLOCK, opus CONCERNS; R2-1..R2-13 all fixed, unreviewed (task skip keys, lint reaching `task mutation`, report hand-off). No CRITICAL remains open
- [x] L task loop — 0.1, 1.1, 2.1–2.8 closed (3.1 is hand-off); probe finding F1: warm test cache made every mutant TIMED OUT → runner exports GOFLAGS=-count=1
- [x] B branch review — round 1: task check 1 red (consistency_toolchain_pins --self-test nested bump, passes alone on branch and base → ENVIRONMENT, /tmp pressure); astra BLOCK (B1 CRITICAL all-timed-out read as green → fixed: formatter fails no-kill/no-live packages, --timeout-coefficient=10), opus CONCERNS; fixes 4.1–4.7 reviewed (astra APPROVE); round 2 (last): astra BLOCK, opus CONCERNS — R2-1..3 fixed unreviewed (cap). No CRITICAL open. HEAD c2781dfb1
- [x] Hand-off — merged; post-merge rows MUT-1, MUT-3, MUT-4 passed (evidence/post-merge.md); archived 2026-10-25

## Fitness functions

| Characteristic | Command | Kind | Baseline | Per task / at B |
|---|---|---|---|---|
| CI guard meta-tests | `bash hack/test/<guard>_test.sh` and `--self-test` (incl. `mutation_signal_test.sh` once it exists, and `ci_failure_report_test.sh` and the CFC-2/DTP guards `consistency_tool_pins_test.sh`, `consistency_toolchain_pins_test.sh` for 2.4) | triggered | green | per task |
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
| 0 | threshold sources A, fixture placement A, flat signal A | decision | astra+opus | fix (A, A, A) | spec set; counterpoints as proposal *Risks* and *Non-goals* | pending operator validation |
| R1 | V1-V16 (`reviews/R-spec-set/verification.md`) | 5 CRITICAL | astra+opus | fix (V7 part reject, V16 none) | spec set 2b14ee6c2 | no |
| R2 | R2-1..R2-13 (`reviews/R-spec-set/round-2/verification.md`) | 3 CRITICAL | astra+opus | fix | spec set, last round, unreviewed: stage B looks at W2/W3 and the hand-off | no |
| 0 | decision brief named `arithmetic-base` and `invert-negatives` as default-off; v0.6.0 has them on (`internal/configuration/mutantenabled.go`) | spec | orchestrator | fix | proposal *Assumptions*; 1.1 confirms from `--help` | no |

## Tasks

| Task | Verdict | Review (legs, rounds, Claude?) | Gaps / decision request |
|---|---|---|---|
| G1 (0.1, 1.1) | CLOSED-WITH-GAPS | astra:diff, 1 round, no Claude (evidence and fixtures only) | LIVED classification is agent-review; finding F1 (warm test cache turns every mutant TIMED OUT) handled in G2 by `GOFLAGS=-count=1` in the runner, not a spec contradiction |
| G2 (2.1-2.8) | CLOSED-WITH-GAPS | astra + claude/opus:diff, 2 rounds (r1 astra REQUEST_CHANGES, opus APPROVE; r2 the same), Claude leg because the diff adds a workflow and edits CI wiring and the privileged reporter list; 9 verified P2 guard gaps fixed (a71c7a5e9, e059e0f56) | post-merge rows are 3.1's; W3 is a text and Taskfile scan, not a shell model (residual: `eval`, run-time names) |
| B fixes (4.1-4.7) | CLOSED-WITH-GAPS | S1-S4 reviewed by one astra:diff leg (APPROVE, one P2 fixed as S5); no Claude leg (no CI wiring changed); S6-S7 from round 2, unreviewed (last round), mutation-checked | B3 rejected, B6 and B7 deferred (`reviews/B-branch/round-1/verification.md`); W3 stays a text plus parsed-YAML scan, not a shell model |

## Known red

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|
| `hack/test/ci_failure_report_test.sh`: exact `workflow_run` list, comment, message, plus a `MUT-1 list loses Mutation testing` mutant | change 7 (TDS-2 entry: change 8) | G2 (3c8d59194) | not wrong: MUT-1 adds `Mutation testing` to the list (spec MUT-1, tasks.md 2.7) |
| `hack/test/ci_workflow_security_test.sh`: `cfc_guards`, CWS-6 header, CFC-6 comment, `CFC-6 mutation signal guard unwired` mutant | change 4 (list grown by 7, 8) | G2 (3c8d59194) | not wrong: the new guard must be pinned in both modes (tasks.md preamble, 2.7) |
| `hack/test/mutation_signal_test.sh`: M4 asserts its own message; its self-test mutant is judged by that message | G2 (3c8d59194) | S1 (7b650b5cd) | not wrong: M9's new check also rejects an empty list, so "exited 0" stopped proving M4's own clause |
| `hack/test/mutation_signal_test.sh`: R1 exact argv gains `--timeout-coefficient=10` | G2 (3c8d59194) | S2 (45b1ab13d) | wrong for the product: the default coefficient timed whole packages out (B1, `reviews/B-branch/round-1/holistic.md`) |
| `hack/test/mutation_signal_test.sh`: fake gremlins answers `unleash --help`; R5, R6, R8 assert on the help query | G2 (3c8d59194) | S3 (de5385760), S5 (2200cb61d) | incomplete: the help query was invisible to R5, R6 and R8 (fix review P2) |
| `hack/test/mutation_signal_test.sh`: W3 joins continuations and scans every parsed YAML string; mutation.yaml's bare `task mutation` exempt | G2 (3c8d59194, a71c7a5e9, e059e0f56) | S4 (49310e2ad), S6 (9037e6e3d) | wrong: split and folded `task mutation` passed (B4, R2-1); the literal-block run step was red (R2-2) |

## Lessons

<!-- master list: one line per reusable lesson with the task ids that hit it; two or more ids → a proposed harness change at hand-off -->

This change (mutation-testing-signal):

- Probe a tool's threshold and config sources by running it, not by reading flags: gremlins v0.6.0 ignores CLI/env thresholds, reads `.gremlins` under any extension, and any `GREMLINS_MUTANTS_*_ENABLED` value disables a mutator (R)
- A Task task can be skipped by `sources:`/`generates:`/`method:` as well as `status:`; pin all of them when a guard says "cannot be skipped" (R)
- gremlins times each mutant out at 3x the coverage run's time; a cached `go test -cover` (about 50 ms) turns every covered mutant TIMED OUT, so run it with `GOFLAGS=-count=1`; the fixture control does not notice (G1)
- gremlins exits 0 and writes no report when `--output` names a missing directory: check the report exists, never the exit status alone (G1)
- The Renovate model in `consistency_toolchain_pins_test.sh` reads tracked files only: stage a new workflow before judging DTP-4 (G2)
- A scan read through `< <(awk ...)` or `... || true` hides its own failure: write it to a file and check the status, or the scan-error clause is decoration (G2, both review rounds)
- A mutant judged by a message another rule also prints proves nothing about its own rule: judge it by its rule's own message (G2 round 2)
- A text scan for an invocation needs cases for quotes, flags before the target, another binary name and templates: reviewers found each in turn (G2, both rounds)
- Keep `mise.toml` out of a scratch tree that runs a guard: mise refuses the untrusted config and the guard prints nothing else (G2; change-5 lesson again)
- Run the real tool several times before calling a measurement sound: one green `task mutation` hid that 2 of 7 runs timed a whole package out at gremlins' default coefficient (B)
- A report-only sensor still needs a "measured nothing" failure: zero killed and zero lived is a broken night, not a clean one (B)
- gremlins copies the whole module, `bin/` included, once per worker into `/tmp`, and a panic skips its cleanup: run it from a tree with a small `bin/` (B; about 8 GB left behind here)
- A fake that answers a side query (`--help`) must record it like any call, or "no call before X" cases cannot see it moved (B fix review)
- YAML folds plain and quoted scalars as well as `>` blocks: a line scan of a workflow needs the parsed strings too (B round 2)

Carried over from change 8 (test-depth-signals):

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
- A command reader must model the tool's operand grammar, not its first word: `kubectl get` takes TYPE/NAME in every operand (5.1)
- In yq, `A | B or C` is `(A | B) or C`: parenthesise a boolean after a pipe, or the check reads the wrong node (5.2)
- A workflow pin must also refuse the keys that skip or soften a job or step (`if:`, `continue-on-error:`), not only its triggers and pins (5.2)
- A retry decision must read the exit status as well as the text: the exact flake text under exit 2 or 137 was retried (5.3)
- A fake that always succeeds cannot test failure aggregation: give it a per-call exit status and a case that fails one call of several (5.4)
- Synthetic fixtures put markers at a line start; replay one real output too, or a line-anchored matcher survives every case (5.5: F18)
- Count matching nodes, never join their names: a node named "" joins to an empty string and passes an emptiness check (G5 review)
- Tab is IFS whitespace in bash `read`: an empty middle field merges; put always-non-empty fields first (G5 review)
- A pin on what a step runs must also refuse what decides how it runs: `defaults.run.shell`, step `shell:`, Task `status:`/`ignore_error:` (B round 2)
- In zsh an unmatched glob aborts the `&&` chain: a leg launched after `rm -f <glob> &&` never started; use `rm -f -- file1 file2` or bash (B round 2)

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

- `.github/workflows/mutation.yaml` (new scheduled, report-only) and its guard — report-only must not be bypassable
- `hack/ci/mutation-*.sh` — runner pins `--config` (committed `{}`), all 11 mutator flags, `GOFLAGS=-count=1`, `--timeout-coefficient=10`, refuses `GREMLINS_` env; fails a package with nothing killed or lived
- `MUTATION_FORCE_FAILURE` knob — repo variable used once for the post-merge forced-failure proof
- `.github/workflows/ci-failure-report.yaml`, `hack/test/ci_failure_report_test.sh`, `hack/test/ci_workflow_security_test.sh` — test changes on other changes' guards
- lint job: +4–5 min (new guard self-test ~250–320 s) → ~18 min, close to tool-pins-selftest (~20 min)

<!-- filled at hand-off -->

## Proposed harness changes

| Lesson | Seen in | Proposed change |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
| 3.1 (owner part) | set repository variable `MUTATION_FORCE_FAILURE=true` for one scheduled night, confirm the red run and its issue, unset, confirm recovery |
