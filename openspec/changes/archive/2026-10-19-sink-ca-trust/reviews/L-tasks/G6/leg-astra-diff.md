## Verdict
verdict: REQUEST_CHANGES
findings:
  - [P1] docs/crds/kollectdatabasesink.md:40 — Effective sslmode precedence omits the selected service-file entry, which overrides PGSSLMODE; unset DSN/environment therefore does not guarantee verify-full.
  - [P1] docs/crds/kollectdatabasesink.md:66 — Reapplying test-connection=true may produce no update: failed probes retain it, and automatic testing never clears it; document a guaranteed metadata change.
  - [P1] api/v1alpha1/kollectsink_types.go:355 — Touching metadata does not invalidate an existing pooled backend; scope rebuilding advice to uncached build failures.
  - [P1] docs/REQUIREMENTS.md:90 — “A CA that cannot take effect is a terminal error” overclaims across all listed types; Git SSH and explicitly insecure Git/Kafka/NATS configurations do not enforce that rule.
  - [P1] openspec/changes/sink-ca-trust/pr-description.md:14 — Release note describes hostname verification but omits the explicitly required unset → `verify-full` mode.
  - [P2] docs/guides/pipeline-cli.md:487 — Quoted both-sources and HTTP errors omit their sentinel suffixes; mark them as excerpts or quote exactly.
matrix_disputes:
  - Rows 3/9/10/11 — Pass claims overlook service-file precedence, conditional annotation removal, cached backends, and the missing release-note mode.
  - Row 7 — “Equal the code” is false for the two truncated sentinel strings.
could_not_check:
  - Did not rerun regeneration, task verify, integration tests, or rendered-site build; inspected generated diffs and reran Markdown lint (358 files).
summary: Generated changes contain only the intended descriptions; documentation and evidence need the corrections above.
