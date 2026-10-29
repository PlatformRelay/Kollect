# Spec-set review brief (stage R)

You are a cold, independent reviewer. Read-only. Repository root: the current directory
(`/home/koni/Projects/PlatformRelay/worktrees/kollect/dependency-update-automation`), commit
`5e2f421c2`. Do not modify files.

Review the OpenSpec change `openspec/changes/dependency-update-automation/`: `proposal.md`,
`design.md`, `specs/dependency-updates/spec.md`, `tasks.md`. It redesigns dependency-update
automation around the existing self-hosted Renovate workflow `.github/workflows/renovate.yaml`
(token secret `RENOVATE_TOKEN`, a maintainer token that is a ruleset bypass actor) with
`renovate.json` and `.github/renovate-config.json`. Read those files, `.github/workflows/ci.yaml`
(the `lint` job), `hack/test/ci_workflow_security_test.sh` (CWS-6 enrolment),
`hack/test/consistency_go_toolchain_test.sh`, `hack/test/consistency_toolchain_pins_test.sh`,
`docs/development/tooling-setup.md` and `docs/development/spec-workflow.md` as needed.

Settled, do not relitigate: the bot identity (self-hosted Renovate with that token, no new App),
and the auto-merge scope "Docker image digests of the dockerfile and devcontainer managers only,
Renovate-side merge" (a recorded decision pending maintainer validation). You MAY point out a
defect in how the spec set implements that scope.

This is round 2 (the last). Round 1 findings and their dispositions are in `openspec/changes/dependency-update-automation/reviews/R-spec-set/verification.md`; check that the fixes hold and look for anything new. Only report CRITICAL for a defect that would ship an unsafe or undeliverable result.

Question: would executing these tasks as written deliver the spec, safely? Look for:

- requirements that are untestable, contradictory, or that the tasks never verify;
- tasks that are missing, misordered, unverifiable, or do not say what would expose their failure;
- test task 1.1's cases: would each fail on the defect it names, and pass on a correct tree? Any
  case that is impossible to satisfy, or that contradicts Renovate's real config schema or
  behaviour (Renovate 44.x; renovatebot/github-action v46.3.10);
- security: anything that lets the bot merge more than the stated scope, run unpinned code with
  the token, or weakens an existing guard (CWS-1..7, CFC, DTP);
- conflicts with existing guards in `hack/test/` that the tasks would trip and not handle;
- verification-table rows that cannot produce evidence, and post-merge rows that are really
  pre-merge (or the reverse).

Output Markdown: a findings table (ID, severity CRITICAL / WARNING / NIT, file:line, finding,
suggested fix), then `## Verdict` with exactly one of APPROVE, CONCERNS, BLOCK. CRITICAL means
executing the tasks as written would ship a defect or an unsafe configuration, or cannot be done.
Be concrete and cite lines. Do not pad.
