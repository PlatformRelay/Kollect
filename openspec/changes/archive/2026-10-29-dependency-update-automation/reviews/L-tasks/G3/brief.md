You are reviewing one OpenSpec task group, independently of whoever implemented it. Read-only: do
not edit the repository. Scratch copies under /tmp are fine (leave mise.toml out of a copy that
runs a guard). Do not run any command whose text contains the gh merge subcommand literally; put
such patterns in a file.

Repository: /home/koni/Projects/PlatformRelay/worktrees/kollect/dependency-update-automation
Base: 11825fa55571945e8a5ec370bcb1604c796870fd · Head: the branch HEAD plus the uncommitted
evidence files (branch spec/dependency-update-automation)
Diff: `git diff 11825fa55..HEAD` (three commits: 2.1 workflow, 2.2 config, 2.3 validator and lint
step), plus the uncommitted `openspec/changes/dependency-update-automation/evidence/2.1.md` and
`evidence/2.4.md`.

Inputs:

1. TASK — `openspec/changes/dependency-update-automation/tasks.md`, tasks 2.1, 2.2, 2.3, 2.4;
   requirements DUA-1, DUA-2, DUA-3, DUA-4, DUA-5, DUA-7, DUA-8, DUA-9 in
   `openspec/changes/dependency-update-automation/specs/dependency-updates/spec.md`. The pin and
   the validator argv contract: `evidence/0.5.md` (*The pin for 2.1*, *The argv that works*).
   0.4's expectations (a) to (c), which 2.4 re-checks: `evidence/0.4.md`.
2. FIXED POINTS — the guard `hack/test/renovate_policy_test.sh` (T1 to T17, written by tasks 1.1
   and 1.2) is not under review and was not changed. Plain mode must be green for T1 to T16; T17
   (docs, task 3.1) is the only allowed red and is not this group's work: do not file it.
3. MATRIX and SENSORS — `evidence/2.1.md` (what changed, design notes, matrix, sensors run,
   probes of the real validator and a bot-bump control) and `evidence/2.4.md` (the Renovate
   lookup on the final tree with deliberately older pins: branches, `automerge`, `groupName`, the
   Helm sites, the image window).
4. RULES — workflow values are judged parsed, not from comments; least privilege for the job
   token; no fallback to the repository token; SHA-pinned actions; evidence under openspec is
   Markdown only; no private strings; `go toolchain` must stay the last `renovate.json` rule that
   sets a `groupName`, the node/python rule the last rule. Tools in CI's lint job: yq (mikefarah
   v4), jq, bash, docker (ubuntu-latest).

Judge, in order:
a. INTENT — does the diff do what 2.1, 2.2 and 2.3 ask, all of it, and nothing they did not ask?
   Does the workflow fail closed on a missing secret on GitHub's runner (default shell
   `bash -e {0}`)? Does the config give exactly one auto-merge (digest-only dockerfile and
   devcontainer branches) and keep every other update, the Renovate image included, out of it?
   Does the validator read the right image, refuse a digest-less one, validate each file in its
   own mode, and fail on any failure (exit 1 or 99), also in CI's lint job?
b. MATRIX — would each sensor fail if its claim were false? Does `evidence/2.4.md` show (a) the
   image in its own branch with `automerge` false and outside its window not scheduled, (b)
   release.yaml's Helm input with every HELM_VERSION site in `renovate/helm` and in
   `renovate/major-helm`, (c) `automerge: true` only on digest-only dockerfile/devcontainer
   branches and `false` on the mixed `go toolchain` branch?
c. Failure modes — the validator under `set -euo pipefail` (yq error, missing workflow, two
   Renovate steps, an image with a tag only), `--user` with the image's HOME, a rule order that
   lets a later rule override `automerge`, `groupName` or `schedule` for the image or digest rule,
   `rebaseWhen` on a mixed branch, a Renovate image bump PR that the validator or the guard would
   wrongly red, the comment in `hack/test/sonar_ko_08_workflow_permissions_test.sh` being false.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

RETURN at most 15 lines, nothing else:
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you did not verify>
summary: <one line>
