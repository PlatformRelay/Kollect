# PR description — sink-breaker-terminal-isolation

- **Source:** `fix/sink-breaker-terminal-isolation`
- **Target:** `main`
- **Title:** `:bug: fix(sink): keep one inventory's ownership conflicts out of the shared breaker`
- **Opens as:** **ready**, rebase merge once every check on the head is green (maintainer branch,
  reviewed through the spec loop).

Paste everything below the line.

---

## TL;DR

- **What:** a Git ownership conflict that belongs to one inventory no longer counts toward the
  shared per-sink circuit breaker. Every other export failure still counts, including sink-wide
  Terminal failures such as a revoked Git token or a protected branch.
- **Why:** five ownership rejections from one misconfigured inventory opened the breaker for every
  healthy inventory on the same sink for 30 s. The same shared breaker also made four reconciler
  tests flaky under `-race -count=3`.
- **Risk:** low. The change is a narrow exclusion (a marker on 7 raise sites). The breaker still
  sheds load for every failure that is not marked, and a site nobody marked keeps today's
  behaviour.
- **Reviewer action:** read `internal/sink/circuit_breaker.go` and the marked sites in
  `internal/sink/git/prune_owned.go` and `prune_target.go`.

## What changes

- **Marker.** `kollecterrors.InventoryScoped(err)` makes an error Terminal, keeps its text and
  chain, and marks it with `ErrInventoryScoped`. `IsInventoryScoped` tests for it.
- **Marked sites.** The ownership-conflict rejections in the Git prune and merge-target claim
  checks (7 sites) go through a new `ownershipConflict` helper. Rejections caused by shared state
  (prune metadata, repository contents) stay unmarked and keep counting.
- **Breaker.** `IsExcluded: kollecterrors.IsInventoryScoped`: a marked result counts neither as a
  failure nor as a success, does not reset the failure streak, and on a half-open trial frees the
  slot without closing or re-opening the breaker. Settings, key, threshold and timeout are
  unchanged; `newBreaker(key, timeout)` is extracted for tests.
- **Tests.** Breaker unit and boundary tests (incl. half-open and a concurrent trial), a
  real-Git-backend boundary test, a per-site table that fails if any marked site loses its
  marker, a go-git engine case. Reconciler tests that shared the `postgres-demo` breaker now use
  a per-invocation sink name; one parallel test that read a global metric delta is now serial.
- **Docs.** Troubleshooting row and an ADR-0602 note describe what counts and how half-open works.

## Risk

- A rejection marked inventory-scoped that is really sink-wide would stop the breaker from
  shedding load for that fault. Each of the 7 sites was classified in the probe and checked by
  two reviewers; ambiguous sites were left unmarked.
- An inventory that keeps failing its ownership check can keep a recovering sink half-open (one
  trial at a time). Each excluded trial frees the slot, so other inventories are not locked out.

## Files to check first

1. `internal/sink/circuit_breaker.go`
2. `internal/errors/errors.go` (`InventoryScoped`)
3. `internal/sink/git/prune_owned.go`, `internal/sink/git/prune_target.go`
4. `internal/sink/circuit_breaker_test.go`, `internal/sink/git/prune_scoped_marker_test.go`

Review commit by commit; each commit builds and passes the packages it touches.

## Testing

- `go test -race -count=3 ./internal/errors/ ./internal/sink/...` and
  `go test -race -count=3 -skip TestControllers ./internal/controller/` (ten consecutive green runs
  of the latter; 3 of 10 failed on the base before the isolation).
- Mutations: dropping the exclusion, swapping in `IsSuccessful`, excluding every Terminal,
  excluding every error, and reverting each marked site one at a time are each caught by a named
  test.
- `task verify`, `task lint`, `task spec:validate`, scrub and gitleaks green on the head.

Release note (fixes): a single inventory's Git ownership conflict no longer opens the sink circuit
breaker for the other inventories on that sink.

---

<details><summary>Optional: how this was decided</summary>

The first design excluded every Terminal error. The spec-set review showed that Git auth and
protected-branch rejections reach the breaker already Terminal, and that inventories re-export on
every resource event, so that design would have stopped the breaker shedding load for a revoked
token. It was re-decided to mark only inventory-scoped results. OpenSpec change
`openspec/changes/sink-breaker-terminal-isolation/` holds the spec, the per-task evidence and the
review records.

</details>
