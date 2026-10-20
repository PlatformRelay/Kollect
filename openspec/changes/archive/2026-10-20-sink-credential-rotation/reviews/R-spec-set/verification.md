# R spec-set review, round 1: verification

Revision reviewed: `84f61bba3` (spec set). Legs: reviewer-a (spec lens, cold brief
`brief-reviewer-a.md`) → BLOCK; reviewer-b (spec lens, fanout.sh `--no-unify`) → CONCERNS. Each
finding below was checked against the tree and the pinned module sources by the orchestrator on
2026-10-19.

| # | Finding | Sev (leg) | Verified | Disposition |
| --- | --- | --- | --- | --- |
| A1 | Cached reads cannot give the next-export guarantee (informer lag) | CRITICAL (a) | yes: the pool would read through the manager client's informer | accepted; design decision run (brief `agent-context/decisions/2026-10-19-rotation-secret-read-path.md`, both legs → C): uncached metadata GET per acquire, full GET on rebuild. SCR-1 text and scenario, design *Option chosen*, *Readers*, tasks 1.1 |
| A2 | `--watch-namespaces` cache rejects allowlisted cross-namespace Secrets | WARNING (a) | yes: `multi_namespace_cache.go:239` (controller-runtime v0.24.1) | fixed by C; envtest spec added (1.1); compatibility row |
| A3 / B3 | Key and values from different reads; slower builder replaces a newer entry | WARNING (a), NOTE (b) | yes: `backend_pool.go:208-214` stores last writer | fixed: key from the full objects, store fence (design step 3-4); tests `_keyFromBuildSnapshot`, `_slowBuilderDoesNotReplaceNewer`; SCR-3 scenario |
| A4 | "No acquire rebuilds it" contradicts the tombstone path | WARNING (a) | yes: `backend_pool.go:145-163` builds an unpooled backend | fixed: wording "re-pools"; tombstone and BEP-1 relation stated in design |
| A5 | SCR-4 promises redaction of driver constructor errors | CRITICAL (a) | yes: `mongodb/backend.go:97-105` wraps driver errors | accepted as a scope defect; SCR-4 narrowed to errors and logs this change adds, a non-goal names client errors |
| A6 | Values copied on every acquire; "read only on rebuild" false | WARNING (a) | yes for the cached design | fixed by C (metadata-only on a hit); *Secret data lifetime* rewritten; SCR-4 scenario "a pool hit reads no values" |
| A7 | `referencedSecretRefs(spec)` cannot resolve an omitted namespace | WARNING (a) | yes | fixed: takes `defaultNamespace`; test rows with omitted and explicit namespace |
| A8 | Planned reds inconsistent across groups; `secretMeta` errors split across groups | WARNING (a) | yes: G3's red only exists on the G2 revision | fixed: each group's base revision named; controls marked; G1/G2 split stated |
| A9 / B5 | Matrix gaps: CA handshake, TTL with holders, CLI `stringData`/env | WARNING (a), NOTE (b) | yes | fixed: `TestS3Export_caRotation`, `_ttlSkipsHeldEntry`, CLI rows |
| A10 | Close assumption contradicted for Kafka and MongoDB | WARNING (a) | yes: kafka-go `writer.go:551-554`, mongo-driver `client.go:299-306` | fixed: assumption rewritten per client; holder draining kept for whole operations |
| A11 | Legacy `KollectSink` has no production export path | WARNING (a) | yes: `resolver.go:35` resolves family kinds only | fixed: claim removed, non-goal added |
| A12 / B7 | Citation drift (`export.go:109`, `cleanup.go:198`, marker `:24`, `role.yaml:6-16`, mirror key, eventsink page) | NOTE | yes | fixed |
| B1 | Stream-sink cleanup acquires a backend it never uses, so a deleted Secret blocks inventory deletion | WARNING (b) | yes: `cleanup.go:184-214` | fixed: retain decided before the acquire for stream types; SCR-2 text and scenario, test, compatibility row |
| B2 | Retire on any error closes a shared backend on one caller's cancelled context | NOTE (b) | yes: design said "any error" | fixed: retire only on not-found, empty, failed rebuild; `_cancelledReadKeepsEntry` |
| B4 | Partial S3 Secret still falls back to ambient credentials | NOTE (b) | yes | fixed in docs scope: "entirely empty" stated; partial case named as unchanged |
| B6 | Line-based `rg` misses a wrapped stale sentence | NOTE (b) | yes | fixed: `rg -U 'keeps its\s+CA'` |

Deferred: a metric for the pool's Secret reads (decision counterpoint) → follow-up, proposal
non-goals.

Two CRITICALs accepted (A1 with a decision, A5 as a scope fix), so round 2 runs with fresh legs on
the fixed revision.
