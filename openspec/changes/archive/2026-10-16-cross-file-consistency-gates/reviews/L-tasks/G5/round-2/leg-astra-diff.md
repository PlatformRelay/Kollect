The round-1 fixes hold, but the mirror parser still permits a CFC-4 false pass. No repository files were modified.

**Intent.** The change covers the requested guard, self-tests, probe, and separate documentation correction. All five added documentation rows have the correct types and labels. No shipped metric was renamed. Namespace/Subsystem and outside-package import checks are justified safeguards for the literal-scan assumption, although their implementation is broader than registration detection and does not universally fail closed.

**Finding — P1: mirror entries can be silently ignored.** At `hack/test/consistency_metrics_test.sh:209`, the parser collects string placeholders inside the slice without validating the remaining entries. I supplied this valid Go modification through an in-memory file substitution:

```go
const phantomMetric = "kollect_phantom_total"

var registeredMetricNames = []string{
    phantomMetric,
    // All existing entries remain.
}
```

The unchanged guard returned **exit 0, “mirror equal”**, although the mirror contains an unregistered name. Resolve supported constant entries or reject unrecognized expressions, and add a targeted self-test.

*Opposite argument:* The current mirror uses literals, and its Go test would catch this addition.  
*Why the finding survives:* CFC-4 independently requires this guard to establish mirror equality; silently ignoring valid entries produces an incorrect success result.

**Matrix assessment.** “Inspected” below means the sensor is substantive but its recorded execution was not independently reproduced.

| Row | Assessment |
|---|---|
| 1 | Reproduced pre-task counts: 29 source names, 25 docs names, five rule names; only `_bucket` appears as a histogram suffix. Probe commands now exist. “Two names” still lists three constants. |
| 2 | Rename fixture and file/name diagnostic are meaningful; inspected. |
| 3 | Reproduced the five undocumented-metric failures using pre-fix docs. Removing completeness enforcement makes the targeted in-memory case pass. |
| 4 | Catalog exclusion and targeted fixture are present; inspected. |
| 5 | Missing, extra literal, and missing-declaration checks respond to clause removal. Constant entries bypass equality as demonstrated above. |
| 6 | Independently accepted `_bucket`, `_sum`, and `_count`. Constant resolution supports the tested untyped declaration; an explicitly typed string constant is rejected. |
| 7 | Non-histogram suffix and unsupported-suffix cases fail; removing their respective restrictions makes them pass. |
| 7a | Both round-1 rule regressions now fail correctly. Restoring lowercase-prefix extraction reproduces acceptance. |
| 8 | Empty docs and rules fail. Empty-rules clause removal is detected by the corresponding in-memory case. Remaining fixtures inspected. |
| 9 | Comment masking behaved correctly in direct probes. Test/catalog exclusion fixtures inspected. |
| 10 | Namespace and Subsystem checks respond to clause removal. Outside-import coverage is narrower than claimed: a valid backtick-delimited import passes, whereas its double-quoted equivalent fails. |
| 11 | Plain guard independently passes: 29 registered, three histograms, 30 docs names. |
| 12 | Copy/no-op/mutant/message structure is substantive. Full Bash self-test execution was unavailable; parser coverage remains incomplete. |
| 13 | The runner now explicitly rejects survivors and unapplied mutations. Its 15 mutations do not establish every parser clause; full runner not rerun. |
| 14 | Documentation changes inspected; Markdown lint and docs-map results not independently rerun. |
| 15 | Go source and mirror are unchanged by this diff; recorded Go test not independently rerun. |
| 16 | EXIT cleanup is present. Aggregate temporary-directory counts do not prove cleanup of each allocated directory. |
| 17 | Both scripts independently pass Bash syntax checks and ShellCheck at warning severity. |

**Rules and test quality.** The production self-test follows the required reference shape and checks targeted diagnostics, not merely nonzero exits. The documentation correction is isolated in its own commit. Eleven in-memory clause-removal probes confirmed that their corresponding negative cases depend on the intended checks. Direct masker probes handled comments, raw strings, escaped quotes, and rune literals correctly.

These probes are not equivalent to running the complete Bash self-test against every weakened guard. `/tmp` rejected `mktemp` as read-only, so exhaustive self-test mutation verification and runtime cleanup verification remain uncompleted. The main testing weakness is incomplete parser coverage, rather than tautological assertions.

## Verdict
verdict: REQUEST_CHANGES
findings:
  - [P1] Mirror parsing silently ignores constant entries and falsely reports equality — hack/test/consistency_metrics_test.sh:209
  - [P2] Outside-package import check misses valid backtick-delimited imports — hack/test/consistency_metrics_test.sh:171
matrix_disputes:
  - Row 5 — literal-only fixtures do not establish exact mirror equality for constant entries.
  - Row 10 — the ordinary quoted-import fixture does not prove fail-closed import detection.
  - Row 16 — aggregate temporary-directory counts do not establish individual cleanup.
could_not_check:
  - Complete Bash self-test, 15-mutant runner, exhaustive clause-removal self-tests, and cleanup execution; scratch filesystem is read-only.
  - Independent reruns of Markdown lint, docs-map guard, and Go tests.
summary: Round-1 fixes hold; repair the mirror false pass and add a regression mutant before approval.
