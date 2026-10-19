## Verdict
verdict: REQUEST_CHANGES
findings:
  - [P1] hack/test/fuzz_retry_test.sh:365: F11 accepts a runner step with `name: ""` and `if: false` or `continue-on-error: true`: joining its name produces empty `soft`. Count matching steps instead.
matrix_disputes:
  - 5.2 “Presence is refused, whatever the value” — counterargument: existing named-step mutants are caught; nevertheless, the in-memory empty-name probe bypasses the assertion.
could_not_check:
  - Scratch-dependent guards, self-tests, and hand mutants could not run under read-only restrictions; reported gate executions and historical reds were assessed from code and evidence.
summary: Remaining G5 changes and named sensors substantiate their tasks; README plain mode, shell syntax, and diff checks passed. Commit format and attribution comply.
