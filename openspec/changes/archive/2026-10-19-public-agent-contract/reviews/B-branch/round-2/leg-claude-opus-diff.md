## Verdict: CONCERNS

## Findings
- [WARNING] `verification.md` gives every round-1 fix a task id (tasks.md 5.1–5.6), but tasks.md has no section 5 and the diff does not touch tasks.md — `openspec/changes/public-agent-contract/reviews/B-branch/round-1/verification.md:13-18`, `openspec/changes/public-agent-contract/tasks.md:219-237`
  Failure: tasks.md only goes up to 4.2. A later reader (or the 4.1 archive step) can't trace these six fixes to a task and won't find their mutants in the task's verification list. tasks.md:133 still lists only `/home`, `/Users` and `~/` as the home-path mutants.
  Fix: add tasks 5.1–5.6, ticked, each citing its commit (`5312acd86`, `8f59b5779`, `3148ff5d6`, `221eb661e`, `48dae840f`, `4d5209acf`), and add the new mutants to the list at tasks.md:133.
  Confidence: 90
- [WARNING] I found no recorded sensor run on the fix head, so nothing shows the six fixes are green — `openspec/changes/public-agent-contract/evidence/` (only G1.md and G2.md exist)
  Failure: plain mode, `--self-test`, `ci_workflow_security_test.sh` and `task lint:shell` were not recorded for `4d5209acf`. The round-2 brief only lists them as "safe to rerun". From the code, each new mutant should go red and would stay green if its fix were reverted (I checked all five by reading the regex), but nobody recorded a run.
  Fix: record command, sha, exit and result for each of these sensors on HEAD before closing round 2.
  Confidence: 75
- [NOTE] The unbounded Host pattern now flags prose. The commit admits this, and it fails closed. The only prose control was rewritten so the regression is no longer tested — `hack/test/agent_contract_test.sh:87,902`
  Failure: an AGENTS.md paragraph that wraps so a line starts with "host the chart in your own registry before you install it" (only letters, no punctuation) now fails the required `lint` job. Before, the pattern allowed at most three names, so this line passed. The current AGENTS.md and copilot-instructions have no line starting with "host" (checked), so nothing breaks today.
  Fix: accept this and say so in decisions or the spec. Optionally add an `expect_red` control that pins the known false red, so a later loosening of the pattern is a deliberate choice.
  Confidence: 70
- [NOTE] A Host line that uses ssh's negation syntax still passes: the alias character class has no `!` — `hack/test/agent_contract_test.sh:87`
  Failure: `Host * !bastion` (valid ssh_config) gives no PAC-6 finding, although the spec at `spec.md:185` covers "a line starting with an SSH Host directive".
  Fix: allow `!` in both alias classes, `[a-z0-9*?._!-]`, and add a mutant.
  Confidence: 80
- [NOTE] Some Windows home paths still pass: WSL and Git Bash forms, and JSON-escaped forms — `hack/test/agent_contract_test.sh:267-269`
  Failure: `/mnt/c/Users/<u>/x` and `/c/Users/<u>/x` pass, because the character before `/Users` is `c`, which the prefix class excludes. `C:\\Users\\<u>` (escaped) also passes, because `[\/]` takes only one character. Contributors on Windows commonly paste these forms.
  Fix: add `(/mnt)?/[a-z]` as an optional prefix before `/Users`, allow `[\/]+` as the separator, and add one mutant each.
  Confidence: 65
- [NOTE] The new comment says the Windows check accepts "either slash, any case", but only the backslash form has a mutant — `hack/test/agent_contract_test.sh:266,850`
  Failure: if a later edit narrowed `[\/]` to `\\`, `c:/users/x` would stop being caught and the self-test would stay green.
  Fix: add `expect_red "PAC-4 home path c:/users" … plant 'see c:/users/someone/x'`.
  Confidence: 70
- [NOTE] Bare `/root` followed by punctuation (`` `/root` ``, `/root.`, `/root)`) passes, because the `/root` alternative needs `/`, whitespace or end of line after it — `hack/test/agent_contract_test.sh:268`
  Failure: low impact. The bare path names no user, and `/root/<x>` is caught.
  Fix: none needed. Optionally use `/root([^A-Za-z0-9_.-]|$)`.
  Confidence: 55

**Round-1 findings, closed or not**
- A3: closed. The empty entry is no longer skipped (`:481-495`), and `grep -P ''` matches every name. The mutant at `:881` goes red with "exempts AGENTS.md", and the real gitleaks.toml has no empty entry.
- A4: closed for the case that was reported; the `!` gap above remains.
- O3: closed for the three forms named; the siblings above remain.
- A5/O2: closed. The step order is unchanged and a missing package still fails loudly.
- A2: closed. The warning in `docs/development/setup.md:385` sits in the contributor rules, next to the AGENTS.local.md guidance.
- A1/O1: closed for paths. No absolute home or harness path is left in `task-prompt.md` or `loop.md` (grep). The model names in commit messages are deferred to the operator, as recorded.
- None of the six fix commit messages names a machine, a home path or a model.

## Could not check
- I didn't run any sensor (this leg is read-only), so plain, `--self-test`, zizmor and shellcheck are unverified on HEAD.
- Whether gitleaks really compiles `''` to match every path: I relied on the round-1 claim and Go's `regexp` semantics, not on the gitleaks source.
- Whether `ripgrep` is in the archive of the next `ubuntu-latest` image.
- I deliberately did not read the round-2 leg files or `AGENTS.local.md`, `CLAUDE.md` or `CLAUDE.local.md`.
