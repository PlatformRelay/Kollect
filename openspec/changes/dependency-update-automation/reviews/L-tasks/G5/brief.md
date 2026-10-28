You are reviewing one OpenSpec task group, independently of whoever implemented it. Read-only: do
not edit the repository. Scratch copies under /tmp are fine (leave mise.toml out of a copy that
runs a guard). Do not run any command whose text contains the gh merge subcommand literally.

Repository: /home/koni/Projects/PlatformRelay/worktrees/kollect/dependency-update-automation
Base: 551b44f7a7561449a5a628d82c6dba76dc83d984 · Head: the branch HEAD (four commits) plus the
uncommitted `openspec/changes/dependency-update-automation/evidence/5.1.md`
Diff: `git diff 551b44f7a..HEAD` (`hack/test/renovate_policy_test.sh`, `hack/check.sh`,
`renovate.json` one description, `design.md`, `docs/development/tooling-setup.md`,
`CONTRIBUTING.md`, the change's `specs/dependency-updates/spec.md`).

Inputs:

1. TASKS — `openspec/changes/dependency-update-automation/tasks.md`, tasks 5.1 to 5.4 (section 5),
   which fix the stage B findings in
   `openspec/changes/dependency-update-automation/reviews/B-branch/round-1/verification.md`
   (B1, B3, B5, B7, B8, B9, B10). Requirements DUA-3, DUA-5, DUA-7, DUA-8, DUA-10 in
   `openspec/changes/dependency-update-automation/specs/dependency-updates/spec.md`.
2. THE FILES THE PROSE DESCRIBES — `.github/workflows/renovate.yaml`, `renovate.json`,
   `.github/renovate-config.json`, `hack/ci/renovate-config-validate.sh`,
   `openspec/changes/dependency-update-automation/evidence/2.4.md` (*Observations*, the
   mixed-group mechanism). The ruleset facts (rebase only; bypass actors: admin role with
   `bypass_mode: pull_request`, one App with `always`) are recorded in `evidence/5.1.md`.
3. FIXED POINTS — `hack/test/task_check_test.sh` is unchanged and must stay green in both modes;
   the rest of `renovate_policy_test.sh` (clauses other than T7) is unchanged.
4. MATRIX and SENSORS — `evidence/5.1.md` (matrix, sensors with exit codes, the 5.3 probes, the
   guard mutation checklist, the T7 test change and its reason).
5. RULES — `CONTRIBUTING.md` and `docs/` are public: no private strings, no secret values, terse
   plain prose. No rule's matching or settings in `renovate.json` may change (only a
   `description`). No exclusion in `hack/check.sh` may be widened.

Judge, in order:
a. GUARD (5.2) — does T7 now refuse `NODE_OPTIONS`, `GITHUB_COM_TOKEN` and every `*_PROXY` /
   `*_proxy` key at workflow, job and step level of `renovate.yaml`? Can a spelling the action
   forwards (see its `env-regex` default recorded in the evidence) still pass? Is each new mutant
   judged by T7's own new message, and could a mutant pass for the wrong reason? Is the decoy
   control meaningful? Any false red on a correct tree?
b. LOCAL GATE (5.3) — does `task check` run the validator when Docker answers, report a failure
   as a failed gate, and print a skip with its reason otherwise? Does it weaken any TCE contract?
c. ACCURACY (5.1, 5.4) — is each new statement true of the files and of Renovate's behaviour as
   recorded (mixed group rebased when behind, never merged; `auto` rejected because Renovate's
   `getBranchForceRebase` reads rulesets; ghcr.io pull in `lint`; rebase-only ruleset; two bypass
   actors; DUA-3's allowed variables)? A false statement is a P1.
d. MATRIX — is each evidence class honest, and is every `pass` backed by a recorded command?

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

RETURN at most 15 lines, nothing else:
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why>
could_not_check:
  - <what you could not verify>
summary: <one line>
