The implementation matches the decision. The new job runs on unfiltered PR events, has no `if`, `needs`, or `continue-on-error`, and has a 45-minute timeout. It uses pinned checkout and the Go-cache action configured from `go.mod`. Lint retains the plain invocation and loses the self-test. Release eligibility requires the new context; branch protection remains deliberately deferred.

The CWS clause correctly matches column 3 of the required-job rows. The docs-gate mutants target conditional execution, dependencies, and soft failure. The coverage mapping corresponds to the actual guard sweep; both re-anchored TCE-3 mutants retain their original meaning.

The fixture name is valid in a Go module path and is propagated consistently through the proxy, probe, and diagnostic. Cleanup targets that fixture, with nonempty-variable checks and empty-parent-only removal. The remaining `kollect-newgo-fixture` text is a task label, not a hard-coded module path.

No guard was loosened, exclusion widened, or action left unpinned. The self-test step propagates failure normally. I reran TCE plain mode, shell syntax checks, diff whitespace checks, and zizmor offline at high severity; all passed. Two evidence qualifications remain below.

## Verdict
verdict: APPROVE
findings:
  - [P2] openspec/changes/cross-file-consistency-gates/evidence/G6.md:293 cites r5.tsv as all-green after A-1, but that file records the pre-fix TCE self-test failure.
matrix_disputes:
  - A8 — the cited log does not substantiate the post-fix all-green claim; the re-anchoring itself is correct by inspection.
  - A10 — g6b-gate-mutants.sh accepts any nonzero exit and prints, rather than asserts, the failure message; its exit alone cannot prove “for the intended reason.”
could_not_check:
  - Scratch mutation runs and temp-dependent guards: mktemp failed because /tmp is read-only.
  - Live runner tools/network, concurrent fixture runs, and post-merge ruleset enforcement.
summary: Implementation satisfies the decision; evidence needs a corrected log reference and qualified mutation-proof wording.
