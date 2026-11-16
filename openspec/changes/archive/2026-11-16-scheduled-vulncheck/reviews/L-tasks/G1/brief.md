# Review brief — G1 (tasks 1.1, 1.2, 1.3: daily vulnerability scan, reported)

You are reviewing one task group independently of whoever implemented it. Read-only. Repository root
is the current directory (branch `ci/scheduled-vulncheck`). You may run read-only `git` and `gh`
commands and run the repository's guards in scratch copies outside the repository (`jq`, `yq` v4 and
`shellcheck` are installed). Do not edit files in the repository.

## Inputs

1. TASK — `openspec/changes/scheduled-vulncheck/tasks.md`, section 1 (tasks 1.1, 1.2, 1.3) and its
   *Test rules*; the spec is `openspec/changes/scheduled-vulncheck/specs/scheduled-vulnerability-scan/spec.md`
   (SVS-1, SVS-1.1, SVS-1.2, SVS-2, SVS-3). The probe that fixed the facts G1 relies on (Task 3.54.0
   exits 201 on a failed command, govulncheck's title indentation and 80-column wrap, job wall time,
   wiring sites) is `openspec/changes/scheduled-vulncheck/evidence/T0.1.md`.
2. MATRIX — the verification matrix in `openspec/changes/scheduled-vulncheck/evidence/G1.md`.
3. DIFF — `git diff 5231f6237..HEAD` — the only code you judge.
4. SENSORS — the *Sensors run* and *Iterations* sections of the evidence file (base red, self-tests,
   sweep, zizmor, markdown, spec validation, docs:verify).
5. RULES — `AGENTS.md`: nothing private ships; actions pinned at 40-hex SHAs; no agent attribution.
   Tests of a guard must be able to fail (each mutant judged by its own message, controls pass, values
   read from the tree, never a plausible next release). Committed evidence is Markdown only.

## Judge, in order

a. INTENT — does the diff do every clause of 1.1, 1.2 and 1.3 (split them on commas and "and"s), and
   nothing the tasks did not ask for? In particular: every V-2 mutant the task lists, V-3 (a)–(d), V-5,
   V-6 (cfc_guards, the header comment, a mutant removing either lint step), the script's
   `set -uo pipefail` / `PIPESTATUS` / status pass-through, the four workflow steps, a daily cron at a
   minute and hour no other workflow uses, `timeout-minutes` with a `# why:`, the reporter list entry,
   the two lint steps, and both docs files.
b. MATRIX — is each sensor real and would it fail if its claim were false? Mutation-verify the
   guard in a scratch copy (copy `.github/`, `hack/ci/vulncheck-scheduled.sh` and
   `hack/test/testdata/vulncheck-scheduled/` plus the test to a temp dir; break one thing; run
   `bash <copy>/hack/test/vulncheck_schedule_test.sh <copy>`; confirm red). Try at least: the script
   always exiting 0; FAIL lines printed on a clean run; a workflow `if:`; a weekly cron. Check the
   fixtures came from the real tool (provenance in the evidence; run 43406013666 is readable with
   `gh run view 43406013666 -R PlatformRelay/Kollect --log --job 131820494832`). Dispute rows that are
   vacuous or over-classified.
c. RULES — any rule above broken; workflow security (permissions, credentials, expressions in `run:`).
d. Failure modes — tautological tests, a parser that passes only on its own fixture, a title rule
   that differs from SVS-1.2 ("the next non-blank line, trimmed"), a dropped clause, docs that claim
   more than the code does.

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
gap, untested new behaviour, dishonest evidence class, missing part of the task) → REQUEST_CHANGES.
P2 → note only, APPROVE allowed.
