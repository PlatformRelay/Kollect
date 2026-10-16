The round-1 fixes hold by inspection, with no new correctness finding.

- Adding the explicit guard path to `cfc_guards` closes complete removal from CI. Removing one mode triggers CWS-6; removing both triggers CFC-6. The diagnostics identify the actual filename and missing mode without requiring a `consistency_*` basename. The self-test copies all of `hack/test`, so the added mandatory file is included.
- The copy pipeline now catches failures under `pipefail` and emits the targeted explanation before exiting.
- All five “Run as” locations now match the original paths. All five fenced sources are byte-identical to the deleted scripts.

Matrix rows 12 and 13 describe meaningful negative sensors: deleting CI steps exercises mandatory invocation checks; deleting a tracked evidence file exercises the copy-failure handler. These would distinguish the claimed fixes from their previous behavior. The prior row-11 dispute is resolved by the explicit mandatory-guard entry.

Independently verified: plain guard passes with 37 files; historical tree contains exactly five non-Markdown offenders; local `origin/main` contains 29 Markdown evidence files; Bash syntax, ShellCheck at warning severity, and committed/working-tree whitespace checks pass. No exclusions were widened.

The self-tests check mutation landing, nonzero status, and targeted messages, with green controls. I found no new tautological assertion or unsupported evidence classification. Dynamic mutation verification remains unavailable: both attempted self-tests stopped at `mktemp` because the filesystem is read-only. No repository files were modified.

## Verdict
verdict: APPROVE
findings: []
matrix_disputes: []
could_not_check:
  - Both self-tests, independent scratch mutants, and dynamic reproduction of rows 12–13: mktemp fails on the read-only filesystem.
  - Post-push SonarCloud quality gate and GitHub runner execution.
summary: Round-1 fixes are sound by inspection; available independent checks pass, with scratch execution limitations explicitly retained.
