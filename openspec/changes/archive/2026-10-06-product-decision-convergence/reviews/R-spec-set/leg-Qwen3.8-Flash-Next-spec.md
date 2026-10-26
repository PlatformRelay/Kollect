## Verdict: CONCERNS

## Findings
- [WARNING] Proposal promises four requirement IDs that no spec delta defines — `openspec/changes/2026-10-06-product-decision-convergence/proposal.md:52-55` (ERA-1..**ERA-3**, TSP-1/**TSP-2**, BEP-1..**BEP-3**, GTE-1..**GTE-4**, incl. an "auth modes" surface with no requirement)
  Failure: the deltas define only ERA-1/2, TSP-1, BEP-1/2, GTE-1/2/3; tasks and the verification table reference only the eight that exist, so executing tasks.md silently ships 4 fewer requirements than the proposal claims, and `task spec:validate` (structural) will not catch the mismatch.
  Fix: renumber the proposal to the eight real IDs, or write the missing requirements with scenarios.
  Confidence: 90
- [WARNING] GTE-1 contradicts itself on `engine: cli` + `file://` — `specs/git-engine/spec.md:13-27`, `tasks.md` T04
  Failure: GTE-1 mandates backend construction rejects `cli`, yet the file:// scenario says "(any engine value it carried)" keeps working "as today", and T04 says the file:// probe tests "keep passing" while T09 deletes the internal `GitEngine` field those fixtures set (`internal/sink/git/config.go:170-177`; e.g. `delete_cold_feature_branch_test.go:52`). Also no scenario defines behaviour for an already-persisted `engine: cli` sink at upgrade — admission never runs on it, only construction rejection, so its exports die with no spec'd condition.
  Fix: scope the file:// scenario to `go-git`/empty engine, and add one scenario pinning the stored-`cli` upgrade outcome (terminal condition naming `go-git`).
  Confidence: 80
- [WARNING] BEP-1 does not address eviction during an in-flight export — `specs/backend-pool/spec.md:5-17`, `internal/sink/backend_pool.go:134,153,244-253`
  Failure: pooled backends are handed out with no-op release closures (no refcount); deleting a DatabaseSink while its COPY runs fires the new delete hook, `evictPoolKey` Closes the backend under the live export → in-flight export dies mid-write. TTL could never hit an in-use entry (48 h idle); eviction makes the hazard reachable, and no scenario tolerates or forbids it.
  Fix: add a BEP-1 scenario stating the in-flight-export semantics (tolerate failure, or mark-and-sweep-after-use).
  Confidence: 65
- [WARNING] "Delete event carries the last known state (UID present)" cites the wrong source and is unconditional — `proposal.md:102-103`
  Failure: `pkg/handler/eventhandler.go:105-146` (v0.24.1) is the `TypedFuncs` dispatch — it forwards `e.Object` verbatim; the tombstone machinery is `pkg/internal/source/event_handler.go:130-150`, which sets `DeleteStateUnknown=true` and can deliver a non-sink object. A D4 `DeleteFunc` without an `event.DeleteObject`/nil guard then panics on `GetNamespace()`/empty-UID path; T03 has no such case.
  Fix: cite the real file, and make T08/T03 cover the DeleteStateUnknown/nil-object branch.
  Confidence: 70
- [WARNING] The five product decisions are untraceable from the repo — `proposal.md:5`
  Failure: `data/kollect-xconsol-final/report.md` is not on disk and never existed in git history (no commit touches `data/`); the "honours §7" claim reduces to the proposal paraphrasing itself. A reviewer cannot check faithfulness, and the commit's `.gitignore` +3 suggests this is permanent.
  Fix: vendor the §7 decision list into the change (e.g. design.md appendix) or commit the report.
  Confidence: 85
- [NOTE] GTE-3's pipeline-image clause is untested and its fact goes stale — `specs/git-engine/spec.md:46-50`, `Dockerfile.pipeline:34-35`
  Failure: no scenario; the Dockerfile comment still names `git.engine: cli` as "unsupported" after the value stops existing; the GTE-3 check is docs-diff only.
  Fix: add the Dockerfile comment to T09's edit list.
  Confidence: 75
- [NOTE] T04 mixes red-first assertions with "keep passing" regressions in one "Red on the assertions" step — `tasks.md` T04; regression rows are green-by-construction, some must be rewritten (engine field removal).
  Confidence: 70
- [NOTE] Citation drift: `defaultSSHKeyExchangeAlgorithms` is at `internal/sink/git/ssh_auth.go:27-36`, not the cited `:23-33`.
  Confidence: 90

Verified code facts (held): zero annotation reads in `api/ internal/ cmd/`; `ANNOTATIONS-LABELS.md:100-101`; pool "no caller" comment `backend_pool.go:33-35` (test-only callers); TTL 48 h = 2×`MaxExportInterval` (`export_interval.go:22`); cluster count prose-only `kollectclustertarget_controller.go:309-316`; namespaced `syncCollectedCount`/`countChanged && !written` at `kollecttarget_controller.go:294,355`; cluster target has no printer columns today; x/crypto v0.57.0 registers `mlkem768x25519-sha256`/`group16-sha512` (`common.go:54,62`, `kex.go:409,448`), sntrup761 testdata-only; CRD enum/validation/config accept both engines today; family reconcilers instantiated for all three kinds (`cmd/main.go:276-296`), no finalizer branch; debounce call sites `:356/:487(preview)/:305` = D1's three; D7 chart facts (`values.yaml:79`, schema enum `["single","cluster"]`); Taskfile `verify/lint/spec:validate/test-integration/coverage:race` and `make generate manifests` exist.

## Could not check
- `data/kollect-xconsol-final/report.md` §7 and `data/kollect-xconsol-b/report.md` — absent from disk and git history; the five decisions' wording unverifiable.
- `task spec:validate` / `task verify` not executed (read-only run); assumed structural-only validation.
- `git ls-remote` probe sharing CLI machinery for both engines — cited via `connection.go`, not line-verified.
- Integration-tier rows (`task test-integration`) — no Docker here, all rows "not-run" by the spec's own account.
- Whether `closeBackendLogged` on a git backend mid-push aborts it — backend Close internals not read.
