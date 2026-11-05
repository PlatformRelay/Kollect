## Verdict
verdict: REQUEST_CHANGES
findings:
  - [P1] docs/operator-manual/troubleshooting.md:214: “reported only in that inventory's status” is false: ownership failures also produce logs (internal/controller/kollectinventory_controller.go:440), warning Events (:277), and sink-error metrics (internal/sink/export.go:210). Remove “only”.
  - [P1] docs/adr/0602-error-taxonomy.md:39: “Every failed export to a sink counts” overclaims: backend acquisition, payload validation and layout resolution can fail before exportThroughBreaker (internal/sink/export.go:92–200). Limit this statement to failures returned by the export plan inside the breaker.
matrix_disputes:
  - Row 2 — IsExcluded and ownershipConflict establish exclusion, not exclusive status reporting; the controller and metrics paths contradict that wording.
  - Row 4 — A short dated addition preserves ADR structure but does not establish the accuracy of “Every failed export”.
could_not_check:
  - Recorded lint/build/unit/guard results were not rerun; full docs:verify remains unverified because of the recorded missing host tool.
summary: Scope, five-column format, other breaker mentions, fixed 60 s counting window and half-open transitions check out; correct the two operator-facing overclaims.
