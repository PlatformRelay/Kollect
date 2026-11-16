# ci: run govulncheck daily on main and report it

**TL;DR:** `govulncheck` now runs on `main` every day, and a failing run opens a `ci-failure` issue that names each
advisory. Until now a new advisory turned the required `vulncheck` check red only on the next code PR. From
2026-11-06 `main` sat red for seven days with nobody told (#548 fixed it).

- **What:** a new workflow, `Vulnerability scan` (`.github/workflows/vulncheck-scheduled.yaml`). It runs daily at
  04:41 UTC and on demand, and the CI failure report now lists it.
- **Why:** advisories arrive without any commit. Dependabot raised no alert for the 2026-11 Go advisories, and
  Renovate does not bump indirect modules.
- **Risk:** low. It is CI only, report-only, never blocks a PR, and has `contents: read`.
- **Reviewer action:** check the guard test's checks and the issue path (the files below).

## What matters

- `hack/ci/vulncheck-scheduled.sh` runs `task vulncheck` and passes on `task`'s exit status. That status is 201
  on failure, not govulncheck's 3. After the report it prints one line per advisory:
  `FAIL: GO-YYYY-NNNN <title>`. The issue body only quotes lines containing `FAIL`, so these lines are what puts
  the advisory into the issue.
- A failure with no advisory prints one "tool, network or build failure" line instead, so an outage is not
  mistaken for a finding.
- No job or step name may contain `FAIL`. `gh run view --log-failed` puts the step name on every log line, and
  the branch review caught a step name that flooded the excerpt.
- `hack/test/vulncheck_schedule_test.sh` pins the workflow shape and the script's verdicts, against a fake
  `task`. It also runs an integration case through the real `hack/ci/ci-failure-issue-body.sh`, and has a
  `--self-test` with 85 checks. The `lint` job runs it in both modes, and CWS-6 lists it.

## Risk

- A vuln.go.dev or module-proxy outage opens an issue as well. Its FAIL line says "no advisory", and the next
  green day closes it.
- The SVS-2 path in production (a scheduled run opening a real issue) is first observed after merge (task 2.1).

## Files to check first

1. `hack/test/vulncheck_schedule_test.sh` (it changes what green means)
2. `hack/ci/vulncheck-scheduled.sh`
3. `.github/workflows/vulncheck-scheduled.yaml`, plus `.github/workflows/ci-failure-report.yaml` (one list entry)
4. `docs/security/sca-remediation-policy.md` (zero-tolerance and enforcement rows)

Review commit by commit. The spec commits come first, then the two code commits and the guard fix.

## Testing

- Guard plain and `--self-test`: exit 0. Reporter guard and workflow-security guard, both modes: exit 0.
  Toolchain pins, offline zizmor, shellcheck, `lint:markdown`, `spec:validate`, scrub and gitleaks: clean.
- `task check`: four reds, none from this branch:
  - a fresh-worktree `setup-envtest` install failure, tracked separately; CI's `task coverage` is green;
  - a kubectl shim;
  - two operator-sdk guards.
- After merge: dispatch the workflow once, then confirm the first scheduled run and its report.

---

## Deep dive (optional)

The OpenSpec change is in `openspec/changes/scheduled-vulncheck/`: proposal, spec SVS-1 to SVS-3, tasks, evidence
and review records.

The reviews:

- The spec set went through two rounds. They found the local action against the SHA-pin rule, and that Task's
  exit status is 201.
- The branch went through two rounds. They found the `FAIL` step name and the case-sensitive token check, both
  fixed with tests that fail on the old code.
