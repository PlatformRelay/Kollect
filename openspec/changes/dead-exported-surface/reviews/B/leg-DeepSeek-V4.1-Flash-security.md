## Verdict: CLEAN

Security lens on a dead-surface deletion. I traced the only security-relevant code touched (inventory auth cache) and confirmed behaviour is unchanged; no allow-list, baseline or exclusion file was widened.

## Findings

- [NOTE] Unbounded auth-cache growth on attacker-controlled namespace (pre-existing, not introduced here) — `internal/inventory/auth_cache.go:40-45`, key built at `internal/inventory/auth.go:113`
  Failure: `get` returns a miss for an expired entry but never deletes it, and `set` is called for `allowed=false` too. Any caller holding *a* valid token (even one with no RBAC) can vary the `namespace` query param; each distinct string creates a new never-reclaimed entry. Expired entries are only overwritten on the identical key, so the map grows without bound → memory-exhaustion DoS. This change removes the dead `user` field only; it neither causes nor worsens the issue.
  Fix: in `get`, `delete(c.items, key)` when expired; optionally cap `len(c.items)` (evict oldest) — a separate change, out of this PR's no-behaviour-change scope.
  Confidence: 80 (code path verified by reading; severity depends on exposure of the inventory server and token-holding attacker — I did not measure that).

## What I checked (clean)

- Auth path equivalence: old `get` returned `(user, allowed, ok)`; new returns `(allowed, ok)`; the single call site `auth.go:115` destructures in the same order, and `user` was `_ = user` before — pure no-op. `set` signature/behaviour otherwise identical. `internal/inventory` tests pass.
- No production caller of any deleted symbol: repo-wide grep for `RunExportItems`, `ExportItemsRequest`, `MergeRequestAPI`, `RemoveCluster`, `MarshalTargetJSON`, `BindClusterTargetNamespaces`, `git.Export`, `ExportMemory`, the four `*Capabilities` aliases, `ConditionConnected`, `ConditionCredentialsVerified` → only test-name matches remain (`internal/sink/git/export_test.go:23,36,48`).
- Production dispatch intact: `RunExportEnvelope` still called from `internal/sink/cleanup.go:364`, `internal/controller/kollectinventory_controller.go:404`, `internal/controller/kollectclusterinventory_controller.go:331`; the deleted `RunExportItems` was a wrapper over it.
- Path-traversal guard preserved: `ExportMemory`'s body moved verbatim into the unexported test helper `exportMemory`, which still calls production `validateObjectPath`; `TestExportMemory_rejectsTraversal` retained.
- No new dependencies (`go.mod`/`go.sum` untouched), no secrets, no authn/authz boundary changed, no TLS/credential handling in the deleted `git.Export` wrapper (it delegated to the untouched `ExportWithBranch` → `guardedGoGitAuth`).
- Fitness: no config/baseline/allow-list file in `git diff --name-only` (only `*.go` + `openspec/`); `.go-arch-lint.yml`, coverage floor, `sonar-project.properties`, `osv-scanner.toml` untouched. `go build ./...` and `go vet` (touched pkgs) exit 0.
- Ran: `go test ./internal/inventory/...` ok; `go test -run 'TestExportMemory|TestExportFileRemote|TestExportGoGit|TestExportErrorReason|TestRunExportEnvelope_guards' ./internal/sink/ ./internal/sink/git/` ok.

## Could not check
- The authoritative Sweep 2 report (`data/kollect-xconsol-final/report.md`) is outside this repo — out of bounds; I relied on the quoted excerpt in `proposal.md:115-137`.
- External module consumers of the removed `api/v1alpha1` constants (module is pre-1.0; unverifiable in-repo) — accepted limit already recorded in `proposal.md:108-113`.
- Did not run the full `task test` envtest suite, `task coverage`, `task lint`, or `task vulncheck`; the loop.md records these green but I did not reproduce them.
- Did not measure real-world exposure of the inventory HTTP server, which sets the severity of the cache NOTE.
