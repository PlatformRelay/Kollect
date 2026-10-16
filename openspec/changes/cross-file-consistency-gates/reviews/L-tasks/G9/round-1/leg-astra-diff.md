The implementation meets the core guard and CI requirements, but the new historical run instructions need correction.

**Intent.** All five fenced blocks are byte-identical to their deleted files at `7fe45f68a`, and match the cited source commits. Recorded results remain intact. The revision mappings for G3/G5’s final fixes and G8’s parser changes agree with history. No exclusions were widened; review records were untouched.

**P1 — incorrect execution location.** `G3.md:311` and `G5.md:201` say the mutant scripts ran from the repository root at `evidence/<script>`. Historically, they lived at `openspec/changes/cross-file-consistency-gates/evidence/<script>`. Their embedded `cd "$(dirname "$0")/../../../.."` requires that original depth. Following the newly stated placement targets a directory outside the repository and cannot reproduce the recorded successful run. The other three blocks repeat the unsupported repository-root claim.

Counterargument: the adjacent scratch instructions explicitly tell readers to replace `cd`, making an adapted rerun possible. That does not establish the claimed historical invocation or make the unmodified command correct. Separate the recorded command and working directory from adapted scratch instructions.

**Matrix.**

- Rows 1–2: independently verified; five identical blocks and plain guard exit 0 with 37 files.
- Rows 3–4: Git tree inventories confirm exactly five historical offenders and 29 Markdown files on local `origin/main`. Historical guard executions were not rerun.
- Rows 5–10: the fixtures, landing checks, targeted messages, controls, and nine hand mutations support the stated claims by inspection. Dynamic verification was blocked by the sandbox.
- Row 11: the wiring exists, but “machine” overstates its evidence. Reading YAML establishes presence; the existing guards can miss removal of both new steps. Use `machine + review`, with the existing gap retained.

**Rules and failure modes.** The CI steps follow their neighbours’ shape. Generic workflow coverage checks apply while this guard remains invoked; adding it to the explicit mandatory-guards list would protect complete removal. That additional protection is not explicitly required by G9.

The NUL-delimited index scan avoids filename splitting and `core.quotepath` issues, checks the exact lowercase suffix, includes nested/archive paths, and does not depend on tracked files still existing on disk. Symlinks are judged by their tracked names, consistent with the suffix contract. These edge cases were inspected, not mutation-tested. The copy operation can fail on a tracked file missing from disk, but fails closed. No evident Ubuntu compatibility problem appears in the Bash/GNU-tar usage or fresh Git repositories.

`bash -n`, ShellCheck at the repository’s warning threshold, and `git diff --check` passed. Commit separation and attribution comply. No repository files were modified.

## Verdict
verdict: REQUEST_CHANGES
findings:
  - [P1] Incorrect historical working directory makes the stated G3/G5 mutant invocations unreproducible — openspec/changes/cross-file-consistency-gates/evidence/G3.md:311; openspec/changes/cross-file-consistency-gates/evidence/G5.md:201.
matrix_disputes:
  - Row 11 “machine” — wiring presence relies on review; existing guards do not reject deletion of both steps.
could_not_check:
  - Scratch mutations, self-test execution, and dynamic filename/pathspec edge cases: mktemp failed because /tmp is read-only.
  - GitHub Ubuntu execution and post-push SonarCloud result.
summary: Sources preserved exactly and core guard looks sound; correct invocation provenance before approval.
