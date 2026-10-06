Round-1 register and all four leg reports read. Now verifying the fixed spec set against the code at HEAD.
All checks done. Every round-1 register item and all four spec deltas verified against code at HEAD `7132b02c`.

## Verdict: CONCERNS
Round-1 register #1–#4, #6, #8 are genuinely closed (verified against code, not prose): the 8 requirement IDs now align across proposal/tasks/deltas; all doc sites from #2 exist and are enumerated in GTE-3 + T09; the BEP-1 re-store race is closed by the delete-tombstone requirement + scenario; T02 compiles-first ordering fixed; .gitignore dedup done; ADR-0802 reservation is real (8 code cites, theme 08 holds only 0801). Residuals: #5 and #7 partially closed, plus one new task-executability defect.

## Findings
- [WARNING] T04's "green-by-construction... pass before and after" regressions cannot pass: the named file:// and ls-remote fixtures construct `Config{Engine: GitEngineCLI}`, which T09 deletes — `tasks.md:36-38`
  Failure: after T09, `delete_cold_feature_branch_test.go:52`, `delete_mirror_regression_test.go:36,102,337,445`, `cli_env_test.go:23,167-185,386` fail to compile; no task owns the fixture rewrite, so the red-first sequence stalls exactly as round-1 feared.
  Fix: T09 bullet "rewrite Engine-field fixtures to engine-less configs"; T04 says "pass after fixture adaptation".
  Confidence: 85
- [WARNING] Register #7 half closed: persisted `engine: cli` sinks at upgrade still have no scenario — `specs/git-engine/spec.md:13-27`
  Failure: admission never re-runs on stored objects; only backend construction rejects at next export. GTE-1's SHALL implies it, but nothing pins the operator-visible outcome, and T04 tests only create/update rejection.
  Fix: one GTE-1 scenario: a stored cli sink's next export fails terminally naming `go-git`.
  Confidence: 75
- [WARNING] Register #5 partially closed: §7 is still unverifiable from the repo — `proposal.md:5-8`
  Failure: the citation is now honest (supervisor knowledge base, decisions relayed), but the register's fix (vendor a decision excerpt or mapping) was not done; "honours the five decisions" remains self-referential. I could verify the four implementable decisions' defects exist at HEAD and the fifth is coherently parked (D7; `values.yaml:79` vs `values.schema.json:40`), but not against §7's wording.
  Fix: append a decision→requirement-ID table (5 rows) to design.md.
  Confidence: 60
- [NOTE] BEP-1 specifies UID-only eviction; the D4/T08 ns/name fallback for tombstones without UID is designed but not required — `specs/backend-pool/spec.md:7-8` vs `design.md:62-64`, `tasks.md:55`
  Failure: an implementer reading only the delta drops the fallback; a `DeleteStateUnknown` event with empty UID evicts nothing.
  Fix: one clause in BEP-1 naming the fallback.
  Confidence: 70
- [NOTE] Tombstone type name inconsistent within the change set: `DeletionFinalStateUnknown` — `proposal.md:113` vs `DeleteStateUnknown` — `design.md:64`
  Failure: neither matches the other doc; implementer guesses the guard's shape. The defensive UID-first design holds either way.
  Fix: pick one name (verify against controller-runtime v0.24.1 source).
  Confidence: 90 (inconsistency), 60 (impact)
- [NOTE] KEX pin citation only half fixed: proposal Impact now says `ssh_auth.go:27` but Assumptions still say `:23-33`; the var is at `:27-36` — `proposal.md:101-102`, `internal/sink/git/ssh_auth.go:27-36`
  Failure: register disagreement #3 left standing; a reader grepping :23 finds nothing.
  Fix: s/23-33/27-36/.
  Confidence: 95
- [NOTE] Auth-mode parity across convergence unpinned (round-1 GLM's substantive GTE-4 concern); TSP-1 has no count+filter compound scenario — `specs/git-engine/spec.md`, `specs/target-status/spec.md:23-26`
  Failure: auth is shared today (`ssh_auth.go:46-48` comment: both paths call `effectiveSSHConfig`), but no regression test list names ssh/token auth after T09; count+filter moving together is covered only structurally by D3's single write.
  Fix: add auth-parity to T04's regression list; optional TSP-1 scenario.
  Confidence: 55

## Could not check
- `data/kollect-xconsol-final/report.md` §7 / pass B — live in the supervisor's knowledge base, absent from repo and git history; five decisions' original wording unverifiable here.
- controller-runtime v0.24.1 tombstone semantics (module cache outside repo) and x/crypto v0.57.0 KEX implementation claims.
- No execution (read-only brief): `task spec:validate`, `task verify`, `task lint`, tests, codegen.
- `reviews/R-spec-set/round-2/` leg outputs (other reviewers; out of bounds).
- The provided SPEC input is a round-1 snapshot (still carries ERA-3/TSP-2/BEP-3/GTE-4 and pre-fix D4); I reviewed the repo files at `7132b02c` as authoritative.
