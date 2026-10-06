Now the fitness-function context: lint coverage, CI wiring guards, and the exclusion registers.
## Verdict: CONCERNS

Verified by running: `task_check_test.sh` plain (green, all 5 assertions), `shellcheck --severity=warning` on `hack/check.sh` + both guards (clean), plus full reads of the range, spec delta, `verify-eligibility.sh` and the CWS-6 machinery. Fitness direction is right: `required_checks` became a fail-closed register in both directions (TCE-3 reads `verify-eligibility.sh` dynamically, and the round-two reviewer's uniqueness ratchet was actually added, `task_check_test.sh:141-145`); the CWS-3 step allow-list was **narrowed** (`shell` dropped, `ci_workflow_security_test.sh:325`); CWS-2's duplicate yq block is gone and the suppression policy tightened with 3 new mutants; no allow-list, exclusion or baseline was widened — no CRITICAL. But two round-two findings survived the "rounds 1-2 fixed" claim.

## Findings

- [WARNING] TCE-4's "SHALL run on pull_request in a required job" is pinned by nothing — `hack/test/task_check_test.sh` (absent self-pin) vs `ci_workflow_security_test.sh:556-563`
  Failure: deleting **both** TCE steps from ci.yaml leaves the guard CI-unrun and nothing reds: CWS-6's coverage loop only audits scripts with ≥1 invocation row (`ci_workflow_security_test.sh:584-591`), so a zero-invocation guard silently exits the gate set. (Deleting only the `--self-test` step *is* caught by CWS-6's mode pinning — verified by reading 565-591.)
  Fix: mirror CWS-6's self-pin in plain mode — assert both modes of `task_check_test.sh` have a required-job row in a `pull_request` workflow (~12 lines).
  Confidence: 85
- [WARNING] `hack/check.sh` is never executed anywhere — `hack/test/task_check_test.sh:299-353`
  Failure: round-two finding, unfixed by the fix commits (all 8 mutants grep mutated copies; neither CI nor the self-test ever runs `check.sh`). A swapped ok/FAILED branch in `run_gate` (`hack/check.sh:30-35`) passes every guard; aggregation, exit-1 and the Docker-skip decision are grep-pinned only.
  Fix: one self-test leg that executes `check.sh` on a copy with a shimmed failing `task` and asserts exit 1 plus the gate named in output.
  Confidence: 85
- [NOTE] Full end-to-end `task check` never run by anyone — `openspec/changes/task-check-entrypoint/tasks.md:34`
  Failure: TCE-1's own matrix row admits "a full local `task check` run is post-merge" — the composed gate's combined runtime and tool prerequisites (helm-unittest plugin, envtest assets, pinned downloads) are unmeasured on the machine it promises to serve.
  Fix: run `task check` once on the dev machine before merge; record runtime + any tool gap in evidence.md.
  Confidence: 90 (admitted in tasks.md; nothing in the diff contradicts it)
- [NOTE] Evidence self-report still stale: duplicate paragraph and wrong mutant count — `openspec/changes/task-check-entrypoint/evidence/evidence.md:14-21,38`
  Failure: the guard-sweep paragraph appears verbatim twice (round-two #6 survivor), and evidence says "7 mutants" where the code has 8 (`grep -c mutant_rejected` = 8) — a reviewer under-audits the self-test on the count.
  Fix: delete lines 18-21; change "7 mutants" to 8.
  Confidence: 100 (verified by read/grep)
- [NOTE] yq still undeclared as a tool prerequisite — `mise.toml` ([tools]) and `docs/development/tooling-setup.md` have no yq; CI installs it via unpinned snap fallback (`ci.yaml:326-331`)
  Failure: round-two #5 survivor; a fresh contributor following the setup docs reds `task check` at the first gate (`check.sh:8` declares the dependency only in a comment).
  Fix: add yq to mise `[tools]` (or one line in tooling-setup.md).
  Confidence: 85
- [NOTE] The Docker-skip register is self-service and its entries unpinned — `hack/check.sh:81-84`
  Failure: any guard can opt out of the sweep by putting "requires docker" in its header; the mechanism's precision was fixed this round (header-only match, `check.sh:81`) but no assertion names today's sole legitimate entry (`integration_no_docker_test.sh`), so a stray header phrase silently retires a guard from `task check` — though CI coverage is unaffected, so it fails only the local promise.
  Fix: pin the excluded-guard set in the meta-test (assert `integration_no_docker_test.sh` is the only header-declared skip).
  Confidence: 70
- [NOTE] Dead statement in the security gate's walk — `hack/test/ci_workflow_security_test.sh:282-283`
  Failure: `rest = $0; sub(..., "", value)` mutates an undefined variable — a no-op leftover; harmless today (config is `rules: {}`, walk exits) but noise in the CWS-2 contract a future editor must reason about.
  Fix: delete both lines.
  Confidence: 90

## Could not check

- `task_check_test.sh --self-test` and both modes of `ci_workflow_security_test.sh` — they write mutated tree copies outside the repo, which my read-only mandate refuses; only plain modes verified (TCE plain green by me; CWS plain per round-2 record, uncorroborated by me).
- Full `task check` end-to-end (never run by anyone, per tasks.md) — zizmor/gitleaks binaries never executed.
- `reviews/branch-round3/` (untracked, outside the range) and the other committed round-2 legs beyond the fitness one — kept for independence.
- Whether `dependency-review` is branch-protected as its exclusion reason claims.
