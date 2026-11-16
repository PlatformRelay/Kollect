## Verdict: CONCERNS

## Findings
- [WARNING] The evidence matrix says every row is `not-run`, but the file's own header says "All claims are **verified**", and *Sensors run* records green results for the same sensors — `openspec/changes/scheduled-vulncheck/evidence/B1.1.md:24-33` (vs `:20`, `:37-48`)
  Failure: someone reading the matrix to close B1.1 sees ten unproven claims, while the header and sensor table say they are proven. The evidence contradicts itself, so the evidence class can't be trusted as written.
  Fix: fill the Result column from *Sensors run*: green / red-on-base for rows 1–6 and 9, the review outcome for rows 7–8. Keep row 10 pending until FINAL.
  Confidence: 90

- [NOTE] Matrix row 7 says `docs/development/testing.md` now uses the new timing wording. The diff doesn't touch that file, and its row has no timing wording at all — `evidence/B1.1.md:30`, `docs/development/testing.md:136`
  Failure: the claim is stronger than the tree. The task clause is only met because there was nothing to change, and the evidence doesn't say so.
  Fix: in row 7, say testing.md had no timing wording to change. Drop it (and `loop.md`, also unchanged) from *Files in scope*, `evidence/B1.1.md:11-14`.
  Confidence: 85

- [NOTE] V-3f checks that the whole `body.md` contains `FAIL: <id> `, not that the line sits inside the fenced excerpt — `hack/test/vulncheck_schedule_test.sh:330-333`
  Failure: right now only the excerpt can hold that string, because the step name in `jobs.json` doesn't. If the body builder ever echoed log text outside the fence, the check would pass for the wrong reason.
  Fix: optionally, cut the text between the fences of `### FAIL lines` and grep only that.
  Confidence: 40

- [NOTE] V-3f reads the step name from the hard-coded positions `.jobs[0]` and `.steps[3]` — `hack/test/vulncheck_schedule_test.sh:318-319`
  Failure: none today, because V-2 pins exactly four steps, the scan step last. Choosing the step by `select(.run == "bash hack/ci/vulncheck-scheduled.sh")` would make the case independent of that ordering.
  Fix: optional; select the step by its `run` value.
  Confidence: 35

Checked and found sound:
- **Intent:** every clause of B1.1 is in the diff:
  - the step is renamed without `FAIL` (`.github/workflows/vulncheck-scheduled.yaml:41`);
  - the job/step-name check has both a step mutant and a job mutant (`vulncheck_schedule_test.sh:290-292`, `:361-364`);
  - the `github[` token form has its mutant (`:286`, `:359`);
  - the timing wording is fixed in the policy rows, workflow header and proposal;
  - "Task 3.54.0" is in G1 (`evidence/G1.md:61`);
  - the SVS-1.3 ids are updated. The SVS-1.2 hits left over are step-shape citations or loop.md history, as intended.
- **`tee` status:** `PIPESTATUS[@]` is copied before anything else runs. The capture-failure path exits before any verdict is read (`hack/ci/vulncheck-scheduled.sh:86-93`). Exit codes match the SVS-1.3 scenario (`spec.md:81`).
- **Title rule:** the parser matches SVS-1.3 `spec.md:70`. A blank line before the title is skipped, one after it ends the title, and both `More info:` and the next entry flush it. The mutants cover the first-line-only cut, joining without a space, `More info:` not ending the title, a blank line not ending it, and a missed flush at the next entry and at end of output. Each expects its own distinct message.
- **V-3f log shape:** lines are `<job>\t<step>\t<ts> <text>`, after 60 lines, more than `MAX_FAIL_LINES=50` (`ci-failure-issue-body.sh:39,132`). The builder gets the same arguments as `ci-failure-report.sh:207-208`. The FAIL-name mutant turns red because the 60 prefixed download lines fill the `grep -m 50`. There is also a control for an unnamed step ("Run …"), and a mutant that deletes the builder.
- **Rules:** no new `uses:`, no expressions in `run:`, no permission change, no attribution, and evidence is Markdown only. Each changed G1 test is justified against the spec in *Test changes*.

## Could not check
- I ran no sensors, mutations or shellcheck. The guard, `--self-test` (82 ok), zizmor and the lint results come from the evidence file only.
- I could not read a real `gh run view 43406013666 --log` to confirm the tab and timestamp prefix, because only git commands were allowed. I relied on `ci-failure-report.sh:199` and the description in the test.
- I didn't read the round-1 review legs or verification.md under `reviews/B-branch/round-1/`, so I didn't cross-check each finding against its fix beyond what tasks.md B1.1 lists.
- I didn't check whether `advisories.txt` really wraps GO-2026-6888 right before `github.com/andybalholm/brotli`. I took that from T0.1 and the V-3a expectation.
