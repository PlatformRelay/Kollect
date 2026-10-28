You are reviewing one OpenSpec task, independently of whoever implemented it. Read-only: do not
edit the repository. Scratch copies under /tmp are fine (leave mise.toml out of a copy that runs
a guard). Do not run any command whose text contains the gh merge subcommand literally.

Repository: /home/koni/Projects/PlatformRelay/worktrees/kollect/dependency-update-automation
Base: c34d2843f1b9b72ac5d0fb4f8e68d153906ad685 · Head: the branch HEAD (one commit) plus the
uncommitted `openspec/changes/dependency-update-automation/evidence/3.1.md`
Diff: `git diff c34d2843f..HEAD` (docs only: `docs/development/tooling-setup.md`).

Inputs:

1. TASK — `openspec/changes/dependency-update-automation/tasks.md`, task 3.1; requirement DUA-10
   in `openspec/changes/dependency-update-automation/specs/dependency-updates/spec.md`. The design
   reasons are in `design.md` of the same change.
2. THE FILES THE DOCS DESCRIBE — `.github/workflows/renovate.yaml`, `renovate.json`,
   `.github/renovate-config.json`, `hack/ci/renovate-config-validate.sh`, the `lint` job of
   `.github/workflows/ci.yaml`. Read them; judge every statement of the rewritten section against
   them (DUA-10 scenario "Docs match the config"), not against the design prose alone.
3. FIXED POINT — guard clause T17 of `hack/test/renovate_policy_test.sh` (its `T17_ITEMS`) was
   written by task 1.2 and is not under review. Judge whether the section satisfies it by plain
   description, not by keyword stuffing.
4. MATRIX and SENSORS — `evidence/3.1.md` (statement-to-source table, matrix, sensors run).
5. RULES — public docs: no private strings, no internal hostnames, no secret values; terse plain
   prose; `SECURITY.md` and `docs/security/sca-remediation-policy.md` change only if they
   contradict the branch (the implementer left them unchanged: check that call).

Judge, in order:
a. INTENT — does the section cover every DUA-10 item (schedule, token and owner, dashboard, what
   auto-merges and why, pausing, rebasing a stuck PR, validating the config, the unused hosted App,
   the release review), and nothing it should not?
b. ACCURACY — any statement that the config or workflow contradicts (schedule, dispatch, token,
   fallback, permissions, image pin and its window, auto-merge scope, merge method, rebase
   behaviour, validator, Docker need) is a P1.
c. MATRIX — is each evidence class honest; would T17 fail if a named item were dropped?
d. Prose that claims more than the files support.

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
