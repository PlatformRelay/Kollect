You are reviewing one task group, independently of whoever implemented it. Read-only: do not modify files in the repository; for mutation checks copy files to a scratch dir under /tmp (mktemp -d) and run there, or run `bash hack/test/openspec_evidence_markdown_only_test.sh <git-work-tree-root>` on a copy.

Repository: the current directory (kollect, branch feat/cross-file-consistency-gates).

1. TASK — G9 of openspec change `cross-file-consistency-gates`: PR #463's SonarCloud quality gate is red because probe scripts committed under `openspec/changes/cross-file-consistency-gates/evidence/` are analysed as new code (`sonar.sources=.`; 44 of 47 uncovered new lines and two Python taint findings are in `G3-rbac-probe.py`). Decision (option B + guard): do NOT widen `sonar.exclusions`. Instead:
   - every non-Markdown file under that evidence/ dir goes into the Markdown evidence file that cites it, as a fenced code block with the exact command line used to run it and the input revision; recorded output kept; the scripts are `git rm`'d; every reference to the removed paths fixed (review records under `reviews/` are verbatim leg output and stay as written);
   - new guard `hack/test/openspec_evidence_markdown_only_test.sh`: plain mode fails if any git-tracked file under `openspec/changes/**/evidence/` (archive included) is not `.md`, naming each; `--self-test` copies the tree to a mktemp dir with an EXIT trap, then a `.py` in an evidence dir is red, a `.sh` in an archived change's evidence dir is red, a no-op copy is green, an empty scan (no openspec dir) is red so it never passes vacuously; style follows `hack/test/consistency_metrics_test.sh`;
   - wire the guard into the lint job of `.github/workflows/ci.yaml`, plain and `--self-test` as separate steps next to the consistency_* steps.

2. MATRIX — `openspec/changes/cross-file-consistency-gates/evidence/G9.md` (uncommitted working-tree file; verification matrix, sensors, gaps, and the guard's hand-mutant script).

3. DIFF — `git diff 7fe45f68a..b73d730f3` (three commits: 4a367e2c0 inline probes into G3.md/G5.md/G8.md and remove the scripts; 852709f11 the guard; b73d730f3 CI wiring).

4. SENSORS — recorded in G9.md "Sensors run": guard plain exit 0 (37 files) and `--self-test` exit 0 (12 ok lines, ~1 s); plain guard on a git copy of `openspec/` at 7fe45f68a exit 1 naming five scripts; on origin/main exit 0; 9 hand mutants of the guard all killed; `task lint:shell` 0; six ci.yaml guards plain + self-test all 0; zizmor 1.30.1 no findings; `task lint:markdown` 0; `task spec:validate` 13 passed. All but zizmor/markdown run in seconds; rerun any you dispute.

5. RULES — repo AGENTS.md: guards follow the consistency_* shape (plain mode checks the real tree, `--self-test` corrupts a mktemp copy per mutant and asserts non-zero WITH the targeted message, plus a no-op copy that must pass, temp dirs cleaned in an EXIT trap); allow-lists and exclusions shrink only; nothing private ships; one logical change per commit.

Judge four things, in this order:
a. INTENT — does the diff do what the task asks, all of it, and nothing it did not ask for? Check each inlined block is byte-identical to the removed file (`git show 7fe45f68a:<path>`), and that the run command and input revision stated for each are supported by the evidence file's own recorded sensor rows and the git history.
b. MATRIX — for each row: is the sensor real, is the evidence class honest, would it fail if the claim were false? Mutation-verify every clause of the guard in a scratch copy (suffix test, glob scope incl. archive and nesting, empty-scan check, top-level root check, offender counting) and confirm `--self-test` goes red. Look for inputs that make the guard pass while a non-Markdown file is tracked under an evidence dir (odd filenames, uppercase `.MD`, symlinks, a directory named `evidence` at another depth, git pathspec magic quirks, `core.quotepath`, files staged but deleted from disk), and for environment assumptions that would break it on a GitHub ubuntu runner (tar flags, `mapfile -d`, `git init` defaults, safe.directory).
c. RULES — any rule above the diff touches or violates; for the CI change, whether the new steps match the neighbouring steps' shape and whether any existing ci.yaml guard should now know about them.
d. AI-code failure modes — tautological tests, weakened assertions, dropped halves, prose that claims more than the code does.

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
