## Verdict: CONCERNS

## Findings
- [WARNING] Nothing fails if someone deletes the two new CI steps, and closing that is a one-line change. G9.md "Gaps" admits it, but leaves it open — `hack/test/ci_workflow_security_test.sh:599`
  Failure: someone removes the `openspec_evidence_markdown_only_test.sh` steps from `ci.yaml:573-576` (the plain step, the `--self-test` step, or both). No guard fails. The loop above line 594 only checks guards that CI already calls, so a deleted guard drops out without warning. A probe `.py` could then be committed under `evidence/` again and turn the SonarCloud gate red, which is the PR #463 failure this task exists to prevent. The CI comment says "like the CFC gates above", but the CFC rule does not enforce that for this guard.
  Fix: add `"${ROOT}/hack/test/openspec_evidence_markdown_only_test.sh"` to `cfc_guards` at line 599, the same way `go_patch_lag_test.sh` is already listed. Then delete the matching Gap line in G9.md.
  Confidence: 90

- [NOTE] The guard checks less than the cause it addresses. Sonar analyses all of `openspec/` (`sonar.sources=.`), but the guard only looks at `evidence/` — `hack/test/openspec_evidence_markdown_only_test.sh:49`
  Failure: a probe committed as `openspec/changes/X/reviews/G9/probe.py`, or directly under `openspec/changes/X/`, passes the guard and is still analysed by Sonar as uncovered new code. Today the only tracked non-Markdown files under `openspec/` are `.openspec.yaml` and `config.yaml` (found by Glob), and `.log` files appear under `reviews/`. So a wider allow-list rule (`*.md`, `.openspec.yaml`, `config.yaml`, `*.log`) is feasible. The task scoped the guard to `evidence/`, so I'm noting this, not blocking on it.
  Fix: either widen the scope to `openspec/**` with an explicit allow-list, or record in `decisions.md` that the narrower scope is deliberate.
  Confidence: 60

- [NOTE] The self-test fails with only a raw tar error when the working tree is dirty — `hack/test/openspec_evidence_markdown_only_test.sh:103`
  Failure: if a tracked file under `openspec/` has been deleted from the working tree but not from the index, `ls-files` still lists it. tar then fails, and `set -e`/pipefail aborts with tar's error and no explanation. This does not happen in CI (clean checkout); it only affects local runs.
  Fix: list files with `ls-files -z --cached` but skip ones that are missing (`[[ -e ]]`), or catch the failure with an explicit `fail "copy of openspec/ failed"`.
  Confidence: 55

What I checked and found sound:
- The pathspec `:(glob)openspec/changes/**/evidence/**` covers the live change, `archive/`, and nested directories. `**/` also matches zero directories.
- `pwd -P` is applied the same way to both the root and the work-tree top, so the comparison holds.
- The empty-scan check, the not-a-work-tree check, and the subdirectory-root check each fail with their own message.
- The mutants assert the failure message (the sentinel), not only a non-zero exit. The probe also confirms each mutant landed in the index.
- The "first offender only" hand mutant is killed, because `ls-files` sorts `a.sh` before `b.txt`.
- The CI wiring is two separate steps in the lint job, matching the pattern used for go-patch-lag.
- `G8-probe.sh` inlined into `G8.md:111-137` matches `git show 7fe45f68a:…/G8-probe.sh` line for line. The remaining body references to the scripts no longer carry the removed `evidence/` path, except inside the "Run as" lines, which say where to save the file.
- Commit subjects follow the gitmoji convention and contain no attribution.

## Could not check
- I did not run either mode of the guard, the 9 hand mutants, zizmor, markdownlint or `spec:validate`. I relied on the results recorded in G9.md "Sensors run".
- Byte-for-byte fidelity of the other four inlined scripts: `G3-rbac-probe.py`, `G3-gate-mutants.sh`, `G5-gate-mutants.sh` and `G8-docker-probe.sh`. I spot-checked only `G8-probe.sh`; G9.md claims 5/5 identical.
- Whether `origin/main`'s archive holds non-Markdown evidence. I relied on G9.md row 4, since I cannot run `git ls-tree` here.
- The SonarCloud result on the PR head (not pushed).
