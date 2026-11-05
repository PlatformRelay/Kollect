# Cold review: branch `fix/sink-breaker-terminal-isolation` (lens: adversarial, whole branch)

You are an independent reviewer with read-only access to this repository (Go Kubernetes operator
"kollect"). Review the whole branch: `git diff $(git merge-base HEAD origin/main)..HEAD` (10 commits).
The spec is the OpenSpec change `openspec/changes/sink-breaker-terminal-isolation/` (`proposal.md`,
`design.md`, `specs/sink-circuit-breaker/spec.md`, `tasks.md`); per-task evidence is in its
`evidence/` and per-task reviews in `reviews/L-tasks/`. Per-task reviews saw one diff each; you see
the feature. Note: the spec set was rewritten after its last spec-set review round (decision changed
from "exclude all Terminal errors" to "exclude only results marked inventory-scoped"), so it has not
been reviewed in its final form — review it too.

Goal of the change: one inventory's Git ownership rejections must no longer open the shared per-sink
circuit breaker for every other inventory on that sink, while sink-wide failures (Transient,
Forbidden, unclassified, and sink-wide Terminal such as Git auth / protected-branch rejections) keep
opening it. Plus: reconciler tests that shared a breaker key are isolated.

Attack it:

1. Compliance per requirement SCB-1, SCB-2, SCB-3: holds / partial / contradicted / absent /
   stronger than spec; and what the code does that no requirement mentions.
2. The marker (`internal/errors`): can it be lost between the raise site and the breaker (any wrap
   without `Unwrap`, `%v`, re-classification, `ClassError` nesting where the outer class wins, Git
   `ClassifyExportError` running on an already-marked error, multipart/layout export paths that call
   a different prune path)? Can it be attached to a result that is actually sink-wide (shared prune
   metadata, repository state, capacity limits)? Check every marked site against T0.1's table in
   `evidence/T0.1.md` and the code.
3. Ownership rejections from paths other than the 5 marked sites (GitLab MR backend, object-store
   layouts, cleanup/delete paths): do any still trip the breaker for other inventories?
4. Half-open behaviour under load: can one inventory's repeated marked results starve recovery
   (serialise a sink indefinitely)? Is that what the spec accepts?
5. Tests: could any new test pass vacuously (no `=== RUN`, keys that collide across `-count`, timing
   windows, a stub that never reaches the breaker)? Were any existing assertions weakened in G2's
   isolation (sink names changed — did every assertion that names the sink follow)?
6. Docs (`docs/operator-manual/troubleshooting.md`, `docs/adr/0602-error-taxonomy.md`): accurate
   against the code; nothing private (no internal ids, agent-context references).
7. Commit-by-commit: does each commit build on its own (the change claims the tree is green at every
   commit)? Conventional gitmoji titles; no AI/co-author trailers; a `fix` that changes user-visible
   behaviour has a title that will read well in a changelog.

Output: findings, each with severity CRITICAL / WARNING / NOTE, file:line evidence, and a concrete
fix; CRITICAL only for a defect that ships wrong behaviour or makes a requirement fail. End with
`## Verdict` and one of APPROVE / CONCERNS / REQUEST_CHANGES / BLOCK.

## Round 2 (last)

Head is now `d243fd5f4` (12 commits). Round-1 findings and their dispositions are in `reviews/B-branch/round-1/verification.md`; the fixes are commit `7a26657c0` (spec wording) and `d243fd5f4` (task B1.1: per-site marker table, go-git engine case, doc comment, evidence wording). Check each round-1 fix holds and look for defects the fixes introduced; do not re-raise dispositions already rejected or deferred there unless you have new evidence.
