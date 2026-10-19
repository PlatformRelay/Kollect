# Stage B round 1 — verification

Range `ae98ef106..df5170e57` (whole branch). Legs: reviewer-a, adversarial lens
(`brief-reviewer-a.md`, verdict **BLOCK**, two CRITICAL); reviewer-b, spec lens
(`brief-reviewer-b.md`, verdict **CONCERNS**, two WARNING, two NOTE). Holistic run:
`holistic.md` (one branch defect, H1). Every finding below was checked against the code on
2026-10-20; "verified" means read in the code and, where the row says so, reproduced by a red test.

## Findings

| Id | Leg | Sev (filed) | Claim | Verified? | Sev (after verification) | Disposition |
|---|---|---|---|---|---|---|
| H1 | holistic | — | `go mod tidy -diff` fails: three new controller tests import `k8s.io/utils/ptr`, which `go.mod` lists as indirect; the Preflight workflow would be red | yes (`task check` go-mod gate, diff shown in `holistic.md`) | WARNING | fix: use the package's `boolPtr` (`family_sink_connection_verified_test.go:23`) |
| A1 | reviewer-a | CRITICAL | a missing or empty Secret seen by the **full** read (after a successful metadata read) neither retires the pooled entry nor bumps the store fence (`backend_pool.go:203-206`), so a concurrent build from the pre-delete value is pooled (or stays pooled) after an acquire observed the deletion | yes, by reading: the error branch returns `caRulesFirst(...)` and touches no pool state; both interleavings (the other build stores after, or before, the failing acquire) are reachable | WARNING: SCR-2's "SHALL retire" is unmet for that acquire, but no later export uses the stale backend: every later acquire starts with an uncached metadata read, which fails on the deleted Secret and retires the entry (`backend_pool.go:313-323`) | fix (red first): a full-read not-found/empty retires the key, forced, as the metadata path's last attempt does |
| A2 | reviewer-a | CRITICAL | an unchanged, successfully exported `0s` inventory is exported again once its coalescing state ages out: the watchdog requeue keeps reconciling it every 30 s (`per_sink_export.go:137-139`), `shouldSkip` refreshes nothing, and another binding's `record()` prunes state whose `lastExport` is older than `coalesceStateTTL` (48 h, `per_sink_export.go:124`); the next reconcile finds no state and exports | yes, by reading; pre-existing on `ae98ef106` (same `nextDue` and TTL), but it contradicts SCR-7's "SHALL NOT be exported again by this requeue or any other timer" and the docs' "identical payloads never re-sent" (`troubleshooting.md:397`) | WARNING: one idempotent re-export per 48 h per binding, no data or credential risk | fix (red first): a skip refreshes the state's activity time; pruning uses the later of export and activity |
| B1 | reviewer-b | WARNING | after a first read that may predate the pooled entry, a failed re-read retires that entry (`backend_pool.go:325-326`) even when it is the newer one, a spurious retire for a timed-out read | yes, by reading: the entry is retired when the first read's key differs, whichever is newer | NOTE: the retire drains (holders keep it open, `retireLocked`), and the next acquire rebuilds from a read taken after the retire, so it costs one build, never stale use; not retiring was G3 round 2's P0 (an older entry left pooled). SCR-2's "SHALL NOT retire" sentence is about what the failed read observed; the retire comes from the earlier successful read | keep the behaviour; pin it with a test (the stale-first-read ordering: the newer entry drains and is rebuilt from the current value); state it in design *Fingerprint and acquire* and in SCR-2's wording |
| B2 | reviewer-b | WARNING | the SCR-7 watchdog sits in `setInventoryDegraded`/`setDegraded`, so every degraded path of a `0s` inventory (suspended, scope-denied, unreachable sink, spill-gated, no targets) requeues every 30 s | yes (callers `kollectinventory_controller.go:145,150,158,220`, cluster `:99,120,126,149,193,206`) | NOTE | reject (as in G2 rounds 1 and 2): tasks.md 2.4 names these two functions and 2.3's tests pin them; none of those paths exports (each returns before `exportToSinks`/`exportClusterToSinks`); a `0s` inventory now requeues them as the default `30s` interval already does |
| B3 | reviewer-b | NOTE | SCR-3's last sentence says a backend built by an acquire that **started** before another acquire stored or retired is not pooled; the fence generation is taken at the lookup (`backend_pool.go:387`), after the metadata read | yes | NOTE, harmless: the build's full read follows the lookup, so it is at least as new as the retire | fix: spec wording (the fence starts at the acquire's pool miss) |
| B4 | reviewer-b | NOTE | no cluster twin of `TestApplyInventoryExportOutcome_zeroIntervalTerminalNoRequeue` (deferred from G2) | yes (G2 *Harness gaps*) | NOTE | fix: a cluster-reconciler test with a `0s` interval and a terminal export failure |

## Must-read items from the dispatch

- **`observeAndLookup` failed re-read path** (G3 round-2 fix, unreviewed until now): both legs read
  it. Reviewer-a: holds (no stale use, holders drain). Reviewer-b: SCR-3 holds, SCR-2 partial (B1).
  Verified: the only effect beyond G3's claim is B1, which is a spurious retire, never a stale use
  or a close under a holder.
- **G3 residual** (the forced second read retires an entry stored during it): both legs agree it is
  harmless. Verified: `retireLocked` never closes a held entry; the acquire's rebuild reads the
  Secrets after the retire, so its build is at least as new as the retired entry's snapshot
  (`backend_pool.go:203` after `:384`); confirmed harmless.
- **G1: cleanup reading a Secret, or a backend closed in use**: neither leg found one. Verified:
  stream cleanup returns before the acquire (`cleanup.go:177`), Retain and shared identity on
  object stores return before it (`:188-191`); Git release and relational Delete need the sink and
  read the Secret by design (SCR-2 "Git release" row, design *Cleanup before acquire*). Every
  production Close of a pooled backend goes through `retireLocked` (held: deferred to the last
  release), `owningRelease` (unpooled builds) or the surplus close of a build nobody received;
  only `ResetBackendPoolForTest` (test only) closes held entries.
- **G2 gap** (cluster terminal no-requeue twin): B4, fixed.

## Round 2

Reviewer-a filed two CRITICALs (A1, A2); both are verified real defects, downgraded to WARNING
because neither lets a stale credential reach a later export. The fixes change behaviour, so
round 2 runs on the fixes with fresh legs.
