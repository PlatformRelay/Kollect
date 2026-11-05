# B round 2 (last) — verification

Head `d243fd5f4`. Reviewer A (adversarial): APPROVE, no findings. Reviewer B (spec): CLEAN, one NOTE.
Both confirm every round-1 fix holds and SCB-1, SCB-2, SCB-3 hold.

| # | Finding | Sev | Reviewers | Verified | Disposition |
|---|---|---|---|---|---|
| 1 | Troubleshooting row says "60 s counting window"; gobreaker's `Interval` is a fixed cycle, not a rolling window | NOTE | B | yes (`circuit_breaker.go:61`, gobreaker generation reset) | fix after round 2, wording only, not reviewed again: "within one 60 s counting interval (the counts reset every 60 s while the breaker is closed)" |

Gates on the final tree (after the wording fix), 2026-11-05: `task verify` exit 0, `task scrub` exit 0,
`task spec:validate` 22/22, `gitleaks detect --no-git` exit 0, `task lint` no warnings,
`task lint:markdown` 0 issues. B1.1 recorded unit, race (`-count=3`) and mutation runs on `d243fd5f4`.

## Per-commit gate

Each commit checked out in its own detached worktree; `go build ./...`, then `go test` on the packages
the commit's Go files touch (`-skip TestControllers` for `internal/controller`, then the Ginkgo suite
separately with `KUBEBUILDER_ASSETS` pointing at the main checkout's `bin/k8s/1.36.0-linux-amd64`; a
fresh worktree has no `bin/`, so the first pass failed in `BeforeSuite` for that reason only).

| Commit | Gates | Exit |
|---|---|---|
| `42a13dfb6` docs(spec): propose | build | 0 |
| `dbfdaef52` docs(spec): address R1 | build | 0 |
| `34b39b11a` docs(spec): option B | build | 0 |
| `354162994` docs(spec): T0.1 probe | build | 0 |
| `1048a66d2` docs(spec): T0.1 corrections | build | 0 |
| `dff6a86bb` fix(sink): keep inventory-scoped failures out | build; test errors, sink, sink/git | 0 |
| `13b5a54d1` test(controller): isolate breaker per invocation | build; test controller (unit + envtest) | 0 |
| `b3cd2f50a` docs(spec): extend 2.2 | build | 0 |
| `228d0c9e2` test(controller): serialize debounce metric test | build; test controller (unit + envtest) | 0 |
| `4e50a9a2f` docs(sink): breaker docs | build | 0 |
| `7a26657c0` docs(spec): triage B1 | build | 0 |
| `d243fd5f4` test(sink): pin marker per site | build; test errors, sink/git | 0 |
