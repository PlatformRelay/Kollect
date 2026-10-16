You are reviewing one task group, independently of whoever implemented it. This is review round 2 (the last). Read-only: do not modify files in the repository; for mutation checks copy files to a scratch dir (mktemp -d) and run there, or run the guards with a `<tree-root>` argument on a copy.

Repository: the current directory (kollect, branch feat/cross-file-consistency-gates).

1. TASK — G9 of openspec change `cross-file-consistency-gates`: PR #463's SonarCloud quality gate is red because probe scripts committed under `openspec/changes/cross-file-consistency-gates/evidence/` are analysed as new code (`sonar.sources=.`). Decision (option B + guard): do NOT widen `sonar.exclusions`. Instead:
   - every non-Markdown file under that evidence/ dir goes into the Markdown evidence file that cites it, as a fenced code block with the exact command line used to run it and the input revision; recorded output kept; the scripts are `git rm`'d; every reference to the removed paths fixed (review records under `reviews/` are verbatim leg output and stay as written);
   - guard `hack/test/openspec_evidence_markdown_only_test.sh`: plain mode fails if any git-tracked file under `openspec/changes/**/evidence/` (archive included) is not `.md`, naming each; `--self-test` copies the tree to a mktemp dir with an EXIT trap: `.py` in an evidence dir red, `.sh` in an archived change's evidence dir red, no-op copy green, empty scan red;
   - wired into the lint job of `.github/workflows/ci.yaml`, plain and `--self-test` as separate steps.

2. MATRIX — `openspec/changes/cross-file-consistency-gates/evidence/G9.md` (uncommitted; includes the round-1 findings and how each was handled).

3. DIFF — `git diff 7fe45f68a..86081a23f` plus the uncommitted working-tree edits to `evidence/G3.md`, `G5.md`, `G8.md` (`git diff`). Round 1 reviewed up to `b73d730f3`; the round-1 fixes are `53eec1806` (named failure when the self-test copy fails), `86081a23f` (the guard added to the CFC-6 list in `hack/test/ci_workflow_security_test.sh`, so deleting either CI step reds), and the working-tree "Run as" wording (the probes ran from the change directory, not the repository root).

4. SENSORS — recorded in G9.md "Sensors run" (rows 12 and 13 cover the fixes). All run in seconds except the workflow-security tests (~a minute); rerun any you dispute.

5. RULES — repo AGENTS.md: guards follow the consistency_* shape (plain checks the real tree, `--self-test` corrupts a mktemp copy per mutant and asserts non-zero WITH the targeted message, plus a no-op copy that passes; temp dirs cleaned in an EXIT trap); allow-lists and exclusions shrink only; one logical change per commit.

Judge, in this order:
a. Do the round-1 fixes hold, and do they introduce anything new? In particular: does adding the guard to `cfc_guards` in `ci_workflow_security_test.sh` behave correctly (its own `--self-test` still green, its failure messages accurate for a non-`consistency_*` guard), and are the "Run as" lines now true to the recorded commands?
b. MATRIX — for rows 12 and 13 and any row you disputed in round 1: is the sensor real, would it fail if the claim were false?
c. Anything in the full range that round 1 could not check (round 1's astra leg could not run mktemp; run the guard's `--self-test` and a few mutants yourself if you can).
d. AI-code failure modes — tautological tests, weakened assertions, prose that claims more than the code does.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.

An APPROVE with an empty findings list is valid when substantiated.

RETURN, as the last section of your output, a block headed `## Verdict` of at most 15 lines, nothing else after it:
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you could not verify>
summary: <one line>

Severity → verdict: P0 (security, data loss, invariant broken, vacuous gate presented as proof) → BLOCK. P1 (correctness gap, untested new behaviour, dishonest evidence class, missing half of the task) → REQUEST_CHANGES. P2/P3 → note only, APPROVE allowed.
