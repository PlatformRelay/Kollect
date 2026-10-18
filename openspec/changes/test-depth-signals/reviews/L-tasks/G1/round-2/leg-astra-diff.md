## Verdict
verdict: APPROVE
findings: []
matrix_disputes:
  - Row 12’s “each case reds” overstates coverage: F10 intentionally passes; F6’s original-loop red proves the diagnostic, not the retry cap. The evidence narrative acknowledges both.
could_not_check:
  - Read-only filesystem prevented scratch mutations and guard execution; inspected all 21 mutants, case-specific failure assertions, and the no-op control statically.
  - Original CI captures, issue #75804 verification, and real GitHub execution were not independently reproduced; local Go source supports fixture wording.
summary: G1 satisfies TDS-1, including unconditional deny checks, ordered target matching, bounded retry, fail-closed corpus checks, exact argv, and CI wiring. Shell syntax checks pass; no surviving code defects found.
