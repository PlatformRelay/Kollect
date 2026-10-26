## Verdict: CONCERNS

## Findings
- [WARNING] T2 runs the token-check body under the wrong shell when the step sets no `shell:`. It runs `bash --noprofile --norc -eo pipefail`, but when `shell:` is unspecified the runner uses `bash -e {0}`, which has no pipefail. — `hack/test/renovate_policy_test.sh:214-221,245`
  Failure: the body `echo "checking RENOVATE_TOKEN"; test -n "$RENOVATE_TOKEN" | cat` with no `shell:`. Under the guard's pipefail it exits 1 on an empty token and names it, so T2 goes green. On the runner the pipeline status is `cat`'s 0, so Renovate starts without the token. That is a false green on DUA-1 fail-closed.
  Fix: run the body with `bash -e` when the effective shell is unspecified. Use `--noprofile --norc -eo pipefail` only for an explicit `shell: bash`. The other option is to require an explicit `shell: bash` on the token step. Add a mutant with this pipe body.
  Confidence: 75
- [NOTE] The T2 body execution can hang the `lint` job, which has no `timeout-minutes`. `timeout 20` sends only TERM, and the body's stdin is inherited. — `hack/test/renovate_policy_test.sh:220-221`, `.github/workflows/ci.yaml:283`
  Failure: a body with `trap '' TERM; sleep infinity` ignores TERM, so the step runs until the 6 h default job limit. Locally, a body that runs `read` consumes the caller's terminal.
  Fix: `timeout -k 5 20 … </dev/null`.
  Confidence: 70
- [NOTE] T9 swallows yq errors per file (`2>/dev/null || true`). A traversal error can empty `bodies` and give a silent green. — `hack/test/renovate_policy_test.sh:466,471`
  Failure: some map in a workflow has a `with:` that is a sequence. `.with.script` then probably makes yq error with "cannot index array", `bodies` comes back empty, and a `gh pr merge` in the same file is not seen. I did not run yq to confirm this.
  Fix: replace `.with.script` with `(.with | select(tag == "!!map") | .script)`, or fail T9 when yq exits non-zero on a workflow that exists.
  Confidence: 45
- [NOTE] T9's coverage matches the spec, but these merge paths pass: the GraphQL `mergePullRequest` mutation, `run:` steps in composite actions (`.github/actions/go-cache/action.yml` and `.github/actions/kind-e2e-setup/action.yml` exist), and a `gh pr merge` split across a line continuation. — `hack/test/renovate_policy_test.sh:463,467`
  Failure: a composite action whose step runs `gh pr merge`, called from any workflow, stays green.
  Fix: add `mergePullRequest` to the pattern and include `.github/actions/**/action.y*ml` in the scan. Alternatively, record these as out of scope under "Harness gaps".
  Confidence: 60
- [NOTE] T7 allows `RENOVATE_REPOSITORIES` with any value. With a maintainer PAT, a value other than `${{ github.repository }}` points Renovate at other repositories. — `hack/test/renovate_policy_test.sh:412`
  Failure: `RENOVATE_REPOSITORIES: other-org/other-repo` stays green.
  Fix: require the value to be exactly `${{ github.repository }}`, or record this as accepted.
  Confidence: 50
- [NOTE] The step names and the ci.yaml comment say "DUA-1 to DUA-8". The guard header says DUA-1 to DUA-5, DUA-7 and DUA-8, and the 1.1 clauses (T1 to T9) cover no DUA-4 or DUA-6. — `.github/workflows/ci.yaml:9-16` (diff hunk at ~582-590)
  Fix: list the requirements the clauses actually cover.
  Confidence: 85
- [NOTE] In check mode, some unguarded yq calls run under `set -e` and abort the guard silently on a malformed tree. Examples are T6's `extra=` (`.with` as a string, then `keys`) and the second T8 query. The guard does fail (exits red), but without a clause message and without running the remaining clauses. — `hack/test/renovate_policy_test.sh:399,451`
  Fix: add `|| true` with a sentinel, or check the tag first.
  Confidence: 55

What I checked and found sound:
- **Wiring:** both lint steps are on one invocation each and sit after the "Ensure yq" step (ci.yaml:342). The CFC-6 enrolment and its CWS mutant are in place.
- **Fixture:** it is a correct DUA-1..3 workflow. It has hourly cron, a literal group with `false`, contents: read at both levels, a token step with no if or continue-on-error, a SHA pin with a `# v46.3.10` line_comment, a digest image and exactly three `with` keys.
- **Mutant harness:** a mutant passes only on its own clause's message and with no other clause's FAIL line. A `ymut` readback runs on every edit.
- **Controls:** a no-op, a bot bump (v99), comment decoys and an `*/N` hour cron all stay green.
- **yq union:** the parenthesisation in T9's `on:` handling is correct.
- **`env -i`:** it strips runner secrets from the T2 run.
- **Mutant values:** none is a plausible next release.
- **Case strictness:** each clause is at least as strict as its case text in tasks.md lines 16-24.

## Could not check
- I did not run any sensor or mutation in a scratch copy, because Bash is restricted to `git` read-only commands here. The self-test's 51-mutant pass and the plain-mode red per case come from evidence/1.1.md only.
- I did not confirm how yq v4 behaves for `.with.script` on a sequence or a string, or that `line_comment` strips the `# ` from the version comment.
- I did not confirm GitHub's documented default shell semantics (`bash -e {0}` when `shell:` is unspecified) against the current runner.
- I did not read `specs/dependency-updates/spec.md` itself. I judged intent from the case text in tasks.md 1.1.
