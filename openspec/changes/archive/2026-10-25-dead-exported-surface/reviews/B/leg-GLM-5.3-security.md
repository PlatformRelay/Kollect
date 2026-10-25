Full code diff read. Now verifying the deletion claims against the tree (zero-caller probes are the load-bearing premise), plus the auth middleware context.
Zero dangling references to the deleted symbols. Now reading the auth middleware in full — the one security-relevant production change.
Auth cache: SHA-256 token hashes as keys, no plaintext tokens, and the dropped `user` field was never read (SAR re-runs on every miss with a fresh TokenReview). Now verifying the compile-clean backstop and the tagged-file claim.
Tagged vet passes, no dependency changes. Now checking the production call sites that must survive, the gitlab RESTClient orphan question, and the production shape the new test helper claims to mirror.
Production mirror confirmed (kollectclustertarget_controller.go:216-232 vs the test helper). The aggregate test run hung; retrying per-package with bounded timeouts to isolate.
The `internal/sink/git` test package fails with a 240s timeout in an `os/exec` goroutine. This is exactly the package this branch rewrote its test helpers in. Pulling the full failure header.
The hanging test file (`owned_prune_fuzz_test.go`) is untouched by this branch — but I need to know if it's a real hang or just package-wide slowness. Running it in isolation.
Green at 332s — my earlier timeout was the problem, not the branch. Now the spec records (tasks.md, loop.md) and repo conventions.
Records are consistent with my independent checks. Final security passes: bearer-token error path (echo risk), credentials in touched test files, and the breaking-change commit bodies that claim to name the removed API.
One last suite I haven't run: `internal/controller` — the most-touched test package. Running it now.
## Verdict: CLEAN

Security review of `3ee21266..HEAD` (dead-exported-surface, 9 code commits + records). Read the full diff, both touched production files in full (auth middleware, git export), the spec, tasks/loop records, and repo gates.

**What was verified against code, not documents:**
- Zero dangling references to all 12 deleted symbols outside openspec records (`rg` over non-`.md` tree, incl. literal strings `"Connected"`/`"CredentialsVerified"` — no bypass usage).
- `go build ./...`, `go vet ./...`, `go vet -tags integration ./...` all green; suites green for inventory, collect, api, pipeline, sink, sink/git, sink/gitlab, controller (git pkg 332s — slow, not hung).
- Live dispatch intact: 3 production `RunExportEnvelope` call sites survive (`internal/sink/cleanup.go:364`, `internal/controller/kollectinventory_controller.go:404`, `internal/controller/kollectclusterinventory_controller.go:331`); `RESTClient` still used (`internal/sink/gitlab/mr.go:137`); no go.mod/go.sum change (zero new dependencies).
- Test helper `newEngineWithBoundClusterTargets` faithfully mirrors production binding (`kollectclustertarget_controller.go:216-232`: synthetic target + `kubernetes.io/metadata.name` selector + `EffectiveNamespaces`).
- Breaking-change commit body names the removed symbols as claimed (04a16ff1).

**Security-audit trace (persona substantiation for the clean pass):**
- Saboteur: tried tenant-confusion via the auth cache — key is SHA-256(token)+verb+ns+name:resource (`internal/inventory/auth_cache.go:62-70`), so cross-tenant collision is impossible; partial-failure/idempotency attacks moot (deletion-only change; live path untouched).
- Security Auditor: traced `Authorization` header → `bearerToken` → `tokenHash` → cache key. No plaintext token stored or logged; bearer error strings carry no token material (`internal/inventory/bearer.go:12-22`); TokenReview+SAR re-run on every cache miss (`internal/inventory/auth.go:127-145`). The removed `authCacheEntry.user` was write-only (base `_ = user` at auth.go:122) — dropping it deletes retained username/groups/UID from process memory: a data-minimisation improvement, no authz semantic change. No insecure defaults, no secrets in the diff (Forgejo creds come from testcontainers env, not literals).
- New Hire: store.go rationale comments rewritten to describe surviving behaviour, no dangling names; test helpers intent-named (`exportForTest`, `exportMemory`).
- Budget Holder: value is one sentence (unreachable surface stops advertising itself); ~2,500 of the +3,132 lines are spec records, the code delta is net deletion.
- No allow-list or exclusion widened: lint config untouched (T9 fixed 2 govet shadow findings in test files, config unchanged).

## Findings
- [NOTE] Breaking deletion of importable `api/v1alpha1` constants is only in-repo-provable — `api/v1alpha1/constants.go:7`
  Failure: an out-of-repo module importing `ConditionConnected`/`ConditionCredentialsVerified` fails to compile after upgrade; not a security hole (constants never wired to a status write).
  Fix: none in-band needed; the `BREAKING CHANGE:` footer (04a16ff1) is the correct mechanism; accepted assumption already recorded (`proposal.md:108-113`).
  Confidence: 95 that the residual risk is real-but-accepted; 0 that it was mishandled.
- [NOTE] Two `t.Parallel()` breaker tests share one process-global breaker keyed on the same sink — `internal/sink/circuit_breaker_test.go:28,115`
  Failure: parallel scheduling lets trip/reset of one test interfere with the other → rare flake.
  Fix: out of scope — identical shape before the migration (same constants, same global breaker); scoped reset already proposed to the owner (`loop.md:100`).
  Confidence: 80 (loop.md itself records it); 100 pre-existing, not introduced here.
- [NOTE] `internal/sink/git` package suite needs >5 min wall on loaded machines — `internal/sink/git` (green at 332s with `-timeout 600s`; my first 240s run hit the alarm mid-package, panic pointing at an unrelated untouched test)
  Failure: a reviewer or gate with a tight per-package timeout reads green as red.
  Fix: none in-band; the 900s-window lesson is already recorded (`loop.md:98`).
  Confidence: 100 (observed twice on this run).

## Could not check
- `task lint`, `task coverage` (the 91.3% floor claim), `task spec:validate`, `govulncheck`, arch-lint: not run by me (time budget); independently ran build/vet/tagged-vet and 8 package suites instead.
- External Go-module consumers of the deleted constants (out of repo, accepted-limit in the proposal).
- The upstream consolidating report (`data/kollect-xconsol-final/`) — outside this repo, out of bounds per brief.
