# Historical spec bundles

These four bundles predate the [spec workflow](../docs/development/spec-workflow.md). They are kept
as the record of how each fix was specified and verified. Do not add new work here; new behavioural
changes go in `openspec/changes/`.

Each bundle's header still says `Status: Draft`. That header was never updated, so the table
below is the current status. It is based on the task lists and on `main`.

| Bundle | Status | Evidence | Successor |
| --- | --- | --- | --- |
| [001-export-correctness](001-export-correctness/spec.md) | Completed: all tasks checked, fix on `main` | fix `79b841cec`; `evidence/` | None yet |
| [001-secretref-authmode](001-secretref-authmode/spec.md) | Completed: all tasks checked, fix on `main` | fix `23e8b8680`; `evidence/` | None yet |
| [001-namespaced-scope-enforcement](001-namespaced-scope-enforcement/spec.md) | Completed: all tasks checked, fix on `main` | fixes `5ba732b49`, `3cb463891`; `evidence/` | None yet |
| [001-error-redaction-choke-point](001-error-redaction-choke-point/spec.md) | Completed: all tasks checked, fix on `main` | fix `7e081faf7`, follow-ups `eac20ceb3`, `135aeba2c`; spec aligned in `58195be8a` | None yet |

When a later change takes over one of these areas, set its Successor to the
`openspec/specs/<capability>` it moved to. Leave the bundle's own files as they are.
