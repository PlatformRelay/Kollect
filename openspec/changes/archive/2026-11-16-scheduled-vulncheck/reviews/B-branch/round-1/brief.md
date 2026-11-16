# Cold review: branch `ci/scheduled-vulncheck` (lens: spec compliance + adversarial, whole branch)

You are an independent reviewer with read-only access to this repository (Go Kubernetes operator
"kollect"). Review the whole branch: `git diff $(git merge-base HEAD origin/main)..HEAD`. The spec is the
OpenSpec change `openspec/changes/scheduled-vulncheck/` (`proposal.md`,
`specs/scheduled-vulnerability-scan/spec.md`, `tasks.md`); per-task evidence is in `evidence/` (T0.1.md,
G1.md) and per-task reviews in `reviews/L-tasks/`. The spec set had two review rounds before the loop; its
last fixes (Task exit status 201, job-key allow-list, daily cron regex, policy wording) were not reviewed in
their final form, so review the spec set too.

Goal: govulncheck runs daily on main (`.github/workflows/vulncheck-scheduled.yaml`, `name: Vulnerability
scan`) through `hack/ci/vulncheck-scheduled.sh`, which prints one `FAIL: <GO id> <title>` line per advisory
so the CI failure report (`.github/workflows/ci-failure-report.yaml`, which now lists the workflow) puts the
advisory into its issue; `hack/test/vulncheck_schedule_test.sh` pins all of it, with a `--self-test`.

Attack it:

1. Compliance per requirement SVS-1, SVS-1.1, SVS-1.2, SVS-2, SVS-3: holds / partial / contradicted /
   absent / stronger than spec; and what the code does that no requirement mentions.
2. The script: can a real failure end green (status lost through the pipe, `tee` failing, `set -u`
   tripping, a trap, mktemp)? Can a real advisory produce no FAIL line (format drift: ANSI codes, CRLF,
   `Vulnerability #1: GO-…` with trailing text, wrapped lines, a module-only "found in modules you require"
   section)? Can a non-advisory failure be mislabelled as an advisory? Does the issue body builder
   (`hack/ci/ci-failure-issue-body.sh`) actually pick the FAIL lines from this job's log (failed-job logs,
   excerpt cap, line cap)?
3. The guard: could any check pass vacuously (yq/jq errors swallowed, a self-test mutant that fails for
   the wrong reason, a message match too loose, a control missing)? Is every SVS-3 scenario covered by a
   check that can fail? Are CWS-6 and the CFR list check really wired (both modes, required job)?
4. Workflow security: zizmor posture, cache use on a scheduled run, permissions, checkout, pinning;
   anything that lets the scheduled run be suppressed or the reporter miss it (name match, CFR-6 guard,
   `timed_out`).
5. Docs (`docs/security/sca-remediation-policy.md`, `docs/development/testing.md`): accurate against the
   code and consistent with the policy's zero-tolerance rule; nothing private (no internal ids, agent or
   model references, home paths).
6. Commits: conventional gitmoji titles, sensible split, no AI/co-author trailers; would each commit
   leave the tree green?

Output: findings, each with severity CRITICAL / WARNING / NOTE, file:line evidence, and a concrete fix;
CRITICAL only for a defect that ships wrong behaviour or makes a requirement fail. End with `## Verdict`
and one of APPROVE / CONCERNS / REQUEST_CHANGES / BLOCK.
