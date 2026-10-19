## Verdict: CONCERNS

## Findings
- [WARNING] `control()` never checks that its edit changed anything, so the bot-bump control can quietly turn into a second no-op control — `hack/test/ci_failure_report_test.sh:218-235`, `:246-247`
  Failure: `judge` refuses an edit that changed nothing (`diff -r`, `:195`), but `control` has no such check. If the checkout line in `ci-failure-report.yaml` stops matching `(actions/checkout@)[0-9a-f]{40} # v[0-9.]+`, `sed -E -i` exits 0 and the copy is unchanged. Example: a pin comment like `# v4.2.2 (renovate)` still matches, but `# 4.2.2` or a quoted `uses:` does not. The control still prints "passes". It has stopped proving that a bot bump stays green, and nothing reports that. The evidence says "a literal that is gone makes the mutation step fail loudly, never pass" (T5.1.md:50-54). That holds for mutants, not for this control.
  Fix: in `control`, when an edit was given (`$# -gt 0`), run the same `diff -r "${BASE}" "${COPY}"` and call `bad` on no change.
  Confidence: 85

- [NOTE] The `cws` gate has no control on the unmutated copy — `hack/test/ci_failure_report_test.sh:200-201`, `:438-443`
  Failure: the three CWS-2 mutants are judged by a non-zero exit plus one message. The full `ci_workflow_security_test.sh` runs on a partial copy (only `.github`, `hack/{ci,test,release,lib,docs}`, `install-zizmor.sh`). If it reds there for an unrelated reason, the non-zero exit tells you nothing. Only the CWS-2 message stops a false pass. Today that message is specific to the mutation, so this is not a live false pass. But the "exit status" half of the judgement is empty for this gate.
  Fix: add `control`-style coverage for `cws`: the unmutated copy must exit 0 under `ci_workflow_security_test.sh`.
  Confidence: 55

- [NOTE] The self-wiring check is a line grep with no idea of jobs or step conditions — `hack/test/ci_failure_report_test.sh:1656-1670`
  Failure: an exact `run: bash hack/test/ci_failure_report_test.sh --self-test` line passes the check in any of these places:
  - in a step that also has `if: false` or step-level `continue-on-error: true`;
  - in the `perf-report` or `sonarcloud` job, which have job-level `continue-on-error: true` (`.github/workflows/ci.yaml:749`, `:847`).

  The task says "wired into the ci.yaml lint job". The evidence (T5.1.md:62-64, :78) leans on CWS-6, "required job", and GATE-CIWIRING-02 to cover this. I did not verify that either of them rejects a step-level `if:` or `continue-on-error` on this step.
  Fix: resolve the step with `yq` (`.jobs.lint.steps[] | select(.run == "…")`) and require that it has no `if` and no `continue-on-error` key. Or record in the evidence that CWS-6 / dist wiring is the gate for this.
  Confidence: 60

Checked, no defect found:
- `judge` requires both a non-zero exit and every expected message on a `FAIL:` line. It refuses failed and no-op edits, and frees the copy on every path.
- `lit` requires exactly one occurrence, keeps the trailing newline (`x` sentinel), and quotes the replacement (so bash 5.2 `patsub_replacement` does not apply).
- The case filter is inert in plain mode: empty `ONLY` means `SKIP` is always 0. A filter that names no case reds (`:1672-1675`), and the self-test exercises that (`:249-255`).
- M20 targets case 12b, whose 5000-byte line fits the budget uncut, so only the line cut can produce that shape.
- The mutant that deletes both lint steps carries an absence probe (`gone`) and expects both self-wiring messages.
- Each mutant's expected message matches the text of its assertion: W1 rename at `:1488`, the CWS-2 backtick formats in CWS `:269/:293`.
- The new step is inside the `lint` job (`ci.yaml:279-608`, step at `:581-582`).
- Every mutant tasks.md 5.1 lists is present.

## Could not check
- I did not run the self-test or the plain test. Per instructions I relied on the results recorded in evidence/T5.1.md (88 ok, 208 s; CWS both modes green; `task lint:shell` green).
- Lines 893-1479 of the test (reporter cases and the start of the wiring section). I did not read whether every reporter case calls `select_case` through `store` before its checks. That matters only under the filter, where a mis-scoped case could make a mutant red against the wrong case's message.
- Whether CWS-6 and `dist_ci_wiring_test.sh` reject a step-level `if:` or `continue-on-error`, or a `CFR_SELFTEST_ONLY` env, on this lint step. This is the "believed" harness gap in the evidence.
- The real CI wall time of `lint` with the new step (the evidence's ~15 min is marked **believed**).
- The diffs to `loop.md` and `tasks.md`, beyond the stat.
