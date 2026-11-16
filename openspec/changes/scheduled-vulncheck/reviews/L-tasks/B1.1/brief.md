# Review brief — B1.1 (branch review round 1 fixes)

You are reviewing one task independently of whoever implemented it. Read-only. Repository root is the
current directory (branch `ci/scheduled-vulncheck`). The change is not committed yet: it is the working
tree against `e70cb32a5`. You may run read-only `git` and `gh` commands and run the repository's guards
in scratch copies outside the repository (`jq`, `yq` v4 and `shellcheck` are installed). Do not edit
files in the repository.

## Inputs

1. TASK — `openspec/changes/scheduled-vulncheck/tasks.md`, section *B. Branch review fixes*, task B1.1,
   and its *Test rules*. The findings it fixes are in
   `openspec/changes/scheduled-vulncheck/reviews/B-branch/round-1/` (leg files and verification.md).
   The spec, amended for them, is `openspec/changes/scheduled-vulncheck/specs/scheduled-vulnerability-scan/spec.md`:
   SVS-1.1 (no token expression), SVS-1.2 (no job or step `name:` containing `FAIL`), SVS-1.3 (the
   script: title lines joined; the capture-failure scenario), SVS-2 (the excerpt, fed the log as
   `gh run view --log-failed` prints it).
2. MATRIX — the verification matrix in `openspec/changes/scheduled-vulncheck/evidence/B1.1.md`.
3. DIFF — `git diff e70cb32a5` (working tree; `evidence/B1.1.md` is staged) — the only code you judge.
4. SENSORS — the *Sensors run*, *Iterations* (the recorded base red) and *Test changes* sections of
   the evidence file. The full gate list runs once more after review.
5. RULES — `AGENTS.md`: nothing private ships; actions pinned at 40-hex SHAs; no agent attribution.
   Tests of a guard must be able to fail (each mutant judged by its own message, controls pass, values
   read from the tree). Committed evidence is Markdown only. Tests written by an earlier task change
   only with evidence citing the spec.

## Judge, in order

a. INTENT — does the diff do every clause of B1.1 (split it on commas and "and"s), and nothing else? In
   particular: the step renamed without `FAIL`; the V-2 name check with a mutant; the V-3 integration
   case (the script's output for `advisories.txt` after more than 50 ordinary lines, each line prefixed
   `<job>\t<step>\t<timestamp> `, job and step names read from the workflow, fed through the real
   `hack/ci/ci-failure-issue-body.sh` with the arguments `hack/ci/ci-failure-report.sh` passes, all
   three GO ids in the excerpt) and a mutant restoring a `FAIL` step name that turns it red; `tee`'s
   status kept, with V-3 cases for task exit 0 (script exits 1) and 201 (exits 201), both printing the
   capture-failure line; wrapped titles joined, V-3a/V-3e expectations per SVS-1.3; the `github[`
   token form with a mutant; the timing wording in the policy rows, `docs/development/testing.md`,
   the workflow header and the proposal; "Task 3.54.0" in `evidence/G1.md`; the SVS-1.3 id everywhere
   the script's rules are cited.
b. MATRIX — is each sensor real and would it fail if its claim were false? Mutation-verify in a scratch
   copy (copy `.github/`, `hack/ci/vulncheck-scheduled.sh`, `hack/ci/ci-failure-issue-body.sh`,
   `hack/test/testdata/vulncheck-scheduled/` and the test to a temp dir; break one thing; run
   `bash <copy>/hack/test/vulncheck_schedule_test.sh <copy>`; confirm red). Try at least: the step
   renamed back to contain `FAIL`; the tee status ignored; titles not joined; `More info:` not ending a
   title. Check that the integration case's log shape matches what `gh run view --log-failed` prints
   (`hack/ci/ci-failure-report.sh` around line 199; a real log: `gh run view 43406013666 -R
   PlatformRelay/Kollect --log --job 131820494832 | head`). Dispute vacuous or over-classified rows.
c. RULES — any rule above broken; workflow security (permissions, credentials, expressions in `run:`).
d. Failure modes — tautological tests, a parser that passes only on its own fixture, a title rule that
   differs from SVS-1.3, a capture-failure path that reads a verdict from an incomplete capture, a
   dropped clause, docs that claim more than the code does.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
APPROVE with no findings is valid when substantiated.

## Return — at most 15 lines, nothing else

```text
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you had no access, time, or evidence to verify>
summary: <one line>
```

Severity: P0 (security, invariant broken, vacuous gate presented as proof) → BLOCK. P1 (correctness
gap, untested new behaviour, dishonest evidence class, missing half of the task) → REQUEST_CHANGES.
P2/P3 → note only, APPROVE allowed. End your file with a `## Verdict` line holding the verdict word.
