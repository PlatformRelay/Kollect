## Verdict: CONCERNS

Round-one register findings are genuinely closed (verified by running both guards: plain green, TCE self-test rejects all 7 mutants with exact messages, CWS plain green). The fitness direction is right: coupling between `required_checks` and the local entry point moved from documented-by-hand to machine-enforced (TCE-3 reads `verify-eligibility.sh` dynamically, so a new required context fails the guard until mapped or excluded); the CWS-3 step allow-list was **narrowed** (`shell` dropped), the correct register direction. But the round-one fix itself left residue the new fitness function structurally cannot see, and two mutant holes survived it.

## Findings

- [WARNING] Round-one fix left a duplicate `run_gate go-mod` with two different drift scopes — `hack/check.sh:50,62`
  Failure: `task check` runs `go mod tidy` + `go mod verify` twice; the second declaration checks only `go.sum` while the guard pins the `go.mod go.sum` one — two definitions of one gate, and no existing function notices a second (or a third) declaration of the same gate, which is how this slipped through.
  Fix: delete line 62; add a gate-name-uniqueness assertion to `c_tce1_gates` (each `run_gate <name>` exactly once) as the ratchet from today's measurement.
  Confidence: 95 (verified by grep; no uniqueness assertion exists in the guard).
- [WARNING] An exclusion whose reason token is deleted entirely passes the guard — `hack/test/task_check_test.sh:176`
  Failure: line `exclusion test-integration` (reason token deleted, no trailing space) → the `${xline#...}` prefix-strip no-ops, `reason` falls back to the whole line, non-empty → guard stays green; only the `""` shape is caught (verified by execution: `MISSED: reason=<exclusion test-integration>`).
  Fix: fail when `[[ "${reason}" == "${xline}" ]]` (nothing was stripped), or assert the tail starts with a quote.
  Confidence: 90.
- [WARNING] Docker-skip register matches the whole guard file, not the header the spec names — `hack/check.sh:73` vs `openspec/.../task-entrypoints/spec.md:12-14`
  Failure: a future guard that merely *mentions* 'requires docker' (error string, comment) is silently dropped from the sweep forever; no assertion pins which guards may legitimately be excluded, and no TCE-4 mutant covers the too-aggressive direction. Round-one #3 was the same class for `--self-test` detection (`check.sh:82`) and that shape is also unchanged.
  Fix: match the phrase in the first ~10 header lines only, and pin today's excluded-guard set in the meta-test.
  Confidence: 70
- [NOTE] `hack/check.sh` is never executed anywhere — `hack/test/task_check_test.sh:264`
  Failure: `mutant_rejected` runs the *meta-test* against mutated copies (greps only); CI runs only the static meta-test — the aggregation loop, exit-1 path and Docker-skip decision are pinned by greps (`failures+=`, `exit 1$`), so a swapped ok/failed branch in `run_gate` (check.sh:13-22) passes every guard.
  Fix: smallest behavioural ratchet — one self-test leg that runs `check.sh` on a copy with a PATH-shimmed failing `task`, asserting exit 1 and the gate named.
  Confidence: 85 that no execution of check.sh exists anywhere (CI runs the guard statically; self-test greps copies).
- [NOTE] `task check`'s tool prerequisites are undeclared — `mise.toml:39` ([tools] has no yq), CONTRIBUTING.md has no mention
  Failure: a fresh contributor following mise.toml and setup docs gets no yq; `task check` reds at "yq not found" and again in the sweep — tooling failure on exactly the no-Docker machine TCE-1 promises to serve. CI installs yq ad hoc (snap, unpinned) at ci.yaml:326.
  Fix: one line documenting yq (or add it to mise [tools], accepting the unpinned-CI mismatch it mirrors).
  Confidence: 80 (mise.toml and docs grep-verified; yq present on this machine only via ad-hoc mise install).
- [NOTE] Evidence hygiene: `evidence.md` duplicates the exclusion paragraph verbatim, and `tasks.md` records "4 mutants + no-op" where 7 exist — `openspec/changes/task-check-entrypoint/evidence.md:14-18`, `tasks.md:34`
  Failure: reviewer trusts the "4 mutants" count and under-audits the self-test.
  Fix: delete the duplicate line; correct to 7.
  Confidence: 100 (verified by read)

## Could not check

- Full `task check` end-to-end — never run by anyone, including the author (tasks.md admits "post-merge"); the 73-guard sweep's combined runtime and tool deps are unmeasured, and nothing in CI ever executes check.sh itself.
- `ci_workflow_security_test.sh --self-test` — my run timed out at 120 s mid-mutants; only its plain mode verified green.
- zizmor/gitleaks binaries were not exercised here (only installed-on-run via pinned installers inside check.sh); the CWS-2 quoted-key spelling bypass noted in the round-one register (its #5) I did not re-verify.
- Whether `dependency-review` is actually branch-protected, as the exclusion reason claims.
- Untracked `reviews/branch-round2/` — not part of the reviewed range.
