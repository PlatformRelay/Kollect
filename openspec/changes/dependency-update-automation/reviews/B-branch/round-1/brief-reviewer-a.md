# Branch review brief (stage B, whole branch)

You are a cold, independent reviewer. Read-only. Repository root: the current directory
(`/home/koni/Projects/PlatformRelay/worktrees/kollect/dependency-update-automation`), branch
`spec/dependency-update-automation`. Review the range `origin/main..HEAD` (`git log`, `git diff
origin/main...HEAD`). Do not modify files. Your sandbox may refuse writes to /tmp; read the
recorded evidence instead of re-running mutants.

The branch implements the OpenSpec change `openspec/changes/dependency-update-automation/`
(proposal.md, design.md, specs/dependency-updates/spec.md, tasks.md with its Verification table,
loop.md, evidence/*.md, reviews/). It hardens the self-hosted Renovate
(`.github/workflows/renovate.yaml`, `renovate.json`, `.github/renovate-config.json`), adds
`hack/ci/renovate-config-validate.sh` and the guard `hack/test/renovate_policy_test.sh` (plain and
`--self-test`, wired in the required `lint` job of `.github/workflows/ci.yaml` and enrolled in
CWS-6 in `hack/test/ci_workflow_security_test.sh`), and rewrites the Renovate section of
`docs/development/tooling-setup.md`.

Settled, do not relitigate: the bot is the self-hosted Renovate with the `RENOVATE_TOKEN` secret (a
maintainer token that is a ruleset bypass actor); auto-merge scope is Docker image digest updates
of the dockerfile and devcontainer managers only, merged by Renovate (`platformAutomerge: false`).
You MAY report defects in how the branch implements that.

Lenses, all required:

1. **Spec compliance** per requirement DUA-1..DUA-5, DUA-7..DUA-10: holds / partial /
   contradicted / absent / stronger than spec, citing file:line, plus anything the code does that
   no requirement mentions.
2. **Adversarial, across task boundaries**: ways the bot could merge more than the scope, run
   unpinned code with the token, or a guard that cannot fail on its own defect. Specifically
   re-check these fixes that were never reviewed after they were made:
   - the spec-set round-2 fixes (`reviews/R-spec-set/round-2/verification.md`): nested presets
     (T10), workflow-level env and `GITHUB_ENV` (T7), `updateNotScheduled` and the whole-weekday
     window (T14), T13 baseline, the mixed-branch fixture, hourly cron with `rebaseWhen` split;
   - task 1.1's round-2 fixes (`reviews/L-tasks/G1/round-2/` and `evidence/1.1.md`), in particular
     the T6 `configurationFile` rule;
   - task 1.2's post-round-2 message wording fix (`evidence/1.2.md`).
3. **Judge this note** (`loop.md` Triage, `evidence/2.4.md` *Observations*): Renovate builds a
   branch's config from its first sorted upgrade (`generate.js:217-229` in Renovate 44.179.1), so a
   mixed group branch (for example `go toolchain` holding a golang digest and a `go` directive bump)
   can inherit `rebaseWhen: behind-base-branch` from the digest automerge rule: it is rebased
   whenever it falls behind (hourly runs, CI re-run each time) but never auto-merged (branch
   `automerge` is recomputed over every upgrade). Is that only a CI-cost issue, or a defect against
   DUA-7/DUA-8 or the design's statement that only auto-merge PRs are rebased when behind? If a fix
   is warranted, name the smallest one that keeps the guard's T11/T12 contract or say how the
   contract should change.
4. **Fitness**: which architectural characteristic the branch moves (CI gate set, supply-chain
   pinning, merge authority) and whether a function guards it.

Also: `CONTRIBUTING.md:139` allows squash merges while the ruleset allows rebase only (out of
scope? say so).

Output Markdown: a findings table (ID, severity CRITICAL / WARNING / NIT, file:line, finding,
suggested fix), the per-requirement compliance table, then `## Verdict` with exactly one of
APPROVE, CONCERNS, BLOCK. CRITICAL means the branch would ship an unsafe or broken result. Be
concrete; do not pad.
