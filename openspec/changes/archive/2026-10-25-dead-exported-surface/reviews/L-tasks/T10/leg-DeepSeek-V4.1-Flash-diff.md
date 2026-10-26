## Verdict: CONCERNS

The code change is correct and complete; the concerns are records-accuracy, which matter because records hygiene *is* this task. I verified mechanically: rename is total (`rg -in exportmemory -g '*.go'` → exit 1, zero hits); `commitInMemoryRepo` body is byte-identical to the deleted production `ExportMemory`; re-anchored lines are right (`internal/sink/export_test.go:70/:126/:180`, `backend_pool_log_test.go:23`, `export.go:75-77`/`:79-81`); no `(line NNN)` anchor remains in either touched test file.

## Findings
- [WARNING] T10 claims to resolve register finding 2 in full, but only the greppability half — the verbatim test-local copy of the deleted production `ExportMemory` survives unchanged. — `openspec/changes/dead-exported-surface/evidence/T10.md:4` (cf. `reviews/B/register.md:8`)
  Failure: orchestrator reads "resolves the two fix-class findings … findings 2 and 3" and drops a WARNING-class entry whose stated substance ("verbatim test-local copy … drift-prone") is untouched — `commitInMemoryRepo` is a byte-identical copy of `6e42ee59^:internal/sink/git/export.go`'s `ExportMemory` (diffed).
  Fix: change the wording to "resolves finding 3 and the greppability half of finding 2; the duplicate helper body is out of scope per 10.1" — no code change needed.
  Confidence: 90
- [NOTE] Matrix row 15 is marked `pass` on the independent-review class while row 3 of the same class is `not-run` and the `Independent review`/`Verdict` sections are still `(pending)`. — `openspec/changes/dead-exported-surface/evidence/T10.md:38`, `:95-101`
  Failure: a reader cannot tell which review-class rows were actually adjudicated; row 15 cites "register confirmation below" that does not exist in the committed tree.
  Fix: mark row 15 `not-run` (or `pending reviewer`) until the leg's register lands.
  Confidence: 95
- [NOTE] tasks.md ticks 10.1–10.3 closed in the same commit that leaves the record at `Status: REVIEW` with pending review/verdict. — `openspec/changes/dead-exported-surface/tasks.md:67-69`, `evidence/T10.md:7`
  Failure: committed state asserts task closure before the independent leg ran, contrary to the record's own "verify findings, amend records, close" ordering (`T10.md:8`).
  Fix: tick on the post-review amend, or mark status CLOSED in the same commit.
  Confidence: 85
- [NOTE] Commit subject under-describes the change: it also rewrites two comments in `internal/sink/export_test.go` and refreshes T1.md anchors (task 10.2). — `efbf4fe6`
  Failure: a reviewer reading only `git log --oneline` misses half the diff.
  Fix: body already implies it; add "and refresh stale anchors" to the subject/body scope line.
  Confidence: 100

## Could not check
- Full gate reproduction: I did not run `go test ./internal/sink/git/` (evidence records 337 s), `go build/vet ./...`, tag-vet, or `task spec:validate`; I relied on T10.md's recorded runs.
- Integration-tagged suites (`export_integration_test.go`, `export_forgejo_integration_test.go`) — compile-only per evidence; not executed.
- `reviews/L-tasks/T10/register.md` and `unify` do not exist yet (only the leg files), so no unified register to cross-check against.
