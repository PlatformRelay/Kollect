## Verdict: CLEAN

## Findings
- [NOTE] Auth-cache key still binds the decision to identity via SHA-256 of the bearer token, so dropping the `user` field cannot let an allow entry leak across principals — `internal/inventory/auth_cache.go:62-70`
  Failure: none — cacheKey is `tokenHash|verb|namespace|name:resource` and identity was never read on hit (`_ = user` before this branch). Removal is purely the dead field.
  Fix: none.
  Confidence: 90
- [NOTE] Pre-existing (not introduced here, unchanged by the diff): a cached `allowed=false` entry is only enforced when `RequireInventoryGet` is set; with it unset the cache-hit branch falls through to `next.ServeHTTP` — `internal/inventory/auth.go:115-125`
  Failure: n/a for this branch — identical logic existed before (`if a.RequireInventoryGet && !allowed`); the miss-path `if !allowed` 403s unconditionally, so a denial is never served from cache without the flag doing the same. Flagged so it is not mistaken for new behaviour.
  Fix: none required in this change; own it in the auth lane if the two branches should be made symmetric.
  Confidence: 80
- [NOTE] Path-traversal rejection on git exports stays covered after deleting `ExportMemory` — the same `validateObjectPath` guards the live path — `internal/sink/git/validate.go:16`, `internal/sink/git/exec_git.go:218`, `internal/sink/git/validate_test.go:27`
  Failure: none — `TestExportMemory_rejectsTraversal` (`internal/sink/git/export_test.go:48`) now calls the test-local helper which invokes the identical production `validateObjectPath`; direct unit coverage of that validator is untouched.
  Fix: none.
  Confidence: 85
- [NOTE] Deleting `Engine.BindClusterTargetNamespaces` removes a footgun rather than a control: it wrote a bare `targetState` that cleared collection-state fingerprints outside `RegisterTarget` — its absence is security-positive, `internal/collect/engine.go` (deleted 397-417 block)
  Failure: n/a.
  Fix: none.
  Confidence: 85
- [NOTE] Exported-constant removal from the externally importable `api/v1alpha1` is an unprovable-here break: in-repo zero references confirmed (code, tests, docs, charts, CRDs; raw-string `"Connected"`/`"CredentialsVerified"` grep also empty) and the commit body carries `BREAKING CHANGE`, but an out-of-repo importer of the two constants would fail to compile
  Failure: external consumer pinning a pre-1.0 pseudo-version compiles today, breaks on bump — acknowledged and accepted in proposal Assumptions (line 108-113).
  Fix: none; changelog already records it via `04a16ff1` body.
  Confidence: 75

Checks performed: full diff of all 9 code commits read; `go build ./...`, `go vet ./...`, `go vet -tags=integration ./internal/sink/git/` all clean; `go test` green on `internal/sink`, `internal/sink/git`, `internal/inventory`, `internal/collect`, `internal/controller`. Traced untrusted-input paths (objectPath → validateObjectPath; bearer token → TokenReview/SAR → auth cache) and confirmed the live paths (`RunExportEnvelope` 3 call sites, `ExportWithBranch`) are byte-identical to the pre-branch code except the deleted wrappers. No new dependencies (imports only removed). No secrets in the touched code; `tokenHash` is SHA-256, no raw token or UserInfo stored after this branch (strictly less identity material retained than before). Circuit-breaker tests genuinely re-drive the live `RunExportEnvelope` with trip/reset assertions intact (`internal/sink/circuit_breaker_test.go:17-160`). Repo grep confirms zero surviving references to every deleted symbol except test-local helper names and historical CHANGELOG rows.

## Could not check
- `data/kollect-xconsol-final/report.md` — outside this repo, out of bounds; verified only the verbatim Sweep 2 quote inside proposal.md.
- Zero-caller claims against external importers of `api/v1alpha1` — no compile can reach them (proposal says so itself).
- Integration-tagged tests (`-tags=integration`) — vetted only, not run (need forgejo/live remotes).
- Evidence-file claims in `openspec/changes/dead-exported-surface/evidence/*.md` (probe outputs, coverage numbers) — took at face value, not re-derived.
