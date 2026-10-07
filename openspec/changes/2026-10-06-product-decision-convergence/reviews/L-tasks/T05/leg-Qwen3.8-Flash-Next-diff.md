## Verdict: CONCERNS

## Findings

- [WARNING] `tasks.md` T05 checkbox never ticked, though the evidence names it in scope — `openspec/changes/2026-10-06-product-decision-convergence/tasks.md:45`, `evidence/T05.md:6`
  Failure: evidence lists "tasks.md tick" among files in scope, and `loop.md`'s ERA-1 known-red row was cleared in the same commit, but the diff (`git diff --stat 81cef0c1..HEAD`) contains no `tasks.md` change — the backlog still shows T05 unimplemented while loop.md says the reds are gone; a reader of tasks.md resumes T05 as fresh work.
  Fix: tick `- [x] T05` (with the closed/evidence trailer the T04 row uses) in the branch.
  Confidence: 95

- [NOTE] Evidence misdescribes its own doc-row diff re ADR-0201 — `evidence/T05.md:29` vs `docs/ANNOTATIONS-LABELS.md:101`
  Failure: evidence says the rewrite "keeps ADR-0201 only where it is the CRD-model context"; the rewritten row keeps no ADR-0201 anchor at all (the old citation was deleted outright). If the grep claim ("ADR-0201 does not mention requestedAt") is right, deletion is correct — but the evidence record is not, and it flags the wrong thing "to review".
  Fix: correct the evidence sentence to "citation dropped entirely".
  Confidence: 90

- [NOTE] The zero-interval interaction claimed in the axis comment has no test — `internal/controller/per_sink_export.go:78-79`
  Failure: the comment justifies check placement ("before the zero-interval early return so an interval==0 binding still re-exports"), but every requestedAt test uses a nonzero interval (`per_sink_export_test.go:185`, harness `ExportMinInterval` 5min; `shouldSkip_zeroIntervalAfterRecord` only exercises `""`). A future reorder of the `interval == 0` return past the requestedAt check keeps all tests green while silently breaking forced re-export for `exportMinInterval: 0` bindings.
  Fix: one unit case in `TestPerSinkCoalesceTracker_requestedAtAxis`: record with `""`, then `shouldSkip(..., "ts-2", 0, now)` must be false.
  Confidence: 85

- [NOTE] Task wording "both inventory controllers (export + preview paths)" overstates the cluster path — `internal/controller/kollectclusterinventory_controller.go` (zero `preview` hits), `tasks.md:45-47`
  Failure: only `KollectInventory` has a `previewAllSinksDebounced`; the cluster controller has no preview path, so the task/evidence phrase is unsatisfiable as literally written. Implementation correctly covers all three real `shouldSkip` call sites (`kollectinventory_controller.go:360,494`, `kollectclusterinventory_controller.go:309`).
  Fix: none in code; note the phrasing when T06/T07 reuse the task text.
  Confidence: 80

Verified correct (checked, not assumed): single spelling of the annotation key (`api/v1alpha1/constants.go:48` only non-test occurrence); no predicate filter on either `SetupWithManager`, so annotation-only edits do enqueue reconciles; absence≡empty-string semantics consistent between constant docs, code (`GetAnnotations()[...]`), and the doc row; one-shot behavior holds — `record` pins the value (`per_sink_export.go:115`) and a failed export leaves it unpinned so the retry re-exports, including mixed partial-success across bindings; preview and export read the annotation identically so the preview-honesty scenario holds; the fake-client `copyFrom` restore claim is real — I read `controller-runtime@v0.24.1/pkg/client/fake/versioned_tracker.go:233-244`, so the DeepCopy helper change is a legitimate TEST-class fix, not a masking change; six T01(a) test names match the loop.md removed row exactly; six reds → the six named tests all exist and assert through the production `exportToSinks`/`exportClusterToSinks`/`previewAllSinksDebounced` paths; mechanical `""` call-site adaptations touch no assertion.

## Could not check
- Ran nothing: no `go build`/`go vet`/`go test`/`task lint`/`gitleaks` (plan mode, read-only) — the entire verification matrix and the "4 expected TSP-1 reds only" claim are taken on trust from `evidence/T05.md`.
- The claimed pre-task red run at `81cef0c1` (red-first discipline) — a historical claim I cannot reproduce without a worktree.
- `docs_launch_truth_test.sh` and `task lint:markdown` results; CI behavior with the four known reds.
- Whether an unmodified `task-prompt.md` MD032 red pre-exists at HEAD (evidence claim, not verified).
