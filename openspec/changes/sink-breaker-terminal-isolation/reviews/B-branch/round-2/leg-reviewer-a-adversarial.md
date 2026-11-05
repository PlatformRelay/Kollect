No new CRITICAL or WARNING findings at `d243fd5f4`. The round-1 fixes hold; I found no regression introduced by either follow-up commit.

| Requirement | Assessment | Evidence |
|---|---|---|
| SCB-1 | **Holds** | `internal/sink/circuit_breaker.go:68` excludes by marker. All five marked sites match T0.1’s inventory-scoped classification; shared metadata and capacity limits remain unmarked. |
| SCB-2 | **Holds** | Unmarked failures retain the existing counting policy. Tests verify Transient, Forbidden, unclassified and sink-wide Terminal failures, including backend-call counts. |
| SCB-3 | **Holds** | Exclusion releases the half-open slot without changing state. Tests at `internal/sink/circuit_breaker_test.go:449`, `:469`, and `:492` exercise success, failure and concurrent admission. |

The round-1 dispositions check out:

- `internal/sink/git/prune_scoped_marker_test.go:39` drives every marked site, including both `checkSingleClaims` branches, with a shared-metadata negative control.
- `internal/sink/git/prune_scoped_marker_test.go:149` genuinely calls the go-git engine directly. Both engine cases ran without skips.
- `internal/errors/errors.go:155` accurately documents the outer Terminal classification and preserved inner chain.
- The revised race command, commit-green wording and cleanup evidence match the implementation.

Marker tracing found no lossy wrapper between the marked sites and the breaker. Git classification preserves already-Terminal errors; GitLab returns the Git error intact. Multipart claim paths reach the same ownership checks. Object-store layouts do not use Git ownership pruning. Git/GitLab release and snapshot deletion bypass this breaker; relational cleanup enters it through `RunExportEnvelope`.

Repeated marked trials can keep recovery serialized indefinitely. This is explicitly accepted in the final spec/proposal; I found no new evidence warranting reopening that disposition.

G2 preserves the existing assertions, including the expected sink status name at `internal/controller/kollectinventory_export_test.go:330`. Its atomic suffix isolates repeated invocations. The two public documentation changes match the behavior and contain no private IDs or coordination references.

All 12 commit titles follow the gitmoji Conventional Commit form. Commit messages contain no prohibited attribution trailers. The behavior-changing `fix(sink)` title is suitable for release notes. Static inspection found no cross-commit compile dependency.

The adversarial checks covered exclusion under concurrent recovery, classification and wrapper maintenance hazards, accidental suppression of shared/authentication failures, and unnecessary production abstraction. No actionable defect survived those checks. Additional helper behavior—nil handling and preservation of `errors.Is`/`errors.As`—is documented and tested beyond the normative requirements.

Validation passed:

- Focused marker, per-site, engine and breaker tests, with explicit `=== RUN` and no skips.
- `go test -race -count=3 ./internal/errors/ ./internal/sink/...`
- `go test -race -count=3 -skip TestControllers ./internal/controller/`

**What I could not check:** I did not independently rerun mutations, full CI, integration tests, or builds at every historical commit. The attempted HEAD `go build ./...` failed before compilation because creating `/tmp/go-build…` was denied by the read-only filesystem. Thus, historical buildability is statically reviewed, not machine-proven here.

## Verdict

APPROVE
