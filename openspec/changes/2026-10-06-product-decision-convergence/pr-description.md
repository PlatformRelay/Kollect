# Product decisions: implement the documented annotations, parity, eviction and one git engine

## TL;DR

- **What:** implements the four implementable product decisions from the final consolidated review §7: `kollect.dev/requestedAt` and `kollect.dev/collectedGeneration` become real behaviour; `KollectClusterTarget` gains `status.collectedCount`/`collectedCountUpdatedAt`; deleting a family sink evicts its pooled backend immediately; the git sink converges to one engine (go-git), with the modern SSH KEX algorithms the pinned x/crypto already implements.
- **Why:** the docs promised all four; the code read none of it. GitOps users coding against `ANNOTATIONS-LABELS.md` got nothing; a deleted sink held a pooled backend (connections, credentials) up to 48 h; the CRD doc's "cli required for SSH/KEX edge cases" claim was stale (the KEX gap was our own in-repo pin; the CLI engine cannot serve the pipeline image, which ships no git binary).
- **Risk:** the git-engine convergence rejects `spec.git.engine: cli` (CRD enum + admission + backend construction). Sinks using it stop exporting with a terminal error naming `go-git`; nothing is deleted. Everything else is additive.
- **Reviewer action:** start with `internal/sink/git/config.go`, `internal/validation/git.go`, `internal/sink/backend_pool.go` (tombstone), `internal/controller/family_sink_controller.go` (delete watch), `internal/controller/per_sink_export.go` (requestedAt), `internal/collect/prune.go` (stamp).

Open product item (not in this PR): the Helm `mode` value — wiring target undefined; options recorded in the change's design.md D7, decision requested from the captain (needs-decision open).

## What changed

1. **Annotations (ERA-1/ERA-2, new capability `export-annotations`)**
   - `kollect.dev/requestedAt` on `KollectInventory`/`KollectClusterInventory`: changing the value (set/unset/any change) invalidates the per-sink export debounce for exactly one export; steady state resumes after.
   - `kollect.dev/collectedGeneration`: Resource-mode embedded object copies get the source `metadata.generation` stamped into `metadata.annotations` after prune and scrub; no metadata map → no stamp (the default `include: SpecAndStatus` drops metadata — the doc row states this).
2. **Cluster-target parity (TSP-1, new `target-status`)**: `status.collectedCount` + `collectedCountUpdatedAt` + printer columns, mirroring `KollectTarget`; the count rides the cluster path's single escape-hatch status write.
3. **Backend-pool evict-on-delete (BEP-1/BEP-2, new `backend-pool`)**: family-sink delete watch evicts by UID (ns/name only as the no-UID tombstone fallback) and Close runs; a delete-tombstone makes an in-flight build's re-store discard instead of re-pooling; the TTL stays as backstop; a closed nats backend can no longer re-dial (evict-during-use leak, T13).
4. **Git engine convergence (GTE-1..GTE-3, new `git-engine`)**: `engine: cli` rejected everywhere; `file://` remotes and `ls-remote` probes keep the CLI machinery; KEX offer gains `mlkem768x25519-sha256` + `diffie-hellman-group16-sha512` (both already in x/crypto v0.57.0 — the offer was the limiter); git config faults are terminal at construction; every `engine: cli` doc site truthed up; ADR-0803 (0802 left reserved — eight pipeline code comments already cite it).

## Risk

- The engine removal is the only behaviour regression: users who set `engine: cli` (opt-in) get an admission error and, for persisted objects, a terminal construction failure named in `docs/operator-manual/upgrading.md`.
- The `requestedAt` bypass adds one export per annotation change even with unchanged content — bounded, and the intended feature.
- Evict-on-delete Close runs inline on the informer dispatch goroutine (same shape as the pre-existing TTL prune); deferred with rationale in design.md D4.
- Known pre-existing test hazards recorded, not introduced: breakerRegistry parallel-test race, collect `-count=2` non-idempotency (loop.md harness proposals).

## Testing

- Spec-first: OpenSpec change `2026-10-06-product-decision-convergence` (proposal, design, 4 spec deltas, 12 tasks all closed with evidence under `evidence/`); every behavioural change went red-first through the production path.
- Gates on the final tree (rev recorded in tasks.md): `task spec:validate` 14/14 strict · `task verify` clean · `task lint` clean · `task coverage` 91.3% (floor 90) · `-race` green on every changed package (`-count=2` where the suite permits, incl. `internal/sink/git` 505 s).
- Three independent review gates (free-model fan-out, no paid legs): spec-set (round 1: 4/6 legs, 1 CRITICAL + 20 findings fixed; round 2: clean of CRITICALs), per-task diff reviews (2 legs each, all findings verified and dispositioned in registers), whole-branch two rounds (round 1: 6/6 legs, 5 fixed in T11; round 2: 3/3, 1 CRITICAL confirmed → fixed in T13).

## Deep-dive pointers

- Requirements and scenarios: `openspec/changes/2026-10-06-product-decision-convergence/specs/{export-annotations,target-status,backend-pool,git-engine}/spec.md`
- Design decisions D1–D7 (incl. the parked helm `mode` decision): `design.md`
- Verification table with executed evidence: `tasks.md` §3
- Review registers: `reviews/R-spec-set/`, `reviews/L-tasks/T*`, `reviews/B-branch/`
- Review the commit-by-commit: test tasks (T01–T04) precede their implementations (T05–T09); review fixes are separate commits.
