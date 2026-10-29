## Unified verdict: BLOCK  (legs ok: 4/5 — DeepSeek-V4.1-Flash-adversarial exit=143, empty file)
| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | CRITICAL (prom) | CWS-6 contradicted: ~15 guards run only via `hack/docs/verify.sh` in the non-required docs job cannot red a PR and are invisible to the gate | `hack/docs/verify.sh:15-40`, `ci_workflow_security_test.sh:361-414` | GLM-5.3, Claude | 2 | 95 |
| 2 | CRITICAL | Schema ratchet never checks composite actions (empty `else:`) while pass message + commit message claim it does; zizmor/actionlint also miss it | `ci_workflow_security_test.sh:519-523` | GLM-5.3 | 2 | 95 |
| 3 | CRITICAL | Gate accepts an extra step that shadows the zizmor binary with a green stub (`-ge 3`, no step-count/order check) — CWS-5 nullifiable, verified by applied mutant | `ci_workflow_security_test.sh:133` | Qwen3.8 | 1 | 95 |
| 4 | WARNING | Mutant-harness counts wrong everywhere recorded: evidence says 33/19, tasks says 33/12; actual 30 (+10 CWS-1) — round-one NOTE re-failed | `evidence.md:11,58`, `tasks.md:58,66` | GLM-5.3 | 2 | 100 |
| 5 | WARNING | Recorded zizmor baseline non-reproducible: `(27,25)` was measured on a parse-broken ci.yaml auditing as zero; HEAD measures `(36,27)` | `evidence.md:42-43`, `tasks.md:40-44` | GLM-5.3 | 1 | 95 |
| 6 | WARNING | CWS-2 suppression-comment check misses flow-style `ignore: [...]` (and per-rule `disable:`): silences findings with no justification | `ci_workflow_security_test.sh:221-231` | Claude | 1 | 75 |
| 7 | WARNING | `--min-severity=high` filters, not suppresses: future medium-severity findings land green silently, against "fail on any unsuppressed finding" | `ci.yaml:194` | Qwen3.8 | 1 | 70 |
## Disagreements
- Composite-action gap: GLM-5.3-spec rated it WARNING, GLM-5.3-fitness CRITICAL (same defect, merged here as CRITICAL).
- CWS-2: Claude flags a suppression-comment bypass; GLM-5.3-spec judges CWS-2 HOLDS because the tree has zero suppressions today — future-bypass vs current-state framing.
- Qwen read the CWS-6 "composite-action rejection" as correct and the meta-test green; GLM-5.3 (×2) measured the composite-action path checking nothing.
- Mutant count: Qwen reports "31 mutants"; GLM counts "30 mutants + no-op" — off-by-one over the no-op control.
## Nobody could check
- Live `protect-main` ruleset / required-context list (all legs; needs `gh api`; spec defers post-merge).
- GitHub load-time rejection of a name-only step inside a composite action (the CRITICAL-2 analogue at the action level).
- Genuineness of pinned SHAs/digests: dependency-review-action v5.0.0 tag SHA; the three non-darwin zizmor digests (only darwin/arm64 exercised).
- PyYAML-dependent guards (`ci_docs_gate_test.sh`, `changelog_sync_release_guard_test.sh`) never executed by any leg.
- Post-merge runtime behaviours (PR supersession, dependency-review on real PRs, required-check evidence) — deferred by spec, no leg ran them.
