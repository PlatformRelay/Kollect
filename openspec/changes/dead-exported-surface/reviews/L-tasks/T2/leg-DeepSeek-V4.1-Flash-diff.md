## Verdict: CONCERNS

## Findings
- [WARNING] The removal of two exported constants from the importable (non-`internal/`) package `api/v1alpha1` is rendered in the commit-derived changelog as a plain Refactoring bullet with no `[**breaking**]` marker — `api/v1alpha1/constants.go:8-14` (deleted lines 10-11); commit `ee736c87`
  Failure: git-cliff (`hack/release/cliff.toml:20-40`) prints only the commit subject and sets `commit.breaking` from a conventional-commit `!` or a `BREAKING CHANGE:` footer. The subject is `:recycle: refactor(api): delete unused exported symbols (…)` and the body has neither marker, so `CHANGELOG.md` will list it under `### Refactoring` without flagging that a module-public Go API shrank. The repo already uses the marker (`[**breaking**]`, e.g. `CHANGELOG.md:19`).
  Fix: one-line, but it rewrites history: `refactor(api)!: …` or append `BREAKING CHANGE: removes exported ConditionConnected/ConditionCredentialsVerified from api/v1alpha1`. If history is frozen, note it in the PR description (which task 8.2 already requires for other exclusions).
  Confidence: 90 (subject/body and cliff config read directly; not verified how a future release run groups it).
- [NOTE] The evidence file still reads `Status: IN PROGRESS` with `Independent review` / `Verdict` `(pending)` — `openspec/changes/dead-exported-surface/evidence/T2.md:7,97,99`. Expected for a not-yet-closed task; this review is the missing input. No action beyond closing it.
- [NOTE] `ConditionConnected`/`ConditionCredentialsVerified` were exported from a non-`internal/` package, so the deletion is externally breaking with no in-repo sensor able to observe it; this is recorded as an accepted limit at `proposal.md:108-113` and the round-2 register already carries it (finding #4). Not re-litigating; noting it is the only residual risk the compile backstop cannot cover.

## What I verified (all reproduced on `ee736c87`)
- Zero references: `grep -rn "MergeRequestAPI|ConditionConnected|ConditionCredentialsVerified"` across the repo returns hits **only** inside `openspec/changes/dead-exported-surface/` (this change's own proposal/tasks/review records). No `docs/`, `config/`, `charts/`, `test/`, `hack/` or Go hit survives; no `"Connected"`/`"CredentialsVerified"` string literal anywhere in Go.
- No build-tag file references any deleted symbol (looped `^//go:build` files, zero hits), so the `//go:build integration` concern is moot.
- `go build ./...` exit 0; `go vet ./api/... ./internal/sink/gitlab/...` exit 0; `go test -count=1 ./api/... ./internal/sink/gitlab/...` green (`api/v1alpha1` 0.46s, `internal/sink/gitlab` 36.9s).
- Diff scope is exactly the two named files, +7/−18; the +7 are gofmt realignment of the shrunken const block. `RESTClient`, its `EnsureOpenMergeRequest` method and the concrete call at `internal/sink/gitlab/mr.go:143` are untouched — the interface had no implementation assertion anywhere.
- Commit body names all three removed symbols (`git log -1 --format=%B`), and `CHANGELOG.md` is git-cliff-generated, not hand-edited.
- Fitness: pure deletion adds no import edge, dependency, package or function; no determinism-sensitive code touched; coverage floor (`Taskfile.yml:206`, 90%) can only be helped by unreachable code leaving with its dead-path tests.

## Could not check
- External module consumers of `api/v1alpha1` (out of the repo; the proposal's stated accepted limit) — cannot confirm none exists.
- Full `task test`, `task lint`, `task coverage`, `task spec:validate` on the final tree (only the two affected packages were run); `go vet ./...` repo-wide.
- Authorial intent behind the `MergeRequestAPI` "testable stub" comment (whether a test seam was planned) — not recoverable from the tree.
