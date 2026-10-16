The change largely matches tasks 5.1–5.2, but the docs/rules scanner has a reproducible false pass.

**Intent.** The separate documentation commit fixes the five missing metrics without renaming shipped metrics. Their types and labels match `metrics.go`. The guard implements the requested source, mirror, documentation, suffix, and empty-scan checks. Namespace/Subsystem and outside-package checks are reasonable safeguards for the literal-scan assumption, although rejecting any matching import or field is broader than detecting registration.

**Findings.**

- **P1 — Docs/rules names are truncated before validation.** At `hack/test/consistency_metrics_test.sh:175`, `NAME.findall()` extracts lowercase substrings without validating whole metric identifiers. I executed the unchanged Python guard with in-memory rule substitutions: replacing `kollect_workqueue_depth` with either `kollect_workqueue_depthTYPO` or `kollect_workqueue_depth:typo` returns **exit 0**. Both queries reference an unregistered metric, but the scanner accepts their registered prefix. The same extraction can falsely satisfy documentation completeness. Extract complete metric tokens, reject unsupported names, and add regression mutants.

  **Opposite argument:** The source set deliberately permits only lowercase literals. **Why the finding survives:** That restriction does not justify silently treating a different docs/rules identifier as a registered name.

- **P2 — The mutation helper does not enforce its result.** At `openspec/changes/cross-file-consistency-gates/evidence/G5-gate-mutants.sh:42`, the final status depends on the restored guard passing. Surviving or unapplied mutants only increment counters; they do not fail the command. Assert both counters are zero. This does not invalidate the recorded `14/14` count, but exit 0 alone cannot prove it.

**Matrix assessment.**

| Rows | Assessment |
|---|---|
| 1 | Probe counts and three histogram declarations agree with inspection. “Commands in 5.1 above” points to prose, not reproducible commands. “Two names” also lists three constants. |
| 2–5 | Targeted fixtures and diagnostic assertions are real; their intended failures are represented. |
| 6–7 | Positive suffix/constant controls and negative suffix/demotion mutants are meaningful. They omit whole-token validation. |
| 8–9 | Empty-source/rules, missing-file, comment, test-file, and catalog fixtures exercise the stated cases. |
| 10 | Tests cover Namespace, unresolved Name, and an ordinary quoted import; they do not establish universal fail-closed behavior. |
| 11 | Independently reran successfully: 29 registered, three histograms, 30 docs names. Using pre-fix docs in memory reproduced exactly five undocumented metrics. |
| 12 | Exit-plus-targeted-message assertions and no-op control are substantive. Raw strings, rune/escape handling, and Subsystem lack dedicated fixtures. |
| 13 | Fourteen mutations are present, but the runner does not enforce zero survivors; it also omits several parser clauses. |
| 14–15 | Documentation changes and unchanged Go files verified by inspection; recorded Markdown/Go runs not independently rerun. |
| 16 | EXIT cleanup exists. Equal aggregate `/tmp/tmp.*` counts are weaker evidence than checking the specific allocated directories. |
| 17 | Independently passed Bash syntax checks and ShellCheck at warning severity for both scripts. |

**Rules and test quality.** No repository files were modified. The production self-test follows the requested copy/no-op/mutant/message/trap structure; its assertions are not merely exit-code checks. In-memory masker probes correctly handled line/block comments, raw strings, escaped rune quotes, and escaped string quotes. However, I could not perform the requested exhaustive clause-removal verification: `mktemp -d` failed because this session makes `/tmp` read-only.

## Verdict
verdict: REQUEST_CHANGES
findings:
  - [P1] Whole metric identifiers are truncated, allowing unknown alert queries to pass — hack/test/consistency_metrics_test.sh:175
  - [P2] Surviving/unapplied gate mutants do not fail the runner — openspec/changes/cross-file-consistency-gates/evidence/G5-gate-mutants.sh:42
matrix_disputes:
  - Row 13 — exit 0 does not establish zero surviving or unapplied mutants.
  - Row 1 — the cited probe commands are absent; the evidence is prose.
could_not_check:
  - Scratch-copy self-tests and exhaustive clause-removal mutations: /tmp is read-only.
  - Independently rerunning Markdown lint, docs-map, Go tests, and temporary-directory cleanup.
summary: Fix complete-token scanning and add regression mutants; the documentation correction and core checks otherwise match the task.
