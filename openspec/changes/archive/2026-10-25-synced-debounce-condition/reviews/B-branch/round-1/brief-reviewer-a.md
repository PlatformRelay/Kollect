You are reviewing the whole branch of the OpenSpec change `synced-debounce-condition`,
independently of whoever wrote it. Read-only: do not modify any file. Work from the repository
root (branch `feat/synced-debounce-condition`, rebased on `origin/main`).

Lens: ADVERSARIAL, across task boundaries. Per-task reviews already saw each group's diff; you
see the feature as one change. Argue as four people in turn: a saboteur who wants the operator
to report a wrong condition or to stop exporting; a new maintainer who will change this code in
six months; a security auditor; and the owner of the alerts and scripts that read these
conditions. Attack, defend, revise each finding; drop what you cannot make concrete with a
path:line and a sequence of events.

Inputs:

1. SPEC — `openspec/changes/synced-debounce-condition/specs/inventory-sync-status/spec.md`
   (SDC-1..SDC-7), `proposal.md` and `design.md` in the same directory, `tasks.md` (groups
   T0.1, G1-G4; 5.1 is the merge, not code).
2. DIFF — `git diff origin/main..HEAD`. Production: `internal/controller/per_sink_export.go`,
   `kollectinventory_controller.go`, `kollectclusterinventory_controller.go`,
   `internal/inventory/status.go`, `internal/sink/resolver.go`. Tests in `internal/controller/`
   and `internal/inventory/`. Docs under `docs/`. Evidence under `evidence/`.
3. EVIDENCE — `evidence/G1.md`..`G4.md`, `T0.1.md` (matrices, reds, mutations, test changes),
   and `loop.md` (*Test changes*, *Triage*).

Attack in particular, each across the groups' boundaries:

a. Steady state: can any sequence of passes (debounce, export, periodic re-export, requeue,
   status-update conflict, operator restart with an empty in-memory tracker, leader change)
   flip the aggregate or a per-sink `Synced` between True and False, or keep re-exporting a sink
   whose inputs did not change? Is the carried-over per-sink record (G2) consistent with the
   identity check (G3) after a restart, when the tracker is empty but status still says
   `True/Debounced`?
b. Hidden failure: can a sink that really failed, was deleted, recreated, repointed by spec
   (generation) or made unresolvable ever end up `True` (per-sink or aggregate)? Does `forget`
   run on every failure branch of both reconcilers and the preview, and can a concurrent export
   goroutine re-`record` a sink after it was forgotten?
c. Concurrency: the tracker, the outcome mutex, the namespaced export goroutines; any data race
   or lock-order problem the race tests would not hit.
d. Read API (`internal/inventory/status.go`): key format `<family>/<name>` versus what both
   reconcilers write; time fallback; reason mapping order; a caller that relied on the old
   behaviour.
e. Breaking change: is every user-visible change (condition reason, message text, aggregate
   semantics, cluster `ExportSucceeded` message) stated in the upgrade guide and in the
   `BREAKING CHANGE:` footer of the commit that makes it? Anything user-visible that is not?
f. Tests: tautologies, assertions weakened by a *Test changes* row beyond what the cited SDC
   requirement allows, envtest specs that would pass with the feature reverted.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

Write your result, and nothing else, in this shape (at most 20 lines):

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [CRITICAL|WARNING|NOTE] <one line, with path:line and the event sequence>
could_not_check:
  - <what you had no access, time, or evidence to verify>
summary: <one line>
```
