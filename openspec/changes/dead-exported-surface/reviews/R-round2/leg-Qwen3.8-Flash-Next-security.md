## Verdict: CLEAN

## Findings

- [NOTE] Task 7 rewrites the auth cache's `get`/`set` but leaves, unremarked, that the cache is an unbounded map whose expired entries are never swept — `internal/inventory/auth_cache.go:44-48`, `:59-63`
  Failure: a valid-token holder repeatedly varies `namespace`/`inventory` query params (`auth.go:113`); each distinct scope adds a permanent map entry — expired entries are only lazily skipped, never evicted, so in-memory growth is unbounded for the process lifetime. Pre-existing, not introduced by this spec, and out of a deletion change's remit — but task 7 edits exactly these functions and records nothing.
  Fix: add one line to the proposal's Non-goals recording the unbounded/no-sweep property as known debt, so the edit is on record.
  Confidence: 85 (defect pre-existing; the finding is only the unrecorded touch)

- [NOTE] Deleting `ConditionConnected`/`ConditionCredentialsVerified` is a breaking change to the only externally importable surface in the sweep — `api/v1alpha1/constants.go:10-11`
  Failure: a downstream Go module importing these identifiers fails to compile on bump; `go build ./...` cannot prove the external half, as the proposal itself admits (`proposal.md:99`).
  Fix: nothing more in-spec — call it out as a breaking-identifier entry in the PR description/CHANGELOG, not just the proposal.
  Confidence: 60 (module pre-1.0, no shipped object ever carried these conditions — literals `"Connected"`/`"CredentialsVerified"` have zero writes or doc/CRD references, verified by grep of `api/`, `internal/`, `docs/`, `config/`, `charts/`, `openapi/`, `specs/`)

- [NOTE] Identity binding of the auth-cache decision survives the `user`-field removal — verified, not a defect — `internal/inventory/auth.go:122` (`_ = user`), `auth_cache.go:66-74`
  Failure: none. A cache hit is bound to the caller solely via `sha256(token)` in the key, so possession of the same token is already proven on a hit; dropping the discarded `UserInfo` cannot weaken authz. Deleting the unsized dead `RunExportItems` path is itself security-positive (`kollectclusterinventory_controller.go:278` says that path applied no size bound).
  Fix: none.
  Confidence: 90

Probes independently re-run at HEAD and confirmed: `RunExportItems`/`ExportItemsRequest` non-test hits only in `internal/sink/export.go` plus the stale comment; `MergeRequestAPI` definition-only; `git.Export`/`ExportMemory` zero package-level callers; `BindClusterTargetNamespaces` definition+tests; capability aliases live in `sink.SnapshotStoreCapabilities()` etc. from `internal/controller` and `internal/pipeline` test packages, which tasks 5.1–5.2 do anticipate ("multiple packages outside internal/sink/"). The live-path facts of task 1.3 check out: `exportThroughBreaker` at `export.go:267` inside `RunExportEnvelope`, and both named breaker tests exist in `circuit_breaker_test.go:20,77`. No new dependencies, no secrets in the touched paths (`bearerToken` errors at `auth.go:106` echo only static strings), no allow-list/baseline widening anywhere in the spec.

## Could not check
- Did not run `go build`, `go vet -tags integration`, or any test suite — every compile/probe claim in tasks.md is asserted-not-executed from my side (all verification-table rows are `not-run`).
- Did not read round-1 review records under `openspec/changes/dead-exported-surface/reviews/` (other reviewers' outputs — out of bounds by instruction).
- External-consumer analysis of `api/v1alpha1` constants is unprovable in-repo, as the proposal states.
- Did not read `AGENTS.md`/constitution or the `data/kollect-xconsol-final/report.md` source the proposal cites (§4 Sweep 2, §7 item 4).
