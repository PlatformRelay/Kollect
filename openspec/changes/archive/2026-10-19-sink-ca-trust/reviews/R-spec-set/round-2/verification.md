# R spec-set review, round 2 (last) — verification

Revision reviewed: `07ac4d2a3`. Fresh legs: astra spec lens (`leg-astra-spec.md`, brief
`brief-astra.md`) → CONCERNS; claude/opus spec lens (`leg-claude-opus-spec.md`) → CONCERNS. No
CRITICAL from either leg. Both legs confirm the round-1 fixes hold apart from the rows below.
Checked against the tree and the module cache on 2026-10-19. This is the last round: the fixes
below are applied without a further review.

| ID | Sev | Finding | Check | Disposition |
| --- | --- | --- | --- | --- |
| A-R2-1 / C-N1 | WARNING | The S3-specific endpoint (`AWS_ENDPOINT_URL_S3`, `services` section) is resolved inside `s3.NewFromConfig`, not in `aws.Config` | both legs; S3 client resolves its own `BaseEndpoint` | fix: design reads `client.Options().BaseEndpoint` after construction; S1b adds both-set and shared-config cases |
| A-R2-2 / C-W1 | WARNING | C1 cannot assert `errors.Is` on a command error: backend failures go to `ContextResult.Fatal` and are only printed; the CA is resolved after a live collect | `cmd/cli/collect.go:143-153`; `internal/pipeline/wire.go:457-496` | fix: C1a pipeline-level with a hand-filled store and `errors.Is`; C1b L2 envtest exit-2 test that asserts collection succeeded first; resolving the CA before collection stays backlog CLI-CA-EARLY-01 (non-goal) |
| C-W2 | WARNING | C3 needs envtest, which the integration job lacks | `internal/pipeline/integration_test.go:36-46`; precedent `git_integration_test.go:36-50` | fix: C3 uses a hand-filled store, no envtest |
| C-W3 | WARNING | Recovery after a Secret fix has no trigger: terminal errors do not requeue, no Secret watch | `kollectinventory_controller.go:262-264`, `:685-700` | fix: SCT-4 states the trigger; docs tell users to touch the sink; non-goal names the gap |
| A-R2-3 / C-N4 | WARNING | Spec rejects `tlsInsecure` when "set", design only when not `false` | `spec.md`, `design.md` *MongoDB* | fix: effective value `true` rejected, `false` accepted, malformed rejected; M4 rows |
| A-R2-4 | WARNING | A service-file `sslrootcert` that is unreadable fails `ParseConfig` before the backstop | pgx `config.go:449-452` merges the service entry before TLS setup | fix: service-selected `sslrootcert` in the pre-parse check; D4 service-file cases |
| A-R2-5 | NOTE | Rejection examples and release wording omit "effective" sslmode | `spec.md` scenario, design table, 6.1 | fix: qualified |
| C-N6 | NOTE | The CLI fetches the credential Secret before the both-sources check | `cmd/cli/collect.go:119` | fix: SCT-4 says "before the CA Secret is fetched" |
| A-stale | NOTE | SRV TLS default is set after the TXT lookup and before the SRV lookup | `connstring.go:994`, `:1000`, `:1016` | fix: proposal wording |

No CRITICAL remains.
