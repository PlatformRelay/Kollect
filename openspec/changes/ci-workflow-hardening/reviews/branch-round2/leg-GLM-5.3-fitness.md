The round-one CRITICAL is closed (actionlint exits 0, gate green). Now let me probe the one claim in the round-two commit message I doubt: that `schema_steps` covers composite actions. The `else:` branch at ci_workflow_security_test.sh:519-521 looks empty, so I'll test it with a mutant on a scratch copy.
Confirmed: the gate passes a tree with a dangling name-only step inside a composite action — the pass message ("and composite action") and the commit-message claim are both false. Now let me check whether any other existing function (actionlint, zizmor, pre-commit) would catch that class, and read the remaining context.
zizmor accepts the dangling-step action too (rc=0) — so no function in the repo would catch that class. But my audit count (36 ignored, 27 suppressed) disagrees with evidence.md's recorded "(27 ignored, 25 suppressed)" for the exact pinned invocation. Let me check whether the counts changed within the range by running zizmor on the pre-fix commit's tree.
The count delta between cd990fe0 and HEAD needs explaining — the only .github diff is one deleted line, yet ignored went 27→36 and suppressed 25→27. Let me localize it to ci.yaml alone.
## Verdict: BLOCK

Fitness lens, range a1a300e1..HEAD at ed93b03f. Executed on the tree: the new gate plain + `--self-test` (green: 31 oks = 30 mutants + no-op, each with pinned message); `dist_ci_wiring_test.sh`, `test-verify-eligibility.sh`, all four relocated CWS-6 orphan guards, shellcheck (clean); zizmor installed via the repo's own installer (darwin/arm64 digest verified by use) and the exact pinned audit run — exit 0 at HEAD, plus controlled probes on scratch copies at three refs. The round-one CRITICAL is genuinely closed in **workflows** (line deleted; actionlint parse-clean; the `schema dangling name-only step` mutant reds). Characteristic moved: CI supply-chain determinism and gate coverage, both up; the mutant harness is a real new ratchet. But the fix commit claims the ratchet covers composite actions too, and I measured that it does not — the round-one defect class is re-opened one level down, with the gate printing a false ok.

## Findings

- [CRITICAL] The schema ratchet does not check composite actions, while its pass message and the commit message claim it does — `hack/test/ci_workflow_security_test.sh:519-521` (empty `else:`), false claim at `:523` and in ed93b03f's message
  Failure: the same stray-edit class round one proved lethal (step with neither `run` nor `uses`), but inside `.github/actions/go-cache/action.yml`, referenced by required jobs — probed: gate exits 0 on such a copy while printing "ok - schema: every step in every workflow **and composite action**…"; zizmor also accepts it (rc=0, 36 ignored/27 suppressed) and actionlint flags nothing, and no required gate runs actionlint anyway — so no fitness function in the repo fails.
  Fix: in `schema_steps`'s else branch, require every `.runs.steps[i]` to have `run` or `uses`, and add the matching mutant next to the existing composite-action one.
  Confidence: 95 (missing coverage measured; GitHub load-rejection inferred from the identical workflow-class behaviour, not run against GitHub).
- [WARNING] Mutant-harness counts are wrong in all four places they are recorded, contradicting each other — `evidence.md:11` says 33, `evidence.md:58` still says 19, `tasks.md:58` says 12 for CWS-1, `tasks.md:66` says 33
  Failure: measured actuals are 30 mutants total (10 for CWS-1); a maintainer auditing against 33/19 cannot detect mutants silently dropped, which is the count's only job — and this exact NOTE was round one's finding, "fixed" into a new wrong number.
  Fix: `grep -cE '^(sed_mutant_rejected|yq_mutant_rejected|mutant_rejected) '` → 30, and correct the three stale spots plus the CWS-1 row.
  Confidence: 100
- [WARNING] The recorded zizmor baseline does not reproduce on HEAD: evidence says `(27 ignored, 25 suppressed)`, the pinned invocation on the branch tip returns `(36 ignored, 27 suppressed)` — `evidence.md:42-43`, `tasks.md:40-44`
  Failure: measured cause — at cd990fe0 (where evidence was taken) the parse-broken ci.yaml yielded zizmor **zero** findings (0 audit lines for that file alone), so the "27 ignored" baseline was measured with ci.yaml silently unaudited; a maintainer re-running the recorded command on HEAD gets different numbers and cannot reconcile the threshold decision's register.
  Fix: re-run the pinned invocation at HEAD and update both records; add one line that a parse-broken workflow audits as zero findings — that is why the schema ratchet is load-bearing.
  Confidence: 95
- [NOTE] Allow-list widening of the range, flagged per lens: `WORKFLOW_KEY_ALLOWLIST` gains `concurrency` (`hack/test/dist_ci_wiring_test.sh:105-108`) — verified defended: in-file justification, exact-expression pins for both workflows, three CWS-4 mutants; `required_checks` also grows (`+workflow-security`), the intended direction for that register. No action. Confidence: 100
- [NOTE] Parse-validity now rests entirely on the home-grown ratchet: actionlint is referenced nowhere in `.github/`, `Taskfile.yml`, `Makefile`, `.pre-commit-config.yaml` or `mise.toml`, zizmor silently audits broken files as clean, and actionlint itself does not check steps inside composite actions (probed). Worth adding a pinned actionlint step only after finding 1's fix; independent of it, it is the cheapest second opinion. Confidence: 80

Round-one closure verified: dangling step deleted and workflow-scoped ratchet mutant-locked; `base64 -w0` correct (measured on darwin and matches GNU semantics on the ubuntu runner); install-step pinning, `KOLLECT_FORCE_SHA256`, `shell:` override, `warn-only`, `continue-on-error`, why-line anchoring and inline `zizmor: ignore` all now enforced with mutants that red in the self-test.

## Could not check

- GitHub's load-time rejection of a name-only step inside a composite action (no GitHub access; the workflow-level analogue was verified via actionlint, the action-level claim rests on the step schema).
- `changelog_sync_release_guard_test.sh` and `ci_docs_gate_test.sh` — need PyYAML, not installed in this environment (evidence.md records both green).
- The three non-darwin digests in `hack/install-zizmor.sh` (only darwin/arm64 exercised by execution).
- Live behaviours deferred post-merge by the spec (PR supersede, three-quick-merges, ruleset required-check list, throwaway-PR probes — tasks 4.2/5.x/6.1).
- The untracked in-flight leg logs under `openspec/changes/ci-workflow-hardening/reviews/branch-round2/` — other reviewers' output, deliberately not read.
